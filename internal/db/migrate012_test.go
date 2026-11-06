package db

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestMigrate012EmptyDB 空库执行 012：不报错、迁移有记录、索引与审计表就绪。
func TestMigrate012EmptyDB(t *testing.T) {
	conn := openTestDB(t)

	var n int
	if err := conn.QueryRow(
		`SELECT count(*) FROM schema_migrations WHERE name='012_opening_balance'`,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("空库应记录 012 一次，实际 %d", n)
	}

	for _, idx := range []string{
		"idx_txn_opening_key",
		"idx_txn_opening_active",
		"idx_txn_reversal_of",
		"idx_migration_log_name",
	} {
		var c int
		if err := conn.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&c); err != nil {
			t.Fatal(err)
		}
		if c != 1 {
			t.Fatalf("索引 %s 不存在", idx)
		}
	}

	// 空库没有账本级数据修正，审计日志应为空。
	if err := conn.QueryRow(`SELECT count(*) FROM migration_log`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("空库 migration_log 应为 0，实际 %d", n)
	}
}

// TestMigrate012LegacyData 在 011 旧库副本上执行 012，覆盖：
// 账本日期回填、老期初识别、历史冲正溯源、重复有效期初去重、权益补种、
// 未关闭账户开户日收敛、已关闭账户零变更。
func TestMigrate012LegacyData(t *testing.T) {
	conn := openTestDB(t)
	seedMigrate012Fixture(t, conn)
	simulatePre012(t, conn)

	// 迁移前记录已关闭账户状态，迁移后逐行比对（FR-23）。
	closedOpenDate := queryText(t, conn, `SELECT open_date FROM accounts WHERE id=3`)
	closedTxn := queryTxnMeta(t, conn, "t5")
	equity3OpenDate := queryText(t, conn, `SELECT open_date FROM accounts WHERE id=6`)

	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("执行 012: %v", err)
	}

	// B：账本 opening_date 回填。
	cases := []struct {
		id   int64
		want string
	}{
		{1, "2025-06-01"}, // 最早交易
		{2, "2026-01-01"}, // 无交易 → 建账日
		{3, "2026-02-05"}, // 无交易 → 建账日
	}
	for _, c := range cases {
		got := queryText(t, conn, `SELECT opening_date FROM ledgers WHERE id=`+strconv.FormatInt(c.id, 10))
		if got != c.want {
			t.Fatalf("账本 %d opening_date 期望 %s，实际 %s", c.id, c.want, got)
		}
	}

	var empty int
	if err := conn.QueryRow(
		`SELECT count(*) FROM ledgers WHERE opening_date IS NULL OR opening_date=''`,
	).Scan(&empty); err != nil {
		t.Fatal(err)
	}
	if empty != 0 {
		t.Fatalf("仍有 %d 个账本 opening_date 为空", empty)
	}

	// C：老期初按 created_at,id 编号 1..N。
	if got := queryText(t, conn, `SELECT opening_key FROM transactions WHERE uuid='t1'`); got != "opening:acc-cash:1" {
		t.Fatalf("t1 opening_key 错误: %s", got)
	}
	if got := queryText(t, conn, `SELECT opening_key FROM transactions WHERE uuid='t2'`); got != "opening:acc-cash:2" {
		t.Fatalf("t2 opening_key 错误: %s", got)
	}
	if got := queryText(t, conn, `SELECT opening_key FROM transactions WHERE uuid='t3'`); got != "opening:acc-cash:3" {
		t.Fatalf("t3 opening_key 错误: %s", got)
	}

	// C.2：历史冲正唯一匹配并双向回填。
	if got := queryText(t, conn, `SELECT reversed_by FROM transactions WHERE uuid='t1'`); got != "t4" {
		t.Fatalf("t1 reversed_by 期望 t4，实际 %q", got)
	}
	if got := queryText(t, conn, `SELECT reversal_of FROM transactions WHERE uuid='t4'`); got != "t1" {
		t.Fatalf("t4 reversal_of 期望 t1，实际 %q", got)
	}
	if got := queryInt(t, conn, `SELECT opening_version FROM transactions WHERE uuid='t4'`); got != 1 {
		t.Fatalf("t4 opening_version 期望 1，实际 %d", got)
	}
	if got := queryText(t, conn, `SELECT COALESCE(opening_key,'') FROM transactions WHERE uuid='t4'`); got != "" {
		t.Fatalf("冲正交易不应写 opening_key，实际 %s", got)
	}

	// D：重复有效期初去重——保留 id 最大者 t3，t2 被真实冲正。
	active := queryInt(t, conn,
		`SELECT count(*) FROM transactions
		 WHERE ledger_id=1 AND opening_account_uuid='acc-cash'
		   AND reversal_of IS NULL
		   AND reversed_by IS NULL AND deleted_at IS NULL`)
	if active != 1 {
		t.Fatalf("账户应只剩 1 条有效期初，实际 %d", active)
	}
	if got := queryText(t, conn, `SELECT COALESCE(reversed_by,'') FROM transactions WHERE uuid='t2'`); got == "" {
		t.Fatalf("t2 应被去重冲正")
	}
	revOfT2 := queryText(t, conn, `SELECT reversed_by FROM transactions WHERE uuid='t2'`)
	var revVersion int
	var revDesc, revTags string
	if err := conn.QueryRow(
		`SELECT opening_version, description, COALESCE(tags,'')
		 FROM transactions WHERE uuid=?`, revOfT2,
	).Scan(&revVersion, &revDesc, &revTags); err != nil {
		t.Fatalf("读取去重冲正交易: %v", err)
	}
	if revVersion != 2 || revDesc != dedupeReversalDesc {
		t.Fatalf("去重冲正元数据错误: version=%d desc=%s", revVersion, revDesc)
	}
	if !strings.Contains(revTags, `"opening"`) || !strings.Contains(revTags, `"reversal"`) {
		t.Fatalf("去重冲正 tags 错误: %s", revTags)
	}
	if got := queryText(t, conn, `SELECT COALESCE(reversal_of,'') FROM transactions WHERE uuid=?`, revOfT2); got != "t2" {
		t.Fatalf("去重冲正 reversal_of 期望 t2，实际 %s", got)
	}

	// E：账本 2 缺权益账户 → 补种；账本 3 已有同名（已关闭）→ 复用不重复插入。
	for _, ledgerID := range []int64{1, 2, 3} {
		cnt := queryInt(t, conn,
			`SELECT count(*) FROM accounts WHERE ledger_id=? AND name='Equity:Opening-Balances'`, ledgerID)
		if cnt != 1 {
			t.Fatalf("账本 %d 权益账户数量期望 1，实际 %d", ledgerID, cnt)
		}
	}
	if got := queryText(t, conn, `SELECT open_date FROM accounts WHERE ledger_id=2 AND name='Equity:Opening-Balances'`); got != "2026-01-01" {
		t.Fatalf("补种权益账户 open_date 期望 2026-01-01，实际 %s", got)
	}

	// F：未关闭账户 open_date 只下调；已关闭账户完全不动（FR-23）。
	if got := queryText(t, conn, `SELECT open_date FROM accounts WHERE id=1`); got != "2025-06-01" {
		t.Fatalf("账户 1 open_date 期望 2025-06-01，实际 %s", got)
	}
	if got := queryText(t, conn, `SELECT open_date FROM accounts WHERE id=2`); got != "2025-06-01" {
		t.Fatalf("账户 2 open_date 期望 2025-06-01，实际 %s", got)
	}
	if got := queryText(t, conn, `SELECT open_date FROM accounts WHERE id=4`); got != "2025-01-01" {
		t.Fatalf("更早开户日不应被上调，实际 %s", got)
	}
	if got := queryText(t, conn, `SELECT open_date FROM accounts WHERE id=3`); got != closedOpenDate {
		t.Fatalf("已关闭账户 open_date 被修改：%s → %s", closedOpenDate, got)
	}
	if got := queryText(t, conn, `SELECT open_date FROM accounts WHERE id=6`); got != equity3OpenDate {
		t.Fatalf("已关闭权益账户 open_date 被修改：%s → %s", equity3OpenDate, got)
	}
	afterClosedTxn := queryTxnMeta(t, conn, "t5")
	if afterClosedTxn != closedTxn {
		t.Fatalf("已关闭账户的期初交易被修改：%+v → %+v", closedTxn, afterClosedTxn)
	}

	// migration_log 的 old/new 与实际变化一致（账户 1、2 各一条）。
	logRows := queryMigrationLog(t, conn, "account")
	if len(logRows) != 2 {
		t.Fatalf("账户迁移日志期望 2 条，实际 %d", len(logRows))
	}
	sort.Slice(logRows, func(i, j int) bool { return logRows[i].oldValue < logRows[j].oldValue })
	for _, r := range logRows {
		if r.oldValue != "2026-11-05" || r.newValue != "2025-06-01" {
			t.Fatalf("迁移日志 old/new 不一致: %+v", r)
		}
	}
}

