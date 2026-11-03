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
			return nil, fmt.Errorf("创建数据库目录 %q 失败: %w\n%s", dir, err, writableHint(dir))
		}
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开 sqlite 失败（%s）: %w\n%s", path, err, writableHint(filepath.Dir(path)))
	}
	// SQLite 单写者模型：限制单连接可显著降低 "database is locked" 概率，
	// 对个人 NAS 低并发场景完全够用。
	conn.SetMaxOpenConns(1)

	// 开启 WAL（更顺滑的读写并发与崩溃恢复）+ 外键约束。
	//
	// 这里是首次真正接触磁盘的操作，目录不可写的问题会在此暴露。
	// 原始错误只有一句 "unable to open database file (14)"，对使用者毫无指引，
	// 而首次 Docker 部署踩这个坑的概率很高，故补上可操作的排查建议。
	if _, err := conn.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("数据库不可写（%s）: %w\n%s", path, err, writableHint(filepath.Dir(path)))
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("启用外键约束失败: %w", err)
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

// writableHint 在数据库无法写入时给出可操作的排查建议。
//
// 典型场景：容器以非 root 用户运行（镜像内 uid 10001），而宿主挂载目录
// 由 docker 或 root 创建、属主不是 10001，于是既建不了库文件，
// 也无法在同目录创建 -wal / -shm 文件。
func writableHint(dir string) string {
	return fmt.Sprintf(`排查建议：
  1. 运行用户必须能写该目录。容器内为非 root 用户（uid 10001），
     宿主机挂载目录需改属主：
         sudo chown -R 10001:10001 %s
  2. SQLite 在 WAL 模式下除主库文件外，还要在同目录创建 -wal / -shm 文件，
     所以「文件可写但目录不可写」同样会失败。
  3. 确认目录不是以只读方式挂载（-v ...:ro，或底层分区为只读）。
  4. 群晖 / 威联通等 NAS：已 chown 仍失败时，检查共享文件夹的 ACL 权限。`, dir)
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
