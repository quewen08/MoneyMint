package handler

import (
	"net/http"
	"strconv"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// requireUserID 从 context 取当前用户 id，未登录返回 false。
func requireUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return 0, false
	}
	return uid, true
}

// requireLedgerID 从 context 取当前账本 id，未解析返回 false。
func requireLedgerID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return 0, false
	}
	return id, true
}

// pathUserID 解析路径参数 {userID}。
func pathUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil {
		writeError(w, domain.Invalidf("无效的成员 id"))
		return 0, false
	}
	return id, true
}

// HandleListMembers 列出账本成员（仅 owner）。
func (h *Handler) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ledgerID, ok := requireLedgerID(w, r)
	if !ok {
		return
	}
	members, err := h.svc.ListMembers(ledgerID, uid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

// HandleAddMember owner 添加成员。
func (h *Handler) HandleAddMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ledgerID, ok := requireLedgerID(w, r)
	if !ok {
		return
	}
	var req struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	memberID, role, err := h.svc.AddMember(ledgerID, uid, req.Username, req.Role)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user_id": memberID, "role": role})
}

// HandleUpdateMember owner 调整成员角色。
func (h *Handler) HandleUpdateMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ledgerID, ok := requireLedgerID(w, r)
	if !ok {
		return
	}
	memberID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.UpdateMemberRole(ledgerID, uid, memberID, req.Role); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user_id": memberID, "role": req.Role})
}

// HandleRemoveMember owner 移除成员。
func (h *Handler) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ledgerID, ok := requireLedgerID(w, r)
	if !ok {
		return
	}
	memberID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(ledgerID, uid, memberID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleResetMemberPassword owner 重置成员密码。
func (h *Handler) HandleResetMemberPassword(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ledgerID, ok := requireLedgerID(w, r)
	if !ok {
		return
	}
	memberID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.ResetMemberPassword(ledgerID, uid, memberID, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
