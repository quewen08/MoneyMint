package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
	"familyledger/internal/handler"
	"familyledger/internal/service"
)

// cors 允许前端跨域访问（开发期放开所有来源）。
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PATCH,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Ledger-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authMiddleware 包裹所有业务路由：
//   - 放行 /api/health 与公开认证端点（register/login）
//   - 其余路由必须携带有效 Bearer token，否则 401
//   - 认证自服务端点（me/logout/change-password/refresh）仅需登录态
//   - 账本管理端点（/api/ledgers）仅需登录态（具体权限在 service 层校验）
//   - 业务端点按 X-Ledger-Id（缺失回退默认账本）解析账本，校验成员身份，
//     viewer 只读（非 GET 一律 403），通过后注入 user id + ledger id + role 到 context
func authMiddleware(svc *service.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/api/health" || path == "/api/auth/register" || path == "/api/auth/login" {
			next.ServeHTTP(w, r)
			return
		}

		tok := handler.BearerToken(r)
		if tok == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}
		uid, err := svc.Authenticate(tok)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "登录已失效，请重新登录"})
			return
		}

		switch path {
		case "/api/auth/me", "/api/auth/logout", "/api/auth/change-password", "/api/auth/refresh":
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), uid)))
			return
		}

		// 账本管理端点：仅需登录态，创建/设置权限由 service 层校验。
		if path == "/api/ledgers" || strings.HasPrefix(path, "/api/ledgers/") {
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), uid)))
			return
		}

		// 业务端点：解析账本上下文。
		ledgerID := ledgerIDFromRequest(r)
		role, err := svc.LedgerRole(uid, ledgerID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询账本角色失败"})
			return
		}
		if role == "" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该账本，请等待 owner 邀请"})
			return
		}
		if r.Method != http.MethodGet && role == domain.RoleViewer {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "只读成员无写权限"})
			return
		}

		ctx := auth.WithUser(r.Context(), uid)
		ctx = auth.WithLedger(ctx, ledgerID, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ledgerIDFromRequest 从 X-Ledger-Id 头解析账本 id；缺失或非法时回退默认账本，
// 保证未升级的旧客户端仍可访问首个账本。
func ledgerIDFromRequest(r *http.Request) int64 {
	if v := r.Header.Get("X-Ledger-Id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return domain.DefaultLedgerID
}

// writeJSON 输出 JSON 响应（中间件专用）。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
