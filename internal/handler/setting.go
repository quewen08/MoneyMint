// 注册开关端点（B-4）：
//   GET  /api/settings/registration  公开（注册页需要先知道是否开放）
//   POST /api/settings/registration  仅系统管理员（owner 运行时切换）
package handler

import (
	"log"
	"net/http"

	"familyledger/internal/auth"
)

// HandleGetRegistration 查询注册是否开放（公开端点，不要求登录）。
func (h *Handler) HandleGetRegistration(w http.ResponseWriter, r *http.Request) {
	allowed, err := h.svc.RegistrationAllowed()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询注册开关失败"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": allowed})
}

// HandleSetRegistration 设置注册开关；仅系统管理员。
func (h *Handler) HandleSetRegistration(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	uid, _ := auth.UserID(r.Context())
	if err := h.svc.SetRegistrationAllowed(uid, req.Enabled); err != nil {
		writeError(w, err)
		return
	}
	log.Printf("event=settings.registration user=%d enabled=%v ip=%s", uid, req.Enabled, r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}
