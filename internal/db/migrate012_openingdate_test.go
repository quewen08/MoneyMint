package db

// 012 迁移的 opening_date 回填边界补充测试（FR-22）：
// 取 min(最早交易, 建账日, 今日)，三类来源各覆盖一次。

import (
	"testing"
	"time"
)

// TestMigrate012OpeningDateTakesMin 验证建账日早于最早交易、以及建账日在未来两种边界。
func TestMigrate012OpeningDateTakesMin(t *testing.T) {
	conn := openTestDB(t)
	simulatePre012(t, conn)

	stmts := []string{
		`INSERT INTO users(id, username, password_hash, display_name)
		  VALUES (1, 'alice', 'x', 'Alice')`,
		// 账本 4：建账日 2025-01-01，最早交易 2025-05-01 → 应取建账日。
		`INSERT INTO ledgers(id, name, owner_id, created_at, updated_at)
		  VALUES (4, '建账更早', 1, '2025-01-01 00:00:00', '2025-01-01 00:00:00')`,
		// 账本 5：建账日在 2099 年（未来），无交易 → 应被今日截断。
		`INSERT INTO ledgers(id, name, owner_id, created_at, updated_at)
		  VALUES (5, '未来建账', 1, '2099-01-01 00:00:00', '2099-01-01 00:00:00')`,
		`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description)
		  VALUES (401, 4, 'txn-l4', '2025-05-01', '*', '普通交易')`,
	}
	for _, q := range stmts {
		if _, err := conn.Exec(q); err != nil {
			t.Fatalf("插入前置数据失败: %v\nSQL: %s", err, q)
		}
	}

	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("执行 012: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	cases := []struct {
		id   int64
		want string
	}{
		{4, "2025-01-01"}, // 建账日早于最早交易
		{5, today},        // 未来建账日被今日截断
	}
	for _, c := range cases {
		got := queryText(t, conn, `SELECT opening_date FROM ledgers WHERE id=?`, c.id)
		if got != c.want {
			t.Fatalf("账本 %d opening_date 期望 %s，实际 %s", c.id, c.want, got)
		}
	}
}
