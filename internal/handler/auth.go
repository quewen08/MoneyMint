package handler

import (
	"net/http"
	"strings"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

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
	res, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "token": newTok})
}
