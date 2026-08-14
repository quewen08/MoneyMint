// Package config 集中管理服务运行配置。
//
// 配置一律从环境变量读取，便于 NAS 部署与本地开发共用同一份二进制，
// 也便于在测试中显式构造 Config 而不依赖真实环境。
package config

import "os"

// Config 是服务启动所需的全部配置。
type Config struct {
	// DBPath 是 SQLite 数据库文件路径（相对路径允许，父目录自动创建）。
	DBPath string
	// ListenAddr 是 HTTP 监听地址，如 ":8080"。
	ListenAddr string
	// SchemaPath 是初始 DDL 文件路径（仅首次建库时使用）。
	SchemaPath string
}

// Load 从环境变量构造 Config，未设置时使用默认值。
//
//	LEDGER_DB     数据库文件（默认 data/ledger.db）
//	LISTEN        监听地址（默认 :8080）
//	LEDGER_SCHEMA 初始 schema 文件（默认 schema.sql）
func Load() Config {
	cfg := Config{
		DBPath:     envOr("LEDGER_DB", "data/ledger.db"),
		ListenAddr: envOr("LISTEN", ":8080"),
		SchemaPath: envOr("LEDGER_SCHEMA", "schema.sql"),
	}
	return cfg
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
