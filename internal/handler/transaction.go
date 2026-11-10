package handler

import (
	"net/http"
	"strings"

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
		Date        string   `json:"date"`
		Flag        string   `json:"flag"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		// ClientRequestID 是客户端幂等键（可选）。为兼容旧前端，缺省即关闭幂等。
		// 与 Idempotency-Key 头同时存在时以头为准（头是标准位置，body 是历史兼容）。
		ClientRequestID string `json:"client_request_id"`
		Postings        []struct {
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
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = req.ClientRequestID
	}
	created, err := h.svc.CreateTransactionIdempotent(
		ledgerID, idempotencyKey, req.Date, req.Flag, req.Description, postings, createdBy, req.Tags...)
	if err != nil {
		writeError(w, err)
		return
	}
	// 新建 → 201；幂等命中 → 200（响应体结构不变，附 "idempotent": true）。
	status := http.StatusCreated
	if created.Idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, created)
}

// HandleListTrash 列出回收站内的交易（已软删，按删除时间倒序）。
func (h *Handler) HandleListTrash(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	items, err := h.svc.ListDeletedTransactions(ledgerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": items})
}

// HandleRestoreTransaction 从回收站恢复一笔交易。
func (h *Handler) HandleRestoreTransaction(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.RestoreTransaction(ledgerID, r.PathValue("uuid")); err != nil {
		if err == domain.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "交易不存在或未被删除"})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
