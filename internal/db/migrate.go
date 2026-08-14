package db

import (
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// migrations 是按顺序应用的迁移列表。每条迁移只应执行一次，
// 由 schema_migrations 表记录已应用项。新增迁移时在此追加。
var migrations = []struct {
	name string
	fn   func(*sql.DB) error
}{
	{"001_sync", migrate001},
	{"002_delete_semantics", migrate002},
	{"003_account_display_name", migrate003},
}

// ApplyMigrations 执行所有尚未应用的迁移。
func ApplyMigrations(conn *sql.DB) error {
	if _, err := conn.Exec(
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, m := range migrations {
		var n int
		if err := conn.QueryRow(
			`SELECT count(*) FROM schema_migrations WHERE name=?`, m.name,
		).Scan(&n); err != nil {
			return fmt.Errorf("check migration %s: %w", m.name, err)
		}
		if n > 0 {
			continue
		}
		if err := m.fn(conn); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if _, err := conn.Exec(
			`INSERT INTO schema_migrations(name) VALUES(?)`, m.name,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
	}
	return nil
}

// columnExists 判断表是否存在某列（用于向后兼容旧库）。
func columnExists(conn *sql.DB, table, col string) (bool, error) {
	rows, err := conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, nil
}

// migrate001：为同步能力补齐结构
//   - accounts/commodities/prices 增加 uuid + updated_at（旧库需 ALTER）
//   - 回填 uuid、建立唯一索引
//   - 新建 sync_log 事件日志
//   - 重建 sync_checkpoints 为 (ledger_id, client_id, last_seq)
func migrate001(conn *sql.DB) error {
	// 1) 旧库补列
	for _, t := range []string{"accounts", "commodities", "prices"} {
		if ok, err := columnExists(conn, t, "uuid"); err != nil {
			return err
		} else if !ok {
			if _, err := conn.Exec(`ALTER TABLE ` + t + ` ADD COLUMN uuid TEXT`); err != nil {
				return fmt.Errorf("alter %s add uuid: %w", t, err)
			}
		}
		if ok, err := columnExists(conn, t, "updated_at"); err != nil {
			return err
		} else if !ok {
			if _, err := conn.Exec(`ALTER TABLE ` + t + ` ADD COLUMN updated_at TEXT NOT NULL DEFAULT (datetime('now'))`); err != nil {
				return fmt.Errorf("alter %s add updated_at: %w", t, err)
			}
		}
	}

	// 2) 回填 uuid（确定性前缀 + 行 id，表内唯一）
	backfills := []struct {
		table, prefix string
	}{
		{"accounts", "acc"},
		{"commodities", "cmd"},
		{"prices", "prc"},
	}
	for _, b := range backfills {
		if _, err := conn.Exec(
			fmt.Sprintf(`UPDATE %s SET uuid = '%s-'||id WHERE uuid IS NULL OR uuid = ''`, b.table, b.prefix),
		); err != nil {
			return fmt.Errorf("backfill %s uuid: %w", b.table, err)
		}
	}

	// 3) 唯一索引（uuid 为同步身份键）
	for _, idx := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_sync_uuid ON accounts(ledger_id, uuid)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_commodities_sync_uuid ON commodities(ledger_id, uuid)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_prices_sync_uuid ON prices(ledger_id, uuid)`,
	} {
		if _, err := conn.Exec(idx); err != nil {
			return fmt.Errorf("create sync uuid index: %w", err)
		}
	}

	// 4) sync_log 事件日志（id 即全局单调水位 server_seq）
	if _, err := conn.Exec(
		`CREATE TABLE IF NOT EXISTS sync_log (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			ledger_id   INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
			entity_type TEXT NOT NULL,           -- transaction | account | commodity | price
			entity_uuid TEXT NOT NULL,
			op          TEXT NOT NULL,           -- create | delete
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	); err != nil {
		return fmt.Errorf("create sync_log: %w", err)
	}
	if _, err := conn.Exec(
		`CREATE INDEX IF NOT EXISTS idx_synclog_ledger_seq ON sync_log(ledger_id, id)`,
	); err != nil {
		return fmt.Errorf("index sync_log: %w", err)
	}

	// 4.1) sessions 表（最小认证：登录后签发 token）
	if _, err := conn.Exec(
		`CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			expires_at TEXT NOT NULL
		)`,
	); err != nil {
		return fmt.Errorf("create sessions: %w", err)
	}

	// 5) 重建 sync_checkpoints 为客户端水位
	if _, err := conn.Exec(`DROP TABLE IF EXISTS sync_checkpoints`); err != nil {
		return fmt.Errorf("drop old sync_checkpoints: %w", err)
	}
	if _, err := conn.Exec(
		`CREATE TABLE sync_checkpoints (
			ledger_id      INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
			client_id      TEXT NOT NULL,
			last_seq       INTEGER NOT NULL DEFAULT 0,   -- 已同步到的 sync_log.id
			last_synced_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (ledger_id, client_id)
		)`,
	); err != nil {
		return fmt.Errorf("create sync_checkpoints: %w", err)
	}

	return nil
}

// migrate002：P1-B 删除语义 + 自然键合并
//   - transactions 增加 deleted_at（交易软删，保留分录做审计）
//   - 重建 accounts：
//   - 增加 deleted_at（账户软删）
//   - 表级 UNIQUE(ledger_id,name) 改为「部分唯一索引」
//     (ledger_id, name) WHERE deleted_at IS NULL —— 软删后可重建同名账户
//   - 保持 (ledger_id, uuid) 唯一索引（同步身份键）
//   - 兼容旧库：seed owner 用户密码为 'unset'（无法登录），置为初始密码 owner123456
//     （P1-B3 起注册语义改变，旧库升级后可用该密码登录后自行修改）。
func migrate002(conn *sql.DB) error {
	// 1) transactions 加 deleted_at
	if ok, err := columnExists(conn, "transactions", "deleted_at"); err != nil {
		return err
	} else if !ok {
		if _, err := conn.Exec(`ALTER TABLE transactions ADD COLUMN deleted_at TEXT`); err != nil {
			return fmt.Errorf("alter transactions add deleted_at: %w", err)
		}
	}

	// 2) 重建 accounts（重建期间临时关闭外键，避免 DROP 时被 postings 引用阻塞；
	//    因 MaxOpenConns=1 单连接，PRAGMA 影响的就是唯一连接）。
	if _, err := conn.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	defer func() { _, _ = conn.Exec(`PRAGMA foreign_keys=ON`) }()

	if _, err := conn.Exec(
		`CREATE TABLE accounts_new (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			ledger_id  INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
			uuid       TEXT,
			name       TEXT NOT NULL,
			type       TEXT NOT NULL,
			open_date  TEXT NOT NULL,
			close_date TEXT,
			commodity_restriction TEXT,
			deleted_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			CHECK (type IN ('Assets','Liabilities','Equity','Income','Expenses'))
		)`,
	); err != nil {
		return fmt.Errorf("create accounts_new: %w", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO accounts_new(id, ledger_id, uuid, name, type, open_date, close_date, commodity_restriction, deleted_at, created_at, updated_at)
		 SELECT id, ledger_id, uuid, name, type, open_date, close_date, commodity_restriction, NULL, created_at, updated_at
		 FROM accounts`,
	); err != nil {
		return fmt.Errorf("copy accounts: %w", err)
	}
	if _, err := conn.Exec(`DROP TABLE accounts`); err != nil {
		return fmt.Errorf("drop old accounts: %w", err)
	}
	if _, err := conn.Exec(`ALTER TABLE accounts_new RENAME TO accounts`); err != nil {
		return fmt.Errorf("rename accounts: %w", err)
	}
	for _, idx := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_sync_uuid ON accounts(ledger_id, uuid)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_ledger_name ON accounts(ledger_id, name) WHERE deleted_at IS NULL`,
	} {
		if _, err := conn.Exec(idx); err != nil {
			return fmt.Errorf("recreate accounts index: %w", err)
		}
	}

	// 3) 兼容旧库：seed owner 用户密码 'unset' 无法登录，置初始密码（仅旧库升级时生效）
	hash, err := bcrypt.GenerateFromPassword([]byte("owner123456"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash seed owner password: %w", err)
	}
	if _, err := conn.Exec(
		`UPDATE users SET password_hash=? WHERE username='owner' AND password_hash='unset'`,
		string(hash),
	); err != nil {
		return fmt.Errorf("fix seed owner password: %w", err)
	}

	return nil
}

// migrate003：账户增加 display_name（中文显示名，可选）。
// 已有账户 display_name 留空，UI 回退显示 ASCII name；不影响 bean-check（导出只用 name）。
func migrate003(conn *sql.DB) error {
	if ok, err := columnExists(conn, "accounts", "display_name"); err != nil {
		return err
	} else if !ok {
		if _, err := conn.Exec(`ALTER TABLE accounts ADD COLUMN display_name TEXT`); err != nil {
			return fmt.Errorf("alter accounts add display_name: %w", err)
		}
	}
	return nil
}
