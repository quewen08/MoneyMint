// Package server 负责依赖装配与 HTTP 路由：把 config → db → repository → service → handler
// 组装成完整的 http.Handler，并包裹 CORS 与鉴权中间件。
//
// 这是组合根（composition root），main 只需加载配置、调用 New、启动监听。
package server

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"familyledger/internal/backup"
	"familyledger/internal/config"
	"familyledger/internal/db"
	"familyledger/internal/handler"
	"familyledger/internal/repository"
	"familyledger/internal/service"
)

// App 是装配完成的运行时对象：持有数据库连接、HTTP handler 与备份引擎。
type App struct {
	DB      *sql.DB
	Handler http.Handler
	// Backup 为 nil 表示备份功能不可用（初始化失败或未启用）。
	Backup *backup.Engine
}

// New 按分层依赖装配整个后端，返回可启动的 App。
func New(cfg config.Config) (*App, error) {
	// 配置校验前置：非法配置在启动时立即失败，不带病运行。
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}

	conn, err := db.Open(cfg.DBPath, cfg.SchemaPath)
	if err != nil {
		return nil, err
	}
	repo := repository.New(conn)
	svc := service.New(repo)

	// ---- 部署与账号初始化（B-3 / B-4）----
	initDeploymentSettings(repo, svc, cfg, conn)

	// 备份初始化失败不应阻断记账：记账是核心功能，备份是保障功能。
	// 降级为「无备份模式」，并在启动日志中明确告警（不能静默失败）。
	var engine *backup.Engine
	if cfg.BackupEnabled {
		engine, err = backup.NewEngine(conn, cfg.ResolvedBackupDir())
		if err != nil {
			log.Printf("⚠️  备份初始化失败，本次启动不提供备份功能：%v", err)
			engine = nil
		}
	}

	h := handler.New(svc, engine)

	return &App{
		DB:      conn,
		Handler: newRouter(h, svc, cfg.WebRoot),
		Backup:  engine,
	}, nil
}

// initDeploymentSettings 部署与账号初始化（B-3 / B-4，启动时执行一次）：
//  1. 注册开关：app_settings 无该键时写入 env 初始值（此后以持久化值为准，env 不再覆盖）
//  2. 预设 owner：LEDGER_OWNER_USER/PASSWORD 均设置且库中无用户时，
//     经 service.Register 创建（自动成为系统管理员 + 建默认账本 + 种子分类），
//     免去 NAS 部署后"抢注首账号"的步骤
//
// 任一步失败只记日志不阻断启动（记账优先），并提示可能的影响。
func initDeploymentSettings(repo *repository.Store, svc *service.Service, cfg config.Config, conn *sql.DB) {
	// 1) 注册开关初始值
	if _, ok, err := repo.GetSetting(repository.SettingRegistration); err != nil {
		log.Printf("⚠️ 读取注册开关失败: %v", err)
	} else if !ok {
		v := "0"
		if cfg.RegistrationEnabled {
			v = "1"
		}
		if err := repo.SetSetting(repository.SettingRegistration, v); err != nil {
			log.Printf("⚠️ 写入注册开关初始值失败: %v", err)
		} else {
			log.Printf("注册开关初始值: %s（来源 LEDGER_REGISTRATION_ENABLED，之后可在界面修改）", map[string]string{"1": "开放", "0": "关闭"}[v])
		}
	}

	// 2) 预设 owner
	if cfg.OwnerUser == "" {
		return
	}
	cnt, err := repo.UserCount()
	if err != nil {
		log.Printf("⚠️ 查询用户数失败，跳过预设 owner: %v", err)
		return
	}
	if cnt > 0 {
		log.Printf("库中已有用户，跳过 LEDGER_OWNER_USER 预设")
		return
	}
	if _, err := svc.Register(cfg.OwnerUser, cfg.OwnerPassword, cfg.OwnerUser); err != nil {
		log.Printf("⚠️ 预设 owner 失败（用户名冲突或密码不合规）: %v", err)
		return
	}
	log.Printf("✅ 已按环境变量预设 owner 账号 %q（系统管理员 + 默认账本）；密码来自 LEDGER_OWNER_PASSWORD，未写入日志", cfg.OwnerUser)
}
