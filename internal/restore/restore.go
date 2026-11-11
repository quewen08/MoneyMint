// Package restore 实现「从 .db 备份还原全库」的安全替换逻辑（M2-a）。
//
// 设计要点（务必与 docs/m2-plan.md 对齐）：
//
//  1. 还原走「ATTACH + 事务内逐表替换」，不动数据库文件、不需要停机、
//     失败自动回滚（由事务保证）——相较「复制文件覆盖」可避免 WAL 句柄冲突。
//  2. 替换前对上传文件做 5 项只读预检（见 Preview），任一不过则整体拒绝。
//  3. 执行前自动对当前库做 pre-restore 快照（trigger=pre-restore），
//     失败则中止，绝不改动数据（安全网）。
//  4. 列取交集写入：旧备份可能缺后续迁移新增的列，直接 SELECT * 会因列数不符失败，
//     故只写两边都有的列，缺失列由表默认值填充，随后靠 ApplyMigrations 补齐语义。
//  5. db_instance 不参与替换：保留并在成功后由 db.RotateInstanceID 重写
//     （库世代变化，触发客户端全量重拉）。schema_migrations **随备份替换**：
//     还原旧 schema 备份后靠 ApplyMigrations 重跑补齐缺失列（迁移均有
//     columnExists / IF NOT EXISTS 守卫，可安全重入）。
//  6. 全库级操作，调用方必须先校验系统管理员权限（备份含全部账本与用户）。
//
// 并发安全：依赖连接池 MaxOpenConns=1（与迁移一致）。replaceAll 用 db.Conn
// 钉死唯一连接，避免 PRAGMA foreign_keys 在多连接下串味。
package restore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"familyledger/internal/backup"
	"familyledger/internal/db"
	"familyledger/internal/domain"
)

const (
	// MaxBackupBytes 是上传备份的体积硬上限（512 MB）。
	MaxBackupBytes = 512 << 20

	// ConfirmWord 是执行还原所需的二次确认词（前端需用户手动输入）。
	ConfirmWord = "RESTORE"

	// bakName 是 ATTACH 时给备份库起的别名。
	bakName = "bak"

	// excludedTables 不参与数据替换的系统表。
	//   db_instance：保留当前库世代，成功后重写而非替换（schema_migrations
	//   必须随备份替换——否则还原旧 schema 备份后主库仍记录「新迁移已应用」，
	//   ApplyMigrations 不会重跑，备份缺失的列永远缺，见 restore_test 回归守卫）。
	excludedTables = "db_instance"
)

// Stats 是库内核心表的行数统计（用于覆盖量对比）。
type Stats struct {
	Users        int64 `json:"users"`
	Ledgers      int64 `json:"ledgers"`
	Accounts     int64 `json:"accounts"`
	Transactions int64 `json:"transactions"`
}

// PreviewResult 是预检结果（只读，不改动任何数据）。Valid=false 时通过 Error/Reason
// 说明被哪项检查拦下；该情形属预期输入（如上传了损坏文件），不视为异常。
type PreviewResult struct {
	Source         string   `json:"source"`           // 上传文件名（已剥离目录）
	Valid          bool     `json:"valid"`            // 是否通过全部预检
	Backup         Stats    `json:"backup"`           // 备份内核心表行数
	Current        Stats    `json:"current"`          // 当前库核心表行数
	SchemaVersion  int      `json:"schema_version"`   // 备份的迁移最高版本
	CurrentVersion int      `json:"current_version"`  // 当前代码内建迁移最高版本
	Warnings       []string `json:"warnings"`         // 还原副作用提示
	Error          string   `json:"error,omitempty"`  // 预检未通过的概括原因
	Reason         string   `json:"reason,omitempty"` // 机器可读的拒绝原因码
}

