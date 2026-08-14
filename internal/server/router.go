package server

import (
	"net/http"

	"familyledger/internal/handler"
	"familyledger/internal/service"
)

// newRouter 注册所有路由并包裹中间件。路由路径与 HTTP 方法保持不变，
// 确保前端与既有冒烟脚本无需改动。
func newRouter(h *handler.Handler, svc *service.Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", h.HandleHealth)

	// 认证端点（公开：register/login；其余自校验）
	mux.HandleFunc("/api/auth/register", methodSwitch(map[string]http.HandlerFunc{
		http.MethodPost: h.HandleRegister,
	}))
	mux.HandleFunc("/api/auth/login", methodSwitch(map[string]http.HandlerFunc{
		http.MethodPost: h.HandleLogin,
	}))
	mux.HandleFunc("/api/auth/logout", methodSwitch(map[string]http.HandlerFunc{
		http.MethodPost: h.HandleLogout,
	}))
	mux.HandleFunc("/api/auth/me", methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet: h.HandleMe,
	}))
	mux.HandleFunc("/api/auth/refresh", methodSwitch(map[string]http.HandlerFunc{
		http.MethodPost: h.HandleRefresh,
	}))
	mux.HandleFunc("/api/auth/change-password", methodSwitch(map[string]http.HandlerFunc{
		http.MethodPost: h.HandleChangePassword,
	}))

	// 业务端点
	mux.HandleFunc("/api/accounts", methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet:  h.HandleListAccounts,
		http.MethodPost: h.HandleCreateAccount,
	}))
	mux.HandleFunc("/api/accounts/{uuid}", methodSwitch(map[string]http.HandlerFunc{
		http.MethodDelete: h.HandleDeleteAccount,
	}))
	mux.HandleFunc("/api/transactions", methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet:  h.HandleListTransactions,
		http.MethodPost: h.HandleCreateTransaction,
	}))
	mux.HandleFunc("/api/transactions/{uuid}", methodSwitch(map[string]http.HandlerFunc{
		http.MethodDelete: h.HandleDeleteTransaction,
	}))

	// 账本管理（多账本 P1-C：列表/创建/设置）
	mux.HandleFunc("/api/ledgers", methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet:  h.HandleListLedgers,
		http.MethodPost: h.HandleCreateLedger,
	}))
	mux.HandleFunc("/api/ledgers/{id}", methodSwitch(map[string]http.HandlerFunc{
		http.MethodPatch: h.HandleUpdateLedger,
	}))

	// 成员管理
	mux.HandleFunc("/api/ledger/members", methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet:  h.HandleListMembers,
		http.MethodPost: h.HandleAddMember,
	}))
	mux.HandleFunc("/api/ledger/members/{userID}", methodSwitch(map[string]http.HandlerFunc{
		http.MethodPost:   h.HandleUpdateMember,
		http.MethodDelete: h.HandleRemoveMember,
	}))
	mux.HandleFunc("/api/ledger/members/{userID}/reset-password", methodSwitch(map[string]http.HandlerFunc{
		http.MethodPost: h.HandleResetMemberPassword,
	}))

	// 导出与同步
	mux.HandleFunc("/api/export", h.HandleExport)
	mux.HandleFunc("/api/export/archive", h.HandleExportArchive)
	mux.HandleFunc("/api/sync/pull", h.HandleSyncPull)
	mux.HandleFunc("/api/sync/push", h.HandleSyncPush)

	// cors 必须包在最外层：浏览器对带 Authorization / X-Ledger-Id 等自定义头的
	// 请求会先发 OPTIONS 预检，预检请求没有 token，若先经过 authMiddleware 会被 401
	// 拦截（到不了 cors 的 OPTIONS 处理），导致浏览器拒绝发真实请求、前端表现为「无响应」。
	return cors(authMiddleware(svc, mux))
}

// methodSwitch 按 HTTP 方法分发到对应 handler。
func methodSwitch(table map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h, ok := table[r.Method]; ok {
			h(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