// TestMigrate012Idempotent 二次执行 012：数据与审计/同步日志均零新增。
func TestMigrate012Idempotent(t *testing.T) {
	conn := openTestDB(t)
	seedMigrate012Fixture(t, conn)
	simulatePre012(t, conn)
	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("首次执行 012: %v", err)
	}

	baseLog := queryInt(t, conn, `SELECT count(*) FROM migration_log`)
	baseSync := queryInt(t, conn, `SELECT count(*) FROM sync_log`)
	baseTxn := queryInt(t, conn, `SELECT count(*) FROM transactions`)
	baseActive := queryInt(t, conn,
		`SELECT count(*) FROM transactions
		 WHERE opening_account_uuid IS NOT NULL AND reversal_of IS NULL
		   AND reversed_by IS NULL AND deleted_at IS NULL`)

	if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE name='012_opening_balance'`); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("二次执行 012: %v", err)
	}

	if got := queryInt(t, conn, `SELECT count(*) FROM migration_log`); got != baseLog {
		t.Fatalf("二次执行 migration_log 行数变化：%d → %d", baseLog, got)
	}
	if got := queryInt(t, conn, `SELECT count(*) FROM sync_log`); got != baseSync {
		t.Fatalf("二次执行 sync_log 行数变化：%d → %d", baseSync, got)
	}
	if got := queryInt(t, conn, `SELECT count(*) FROM transactions`); got != baseTxn {
		t.Fatalf("二次执行交易数变化：%d → %d", baseTxn, got)
	}
	if got := queryInt(t, conn,
		`SELECT count(*) FROM transactions
		 WHERE opening_account_uuid IS NOT NULL AND reversal_of IS NULL
		   AND reversed_by IS NULL AND deleted_at IS NULL`,
	); got != baseActive {
		t.Fatalf("二次执行有效期初数变化：%d → %d", baseActive, got)
	}
}

// TestMigrate012UniqueIndex 部分唯一索引必须能阻止第二条有效期初/重复幂等键。
func TestMigrate012UniqueIndex(t *testing.T) {
	conn := openTestDB(t)
	seedMigrate012Fixture(t, conn)
	simulatePre012(t, conn)
	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("执行 012: %v", err)
	}

	// 账户 acc-cash 已有一条有效期初（t3），再插一条必须被索引拒绝。
	if _, err := conn.Exec(
		`INSERT INTO transactions
		 (ledger_id, uuid, date, flag, description, opening_account_uuid, opening_version)
		 VALUES (1, 'dup-active', '2025-06-01', '*', 'dup', 'acc-cash', 4)`,
	); err == nil {
		t.Fatalf("部分唯一索引未阻止第二条有效期初")
	}

	// opening_key 幂等键同样必须唯一。
	if _, err := conn.Exec(
		`INSERT INTO transactions
		 (ledger_id, uuid, date, flag, description, opening_key, opening_version)
		 VALUES (1, 'dup-key', '2025-06-01', '*', 'dup', 'opening:acc-cash:3', 4)`,
	); err == nil {
		t.Fatalf("opening_key 唯一索引未生效")
	}
}

// ---- 测试辅助 ----

// seedMigrate012Fixture 构造一份「012 之前」的账本数据。
//
// 账本 1：最早交易 2025-06-01、建账日 2025-08-01；含一条正常期初、两条重复期初、
// 一条历史冲正、一条已关闭账户的期初；账户含更早开户日与已关闭账户。
// 账本 2：无交易、无权益账户（验证补种）。
// 账本 3：已有同名但已关闭的权益账户（验证同名复用，不重复插入）。
func seedMigrate012Fixture(t *testing.T, conn *sql.DB) {
	t.Helper()
	stmts := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users(id, username, password_hash, display_name, status)
		  VALUES (1, 'alice', 'x', 'Alice', 'active')`, nil},
		{`INSERT INTO ledgers(id, name, owner_id, default_commodity, created_at, updated_at)
		  VALUES (1, '家庭账本', 1, 'CNY', '2025-08-01 00:00:00', '2025-08-01 00:00:00')`, nil},
		{`INSERT INTO ledgers(id, name, owner_id, default_commodity, created_at, updated_at)
		  VALUES (2, '旅行账本', 1, 'CNY', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`, nil},
		{`INSERT INTO ledgers(id, name, owner_id, default_commodity, created_at, updated_at)
		  VALUES (3, '旧账本', 1, 'CNY', '2026-02-05 00:00:00', '2026-02-05 00:00:00')`, nil},

		{`INSERT INTO accounts(id, ledger_id, uuid, name, display_name, type, open_date, close_date, sort_order)
		  VALUES (1, 1, 'acc-cash', 'Assets:Cash:CNY', '现金', 'Assets', '2026-11-05', NULL, 0)`, nil},
		{`INSERT INTO accounts(id, ledger_id, uuid, name, display_name, type, open_date, close_date, sort_order)
		  VALUES (2, 1, 'acc-equity', 'Equity:Opening-Balances', '期初余额', 'Equity', '2026-11-05', NULL, 0)`, nil},
		{`INSERT INTO accounts(id, ledger_id, uuid, name, display_name, type, open_date, close_date, sort_order)
		  VALUES (3, 1, 'acc-closed', 'Liabilities:CreditCard', '信用卡', 'Liabilities', '2026-11-05', '2026-10-01', 0)`, nil},
		{`INSERT INTO accounts(id, ledger_id, uuid, name, display_name, type, open_date, close_date, sort_order)
		  VALUES (4, 1, 'acc-bank', 'Assets:Bank', '银行', 'Assets', '2025-01-01', NULL, 0)`, nil},
		{`INSERT INTO accounts(id, ledger_id, uuid, name, display_name, type, open_date, close_date, sort_order)
		  VALUES (6, 3, 'acc-equity3', 'Equity:Opening-Balances', '旧权益', 'Equity', '2026-02-01', '2026-03-01', 0)`, nil},

		{`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description, created_at, updated_at)
		  VALUES (1, 1, 't1', '2025-06-01', '*', '期初余额', '2025-06-01 01:00:00', '2025-06-01 01:00:00')`, nil},
		{`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description, created_at, updated_at)
		  VALUES (2, 1, 't2', '2025-06-01', '*', '期初余额', '2025-06-02 01:00:00', '2025-06-02 01:00:00')`, nil},
		{`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description, created_at, updated_at)
		  VALUES (3, 1, 't3', '2025-06-01', '*', '期初余额', '2025-06-03 01:00:00', '2025-06-03 01:00:00')`, nil},
		{`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description, created_at, updated_at)
		  VALUES (4, 1, 't4', '2025-06-01', '*', '冲正: 期初余额', '2025-06-04 01:00:00', '2025-06-04 01:00:00')`, nil},
		{`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description, created_at, updated_at)
		  VALUES (5, 1, 't5', '2025-07-01', '*', '期初余额', '2025-07-01 01:00:00', '2025-07-01 01:00:00')`, nil},

		// t1：账户 1 期初 +1000
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (1, 1, 'CNY', '1000.00', 0)`, nil},
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (1, 2, 'CNY', '-1000.00', 1)`, nil},
		// t2：重复期初 +200
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (2, 1, 'CNY', '200.00', 0)`, nil},
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (2, 2, 'CNY', '-200.00', 1)`, nil},
		// t3：重复期初 +300（id 最大，去重后保留）
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (3, 1, 'CNY', '300.00', 0)`, nil},
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (3, 2, 'CNY', '-300.00', 1)`, nil},
		// t4：t1 的历史冲正
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (4, 1, 'CNY', '-1000.00', 0)`, nil},
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (4, 2, 'CNY', '1000.00', 1)`, nil},
		// t5：已关闭账户的期初（迁移必须零修改）
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (5, 3, 'CNY', '-500.00', 0)`, nil},
		{`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		  VALUES (5, 2, 'CNY', '500.00', 1)`, nil},
	}
	for _, s := range stmts {
		if _, err := conn.Exec(s.query, s.args...); err != nil {
			t.Fatalf("插入测试数据失败: %v\nSQL: %s", err, s.query)
		}
	}
}

// simulatePre012 把已跑完 012 的库还原成「012 之前」的数据形态：
// 删除 012 的索引/审计表与 schema_migrations 记录。
// 新增列无法在 SQLite 中安全删除，保留为 NULL/默认值，与旧库语义一致。
func simulatePre012(t *testing.T, conn *sql.DB) {
	t.Helper()
	for _, idx := range []string{
		"idx_txn_opening_key",
		"idx_txn_opening_active",
		"idx_txn_reversal_of",
	} {
		if _, err := conn.Exec(`DROP INDEX IF EXISTS ` + idx); err != nil {
			t.Fatalf("删除索引 %s: %v", idx, err)
		}
	}
	if _, err := conn.Exec(`DROP TABLE IF EXISTS migration_log`); err != nil {
		t.Fatalf("删除 migration_log: %v", err)
	}
	if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE name='012_opening_balance'`); err != nil {
		t.Fatalf("删除 012 迁移记录: %v", err)
	}
}