// Result 是执行结果。
type Result struct {
	Source        string   `json:"source"`             // 上传文件名
	Replaced      Stats    `json:"replaced"`           // 还原后核心表行数
	Snapshot      string   `json:"snapshot,omitempty"` // pre-restore 快照文件名（可再还原回去）
	NewInstanceID string   `json:"new_instance_id"`    // 重写后的库世代 UUID
	Warnings      []string `json:"warnings"`
}

// Preview 对备份文件做只读预检，返回结构化摘要。它不修改任何数据。
// backupPath 为上传落盘的临时文件路径（绝对或相对均可）。
func Preview(mainDB *sql.DB, backupPath string) (*PreviewResult, error) {
	src := filepath.Base(backupPath)
	p := &PreviewResult{Source: src, Warnings: []string{
		"还原后当前账号会消失（若备份里没有该账号），请用备份时期的账号重新登录",
	}}

	cur, err := statsOf(mainDB)
	if err != nil {
		return nil, fmt.Errorf("统计当前库: %w", err)
	}
	p.Current = cur
	p.CurrentVersion = db.MaxMigrationVersion()

	// 1) 文件大小上限
	fi, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("备份文件不存在: %w", err)
	}
	if fi.Size() > MaxBackupBytes {
		p.Valid = false
		p.Reason = "file_too_large"
		p.Error = fmt.Sprintf("备份文件 %.0f MB 超过上限 %d MB",
			float64(fi.Size())/float64(1<<20), MaxBackupBytes>>20)
		return p, nil
	}

	// 2~4) SQLite 可打开 + 完整性 + 核心表存在
	bk, err := openBackup(backupPath)
	if err != nil {
		p.Valid = false
		p.Reason = "not_sqlite"
		p.Error = "不是有效的 SQLite 数据库文件"
		return p, nil
	}
	defer bk.Close()

	var integ string
	if err := bk.QueryRow("PRAGMA integrity_check").Scan(&integ); err != nil {
		p.Valid = false
		p.Reason = "not_sqlite"
		p.Error = "无法读取数据库完整性信息"
		return p, nil
	}
	if integ != "ok" {
		p.Valid = false
		p.Reason = "integrity_failed"
		p.Error = fmt.Sprintf("备份文件已损坏（integrity_check=%s）", integ)
		return p, nil
	}
	for _, t := range []string{"users", "ledgers", "accounts", "transactions"} {
		ok, err := tableExistsDB(bk, t)
		if err != nil {
			return nil, err
		}
		if !ok {
			p.Valid = false
			p.Reason = "not_our_backup"
			p.Error = fmt.Sprintf("缺少核心表 %s，不是本系统的备份", t)
			return p, nil
		}
	}

	// 5) 备份版本不得高于当前（降级还原会丢列）
	bkVer, err := db.MaxMigrationVersionDB(bk)
	if err != nil {
		return nil, err
	}
	p.SchemaVersion = bkVer
	if bkVer > p.CurrentVersion {
		p.Valid = false
		p.Reason = "future_version"
		p.Error = fmt.Sprintf("备份来自更新的版本（v%d > 当前 v%d），降级还原会丢失列",
			bkVer, p.CurrentVersion)
		return p, nil
	}

	bakStats, err := statsOf(bk)
	if err != nil {
		return nil, err
	}
	p.Backup = bakStats
	p.Valid = true
	return p, nil
}

