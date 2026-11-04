package handler

import (
	"net/http"
	"strings"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
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
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return ""
}

// HandleHealth 健康检查（公开）。
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
	writeJSON(w, http.StatusCreated, res)
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
	writeJSON(w, http.StatusOK, res)
}

// HandleLogout 注销当前会话。
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if tok := BearerToken(r); tok != "" {
		_ = h.svc.Logout(tok)
	}
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

// HandleRefresh 用当前 token 换新 token。
func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	tok := BearerToken(r)
	newTok, err := h.svc.Refresh(tok)
	if err != nil {
		writeError(w, err)
		return
	}
	h.svc.AttachSessionMeta(newTok, ClientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{"token": newTok})
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "token": newTok})
}

// HandleListSessions 列出当前用户的登录设备（不含 token 本身）。
func (h *Handler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}
	sessions, err := h.svc.ListSessions(uid, BearerToken(r))
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
	if err := h.svc.RevokeSession(uid, r.PathValue("sid"), BearerToken(r)); err != nil {
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
	n, err := h.svc.RevokeOtherSessions(uid, BearerToken(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "revoked": n})
}
