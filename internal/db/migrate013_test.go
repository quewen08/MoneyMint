package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// TestMigrate013FreshDB 新库首次建库：013 记录一次，两列与部分唯一索引就绪。
func TestMigrate013FreshDB(t *testing.T) {
	conn := openTestDB(t)

	var n int
	if err := conn.QueryRow(
		`SELECT count(*) FROM schema_migrations WHERE name='013_txn_idempotency'`,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("新库应记录 013 一次，实际 %d", n)
	}

	for _, col := range []string{"client_request_id", "client_payload_hash"} {
		ok, err := columnExists(conn, "transactions", col)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("transactions 缺少列 %s", col)
		}
	}

	if c := queryInt(t, conn, `SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_txn_client_request'`); c != 1 {
		t.Fatalf("幂等键唯一索引不存在")
	}

	// 重复执行必须幂等：不报错、记录仍为 1。
	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("重复执行迁移应幂等，实际报错: %v", err)
	}
	if n := queryInt(t, conn, `SELECT count(*) FROM schema_migrations WHERE name='013_txn_idempotency'`); n != 1 {
		t.Fatalf("重复执行后 013 记录数应为 1，实际 %d", n)
	}
}

// TestMigrate013LegacyDB 模拟「只有初始 schema、未跑任何迁移」的旧库：
// 013 需要自行 ALTER TABLE 加列并建部分唯一索引，且可安全重跑。
func TestMigrate013LegacyDB(t *testing.T) {
	dir := t.TempDir()
	conn, err := sql.Open("sqlite", filepath.Join(dir, "legacy.db"))
	if err != nil {
		t.Fatalf("打开旧库: %v", err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })

	schema, err := os.ReadFile(filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("读取 schema.sql: %v", err)
	}
	if _, err := conn.Exec(string(schema)); err != nil {
		t.Fatalf("写入初始 schema: %v", err)
	}
	// 迁移前两列不存在（确认真的是旧库）。
	for _, col := range []string{"client_request_id", "client_payload_hash"} {
		ok, err := columnExists(conn, "transactions", col)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatalf("旧库不应已有列 %s", col)
		}
	}

	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("旧库升级失败: %v", err)
	}
	for _, col := range []string{"client_request_id", "client_payload_hash"} {
		ok, err := columnExists(conn, "transactions", col)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("升级后仍缺少列 %s", col)
		}
	}
	if c := queryInt(t, conn, `SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_txn_client_request'`); c != 1 {
		t.Fatalf("升级后幂等键唯一索引不存在")
	}

	// 再次执行（模拟重启 / 重复部署）不得报错。
	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("幂等重跑失败: %v", err)
	}
}

// TestMigration013UniqueIndex 校验部分唯一索引语义：
// 同账本同键只能一条、不同账本可重复、NULL 键（旧数据/旧客户端）不受约束。
func TestMigration013UniqueIndex(t *testing.T) {
	conn := openTestDB(t)
	// 本用例只验证部分唯一索引，不依赖账本/用户外键数据。
	if _, err := conn.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatalf("关闭外键: %v", err)
	}

	insert := func(uuid string, ledgerID int64, key any) error {
		_, err := conn.Exec(
			`INSERT INTO transactions(ledger_id, uuid, date, flag, description, client_request_id,
			        client_payload_hash)
			 VALUES(?, ?, '2026-08-01', '*', 'test', ?, ?)`,
			ledgerID, uuid, key, "hash",
		)
		return err
	}

	if err := insert("u1", 1, "key-1"); err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	if err := insert("u2", 1, "key-1"); err == nil {
		t.Fatalf("同账本同幂等键应被唯一索引拒绝")
	}
	if err := insert("u3", 2, "key-1"); err != nil {
		t.Fatalf("不同账本同幂等键应允许: %v", err)
	}
	if err := insert("u4", 1, nil); err != nil {
		t.Fatalf("NULL 幂等键应允许: %v", err)
	}
	if err := insert("u5", 1, nil); err != nil {
		t.Fatalf("多条 NULL 幂等键应允许: %v", err)
	}
	if n := queryInt(t, conn, `SELECT count(*) FROM transactions WHERE client_request_id='key-1'`); n != 2 {
		t.Fatalf("key-1 应有 2 条（两个账本各一条），实际 %d", n)
	}
}