// Execute 执行完整还原流程：预检 → pre-restore 快照 → 逐表替换 → 重跑迁移 →
// 重写库世代。任何一步失败都会回滚（事务保证数据原样），并返回错误。
//
// engine 为备份引擎；非 nil 时用于生成 pre-restore 安全网快照。若 engine 为 nil
// （备份功能未启用），则跳过快照并记告警后继续（还原本身不依赖快照）。
func Execute(mainDB *sql.DB, backupPath string, engine *backup.Engine) (*Result, error) {
	// 防御性复检：预检与执行之间文件可能被替换。
	prev, err := Preview(mainDB, backupPath)
	if err != nil {
		return nil, err
	}
	if !prev.Valid {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalid, prev.Error)
	}

	res := &Result{Source: prev.Source, Warnings: prev.Warnings}

	// 安全网：还原前自动快照当前库（失败则中止）。
	if engine != nil {
		item, err := engine.Snapshot("pre-restore")
		if err != nil {
			return nil, fmt.Errorf("还原前自动快照失败: %w", err)
		}
		res.Snapshot = item.Name
	} else {
		res.Warnings = append(res.Warnings,
			"未启用备份功能，本次还原未生成 pre-restore 安全网快照")
	}

	if err := replaceAll(mainDB, backupPath); err != nil {
		return nil, fmt.Errorf("还原数据失败: %w", err)
	}

	// 补齐任何缺失的列/表（对同版本备份通常为空操作）。
	if err := db.ApplyMigrations(mainDB); err != nil {
		return nil, fmt.Errorf("还原后迁移失败: %w", err)
	}

	// 重写库世代：客户端据此判定「库被替换」，清空本地并全量重拉。
	newID, err := db.RotateInstanceID(mainDB)
	if err != nil {
		return nil, fmt.Errorf("重置库世代失败: %w", err)
	}
	res.NewInstanceID = newID

	repl, err := statsOf(mainDB)
	if err != nil {
		return nil, err
	}
	res.Replaced = repl
	return res, nil
}

// ---- 内部实现 ----

