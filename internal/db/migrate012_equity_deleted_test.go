package db

// 回归测试：012 的权益补种必须保证「存在唯一未删除的 Equity:Opening-Balances」。
//
// 当前实现用 count(*)（不带 deleted_at IS NULL）判断同名账户是否已存在，
// 当老库中同名权益账户被软删（002 删除语义）时会误判为「已存在」而跳过补种，
// 迁移后账本没有任何有效权益账户；若该账户仍被老期初交易引用，导出会引用
// 已删除账户，bean-check 直接失败。

import "testing"

func TestMigrate012SeedsEquityAfterSameNameDeleted(t *testing.T) {
	conn := openTestDB(t)
	seedMigrate012Fixture(t, conn)
	simulatePre012(t, conn)

	// 模拟老版本软删了默认期初权益账户。
	if _, err := conn.Exec(
		`UPDATE accounts SET deleted_at='2025-05-01' WHERE id=2`,
	); err != nil {
		t.Fatalf("软删权益账户: %v", err)
	}

	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("执行 012: %v", err)
	}

	active := queryInt(t, conn,
		`SELECT count(*) FROM accounts
		 WHERE ledger_id=1 AND name='Equity:Opening-Balances' AND deleted_at IS NULL`)
	if active != 1 {
		t.Fatalf("BUG: 权益补种未保证唯一未删除账户：期望 1，实际 %d", active)
	}
}
