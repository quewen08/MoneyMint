package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// openTestDB 打开一个跑完全部迁移的测试库。
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "test.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// simulateOldDB 把库还原成「008 之前」的状态：清空 is_admin 并删掉迁移记录。
// 这样再跑一次 ApplyMigrations 就等于模拟旧库升级。
func simulateOldDB(t *testing.T, conn *sql.DB) {
	t.Helper()
	if _, err := conn.Exec(`UPDATE users SET is_admin=0`); err != nil {
		t.Fatalf("reset is_admin: %v", err)
	}
	if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE name='008_user_admin'`); err != nil {
		t.Fatalf("delete migration record: %v", err)
	}
}

func insertUser(t *testing.T, conn *sql.DB, id int64, username, status string) {
	t.Helper()
	if _, err := conn.Exec(
		`INSERT INTO users(id, username, password_hash, display_name, status) VALUES(?, ?, 'x', ?, ?)`,
		id, username, username, status,
	); err != nil {
		t.Fatalf("insert user %s: %v", username, err)
	}
}

// TestMigrate008BackfillsAdmin 旧库升级时应自动补选一个管理员。
// 若这条失效，升级后全库没人能执行备份 —— 数据保护形同虚设。
func TestMigrate008BackfillsAdmin(t *testing.T) {
	conn := openTestDB(t)

	insertUser(t, conn, 1, "alice", "active")
	insertUser(t, conn, 2, "bob", "active")
	simulateOldDB(t, conn)

	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var adminID int64
	var cnt int
	if err := conn.QueryRow(`SELECT count(*) FROM users WHERE is_admin=1`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("期望恰好 1 个管理员，实际 %d 个", cnt)
	}
	if err := conn.QueryRow(`SELECT id FROM users WHERE is_admin=1`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if adminID != 1 {
		t.Fatalf("期望 id 最小的用户（1）成为管理员，实际为 %d", adminID)
	}
}

// TestMigrate008SkipsDisabledUser 补选应跳过 disabled 用户。
func TestMigrate008SkipsDisabledUser(t *testing.T) {
	conn := openTestDB(t)

	insertUser(t, conn, 1, "alice", "disabled")
	insertUser(t, conn, 2, "bob", "active")
	simulateOldDB(t, conn)

	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var adminID int64
	if err := conn.QueryRow(`SELECT id FROM users WHERE is_admin=1`).Scan(&adminID); err != nil {
		t.Fatalf("未选出管理员: %v", err)
	}
	if adminID != 2 {
		t.Fatalf("期望跳过 disabled 的 1 号、选 2 号，实际选 %d", adminID)
	}
}

// TestMigrate008NewDBHasNoAdmin 全新库不预置管理员，等首个注册用户自然成为管理员。
func TestMigrate008NewDBHasNoAdmin(t *testing.T) {
	conn := openTestDB(t)

	var cnt int
	if err := conn.QueryRow(`SELECT count(*) FROM users WHERE is_admin=1`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("全新库不应预置管理员，实际 %d 个", cnt)
	}
}

// TestMigrate008Idempotent 重复执行迁移不应报错，也不应改动已有管理员。
func TestMigrate008Idempotent(t *testing.T) {
	conn := openTestDB(t)

	insertUser(t, conn, 1, "alice", "active")
	insertUser(t, conn, 2, "bob", "active")

	// 手动指定 2 号是管理员，再重跑迁移
	if _, err := conn.Exec(`UPDATE users SET is_admin=1 WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE name='008_user_admin'`); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(conn); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var adminID int64
	if err := conn.QueryRow(`SELECT id FROM users WHERE is_admin=1`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if adminID != 2 {
		t.Fatalf("已有管理员时不应重新补选，期望 2，实际 %d", adminID)
	}
}
