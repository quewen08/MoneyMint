package export

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"familyledger/internal/db"
)

func TestFormatAmount(t *testing.T) {
	cases := []struct {
		in   string
		prec int
		want string
	}{
		{"10", 2, "10.00"},
		{"10.5", 2, "10.50"},
		{"-3.14159", 2, "-3.14"},
		{"0", 2, "0.00"},
		{"1.005", 2, "1.01"}, // big.Rat.FloatString 四舍五入
	}
	for _, c := range cases {
		if got := formatAmount(c.in, c.prec); got != c.want {
			t.Fatalf("formatAmount(%q, %d) = %q, want %q", c.in, c.prec, got, c.want)
		}
	}
	// 非法输入回退原串
	if got := formatAmount("abc", 2); got != "abc" {
		t.Fatalf("formatAmount(abc) = %q, want abc", got)
	}
}

func TestExportLedgerBasic(t *testing.T) {
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// 最小数据：用户 → 账本 → 币种 → 账户 → 交易 → 分录
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	mustExec(`INSERT INTO users(id, username, password_hash, display_name) VALUES(1,'u','x','U')`)
	mustExec(`INSERT INTO ledgers(id, name, owner_id) VALUES(1,'测试账本',1)`)
	mustExec(`INSERT INTO commodities(ledger_id, uuid, symbol, name, precision) VALUES(1,'cmd-1','CNY','人民币',2)`)
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date) VALUES(1,1,'acc-1','Assets:Cash:CNY','Assets','2026-08-01')`)
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date) VALUES(2,1,'acc-2','Expenses:Food','Expenses','2026-08-01')`)
	mustExec(`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description) VALUES(1,1,'txn-1','2026-08-02','*','早餐')`)
	mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position) VALUES(1,1,'CNY','-10.00',0)`)
	mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position) VALUES(1,2,'CNY','10.00',1)`)

	text, err := ExportLedger(conn, 1)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	for _, want := range []string{
		`option "title" "测试账本"`,
		"commodity CNY",
		"2026-08-01 open Assets:Cash:CNY",
		"2026-08-01 open Expenses:Food",
		`2026-08-02 * "早餐"`,
		"Assets:Cash:CNY -10.00 CNY",
		"Expenses:Food 10.00 CNY",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("导出应包含 %q，实际：\n%s", want, text)
		}
	}
}

// TestExportTags 验证交易标签以 `#tag` 语法导出，且导出文件能被 bean-check 校验通过。
func TestExportTags(t *testing.T) {
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	mustExec(`INSERT INTO users(id, username, password_hash, display_name) VALUES(1,'u','x','U')`)
	mustExec(`INSERT INTO ledgers(id, name, owner_id) VALUES(1,'标签账本',1)`)
	mustExec(`INSERT INTO commodities(ledger_id, uuid, symbol, name, precision) VALUES(1,'cmd-1','CNY','人民币',2)`)
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date) VALUES(1,1,'acc-1','Assets:Cash:CNY','Assets','2026-08-01')`)
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date) VALUES(2,1,'acc-2','Expenses:Food','Expenses','2026-08-01')`)
	// 带标签的交易（tags 为 JSON 数组字符串）
	mustExec(`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description, tags) VALUES(1,1,'txn-1','2026-08-02','*','午餐','["餐饮","出差"]')`)
	mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position) VALUES(1,1,'CNY','-38.50',0)`)
	mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position) VALUES(1,2,'CNY','38.50',1)`)

	text, err := ExportLedger(conn, 1)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(text, `2026-08-02 * "午餐"`) {
		t.Fatalf("导出应包含带标签的交易头行，实际：\n%s", text)
	}
	// 标签以 Beancount 元数据形式写出（支持中文），而非 #tag 头缀。
	if !strings.Contains(text, `tags: "餐饮 出差"`) {
		t.Fatalf("导出应将标签写为 tags 元数据，实际：\n%s", text)
	}

	// 若环境中存在 bean-check，则校验导出文件能通过 bean-check（退出码 0）。
	beanFile := filepath.Join(dir, "out.bean")
	if err := os.WriteFile(beanFile, []byte(text), 0o644); err != nil {
		t.Fatalf("write bean: %v", err)
	}
	// 优先用 venv 的 python -m beancount.scripts.check（输出稳定、退出码可靠）；
	// 其次回退 PATH 上的 bean-check 命令。t.TempDir 返回原生 Windows 路径，python 可直接读取。
	for _, cand := range [][]string{
		{filepath.Join("..", "..", ".venv", "Scripts", "python.exe"), "-m", "beancount.scripts.check"},
		{"bean-check"},
	} {
		args := append(append([]string{}, cand[1:]...), beanFile)
		cmd := exec.Command(cand[0], args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if _, ok := err.(*exec.Error); ok {
				t.Logf("bean-check 不可用（%s），跳过校验", cand[0])
				continue
			}
			t.Fatalf("bean-check 校验失败(%v): %v\n输出:\n%s", cand, err, string(out))
		}
		t.Logf("bean-check 通过（%v）", cand)
		break
	}
}
