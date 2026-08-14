package handler

import (
	"net/http"
	"strconv"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
	"familyledger/internal/service"
)

// HandleSyncPull 增量拉取变更（since 水位之后）。
func (h *Handler) HandleSyncPull(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	q := r.URL.Query()
	since, _ := strconv.Atoi(q.Get("since"))
	clientID := q.Get("client_id")

	res, err := h.svc.SyncPull(ledgerID, int64(since), clientID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// HandleSyncPush 逐条幂等提交离线变更。
func (h *Handler) HandleSyncPush(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	var req service.PushRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := h.svc.SyncPush(ledgerID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
