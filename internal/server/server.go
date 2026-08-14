// Package server 负责依赖装配与 HTTP 路由：把 config → db → repository → service → handler
// 组装成完整的 http.Handler，并包裹 CORS 与鉴权中间件。
//
// 这是组合根（composition root），main 只需加载配置、调用 New、启动监听。
package server

import (
	"database/sql"
	"net/http"

	"familyledger/internal/config"
	"familyledger/internal/db"
	"familyledger/internal/handler"
	"familyledger/internal/repository"
	"familyledger/internal/service"
)

// App 是装配完成的运行时对象：持有数据库连接与 HTTP handler。
type App struct {
	DB      *sql.DB
	Handler http.Handler
}

// New 按分层依赖装配整个后端，返回可启动的 App。
func New(cfg config.Config) (*App, error) {
	conn, err := db.Open(cfg.DBPath, cfg.SchemaPath)
	if err != nil {
		return nil, err
	}
	repo := repository.New(conn)
	svc := service.New(repo)
	h := handler.New(svc)

	return &App{
		DB:      conn,
		Handler: newRouter(h, svc),
	}, nil
}
