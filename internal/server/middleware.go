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

// authMiddleware 包裹所有路由：
//   - 非 /api/ 前缀（前端静态资源与 SPA 入口）完全放行，交由 static.go 处理
//   - 放行 /api/health 与公开认证端点（register/login）
//   - 其余 /api/ 路由必须携带有效 Bearer token，否则 401
//   - 处于「必须先改密」状态时，除自服务端点外一律 428
//   - 认证自服务端点（me/logout/change-password/refresh）仅需登录态
//   - 账本管理端点（/api/ledgers）仅需登录态（具体权限在 service 层校验）
//   - 业务端点按 X-Ledger-Id（缺失回退默认账本）解析账本，校验成员身份，
//     viewer 只读（非 GET 一律 403），通过后注入 user id + ledger id + role 到 context
//
// 鉴权范围按 /api/ 前缀划分（而非逐一列白名单）：这样新增前端路由、
// 新增静态资源都无需回来改这里，也避免「忘了加白名单导致页面 401」。
func authMiddleware(svc *service.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path

		// 非 API 路径：静态文件与 SPA 入口，鉴权由前端登录态自行控制。
		if !strings.HasPrefix(reqPath, "/api/") && reqPath != "/api" {
			next.ServeHTTP(w, r)
			return
		}

		if reqPath == "/api/health" || reqPath == "/api/auth/register" || reqPath == "/api/auth/login" {
			next.ServeHTTP(w, r)
			return
		}

		// 注册开关查询（B-4）：注册页在登录前就要用，必须无 token 可访问。
		// 注意必须放在 token 校验之前——放在后面永远走不到（此处曾实测踩坑）。
		if reqPath == "/api/settings/registration" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		tok := handler.BearerToken(r)
		if tok == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}
		sess, err := svc.Authenticate(tok)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "登录已失效，请重新登录"})
			return
		}
		uid := sess.UserID

		// 「登录设备」也是自服务：不涉及账本数据，强制改密期间也允许查看与吊销
		// （用户可能正是发现异常登录才来处理的）。
		if strings.HasPrefix(reqPath, "/api/auth/sessions") {
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), uid)))
			return
		}

		switch reqPath {
		case "/api/auth/me", "/api/auth/logout", "/api/auth/change-password", "/api/auth/refresh":
			svc.TouchSession(tok) // 刷新最近活跃（内部 10 分钟节流）
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), uid)))
			return
		}

		// 强制改密闸门（M3）：密码是别人代设的（预设 owner / 管理员重置 / CLI 重置）
		// 时，除上面四个自服务端点外一律 428，前端据此跳转改密页。
		// 放在自服务放行之后、业务端点之前——改密本身必须能用，业务必须被挡。
		if sess.MustChangePassword {
			writeJSON(w, http.StatusPreconditionRequired, map[string]any{
				"error": "首次登录需要先修改密码",
				"code":  "password_change_required",
			})
			return
		}

		// 账本管理端点：仅需登录态，创建/设置权限由 service 层校验。
		if reqPath == "/api/ledgers" || strings.HasPrefix(reqPath, "/api/ledgers/") {
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), uid)))
			return
		}

		// 备份端点：全库级操作，与具体账本无关（备份包含所有账本），
		// 因此不解析 X-Ledger-Id、不校验账本成员身份；管理员校验在 handler 内完成。
		if strings.HasPrefix(reqPath, "/api/backup/") {
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), uid)))
			return
		}

		// 其余设置写入（B-4）：要求登录态，管理员校验在 handler 内完成。
		if strings.HasPrefix(reqPath, "/api/settings/") {
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