// replaceAll 在单连接事务内逐表替换主库数据，并用列交集兼容旧 schema 备份。
func replaceAll(mainDB *sql.DB, backupPath string) error {
	ctx := context.Background()

	// 必须在钉死连接之前取业务表清单：连接池 MaxOpenConns=1，一旦 db.Conn 占住
	// 唯一连接，再对 mainDB.Query 就会因拿不到连接而死锁。
	tables, err := businessTables(mainDB)
	if err != nil {
		return err
	}

	conn, err := mainDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 关闭外键约束（仅影响钉死的这条连接），结束后恢复。
	// 注意：PRAGMA foreign_keys 不能放进事务内切换，必须在 BEGIN 之前/之后设置。
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer func() { _, _ = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON") }()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
		// 无论提交还是回滚都要摘掉 bak 挂载：ATTACH 是**连接级**状态，
		// 连接归还池后仍保持，不 DETACH 会让下一次还原报
		// 「database bak is already in use」（含「用 pre-restore 快照再还原回去」场景）。
		// 回滚路径先 Rollback 再 DETACH（SQLite 不允许在活动事务内 DETACH）。
		_, _ = conn.ExecContext(ctx, "DETACH DATABASE "+bakName)
	}()

	if _, err := tx.ExecContext(ctx, "ATTACH DATABASE ? AS "+bakName, backupPath); err != nil {
		return fmt.Errorf("挂载备份库: %w", err)
	}

	for _, t := range tables {
		exists, err := tableExistsTx(tx, bakName, t)
		if err != nil {
			return err
		}
		if !exists {
			// 备份不含该表（如未来新增表）：清空主库对应表，避免残留旧数据。
			if _, err := tx.ExecContext(ctx,
				fmt.Sprintf("DELETE FROM main.%q", t)); err != nil {
				return fmt.Errorf("清空表 %s: %w", t, err)
			}
			continue
		}

		cols, err := intersectColumns(tx, "main", t, bakName, t)
		if err != nil {
			return err
		}
		if len(cols) == 0 {
			return fmt.Errorf("表 %s 主库与备份无公共列，无法还原", t)
		}
		colList := strings.Join(cols, ", ")

		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM main.%q", t)); err != nil {
			return fmt.Errorf("清空表 %s: %w", t, err)
		}
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO main.%q (%s) SELECT %s FROM %s.%q",
				t, colList, colList, bakName, t)); err != nil {
			return fmt.Errorf("还原表 %s: %w", t, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// businessTables 返回应参与替换的全部业务表（排除系统表）。
// 采用动态发现：主库内所有用户表，去掉 excludedTables，天然兼容未来新增表。
func businessTables(mainDB *sql.DB) ([]string, error) {
	rows, err := mainDB.Query(
		`SELECT name FROM sqlite_master
		 WHERE type='table' AND name NOT LIKE 'sqlite_%'
		   AND name NOT IN (` + quotedList(excludedTables) + `)
		 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("列举业务表: %w", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// quotedList 把 "a,b,c" 转成 "'a','b','c'"，用于 SQL IN 列表。
func quotedList(csv string) string {
	parts := strings.Split(csv, ",")
	qs := make([]string, 0, len(parts))
	for _, p := range parts {
		qs = append(qs, "'"+strings.TrimSpace(p)+"'")
	}
	return strings.Join(qs, ",")
}

// intersectColumns 取主库表与备份表列名的交集（按主库列顺序），用于列交集写入。
func intersectColumns(tx *sql.Tx, mainDB, mainTable, bakDB, bakTable string) ([]string, error) {
	mainCols, err := columnsOfTx(tx, mainDB, mainTable)
	if err != nil {
		return nil, err
	}
	bakCols, err := columnsOfTx(tx, bakDB, bakTable)
	if err != nil {
		return nil, err
	}
	bakSet := make(map[string]bool, len(bakCols))
	for _, c := range bakCols {
		bakSet[c] = true
	}
	var out []string
	for _, c := range mainCols {
		if bakSet[c] {
			out = append(out, c)
		}
	}
	return out, nil
}

// columnsOfTx 用 pragma_table_info 表值函数取某库某表的列名（按定义顺序）。
// 第二个参数指定 schema（'main' 或 'bak'），适配 ATTACH 的备份库。
func columnsOfTx(tx *sql.Tx, schema, table string) ([]string, error) {
	rows, err := tx.QueryContext(context.Background(),
		`SELECT name FROM pragma_table_info(?, ?)`, table, schema)
	if err != nil {
		return nil, fmt.Errorf("读取列信息 %s.%s: %w", schema, table, err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// tableExistsTx 判断某库是否存在某表。
func tableExistsTx(tx *sql.Tx, schema, table string) (bool, error) {
	var n int
	if err := tx.QueryRowContext(context.Background(),
		fmt.Sprintf("SELECT count(*) FROM %s.sqlite_master WHERE type='table' AND name=?", schema),
		table).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// tableExistsDB 在独立连接上判断表是否存在。
func tableExistsDB(db *sql.DB, table string) (bool, error) {
	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table,
	).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// openBackup 以 SQLite 打开备份文件（只读校验，不写盘）。
func openBackup(path string) (*sql.DB, error) {
	bk, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开备份: %w", err)
	}
	// 轻量探活：确认可查询。
	var n int
	if err := bk.QueryRow(`SELECT count(*) FROM sqlite_master`).Scan(&n); err != nil {
		bk.Close()
		return nil, err
	}
	return bk, nil
}

// statsOf 统计核心表行数。
func statsOf(db *sql.DB) (Stats, error) {
	var s Stats
	var err error
	if s.Users, err = countTable(db, "users"); err != nil {
		return s, err
	}
	if s.Ledgers, err = countTable(db, "ledgers"); err != nil {
		return s, err
	}
	if s.Accounts, err = countTable(db, "accounts"); err != nil {
		return s, err
	}
	if s.Transactions, err = countTable(db, "transactions"); err != nil {
		return s, err
	}
	return s, nil
}

func countTable(db *sql.DB, t string) (int64, error) {
	var n int64
	if err := db.QueryRow(`SELECT count(*) FROM ` + t).Scan(&n); err != nil {
		return 0, fmt.Errorf("统计表 %s: %w", t, err)
	}
	return n, nil
}
