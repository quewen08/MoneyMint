// 家庭协作记账系统 — 后端入口。
//
// 运行：
//
//	LEDGER_DB=data/ledger.db go run ./cmd/server
//
// 默认监听 :8080（可用 LISTEN 环境变量覆盖）。
package main

import (
	"log"
	"net/http"

	"familyledger/internal/config"
	"familyledger/internal/server"
)

func main() {
	cfg := config.Load()

	app, err := server.New(cfg)
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	defer app.DB.Close()

	log.Printf("家庭记账后端监听 %s (db=%s)", cfg.ListenAddr, cfg.DBPath)
	if err := http.ListenAndServe(cfg.ListenAddr, app.Handler); err != nil {
		log.Fatalf("服务异常: %v", err)
	}
}
