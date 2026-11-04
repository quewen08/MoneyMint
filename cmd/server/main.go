// 家庭协作记账系统 — 后端入口。
//
// 运行：
//
//	LEDGER_DB=data/ledger.db go run ./cmd/server
//
// 默认监听 :8080（可用 LISTEN 环境变量覆盖）。
//
// 内置 CLI（B-1）：owner 忘记密码时的兜底保险丝——
//
//	./server -reset-password -user alice -new-password newpass123
//
// 直接重置指定用户密码并注销其全部会话（不经 HTTP，无需登录）。
// Docker：docker exec famben /app/server -reset-password -user alice -new-password xxx
// 信任模型：能登录 NAS 服务器 = 有权重置（与能拔硬盘同级，不引入新攻击面）。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"familyledger/internal/auth"
	"familyledger/internal/backup"
	"familyledger/internal/config"
	"familyledger/internal/db"
	"familyledger/internal/repository"
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
	// ---- CLI：密码重置（B-1），先于常规启动解析 ----
	resetUser := flag.String("reset-password", "", "要重置密码的用户名；设置后进入重置模式，不启动 HTTP 服务。用法: server -reset-password alice -new-password xxx")
	resetPass := flag.String("new-password", "", "新密码（至少 6 位）")
	flag.Parse()

	if *resetUser != "" {
		runPasswordReset(*resetUser, *resetPass)
		return
	}

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

// runPasswordReset 执行 CLI 密码重置（B-1）。
// 直连数据库，不经 HTTP；重置成功后注销该用户全部会话。
func runPasswordReset(user, newPass string) {
	if newPass == "" {
		fmt.Fprintln(os.Stderr, "用法: server -reset-password -user <用户名> -new-password <新密码>")
		fmt.Fprintln(os.Stderr, "新密码不能为空（至少 6 位）")
		os.Exit(1)
	}
	if len(newPass) < 6 {
		fmt.Fprintln(os.Stderr, "❌ 新密码至少 6 位")
		os.Exit(1)
	}

	cfg := config.Load()
	conn, err := db.Open(cfg.DBPath, cfg.SchemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	repo := repository.New(conn)
	u, err := repo.UserByUsername(user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 用户 %q 不存在\n", user)
		os.Exit(1)
	}

	hash, err := auth.HashPassword(newPass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 生成密码哈希失败: %v\n", err)
		os.Exit(1)
	}
	ok, err := repo.UpdateUserPassword(u.ID, hash)
	if err != nil || !ok {
		fmt.Fprintf(os.Stderr, "❌ 更新密码失败: %v\n", err)
		os.Exit(1)
	}
	// CLI 重置 = 密码由运维代设，要求用户下次登录先改密（M3）。
	if _, err := repo.SetMustChangePassword(u.ID, true); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 密码已重置，但置「强制改密」标记失败: %v\n", err)
		os.Exit(1)
	}
	if err := repo.DeleteSessionsByUser(u.ID); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 密码已重置，但注销会话失败（请重启服务兜底）: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 已重置用户 %q 的密码，并注销其全部会话。对方下次登录须先设置新密码。\n", user)
}
