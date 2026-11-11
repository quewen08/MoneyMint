package handler

import (
	"net/http"
	"strings"
	"time"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
	"familyledger/internal/service"
)

// ClientIP 提取客户端 IP：优先 X-Forwarded-For 首段（反代场景），
// 否则回退 RemoteAddr 的主机部分。
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return strings.TrimSpace(rip)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}

// BearerToken 从 Authorization 头提取 Bearer token。
//
// D-7 起会话默认走 httpOnly Cookie，本函数仅作两层用途：
// 1) RequestToken 的兜底读取（旧版 /legacy/ 仍用 Bearer）；
// 2) 鉴权中间件判断「本次请求是否显式使用 Bearer」——显式 Bearer 豁免 CSRF。
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return ""
}

// SessionCookieName 是会话 Cookie 名（D-7）。
const SessionCookieName = "fambean_session"

// sessionCookieMaxAge 与会话有效期对齐：30 天（2592000 秒）。
const sessionCookieMaxAge = 2592000

// AuthOptions 是会话凭据的传输选项（D-7）。
//
// 默认值（DefaultAuthOptions）是「兼容期」形态：Cookie 开启、内网 HTTP 不设 Secure、
// 响应体仍返回 token。三项分别对应环境变量 LEDGER_AUTH_COOKIE / LEDGER_COOKIE_SECURE /
// LEDGER_AUTH_EXPOSE_TOKEN，均为服务端回退开关，切换无需重新构建前端。
type AuthOptions struct {
	// CookieEnabled 为 false 时完全退回 Bearer 旧行为：不下发、不读取 Cookie，
	// 鉴权中间件也不要求 X-CSRF（旧客户端不受影响）。
	CookieEnabled bool
	// CookieSecure 控制 Set-Cookie 的 Secure 属性。
	CookieSecure bool
	// ExposeToken 控制登录/刷新/改密响应体是否包含 token 字段。
	ExposeToken bool
}

// DefaultAuthOptions 返回兼容期默认选项（Cookie 开、Secure 关、暴露 token）。
func DefaultAuthOptions() AuthOptions {
	return AuthOptions{CookieEnabled: true, CookieSecure: false, ExposeToken: true}
}

// RequestToken 提取请求会话令牌：Cookie 优先、Authorization: Bearer 兜底。
// 空串表示未登录（调用方不得把空串当作有效会话）。
//
// 该函数只做「取」，不判断 LEDGER_AUTH_COOKIE 开关；需要尊重开关的场景
// 应使用 Handler.RequestToken（关闭 Cookie 时只看 Bearer）。
func RequestToken(r *http.Request) string {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return BearerToken(r)
}

// RequestToken 返回当前请求的会话令牌：按 LEDGER_AUTH_COOKIE 决定是否接受 Cookie，
// Cookie 优先、Bearer 兜底；空串表示未登录。
//
// 关闭 Cookie 时只看 Bearer，连「浏览器残留的旧 Cookie」也不会被接受，
// 保证 LEDGER_AUTH_COOKIE=false 是真正的旧行为回退。
func (h *Handler) RequestToken(r *http.Request) string {
	if h.auth.CookieEnabled {
		return RequestToken(r)
	}
	return BearerToken(r)
}

// setSessionCookie 下发会话 Cookie（D-7）：
// HttpOnly + Path=/ + SameSite=Lax + Max-Age=30 天，不设 Domain；
// Secure 由 LEDGER_COOKIE_SECURE 控制（内网 HTTP 默认关闭）。
func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	if !h.auth.CookieEnabled || token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		Secure:   h.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie 清空会话 Cookie。
//
// MaxAge 取 -1 时 Go 会输出 `Max-Age=0` 且带过期时间，浏览器立即删除该 Cookie；
// 不能写 MaxAge: 0——那表示「不设置 Max-Age」的会话 Cookie，登出后 Cookie 仍在。
func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	if !h.auth.CookieEnabled {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   h.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// HandleHealth 健康检查（公开）。
//
// instance_id（迁移 014 库世代）随健康检查透出：客户端可在不登录的情况下
// 先比对库世代再决定是否全量重拉。该字段是不敏感的库标识（UUID），
// 且健康检查永不失败——读取失败时降级为不带该字段，status 仍为 ok。
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	body := map[string]string{"status": "ok"}
	if id, err := h.svc.GetInstanceID(); err == nil {
		body["instance_id"] = id
	}
	writeJSON(w, http.StatusOK, body)
}

// HandleRegister 注册新用户（公开）。
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := h.svc.Register(req.Username, req.Password, req.DisplayName)
	if err != nil {
		writeError(w, err)
		return
	}
	h.svc.AttachSessionMeta(res.Token, ClientIP(r), r.UserAgent())
	h.setSessionCookie(w, res.Token)
	writeJSON(w, http.StatusCreated, h.authResult(res))
}

// HandleLogin 登录（公开）。
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := h.svc.LoginFrom(req.Username, req.Password, ClientIP(r))
	if err != nil {
		writeError(w, err)
		return
	}
	h.svc.AttachSessionMeta(res.Token, ClientIP(r), r.UserAgent())
	h.setSessionCookie(w, res.Token)
	writeJSON(w, http.StatusOK, h.authResult(res))
}

// authResult 按 LEDGER_AUTH_EXPOSE_TOKEN 决定登录/注册响应是否携带 token。
//
// 兼容期默认保留：/legacy/ 的 Flutter 版本登录后仍要拿 token 走 Bearer。
// 置 false 时清空字段（json tag 为 omitempty），响应只留 user。
func (h *Handler) authResult(res service.AuthResult) service.AuthResult {
	if !h.auth.ExposeToken {
		res.Token = ""
	}
	return res
}

// HandleLogout 注销当前会话，并清空会话 Cookie。
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if tok := h.RequestToken(r); tok != "" {
		_ = h.svc.Logout(tok)
	}
	h.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// HandleMe 返回当前登录用户信息。
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}
	user, err := h.svc.Me(uid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

// HandleRefresh 用当前 token 换新 token（旧 token 立即失效），并写回新 Cookie。
func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	tok := h.RequestToken(r)
	newTok, err := h.svc.Refresh(tok)
	if err != nil {
		writeError(w, err)
		return
	}
	h.svc.AttachSessionMeta(newTok, ClientIP(r), r.UserAgent())
	h.setSessionCookie(w, newTok)
	resp := map[string]any{}
	if h.auth.ExposeToken {
		resp["token"] = newTok
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleChangePassword 凭旧密码修改密码。
func (h *Handler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	newTok, err := h.svc.ChangePassword(uid, req.OldPassword, req.NewPassword)
	if err != nil {
		writeError(w, err)
		return
	}
	h.svc.AttachSessionMeta(newTok, ClientIP(r), r.UserAgent())
	h.setSessionCookie(w, newTok)
	resp := map[string]any{"ok": true}
	if h.auth.ExposeToken {
		resp["token"] = newTok
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleListSessions 列出当前用户的登录设备（不含 token 本身）。
func (h *Handler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}
	sessions, err := h.svc.ListSessions(uid, h.RequestToken(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// HandleRevokeSession 吊销指定设备（按 SID）；吊销当前设备等价于退出登录。
func (h *Handler) HandleRevokeSession(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}
	if err := h.svc.RevokeSession(uid, r.PathValue("sid"), h.RequestToken(r)); err != nil {
		if err == domain.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "会话不存在或已失效"})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleRevokeOtherSessions 退出其他设备（保留当前会话）。
func (h *Handler) HandleRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}
	n, err := h.svc.RevokeOtherSessions(uid, h.RequestToken(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "revoked": n})
}
