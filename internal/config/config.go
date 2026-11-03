// Package config 集中管理服务运行配置。
//
// 配置一律从环境变量读取，便于 NAS 部署与本地开发共用同一份二进制，
// 也便于在测试中显式构造 Config 而不依赖真实环境。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 是服务启动所需的全部配置。
type Config struct {
	// DBPath 是 SQLite 数据库文件路径（相对路径允许，父目录自动创建）。
	DBPath string
	// ListenAddr 是 HTTP 监听地址，如 ":8080"。
	ListenAddr string
	// SchemaPath 是初始 DDL 文件路径（仅首次建库时使用）。
	SchemaPath string
	// WebRoot 是前端构建产物目录（flutter build web 的产出）。
	// 为空或目录不存在时不托管前端，服务仍可独立运行（本地开发单独跑 flutter 的场景）。
	WebRoot string

	// ---- 备份（M1）----
	// BackupEnabled 控制是否启用自动定时备份。
	BackupEnabled bool
	// BackupDir 是备份存放目录；为空时取 DBPath 同级的 backups/。
	BackupDir string
	// BackupHour 是每日自动备份触发小时（0-23，按进程时区）。
	BackupHour int
	// BackupKeepDaily 是每日备份保留份数。
	BackupKeepDaily int
	// BackupKeepMonthly 是月度首日备份保留份数。
	BackupKeepMonthly int
}

// Load 从环境变量构造 Config，未设置时使用默认值。
//
//	LEDGER_DB                 数据库文件（默认 data/ledger.db）
//	LISTEN                    监听地址（默认 :8080）
//	LEDGER_SCHEMA             初始 schema 文件（默认 schema.sql）
//	LEDGER_WEB_ROOT           前端产物目录（默认 ./web）
//	LEDGER_BACKUP_ENABLED     自动备份开关（默认 true）
//	LEDGER_BACKUP_DIR         备份目录（默认 {DBPath 目录}/backups）
//	LEDGER_BACKUP_HOUR        每日备份小时（默认 3）
//	LEDGER_BACKUP_KEEP_DAILY  每日备份保留份数（默认 14）
//	LEDGER_BACKUP_KEEP_MONTHLY 月度备份保留份数（默认 12）
func Load() Config {
	return Config{
		DBPath:            envOr("LEDGER_DB", "data/ledger.db"),
		ListenAddr:        envOr("LISTEN", ":8080"),
		SchemaPath:        envOr("LEDGER_SCHEMA", "schema.sql"),
		WebRoot:           envOr("LEDGER_WEB_ROOT", "./web"),
		BackupEnabled:     envBool("LEDGER_BACKUP_ENABLED", true),
		BackupDir:         envOr("LEDGER_BACKUP_DIR", ""),
		BackupHour:        envInt("LEDGER_BACKUP_HOUR", 3),
		BackupKeepDaily:   envInt("LEDGER_BACKUP_KEEP_DAILY", 14),
		BackupKeepMonthly: envInt("LEDGER_BACKUP_KEEP_MONTHLY", 12),
	}
}

// Validate 在启动时集中校验配置，不合法即刻失败（fail fast），
// 避免服务带着非法参数跑起来、在运行中途才暴露问题。
func (c Config) Validate() error {
	if strings.TrimSpace(c.DBPath) == "" {
		return fmt.Errorf("LEDGER_DB 不能为空")
	}
	if strings.TrimSpace(c.ListenAddr) == "" {
		return fmt.Errorf("LISTEN 不能为空")
	}
	if c.BackupHour < 0 || c.BackupHour > 23 {
		return fmt.Errorf("LEDGER_BACKUP_HOUR 必须在 0-23 之间，当前为 %d", c.BackupHour)
	}
	if c.BackupKeepDaily < 1 {
		return fmt.Errorf("LEDGER_BACKUP_KEEP_DAILY 必须 >= 1，当前为 %d", c.BackupKeepDaily)
	}
	if c.BackupKeepMonthly < 1 {
		return fmt.Errorf("LEDGER_BACKUP_KEEP_MONTHLY 必须 >= 1，当前为 %d", c.BackupKeepMonthly)
	}
	return nil
}

// ResolvedBackupDir 返回实际使用的备份目录：未显式配置时放在数据库同级 backups/。
func (c Config) ResolvedBackupDir() string {
	if strings.TrimSpace(c.BackupDir) != "" {
		return c.BackupDir
	}
	dir := "."
	if i := strings.LastIndexAny(c.DBPath, `/\`); i >= 0 {
		dir = c.DBPath[:i]
		if dir == "" {
			dir = string(os.PathSeparator)
		}
	}
	return dir + string(os.PathSeparator) + "backups"
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envBool 解析布尔环境变量，非法值回退默认值（宽松处理，避免一个笔误让服务起不来）。
func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// envInt 解析整型环境变量，非法值回退默认值（由 Validate 兜底检查范围）。
func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
