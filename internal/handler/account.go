package handler

import (
	"net/http"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// HandleListAccounts 列出账户及其余额。?include_closed=1 含已关闭账户。
func (h *Handler) HandleListAccounts(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	includeClosed := r.URL.Query().Has("include_closed")
	accounts, err := h.svc.ListAccounts(ledgerID, includeClosed)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

// HandleCreateAccount 创建账户。
func (h *Handler) HandleCreateAccount(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Type        string `json:"type"`
		OpenDate    string `json:"open_date"`
		Commodity   string `json:"commodity"`
		Icon        string `json:"icon"`
		Color       string `json:"color"`
		ParentUUID  string `json:"parent_uuid"`
		SubType     string `json:"sub_type"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	created, err := h.svc.CreateAccount(ledgerID, req.Name, req.DisplayName, req.Type, req.OpenDate, req.Commodity, req.Icon, req.Color, req.ParentUUID, req.SubType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// HandleDeleteAccount 按 uuid 关闭账户（Beancount close 语义，0.4-A 起）。
// 置 close_date，保留全部交易引用；HTTP 动词仍为 DELETE，语义从软删改为关闭。
func (h *Handler) HandleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteAccount(ledgerID, r.PathValue("uuid")); err != nil {
		if err == domain.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "账户不存在或已关闭"})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
