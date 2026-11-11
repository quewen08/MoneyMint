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

// corsOptions 是 CORS 策略（D-7）。
//
//   - zero value（未配置）→ 仅同源：不输出 Access-Control-Allow-Origin，
//     浏览器同源请求不受影响，跨域脚本拿不到响应；
//   - wildcard=true（LEDGER_CORS_ORIGINS=*）→ 恢复旧行为，但不允许携带凭证；
//   - origins 白名单 → 仅命中的 Origin 回显，并允许凭证（Cookie 认证必需），
//     同时输出 Vary: Origin 防止 CDN/代理把 A 源的响应缓存给 B 源。
type corsOptions struct {
	wildcard bool
	origins  []string
}

// allows 判断来源是否命中白名单（精确匹配 scheme://host[:port]）。
func (o corsOptions) allows(origin string) bool {
	for _, it := range o.origins {
		if it == origin {
			return true
		}
	}
	return false
}

// cors 应用 CORS 策略；OPTIONS 预检直接 204，不进入认证。
//
// 预检必须由最外层处理：浏览器对带 Authorization / X-Ledger-Id / X-CSRF
// 等自定义头的请求会先发 OPTIONS，预检没有 token，若先经过 authMiddleware 会被 401，
// 浏览器随即拒绝发真实请求，前端表现为「无响应」。
func cors(opt corsOptions, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		switch {
		case opt.wildcard:
			// 通配模式不能与凭证共存（浏览器规范），旧版 Bearer 客户端不依赖 Cookie。
			w.Header().Set("Access-Control-Allow-Origin", "*")
		case len(opt.origins) > 0:
			// 无论命中与否都声明 Vary，避免共享缓存串源。
			w.Header().Add("Vary", "Origin")
			if origin != "" && opt.allows(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PATCH,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Ledger-Id, X-CSRF")
		// 导出接口通过响应头附带恒等式提示，跨域部署时浏览器必须被显式允许读取。
		w.Header().Set("Access-Control-Expose-Headers", "X-Ledger-Balanced, X-Ledger-Diffs, Content-Disposition")
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
//   - 其余 /api/ 路由必须携带有效会话（Cookie 优先、Bearer 兜底），否则 401
//   - Cookie 会话的非 GET/HEAD/OPTIONS 请求必须带 X-CSRF 头，缺失 403 csrf_required；
//     显式使用 Bearer 的请求豁免（旧版 Flutter 不在浏览器里受跨站脚本影响）
//   - 处于「必须先改密」状态时，除自服务端点外一律 428
//   - 认证自服务端点（me/logout/change-password/refresh）仅需登录态
//   - 账本管理端点（/api/ledgers）仅需登录态（具体权限在 service 层校验）
//   - 业务端点按 X-Ledger-Id（缺失回退默认账本）解析账本，校验成员身份，
//     viewer 只读（非 GET 一律 403），通过后注入 user id + ledger id + role 到 context
//
// 鉴权范围按 /api/ 前缀划分（而非逐一列白名单）：这样新增前端路由、
// 新增静态资源都无需回来改这里，也避免「忘了加白名单导致页面 401」。
func authMiddleware(h *handler.Handler, svc *service.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path

		// 非 API 路径：静态文件与 SPA 入口，鉴权由前端登录态自行控制。
		if !strings.HasPrefix(reqPath, "/api/") && reqPath != "/api" {
			next.ServeHTTP(w, r)
			return
		}

		// 公开端点必须放在 token 校验之前：注册/登录本身没有 Cookie，
		// 也不应被 CSRF 拦截（此处曾把注册开关查询放在校验之后，永远走不到）。
		if reqPath == "/api/health" || reqPath == "/api/auth/register" || reqPath == "/api/auth/login" {
			next.ServeHTTP(w, r)
			return
		}

		// 注册开关查询（B-4）：注册页在登录前就要用，必须无 token 可访问。
		if reqPath == "/api/settings/registration" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		tok := h.RequestToken(r)
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

		// CSRF（D-7）：只针对 Cookie 会话的写请求。Bearer 请求豁免——
		// 旧版 Flutter 用 localStorage token 走 Authorization，不能被新规则打断；
		// 且跨站表单无法设置 Authorization 头，豁免不降低安全性。
		if csrfRequired(h, r) && strings.TrimSpace(r.Header.Get("X-CSRF")) == "" {
			handler.WriteError(w, domain.Codedf(domain.ErrForbidden, domain.CodeCSRFRequired,
				"缺少 CSRF 头 X-CSRF，写请求被拒绝"))
			return
		}

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

		// 导入端点（M2-e）：按当前用户新建账本，与既有账本上下文无关；
		// 权限即登录态本身。放在 428 闸门之后——导入属于业务写入，
		// 强制改密期间与其他业务一样被挡。
		if strings.HasPrefix(reqPath, "/api/import/") || reqPath == "/api/import" {
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), uid)))
			return
		}

		// 库世代（迁移 014）：只读、不敏感的库标识，任何已登录用户可读；
		// 客户端用它判断「库被 restore」后是否需要清本地全量重拉。
		if strings.HasPrefix(reqPath, "/api/db/") {
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

// csrfRequired 判断本次请求是否必须携带 X-CSRF。
//
// 规则（D-7 定死）：
//   - LEDGER_AUTH_COOKIE=false 时整条 Cookie 通道关闭，不需要 CSRF；
//   - 请求显式带 Authorization: Bearer 时豁免——旧版客户端不受影响，
//     且跨站攻击者无法给表单/图片请求设置该头；
//   - 仅 GET/HEAD/OPTIONS 视为安全方法；其余（POST/PUT/PATCH/DELETE）视为写操作。
func csrfRequired(h *handler.Handler, r *http.Request) bool {
	if !h.AuthOptions().CookieEnabled {
		return false
	}
	if handler.BearerToken(r) != "" {
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
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