type txnMeta struct {
	openingAccount string
	openingKey     string
	reversalOf     string
	reversedBy     string
}

// queryTxnMeta 读取一笔交易的期初/冲正元数据。
func queryTxnMeta(t *testing.T, conn *sql.DB, txnUUID string) txnMeta {
	t.Helper()
	var m txnMeta
	var a, k, ro, rb sql.NullString
	if err := conn.QueryRow(
		`SELECT opening_account_uuid, opening_key, reversal_of, reversed_by
		 FROM transactions WHERE uuid=?`, txnUUID,
	).Scan(&a, &k, &ro, &rb); err != nil {
		t.Fatalf("读取交易 %s 元数据: %v", txnUUID, err)
	}
	m.openingAccount = a.String
	m.openingKey = k.String
	m.reversalOf = ro.String
	m.reversedBy = rb.String
	return m
}

type migrationLogRow struct {
	oldValue, newValue string
}

// queryMigrationLog 返回指定实体类型的迁移日志（old/new）。
func queryMigrationLog(t *testing.T, conn *sql.DB, entityType string) []migrationLogRow {
	t.Helper()
	rows, err := conn.Query(
		`SELECT COALESCE(old_value,''), COALESCE(new_value,'') FROM migration_log
		 WHERE migration='012_opening_balance' AND entity_type=? ORDER BY id`, entityType,
	)
	if err != nil {
		t.Fatalf("读取 migration_log: %v", err)
	}
	defer rows.Close()
	var out []migrationLogRow
	for rows.Next() {
		var oldValue, newValue string
		if err := rows.Scan(&oldValue, &newValue); err != nil {
			t.Fatal(err)
		}
		out = append(out, migrationLogRow{oldValue: oldValue, newValue: newValue})
	}
	return out
}

func queryText(t *testing.T, conn *sql.DB, query string, args ...any) string {
	t.Helper()
	var v sql.NullString
	if err := conn.QueryRow(query, args...).Scan(&v); err != nil {
		t.Fatalf("查询失败: %v\nSQL: %s", err, query)
	}
	return v.String
}

func queryInt(t *testing.T, conn *sql.DB, query string, args ...any) int {
	t.Helper()
	var v int
	if err := conn.QueryRow(query, args...).Scan(&v); err != nil {
		t.Fatalf("查询失败: %v\nSQL: %s", err, query)
	}
	return v
}
