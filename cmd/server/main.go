// 家庭协作记账系统 — 后端入口。
//
// 运行：
//
//	LEDGER_DB=data/ledger.db go run ./cmd/server
//
// 默认监听 :8080（可用 LISTEN 环境变量覆盖）。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"familyledger/internal/backup"
	"familyledger/internal/config"
	"familyledger/internal/server"
)

// 构建信息由 -ldflags -X 注入（见 Dockerfile），本地 go build 时为 unknown。
// 排障时最先要确认的就是「跑的是哪个版本」，所以启动即打印。
var (
	buildDate  = "unknown"
	buildHash  = "unknown"
	gitVersion = "unknown"
)

// shutdownGrace 是优雅停机的最长等待时间。
// 备份任务可能正在执行，需要给它收尾的时间；超过则强制退出。
const shutdownGrace = 15 * time.Second

func main() {
	cfg := config.Load()

	app, err := server.New(cfg)
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	defer app.DB.Close()

	log.Printf("FamBean %s (hash=%s build=%s)", gitVersion, buildHash, buildDate)
	log.Printf("监听 %s | db=%s | web=%s | 备份=%v(hour=%d keep=%d/%d)",
		cfg.ListenAddr, cfg.DBPath, cfg.WebRoot,
		cfg.BackupEnabled && app.Backup != nil,
		cfg.BackupHour, cfg.BackupKeepDaily, cfg.BackupKeepMonthly)

	// 备份调度：启动补跑 + 每日定时。engine 为 nil 时跳过（已降级为无备份模式）。
	var sched *backup.Scheduler
	if app.Backup != nil {
		sched = backup.NewScheduler(app.Backup, cfg.BackupHour, cfg.BackupKeepDaily, cfg.BackupKeepMonthly)
		sched.Start()
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 优雅停机：收到 SIGINT/SIGTERM 后先停止接收新请求，再等备份任务收尾。
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		log.Printf("收到退出信号，开始优雅停机…")

		ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP 停机超时: %v", err)
		}
		if sched != nil {
			sched.Stop()
		}
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("服务异常: %v", err)
	}
	log.Printf("已停止")
}
