package handler

import (
	"net/http"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// HandleListAccounts 列出账户及其余额。
func (h *Handler) HandleListAccounts(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	accounts, err := h.svc.ListAccounts(ledgerID)
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
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	created, err := h.svc.CreateAccount(ledgerID, req.Name, req.DisplayName, req.Type, req.OpenDate, req.Commodity)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// HandleDeleteAccount 按 uuid 软删账户（连带其引用交易）。
func (h *Handler) HandleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteAccount(ledgerID, r.PathValue("uuid")); err != nil {
		if err == domain.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "账户不存在或已删除"})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
