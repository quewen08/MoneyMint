package export

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"familyledger/internal/db"
)

// TestExportLedgerFilesStructure 校验多文件目录导出的结构完整性。
//
// 只验结构（文件是否齐全、main.bean 是否 include 了各类文件），
// 语法正确性由 TestExportArchiveBeanCheck 用外部 bean-check 验证。
func TestExportLedgerFilesStructure(t *testing.T) {
	conn := newArchiveFixture(t)
	defer conn.Close()

	files, err := ExportLedgerFiles(conn, 1)
	if err != nil {
		t.Fatalf("ExportLedgerFiles: %v", err)
	}

	// 必备文件：主入口 + 五类账户文件 + 两个月的交易文件 + 两个年度入口
	mustHave := []string{
		"main.bean",
		"accounts/assets.bean",
		"accounts/expenses.bean",
		"date/2026/2026-08.bean",
		"date/2026/2026-09.bean",
		"date/2026/2026.bean",
	}
	for _, name := range mustHave {
		if _, ok := files[name]; !ok {
			keys := make([]string, 0, len(files))
			for k := range files {
				keys = append(keys, k)
			}
			t.Fatalf("缺少文件 %q，实际产出：%v", name, keys)
		}
	}

	// main.bean 必须 include 账户文件与年度入口
	main := string(files["main.bean"])
	for _, want := range []string{
		`include "accounts/assets.bean"`,
		`include "date/2026/2026.bean"`,
	} {
		if !strings.Contains(main, want) {
			t.Fatalf("main.bean 应包含 %q，实际：\n%s", want, main)
		}
	}
}

// TestExportArchiveBeanCheck 用外部 bean-check 校验多文件目录导出。
//
// 这是 M0-a 的核心新增覆盖：此前只有单文件导出被 bean-check 校验过，
// 多文件目录（main.bean + include）从未验证。
//
// 关键点：include 是相对路径，必须在解压根目录执行 bean-check，
// 否则会报「找不到被 include 的文件」——那是测试写法问题，不是导出缺陷。
func TestExportArchiveBeanCheck(t *testing.T) {
	conn := newArchiveFixture(t)
	defer conn.Close()

	files, err := ExportLedgerFiles(conn, 1)
	if err != nil {
		t.Fatalf("ExportLedgerFiles: %v", err)
	}

	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %q: %v", rel, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatalf("write %q: %v", rel, err)
		}
	}

	// main.bean 位于根目录，其 include 相对自身解析，故直接传绝对路径即可。
	runBeanCheck(t, filepath.Join(root, "main.bean"))
}

// newArchiveFixture 构造一个足以生成多文件目录的数据集：
// 跨两个月的交易（产生 date/{year}/{MM}.bean）+ 一个已关闭账户（验证 close 指令）。
func newArchiveFixture(t *testing.T) *sql.DB {
	t.Helper()

	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "archive.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}

	mustExec(`INSERT INTO users(id, username, password_hash, display_name) VALUES(1,'u','x','U')`)
	mustExec(`INSERT INTO ledgers(id, name, owner_id) VALUES(1,'归档账本',1)`)
	mustExec(`INSERT INTO commodities(ledger_id, uuid, symbol, name, precision) VALUES(1,'cmd-1','CNY','人民币',2)`)

	// 三个账户：现金（资产）、餐饮（支出）、已关闭的旧卡（资产）
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date)
	          VALUES(1,1,'acc-1','Assets:Cash:CNY','Assets','2026-08-01')`)
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date)
	          VALUES(2,1,'acc-2','Expenses:Food','Expenses','2026-08-01')`)
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date, close_date)
	          VALUES(3,1,'acc-3','Assets:OldCard','Assets','2026-08-01','2026-09-30')`)

	// 两笔跨月交易
	mustExec(`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description)
	          VALUES(1,1,'txn-1','2026-08-15','*','八月午餐')`)
	mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
	          VALUES(1,1,'CNY','-38.50',0)`)
	mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
	          VALUES(1,2,'CNY','38.50',1)`)

	mustExec(`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description)
	          VALUES(2,1,'txn-2','2026-09-03','*','九月早餐')`)
	mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
	          VALUES(2,1,'CNY','-12.00',0)`)
	mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
	          VALUES(2,2,'CNY','12.00',1)`)

	return conn
}
