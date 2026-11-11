package db

import (
	"testing"
)

// TestMigrate014FreshDB 新库首次建库：014 记录一次，db_instance 单行且实例 ID 非空。
func TestMigrate014FreshDB(t *testing.T) {
	conn := openTestDB(t)

	var n int
	if err := conn.QueryRow(
		`SELECT count(*) FROM schema_migrations WHERE name='014_db_instance_id'`,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("新库应记录 014 一次，实际 %d", n)
	}

	id, err := GetInstanceID(conn)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("instance_id 不应为空")
	}

	var rows int
	if err := conn.QueryRow(`SELECT count(*) FROM db_instance`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("db_instance 应仅 1 行（单例行约束），实际 %d", rows)
	}
}

// TestMigrate014Idempotent 重跑迁移不应改变已写入的 instance_id。
func TestMigrate014Idempotent(t *testing.T) {
	conn := openTestDB(t)

	before, err := GetInstanceID(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate014(conn); err != nil {
		t.Fatal(err)
	}
	after, err := GetInstanceID(conn)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("重跑迁移不应改变 instance_id: before=%s after=%s", before, after)
	}
}

// TestRotateInstanceID 旋转后 instance_id 变化且读取即新值。
func TestRotateInstanceID(t *testing.T) {
	conn := openTestDB(t)

	before, err := GetInstanceID(conn)
	if err != nil {
		t.Fatal(err)
	}
	after, err := RotateInstanceID(conn)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("RotateInstanceID 应改变 instance_id")
	}

	got, err := GetInstanceID(conn)
	if err != nil {
		t.Fatal(err)
	}
	if got != after {
		t.Fatalf("旋转后读取应等于新值: got=%s after=%s", got, after)
	}
}
