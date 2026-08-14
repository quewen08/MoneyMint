package handler

import (
	"net/http"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
	"familyledger/internal/service"
)

// HandleListTransactions 列出交易（含分录）。
func (h *Handler) HandleListTransactions(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	txns, err := h.svc.ListTransactions(ledgerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": txns})
}

// HandleCreateTransaction 创建交易（postings 用 account_id 引用）。
func (h *Handler) HandleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	var req struct {
		Date        string `json:"date"`
		Flag        string `json:"flag"`
		Description string `json:"description"`
		Postings    []struct {
			AccountID int64  `json:"account_id"`
			Commodity string `json:"commodity"`
			Amount    string `json:"amount"`
		} `json:"postings"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	postings := make([]service.PostingInput, 0, len(req.Postings))
	for _, p := range req.Postings {
		postings = append(postings, service.PostingInput{
			AccountID: p.AccountID,
			Commodity: p.Commodity,
			Amount:    p.Amount,
		})
	}
	var createdBy int64
	if uid, ok := auth.UserID(r.Context()); ok {
		createdBy = uid
	}
	created, err := h.svc.CreateTransaction(ledgerID, req.Date, req.Flag, req.Description, postings, createdBy)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// HandleDeleteTransaction 按 uuid 软删一笔交易。
func (h *Handler) HandleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteTransaction(ledgerID, r.PathValue("uuid")); err != nil {
		if err == domain.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "交易不存在或已删除"})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
