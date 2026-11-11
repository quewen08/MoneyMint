package service

import (
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"familyledger/internal/beancount"
	"familyledger/internal/db"
	"familyledger/internal/export"
	"familyledger/internal/repository"
)

// openTestDB 打开一份临时库（含完整 schema + 全部迁移），供导入测试使用。
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	schemaPath, err := filepath.Abs("../../schema.sql")
	if err != nil {
		t.Fatalf("解析 schema 路径: %v", err)
	}
	conn, err := db.Open(dbPath, schemaPath)
	if err != nil {
		t.Fatalf("打开测试库: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// insertUser 直接写入一个普通用户（绕过注册流程，仅测试用）。返回其 id。
func insertUser(t *testing.T, conn *sql.DB, name string, admin bool) int64 {
	t.Helper()
	res, err := conn.Exec(
		`INSERT INTO users(username, password_hash, display_name, is_admin)
		 VALUES(?, 'x', ?, ?)`, name, name, admin)
	if err != nil {
		t.Fatalf("插入用户 %s: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestImportManualText 验证：解析已知 Beancount 文本 → 预览计数正确 →
// 单一大事务导入新账本，且重复导入自动加序号、不覆盖。
func TestImportManualText(t *testing.T) {
	conn := openTestDB(t)
	owner := insertUser(t, conn, "importer", false)
	store := repository.New(conn)
	svc := New(store)

	const text = `option "title" "导入测试账本"

2024-01-01 commodity CNY

2024-01-01 open Assets:Cash
2024-01-01 open Expenses:Food
2024-01-01 open Income:Salary

2024-02-01 * "工资"
  Assets:Cash 5000 CNY
  Income:Salary -5000 CNY
`

	// 预览
	prev, err := svc.PreviewImport(owner, text)
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if !prev.Valid {
		t.Fatalf("有效文本被误判为无效: %+v", prev.Errors)
	}
	if prev.Accounts != 3 || prev.Transactions != 1 || prev.Commodities != 1 {
		t.Fatalf("预览计数错误: accounts=%d txns=%d commodities=%d",
			prev.Accounts, prev.Transactions, prev.Commodities)
	}
	if prev.LedgerName != "导入测试账本" {
		t.Fatalf("预览账本名应为「导入测试账本」，实际 %q", prev.LedgerName)
	}

	// 执行导入
	res, err := svc.ExecuteImport(owner, text)
	if err != nil {
		t.Fatalf("ExecuteImport: %v", err)
	}
	if res.Accounts != 3 || res.Transactions != 1 {
		t.Fatalf("导入结果计数错误: accounts=%d txns=%d", res.Accounts, res.Transactions)
	}
	assertLedgerCounts(t, conn, res.LedgerID, 3, 1, 2)

	// 重复导入同名 → 自动加序号，且不破坏首个账本
	prev2, err := svc.PreviewImport(owner, text)
	if err != nil {
		t.Fatalf("第二次 PreviewImport: %v", err)
	}
	if prev2.LedgerName != "导入测试账本 (2)" {
		t.Fatalf("重复导入应去重为「导入测试账本 (2)」，实际 %q", prev2.LedgerName)
	}
	res2, err := svc.ExecuteImport(owner, text)
	if err != nil {
		t.Fatalf("第二次 ExecuteImport: %v", err)
	}
	if res2.LedgerName != "导入测试账本 (2)" {
		t.Fatalf("第二次导入账本名应为「导入测试账本 (2)」，实际 %q", res2.LedgerName)
	}
	// 首个账本数据应完好无损
	assertLedgerCounts(t, conn, res.LedgerID, 3, 1, 2)
}

// TestImportRejectsUnsupported 验证含不支持指令（balance / include）时整体拒绝。
func TestImportRejectsUnsupported(t *testing.T) {
	conn := openTestDB(t)
	owner := insertUser(t, conn, "importer", false)
	svc := New(repository.New(conn))

	const text = `2024-01-01 balance Assets:Cash 0 CNY
2024-01-01 open Assets:Cash
`
	prev, err := svc.PreviewImport(owner, text)
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if prev.Valid {
		t.Fatalf("含不支持指令应判定为无效")
	}
	if len(prev.Errors) == 0 {
		t.Fatalf("应收集到解析错误")
	}
	// 执行也应被服务层拒绝（先解析再校验）
	if _, err := svc.ExecuteImport(owner, text); err == nil {
		t.Fatalf("ExecuteImport 应拒绝含不支持语法的文本")
	}
}

// TestImportExportRoundTrip 验证「导出 → 导入 → 再导出」闭环：
// 源账本经 ExportLedger 序列化为 Beancount，再经服务导入为独立新账本，
// 再次导出后解析，账户/交易/价格计数与原库一致；若环境装有 bean-check 则额外校验。
func TestImportExportRoundTrip(t *testing.T) {
	conn := openTestDB(t)
	owner := insertUser(t, conn, "owner", false)
	srcLedger := seedRoundTripLedger(t, conn, owner)

	// 1) 源账本导出
	srcText, err := export.ExportLedger(conn, srcLedger)
	if err != nil {
		t.Fatalf("ExportLedger: %v", err)
	}

	// 2) 解析后导入为新账本（按 owner 维度新建）
	parsed, err := beancount.Parse(srcText)
	if err != nil {
		t.Fatalf("解析源导出文本: %v", err)
	}
	if len(parsed.Errors) > 0 {
		t.Fatalf("源导出文本应可被解析，错误: %+v", parsed.Errors)
	}
	svc := New(repository.New(conn))
	res, err := svc.ExecuteImport(owner, srcText)
	if err != nil {
		t.Fatalf("ExecuteImport: %v", err)
	}
	if res.LedgerName != "RoundTripLedger (2)" {
		t.Fatalf("导入账本名应为「RoundTripLedger (2)」，实际 %q", res.LedgerName)
	}
	assertLedgerCounts(t, conn, res.LedgerID, 4, 1, 2)

	// 3) 新账本再次导出，解析后计数应一致
	outText, err := export.ExportLedger(conn, res.LedgerID)
	if err != nil {
		t.Fatalf("再导出: %v", err)
	}
	round, err := beancount.Parse(outText)
	if err != nil {
		t.Fatalf("解析再导出文本: %v", err)
	}
	if len(round.Errors) > 0 {
		t.Fatalf("再导出文本应可通过解析，错误: %+v", round.Errors)
	}
	if len(round.Accounts) != 4 {
		t.Fatalf("再导出账户数应为 4，实际 %d", len(round.Accounts))
	}
	if len(round.Transactions) != 1 {
		t.Fatalf("再导出交易数应为 1，实际 %d", len(round.Transactions))
	}
	if len(round.Prices) != 1 {
		t.Fatalf("再导出价格数应为 1，实际 %d", len(round.Prices))
	}
	if len(round.Transactions[0].Postings) != 2 {
		t.Fatalf("再导出交易应含 2 条分录，实际 %d", len(round.Transactions[0].Postings))
	}

	// 4) 可选：bean-check 校验再导出文本（环境未安装则跳过）。
	//    本项目 venv 的 bean-check.exe 包装器在本机无法输出，故改用
	//    `python -m beancount.scripts.check`（同为 3.2.3，与导出器契约同版本）。
	cmd := beanCheckCmd(t, writeTemp(t, "roundtrip-*.beancount", outText))
	if cmd == nil {
		t.Logf("环境未安装 bean-check，跳过 bean-check 校验（解析器与导出器单测已覆盖语法）")
		return
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bean-check 失败: %v\n%s", err, string(out))
	}
	t.Logf("bean-check 通过：再导出文本可经 3.2.3 校验")
}

// ---------- 测试辅助 ----------

func assertLedgerCounts(t *testing.T, conn *sql.DB, ledgerID int64, accounts, txns, postings int) {
	t.Helper()
	var gotA, gotT, gotP int
	if err := conn.QueryRow(`SELECT count(*) FROM accounts WHERE ledger_id=? AND deleted_at IS NULL`, ledgerID).Scan(&gotA); err != nil {
		t.Fatalf("统计账户: %v", err)
	}
	if err := conn.QueryRow(`SELECT count(*) FROM transactions WHERE ledger_id=? AND deleted_at IS NULL`, ledgerID).Scan(&gotT); err != nil {
		t.Fatalf("统计交易: %v", err)
	}
	if err := conn.QueryRow(
		`SELECT count(*) FROM postings p JOIN transactions t ON t.id=p.transaction_id WHERE t.ledger_id=?`, ledgerID).Scan(&gotP); err != nil {
		t.Fatalf("统计分录: %v", err)
	}
	if gotA != accounts || gotT != txns || gotP != postings {
		t.Fatalf("账本 %d 计数不符: accounts=%d(期望%d) txns=%d(期望%d) postings=%d(期望%d)",
			ledgerID, gotA, accounts, gotT, txns, gotP, postings)
	}
}

// seedRoundTripLedger 在测试库里造一个有 4 账户 + 1 交易 + 1 价格 + 标签的账本。
func seedRoundTripLedger(t *testing.T, conn *sql.DB, owner int64) int64 {
	t.Helper()
	res, err := conn.Exec(`INSERT INTO ledgers(name, owner_id, default_commodity) VALUES('RoundTripLedger', ?, NULL)`, owner)
	if err != nil {
		t.Fatalf("建账本: %v", err)
	}
	ledgerID, _ := res.LastInsertId()
	if _, err := conn.Exec(
		`INSERT INTO ledger_members(ledger_id, user_id, role, invited_by) VALUES(?, ?, 'owner', ?)`,
		ledgerID, owner, owner); err != nil {
		t.Fatalf("建成员: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO commodities(ledger_id, symbol, name, precision) VALUES(?, 'CNY','人民币',2),(?, 'USD','美元',2)`,
		ledgerID, ledgerID); err != nil {
		t.Fatalf("建币种: %v", err)
	}
	accts := []struct {
		name, typ string
	}{
		{"Assets:Cash", "Assets"},
		{"Expenses:Food", "Expenses"},
		{"Income:Salary", "Income"},
		{"Equity:Opening", "Equity"},
	}
	for _, a := range accts {
		if _, err := conn.Exec(
			`INSERT INTO accounts(ledger_id, uuid, name, type, open_date) VALUES(?, ?, ?, ?, '2024-01-01')`,
			ledgerID, "uuid-"+a.name, a.name, a.typ); err != nil {
			t.Fatalf("建账户 %s: %v", a.name, err)
		}
	}
	if _, err := conn.Exec(
		`INSERT INTO prices(ledger_id, uuid, commodity, currency, date, rate, source) VALUES(?, 'uuid-price1','USD','CNY','2024-01-15','7.2','manual')`,
		ledgerID); err != nil {
		t.Fatalf("建价格: %v", err)
	}
	tags, _ := json.Marshal([]string{"food", "工资"})
	res, err = conn.Exec(
		`INSERT INTO transactions(ledger_id, uuid, date, flag, description, tags, created_by)
		 VALUES(?, 'uuid-txn1','2024-02-01','*','工资',?, ?)`, ledgerID, string(tags), owner)
	if err != nil {
		t.Fatalf("建交易: %v", err)
	}
	txnID, _ := res.LastInsertId()
	postings := [][4]string{
		{"Assets:Cash", "CNY", "5000", "0"},
		{"Income:Salary", "CNY", "-5000", "1"},
	}
	for _, p := range postings {
		acctID := queryAccountID(t, conn, ledgerID, p[0])
		if _, err := conn.Exec(
			`INSERT INTO postings(transaction_id, account_id, commodity, amount, position) VALUES(?, ?, ?, ?, ?)`,
			txnID, acctID, p[1], p[2], p[3]); err != nil {
			t.Fatalf("建分录 %s: %v", p[0], err)
		}
	}
	return ledgerID
}

func queryAccountID(t *testing.T, conn *sql.DB, ledgerID int64, name string) int64 {
	t.Helper()
	var id int64
	if err := conn.QueryRow(`SELECT id FROM accounts WHERE ledger_id=? AND name=?`, ledgerID, name).Scan(&id); err != nil {
		t.Fatalf("查账户 %s: %v", name, err)
	}
	return id
}

func writeTemp(t *testing.T, pattern, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("建临时文件: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("写临时文件: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("关闭临时文件: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// beanCheckCmd 构造一个运行 bean-check 的命令；环境无 bean-check 时返回 nil。
// 优先用项目 venv 的 `python -m beancount.scripts.check`（其 bean-check.exe 包装器
// 在本机不输出，而 python 模块可正常工作），其次回退 PATH 上的 bean-check。
func beanCheckCmd(t *testing.T, file string) *exec.Cmd {
	t.Helper()
	if wd, err := os.Getwd(); err == nil {
		py := filepath.Join(wd, "..", "..", ".venv", "Scripts", "python.exe")
		if _, err := os.Stat(py); err == nil {
			return exec.Command(py, "-m", "beancount.scripts.check", file)
		}
	}
	if _, err := exec.LookPath("bean-check"); err == nil {
		return exec.Command("bean-check", file)
	}
	return nil
}
