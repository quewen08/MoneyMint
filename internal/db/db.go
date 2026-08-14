// Package db 负责 SQLite 连接、Schema 迁移与初始化。
//
// 使用 modernc.org/sqlite（纯 Go、零 CGO），便于在开发机交叉编译到
// ARM/ARM64 架构的 NAS 上部署，无需 C 编译器。
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open 打开（必要时创建）SQLite 数据库，应用初始 schema 与增量迁移。
// path 支持相对路径，父目录自动创建；schemaPath 仅在首次建库时使用。
func Open(path, schemaPath string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite 单写者模型：限制单连接可显著降低 "database is locked" 概率，
	// 对个人 NAS 低并发场景完全够用。
	conn.SetMaxOpenConns(1)

	// 开启 WAL（更顺滑的读写并发与崩溃恢复）+ 外键约束。
	if _, err := conn.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable FK: %w", err)
	}

	if err := Migrate(conn, schemaPath); err != nil {
		conn.Close()
		return nil, err
	}
	if err := ApplyMigrations(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Migrate 幂等地执行初始 schema.sql。仅当核心表不存在时才执行，
// 因此服务重启、或已存在数据库都不会报错。
func Migrate(conn *sql.DB, schemaPath string) error {
	var n int
	if err := conn.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'`,
	).Scan(&n); err != nil {
		return fmt.Errorf("check users table: %w", err)
	}
	if n > 0 {
		return nil
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema %q: %w", schemaPath, err)
	}
	if _, err := conn.Exec(string(schema)); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
