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
