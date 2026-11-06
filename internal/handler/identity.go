package handler

import (
	"net/http"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// HandleIdentityCheck 返回账本恒等式检查结果（GET /api/identity/check）。
func (h *Handler) HandleIdentityCheck(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	result, err := h.svc.CheckIdentity(ledgerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
