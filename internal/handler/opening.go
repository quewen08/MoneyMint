package handler

import (
	"net/http"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
	"familyledger/internal/service"
)

// HandleGetOpening 读取账户期初（GET /api/accounts/{uuid}/opening）。
func (h *Handler) HandleGetOpening(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	view, err := h.svc.GetOpening(ledgerID, r.PathValue("uuid"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// HandleSetOpening 设置/修正/清空账户期初（POST /api/accounts/{uuid}/opening）。
// viewer 的只读限制由路由中间件统一拦截（非 GET 一律 403）。
func (h *Handler) HandleSetOpening(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	var req struct {
		Amount    string `json:"amount"`
		Direction string `json:"direction"`
		Commodity string `json:"commodity"`
		Date      string `json:"date"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.svc.SetOpening(ledgerID, r.PathValue("uuid"), service.OpeningInput{
		Amount:    req.Amount,
		Direction: req.Direction,
		Commodity: req.Commodity,
		Date:      req.Date,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
