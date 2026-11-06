package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// 迁移 012 使用的常量。集中定义避免散落的魔法字符串。
const (
	migration012Name = "012_opening_balance"

	openingEquityName        = "Equity:Opening-Balances"
	openingEquityDisplayName = "期初余额权益"
	openingTxnDescription    = "期初余额"
	reversalDescriptionPref  = "冲正:"
	dedupeReversalDesc       = "冲正: 期初余额（迁移去重）"

	entityAccount     = "account"
	entityTransaction = "transaction"
	opCreate          = "create"
	opUpdate          = "update"
)

// legacyOpening 是迁移 012 从一个老「期初余额」交易中识别出的元数据。
type legacyOpening struct {
	txnID       int64
	txnUUID     string
	ledgerID    int64
	date        string
	createdAt   string
	accountUUID string
	commodity   string
	amount      *big.Rat
	version     int64
}

// candidatePosting 是迁移期读取的一行 posting（带账户状态）。
type candidatePosting struct {
	accountID   int64
	accountUUID sql.NullString
	accountName string
	ledgerID    int64
	closeDate   sql.NullString
	deletedAt   sql.NullString
	commodity   string
	amount      string
	position    int64
}

// migrate012：期初余额与冲正元数据（PRD FR-01 / FR-21~FR-24）。
//
// 执行顺序严格按设计 §5.3：加列（A）→ 回填账本期初日期（B）→
// 老数据期初元数据回填（C）→ 重复有效期初去重（D）→ 权益账户补种（E）→
// 未关闭账户开户日收敛（F）→ 创建唯一/溯源索引（G）。
//
// 全部步骤在同一个显式事务内完成：任一步失败整体回滚，schema_migrations
// 不会记录本迁移，修复后可安全重跑。所有写操作只使用 tx，避免单连接死锁。
func migrate012(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("开启 012 事务: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	steps := []struct {
		label string
		fn    func(*sql.Tx) error
	}{
		{"A 新增列与迁移审计表", migrate012AddColumns},
		{"B 回填账本期初日期", migrate012BackfillLedgerOpeningDate},
		{"C 回填老期初交易元数据", migrate012BackfillOpeningMetadata},
		{"D 去重重复有效期初", migrate012DedupeActiveOpenings},
		{"E 补种期初权益账户", migrate012SeedOpeningEquity},
		{"F 收敛未关闭账户开户日", migrate012LowerAccountOpenDates},
		{"G 创建期初/冲正索引", migrate012CreateIndexes},
	}
	for _, s := range steps {
		if err := s.fn(tx); err != nil {
			return fmt.Errorf("%s: %w", s.label, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交 012 事务: %w", err)
	}
	return nil
}

// migrate012AddColumns 增加账本期初日期、交易期初/冲正元数据列与迁移审计表。
// ledgers.opening_date 采用可空 TEXT（PRD 明确允许 SQLite 加列不设 NOT NULL，
// 非空由应用层保证；本迁移会回填所有旧行）。
func migrate012AddColumns(tx *sql.Tx) error {
	columns := []struct {
		table string
		name  string
		ddl   string
	}{
		{"ledgers", "opening_date", "TEXT"},
		{"transactions", "opening_account_uuid", "TEXT"},
		{"transactions", "opening_version", "INTEGER NOT NULL DEFAULT 0"},
		{"transactions", "opening_key", "TEXT"},
		{"transactions", "reversal_of", "TEXT"},
		{"transactions", "reversed_by", "TEXT"},
	}
	for _, c := range columns {
		ok, err := columnExistsTx(tx, c.table, c.name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := tx.Exec(
			`ALTER TABLE ` + c.table + ` ADD COLUMN ` + c.name + ` ` + c.ddl,
		); err != nil {
			return fmt.Errorf("增加 %s.%s: %w", c.table, c.name, err)
		}
	}

	// 迁移审计日志：只记录迁移期修正，不参与同步、不暴露 HTTP。
	if _, err := tx.Exec(
		`CREATE TABLE IF NOT EXISTS migration_log (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			migration   TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_uuid TEXT,
			ledger_id   INTEGER,
			old_value   TEXT,
			new_value   TEXT,
			detail      TEXT,
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	); err != nil {
		return fmt.Errorf("创建 migration_log: %w", err)
	}
	if _, err := tx.Exec(
		`CREATE INDEX IF NOT EXISTS idx_migration_log_name
		 ON migration_log(migration, id)`,
	); err != nil {
		return fmt.Errorf("创建 migration_log 索引: %w", err)
	}
	return nil
}

// migrate012BackfillLedgerOpeningDate 回填旧账本的 opening_date：
// min(最早未删交易日期, 建账日期, 今日)；无交易时取建账日期（未来建账日被今日截断）。
//
// created_at 存 UTC，必须转本地日期再比较，避免跨零点偏一天。
func migrate012BackfillLedgerOpeningDate(tx *sql.Tx) error {
	if _, err := tx.Exec(
		`UPDATE ledgers
		 SET opening_date = MIN(
			COALESCE(
				(SELECT MIN(t.date) FROM transactions t
				 WHERE t.ledger_id = ledgers.id AND t.deleted_at IS NULL),
				'9999-12-31'
			),
			COALESCE(date(ledgers.created_at, 'localtime'), date('now', 'localtime')),
			date('now', 'localtime')
		 )
		 WHERE opening_date IS NULL OR opening_date = ''`,
	); err != nil {
		return fmt.Errorf("回填 ledgers.opening_date: %w", err)
	}
	return nil
}

// migrate012BackfillOpeningMetadata 识别迁移前生成的老期初交易，并回填
// opening_account_uuid/opening_version/opening_key/reversal_of/reversed_by。
//
// 只处理未关闭账户（FR-23）：已关闭账户的期初交易保持所有新列为 NULL。
func migrate012BackfillOpeningMetadata(tx *sql.Tx) error {
	openings, err := findLegacyOpenings(tx)
	if err != nil {
		return err
	}
	if len(openings) == 0 {
		return nil
	}

	// 每账户按期初交易 created_at,id 升序编号 1..N，并只给原期初写幂等键。
	sort.SliceStable(openings, func(i, j int) bool {
		if openings[i].ledgerID != openings[j].ledgerID {
			return openings[i].ledgerID < openings[j].ledgerID
		}
		if openings[i].accountUUID != openings[j].accountUUID {
			return openings[i].accountUUID < openings[j].accountUUID
		}
		if openings[i].createdAt != openings[j].createdAt {
			return openings[i].createdAt < openings[j].createdAt
		}
		return openings[i].txnID < openings[j].txnID
	})
	version := map[string]int64{}
	for i := range openings {
		key := fmt.Sprintf("%d|%s", openings[i].ledgerID, openings[i].accountUUID)
		version[key]++
		openings[i].version = version[key]

		openingKey := fmt.Sprintf("opening:%s:%d", openings[i].accountUUID, openings[i].version)
		if _, err := tx.Exec(
			`UPDATE transactions
			 SET opening_account_uuid=?, opening_version=?, opening_key=?
			 WHERE id=? AND (opening_account_uuid IS NULL OR opening_account_uuid='')`,
			openings[i].accountUUID, openings[i].version, openingKey, openings[i].txnID,
		); err != nil {
			return fmt.Errorf("回填期初交易 %s: %w", openings[i].txnUUID, err)
		}
	}

	// 建立「账本|日期|账户|币种|金额」→ 原期初 的索引，用于唯一匹配历史冲正。
	byKey := map[string][]legacyOpening{}
	for _, o := range openings {
		k := openingMatchKey(o.ledgerID, o.date, o.accountUUID, o.commodity, o.amount)
		byKey[k] = append(byKey[k], o)
	}

	reversals, err := findLegacyReversals(tx)
	if err != nil {
		return err
	}
	for _, rev := range reversals {
		negAmount := new(big.Rat).Neg(rev.amount)
		k := openingMatchKey(rev.ledgerID, rev.date, rev.accountUUID, rev.commodity, negAmount)
		matches := byKey[k]
		if len(matches) != 1 {
			// 0 条：没有对应的老期初；>1 条：金额/日期完全相同，无法唯一溯源。
			// 两种情况都保持 NULL，交由 D 步骤保证「每账户至多一条有效期初」。
			continue
		}
		orig := matches[0]

		res, err := tx.Exec(
			`UPDATE transactions SET reversed_by=?
			 WHERE id=? AND (reversed_by IS NULL OR reversed_by='')`,
			rev.txnUUID, orig.txnID,
		)
		if err != nil {
			return fmt.Errorf("标记老期初 %s 已冲正: %w", orig.txnUUID, err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE transactions
			 SET reversal_of=?, opening_account_uuid=?, opening_version=?
			 WHERE id=? AND (reversal_of IS NULL OR reversal_of='')`,
			orig.txnUUID, orig.accountUUID, orig.version, rev.txnID,
		); err != nil {
			return fmt.Errorf("回填历史冲正 %s: %w", rev.txnUUID, err)
		}
	}
	return nil
}

// findLegacyOpenings 识别「期初余额」老交易（无 opening_* 列时代生成）。
func findLegacyOpenings(tx *sql.Tx) ([]legacyOpening, error) {
	rows, err := tx.Query(
		`SELECT id, uuid, ledger_id, date, created_at
		 FROM transactions
		 WHERE description=? AND flag='*' AND deleted_at IS NULL
		   AND (opening_account_uuid IS NULL OR opening_account_uuid='')
		 ORDER BY ledger_id, created_at, id`,
		openingTxnDescription,
	)
	if err != nil {
		return nil, fmt.Errorf("查询老期初交易: %w", err)
	}

	type rawTxn struct {
		id        int64
		uuid      string
		ledgerID  int64
		date      string
		createdAt string
	}
	var raws []rawTxn
	for rows.Next() {
		var r rawTxn
		if err := rows.Scan(&r.id, &r.uuid, &r.ledgerID, &r.date, &r.createdAt); err != nil {
			rows.Close()
			return nil, err
		}
		raws = append(raws, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []legacyOpening
	for _, r := range raws {
		posts, err := loadCandidatePostings(tx, r.id)
		if err != nil {
			return nil, err
		}
		target, ok := classifyOpeningPostings(posts, r.ledgerID)
		if !ok {
			continue
		}
		amount, err := parseAmountRat(target.amount)
		if err != nil {
			return nil, fmt.Errorf("期初交易 %s 金额非法: %w", r.uuid, err)
		}
		// 仅未关闭账户参与回填（FR-23）；classifyOpeningPostings 已保证 target 未关闭。
		out = append(out, legacyOpening{
			txnID:       r.id,
			txnUUID:     r.uuid,
			ledgerID:    r.ledgerID,
			date:        r.date,
			createdAt:   r.createdAt,
			accountUUID: target.accountUUID.String,
			commodity:   target.commodity,
			amount:      amount,
		})
	}
	return out, nil
}

// findLegacyReversals 识别「冲正:...」老交易（迁移前无 reversal_of 列）。
func findLegacyReversals(tx *sql.Tx) ([]legacyOpening, error) {
	rows, err := tx.Query(
		`SELECT id, uuid, ledger_id, date, created_at
		 FROM transactions
		 WHERE description LIKE ? AND deleted_at IS NULL
		   AND (reversal_of IS NULL OR reversal_of='')
		 ORDER BY ledger_id, created_at, id`,
		reversalDescriptionPref+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("查询历史冲正交易: %w", err)
	}

	type rawTxn struct {
		id        int64
		uuid      string
		ledgerID  int64
		date      string
		createdAt string
	}
	var raws []rawTxn
	for rows.Next() {
		var r rawTxn
		if err := rows.Scan(&r.id, &r.uuid, &r.ledgerID, &r.date, &r.createdAt); err != nil {
			rows.Close()
			return nil, err
		}
		raws = append(raws, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []legacyOpening
	for _, r := range raws {
		posts, err := loadCandidatePostings(tx, r.id)
		if err != nil {
			return nil, err
		}
		target, ok := classifyOpeningPostings(posts, r.ledgerID)
		if !ok {
			continue
		}
		amount, err := parseAmountRat(target.amount)
		if err != nil {
			return nil, fmt.Errorf("冲正交易 %s 金额非法: %w", r.uuid, err)
		}
		out = append(out, legacyOpening{
			txnID:       r.id,
			txnUUID:     r.uuid,
			ledgerID:    r.ledgerID,
			date:        r.date,
			createdAt:   r.createdAt,
			accountUUID: target.accountUUID.String,
			commodity:   target.commodity,
			amount:      amount,
		})
	}
	return out, nil
}

// loadCandidatePostings 读取某笔交易的全部分录及账户状态。
func loadCandidatePostings(tx *sql.Tx, txnID int64) ([]candidatePosting, error) {
	rows, err := tx.Query(
		`SELECT p.account_id, a.uuid, a.name, a.ledger_id, a.close_date, a.deleted_at,
		        p.commodity, CAST(p.amount AS TEXT), p.position
		 FROM postings p JOIN accounts a ON a.id=p.account_id
		 WHERE p.transaction_id=?
		 ORDER BY p.position, p.id`,
		txnID,
	)
	if err != nil {
		return nil, fmt.Errorf("读取交易 %d 分录: %w", txnID, err)
	}
	defer rows.Close()

	var out []candidatePosting
	for rows.Next() {
		var p candidatePosting
		if err := rows.Scan(&p.accountID, &p.accountUUID, &p.accountName, &p.ledgerID,
			&p.closeDate, &p.deletedAt, &p.commodity, &p.amount, &p.position); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// classifyOpeningPostings 校验一笔交易是否符合「期初余额」形态：
// 恰好两条分录、同账本、同币种、金额互为相反数；一条指向未删的
// Equity:Opening-Balances，另一条指向未关闭且未删的账户。
// 返回目标账户那条 posting；不符合返回 ok=false。
func classifyOpeningPostings(posts []candidatePosting, ledgerID int64) (candidatePosting, bool) {
	if len(posts) != 2 {
		return candidatePosting{}, false
	}
	equityIdx, targetIdx := -1, -1
	for i, p := range posts {
		if p.ledgerID != ledgerID {
			return candidatePosting{}, false
		}
		if !p.accountUUID.Valid || p.accountUUID.String == "" {
			return candidatePosting{}, false
		}
		if p.accountName == openingEquityName && !p.deletedAt.Valid {
			equityIdx = i
			continue
		}
		if !p.deletedAt.Valid && !p.closeDate.Valid {
			targetIdx = i
		}
	}
	if equityIdx < 0 || targetIdx < 0 || equityIdx == targetIdx {
		return candidatePosting{}, false
	}
	equity, target := posts[equityIdx], posts[targetIdx]
	if equity.accountID == target.accountID || equity.commodity != target.commodity {
		return candidatePosting{}, false
	}
	a, err := parseAmountRat(equity.amount)
	if err != nil {
		return candidatePosting{}, false
	}
	b, err := parseAmountRat(target.amount)
	if err != nil {
		return candidatePosting{}, false
	}
	if new(big.Rat).Add(a, b).Sign() != 0 {
		return candidatePosting{}, false
	}
	return target, true
}

// migrate012DedupeActiveOpenings 对未关闭账户去重：若同一账户仍有 >1 条有效期初，
// 保留 id 最大者，其余逐条生成真实冲正交易并置 reversed_by。
func migrate012DedupeActiveOpenings(tx *sql.Tx) error {
	rows, err := tx.Query(
		`SELECT t.ledger_id, t.opening_account_uuid
		 FROM transactions t
		 JOIN accounts a ON a.ledger_id=t.ledger_id AND a.uuid=t.opening_account_uuid
		 WHERE t.opening_account_uuid IS NOT NULL AND t.opening_account_uuid <> ''
		   AND t.reversal_of IS NULL
		   AND (t.reversed_by IS NULL OR t.reversed_by='')
		   AND t.deleted_at IS NULL
		   AND a.close_date IS NULL AND a.deleted_at IS NULL
		 GROUP BY t.ledger_id, t.opening_account_uuid
		 HAVING count(*) > 1`,
	)
	if err != nil {
		return fmt.Errorf("查询重复有效期初: %w", err)
	}
	type group struct {
		ledgerID    int64
		accountUUID string
	}
	var groups []group
	for rows.Next() {
		var g group
		if err := rows.Scan(&g.ledgerID, &g.accountUUID); err != nil {
			rows.Close()
			return err
		}
		groups = append(groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, g := range groups {
		actives, err := activeOpenings(tx, g.ledgerID, g.accountUUID)
		if err != nil {
			return err
		}
		if len(actives) <= 1 {
			continue
		}
		// 保留 id 最大者（activeOpenings 按 id 升序）。
		for _, dup := range actives[:len(actives)-1] {
			if err := reverseMigratedOpening(tx, dup, g.ledgerID, g.accountUUID); err != nil {
				return err
			}
		}
	}
	return nil
}

// activeOpenings 返回某账户当前有效的期初交易（未冲正、未删除），按 id 升序。
func activeOpenings(tx *sql.Tx, ledgerID int64, accountUUID string) ([]legacyOpening, error) {
	rows, err := tx.Query(
		`SELECT id, uuid, ledger_id, date, opening_version
		 FROM transactions
		 WHERE ledger_id=? AND opening_account_uuid=?
		   AND reversal_of IS NULL
		   AND (reversed_by IS NULL OR reversed_by='')
		   AND deleted_at IS NULL
		 ORDER BY id`,
		ledgerID, accountUUID,
	)
	if err != nil {
		return nil, fmt.Errorf("读取账户 %s 有效期初: %w", accountUUID, err)
	}
	defer rows.Close()

	var out []legacyOpening
	for rows.Next() {
		var o legacyOpening
		if err := rows.Scan(&o.txnID, &o.txnUUID, &o.ledgerID, &o.date, &o.version); err != nil {
			return nil, err
		}
		o.accountUUID = accountUUID
		out = append(out, o)
	}
	return out, rows.Err()
}

// reverseMigratedOpening 为重复的期初交易生成一条真实冲正交易：
// 同日期、posting 全部取负、reversal_of=原 uuid、保留原 opening_version，
// 并写 sync_log(transaction, create) 与 migration_log。
func reverseMigratedOpening(tx *sql.Tx, dup legacyOpening, ledgerID int64, accountUUID string) error {
	posts, err := loadCandidatePostings(tx, dup.txnID)
	if err != nil {
		return err
	}
	if len(posts) == 0 {
		return fmt.Errorf("重复期初 %s 无分录", dup.txnUUID)
	}

	tagsJSON, err := json.Marshal([]string{"opening", "reversal"})
	if err != nil {
		return fmt.Errorf("序列化冲正标签: %w", err)
	}
	revUUID := uuid.NewString()
	res, err := tx.Exec(
		`INSERT INTO transactions
		 (ledger_id, uuid, date, flag, description, tags,
		  opening_account_uuid, opening_version, reversal_of)
		 VALUES (?, ?, ?, '*', ?, ?, ?, ?, ?)`,
		ledgerID, revUUID, dup.date, dedupeReversalDesc, string(tagsJSON),
		accountUUID, dup.version, dup.txnUUID,
	)
	if err != nil {
		return fmt.Errorf("写入去重冲正交易 %s: %w", dup.txnUUID, err)
	}
	revID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	for _, p := range posts {
		neg, err := negateDecimalString(p.amount)
		if err != nil {
			return fmt.Errorf("冲正交易 %s 金额非法: %w", dup.txnUUID, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
			 VALUES(?, ?, ?, ?, ?)`,
			revID, p.accountID, p.commodity, neg, p.position,
		); err != nil {
			return fmt.Errorf("写入冲正交易 %s 分录: %w", dup.txnUUID, err)
		}
	}

	res, err = tx.Exec(
		`UPDATE transactions SET reversed_by=?
		 WHERE id=? AND (reversed_by IS NULL OR reversed_by='')`,
		revUUID, dup.txnID,
	)
	if err != nil {
		return fmt.Errorf("标记重复期初 %s 已冲正: %w", dup.txnUUID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("重复期初 %s 已被并发标记冲正", dup.txnUUID)
	}

	if err := insertMigrationLogTx(tx, entityTransaction, dup.txnUUID, ledgerID,
		"duplicate", "reversed", "due to concurrent duplicate"); err != nil {
		return err
	}
	if err := appendSyncLogTx(tx, ledgerID, entityTransaction, revUUID, opCreate); err != nil {
		return err
	}
	return nil
}

// migrate012SeedOpeningEquity 为缺少有效 Equity:Opening-Balances 的账本补种。
// 只复用「未删除」的同名账户：同名账户可能被 002 删除语义软删，此时必须
// 新建一个有效账户，否则账本将没有任何可用的期初权益账户（BUG-03）。
// 已关闭但未删除的同名账户仍复用（006 之后账户名在账本内全局唯一）。
//
// 已执行过 012 的库不会重跑本函数，由运行时 ensureOpeningEquityTx 兜底补种。
func migrate012SeedOpeningEquity(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, opening_date FROM ledgers ORDER BY id`)
	if err != nil {
		return fmt.Errorf("读取账本: %w", err)
	}
	type ledgerRow struct {
		id          int64
		openingDate sql.NullString
	}
	var ledgers []ledgerRow
	for rows.Next() {
		var l ledgerRow
		if err := rows.Scan(&l.id, &l.openingDate); err != nil {
			rows.Close()
			return err
		}
		ledgers = append(ledgers, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	for _, l := range ledgers {
		openDate := today
		if l.openingDate.Valid && l.openingDate.String != "" {
			openDate = l.openingDate.String
		}

		var active int
		if err := tx.QueryRow(
			`SELECT count(*) FROM accounts WHERE ledger_id=? AND name=? AND deleted_at IS NULL`,
			l.id, openingEquityName,
		).Scan(&active); err != nil {
			return fmt.Errorf("检查账本 %d 权益账户: %w", l.id, err)
		}
		if active > 0 {
			continue
		}

		// 同名账户全部已软删：006 之后账户名在账本内全局唯一（不区分 deleted_at），
		// 无法插入同名新账户，只能就地恢复已有账户为有效状态，避免历史分录悬空。
		var softID int64
		err := tx.QueryRow(
			`SELECT id FROM accounts WHERE ledger_id=? AND name=? AND deleted_at IS NOT NULL
			 ORDER BY id LIMIT 1`,
			l.id, openingEquityName,
		).Scan(&softID)
		if err == nil {
			if _, err := tx.Exec(
				`UPDATE accounts
				 SET deleted_at=NULL, close_date=NULL, updated_at=datetime('now')
				 WHERE id=? AND deleted_at IS NOT NULL`,
				softID,
			); err != nil {
				return fmt.Errorf("恢复账本 %d 期初权益账户: %w", l.id, err)
			}
			var softUUID string
			if err := tx.QueryRow(
				`SELECT COALESCE(uuid, '') FROM accounts WHERE id=?`, softID,
			).Scan(&softUUID); err != nil {
				return fmt.Errorf("读取账本 %d 期初权益账户 uuid: %w", l.id, err)
			}
			if err := appendSyncLogTx(tx, l.id, entityAccount, softUUID, opUpdate); err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("查询账本 %d 已软删权益账户: %w", l.id, err)
		}

		equityUUID := uuid.NewString()
		if _, err := tx.Exec(
			`INSERT INTO accounts
			 (ledger_id, uuid, name, display_name, type, open_date,
			  close_date, commodity_restriction, icon, color, parent_uuid, sub_type, sort_order,
			  deleted_at, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'Equity', ?,
			         NULL, NULL, '⚖️', '#868E96', NULL, NULL, 0,
			         NULL, datetime('now'), datetime('now'))`,
			l.id, equityUUID, openingEquityName, openingEquityDisplayName, openDate,
		); err != nil {
			return fmt.Errorf("补种账本 %d 期初权益账户: %w", l.id, err)
		}
		if err := appendSyncLogTx(tx, l.id, entityAccount, equityUUID, opCreate); err != nil {
			return err
		}
	}
	return nil
}

// migrate012LowerAccountOpenDates 对未关闭账户执行
// open_date = min(open_date, 账本 opening_date)，只下调不上调；
// 逐行写 migration_log 与 sync_log(account, update)。已关闭账户完全不动（FR-23）。
func migrate012LowerAccountOpenDates(tx *sql.Tx) error {
	rows, err := tx.Query(
		`SELECT a.id, a.uuid, COALESCE(a.open_date, ''), a.ledger_id, l.opening_date
		 FROM accounts a JOIN ledgers l ON l.id=a.ledger_id
		 WHERE a.deleted_at IS NULL AND a.close_date IS NULL
		   AND l.opening_date IS NOT NULL AND l.opening_date <> ''
		   AND (a.open_date IS NULL OR a.open_date = '' OR a.open_date > l.opening_date)
		 ORDER BY a.ledger_id, a.id`,
	)
	if err != nil {
		return fmt.Errorf("查询待收敛开户日账户: %w", err)
	}
	type accountRow struct {
		id          int64
		uuid        sql.NullString
		oldOpenDate string
		ledgerID    int64
		openingDate string
	}
	var accounts []accountRow
	for rows.Next() {
		var a accountRow
		if err := rows.Scan(&a.id, &a.uuid, &a.oldOpenDate, &a.ledgerID, &a.openingDate); err != nil {
			rows.Close()
			return err
		}
		accounts = append(accounts, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, a := range accounts {
		// 条件再次校验，确保只下调且绝不碰已关闭账户（防御并发/重复执行）。
		res, err := tx.Exec(
			`UPDATE accounts SET open_date=?, updated_at=datetime('now')
			 WHERE id=? AND deleted_at IS NULL AND close_date IS NULL
			   AND (open_date IS NULL OR open_date='' OR open_date > ?)`,
			a.openingDate, a.id, a.openingDate,
		)
		if err != nil {
			return fmt.Errorf("收敛账户 %d 开户日: %w", a.id, err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		}
		uuidStr := ""
		if a.uuid.Valid {
			uuidStr = a.uuid.String
		}
		detail := fmt.Sprintf("open_date lowered to ledger opening_date (ledger_id=%d)", a.ledgerID)
		if err := insertMigrationLogTx(tx, entityAccount, uuidStr, a.ledgerID,
			a.oldOpenDate, a.openingDate, detail); err != nil {
			return err
		}
		if err := appendSyncLogTx(tx, a.ledgerID, entityAccount, uuidStr, opUpdate); err != nil {
			return err
		}
	}
	return nil
}

// migrate012CreateIndexes 创建期初唯一性与冲正溯源索引。
// 必须在 C/D 之后：先去重再建唯一索引，避免历史重复数据导致迁移失败。
//
// 注意：有效唯一索引额外排除 reversal_of 非空的冲正交易。冲正交易同样携带
// opening_account_uuid（用于溯源），若不排除，它会被误判为「另一条有效期初」，
// 导致唯一索引在迁移自身写入冲正交易后立即冲突。语义上「有效期初」只应是
// 非冲正交易本身。
func migrate012CreateIndexes(tx *sql.Tx) error {
	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_txn_opening_key
		 ON transactions (ledger_id, opening_key)
		 WHERE opening_key IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_txn_opening_active
		 ON transactions (ledger_id, opening_account_uuid)
		 WHERE opening_account_uuid IS NOT NULL
		   AND reversal_of IS NULL
		   AND reversed_by IS NULL
		   AND deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_txn_reversal_of
		 ON transactions (ledger_id, reversal_of)
		 WHERE reversal_of IS NOT NULL`,
	}
	for _, idx := range indexes {
		if _, err := tx.Exec(idx); err != nil {
			return fmt.Errorf("创建期初索引: %w", err)
		}
	}
	return nil
}

// columnExistsTx 判断表是否已有某列（PRAGMA 查询，仅用于硬编码表名）。
func columnExistsTx(tx *sql.Tx, table, col string) (bool, error) {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
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
	return false, rows.Err()
}

// parseAmountRat 把定点十进制字符串解析为精确有理数，绝不使用浮点。
func parseAmountRat(s string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return nil, fmt.Errorf("非法金额 %q", s)
	}
	return r, nil
}

// negateDecimalString 取负一个定点十进制字符串，保留原精度。
// 例："10.00" → "-10.00"、"-0.50" → "0.50"、"0" → "0"。
func negateDecimalString(s string) (string, error) {
	r, err := parseAmountRat(s)
	if err != nil {
		return "", err
	}
	if r.Sign() == 0 {
		return "0", nil
	}
	t := strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(t, "-"):
		return strings.TrimPrefix(t, "-"), nil
	case strings.HasPrefix(t, "+"):
		return "-" + strings.TrimPrefix(t, "+"), nil
	default:
		return "-" + t, nil
	}
}

// openingMatchKey 是期初交易与历史冲正唯一匹配用的规范化键。
// 金额使用 big.Rat 的 RatString()，保证 "10" 与 "10.00" 视为同一金额。
func openingMatchKey(ledgerID int64, date, accountUUID, commodity string, amount *big.Rat) string {
	return fmt.Sprintf("%d|%s|%s|%s|%s", ledgerID, date, accountUUID, commodity, amount.RatString())
}

// appendSyncLogTx 在迁移事务内追加一条同步变更事件。
func appendSyncLogTx(tx *sql.Tx, ledgerID int64, entityType, entityUUID, op string) error {
	if _, err := tx.Exec(
		`INSERT INTO sync_log(ledger_id, entity_type, entity_uuid, op) VALUES(?, ?, ?, ?)`,
		ledgerID, entityType, entityUUID, op,
	); err != nil {
		return fmt.Errorf("写入 sync_log(%s %s): %w", entityType, entityUUID, err)
	}
	return nil
}

// insertMigrationLogTx 在迁移事务内写一条审计记录（old/new 值可追溯）。
func insertMigrationLogTx(tx *sql.Tx, entityType, entityUUID string, ledgerID int64,
	oldValue, newValue, detail string) error {
	var uuidArg any
	if entityUUID != "" {
		uuidArg = entityUUID
	}
	if _, err := tx.Exec(
		`INSERT INTO migration_log
		 (migration, entity_type, entity_uuid, ledger_id, old_value, new_value, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		migration012Name, entityType, uuidArg, ledgerID, oldValue, newValue, detail,
	); err != nil {
		return fmt.Errorf("写入 migration_log(%s %s): %w", entityType, entityUUID, err)
	}
	return nil
}
