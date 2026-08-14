package handler

import (
	"net/http"
	"strconv"

	"familyledger/internal/domain"
)

// HandleListLedgers 列出当前用户所属的全部账本（含角色）。
func (h *Handler) HandleListLedgers(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ledgers, err := h.svc.ListLedgers(uid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ledgers": ledgers})
}

// HandleCreateLedger 创建新账本（创建者成为 owner）。
func (h *Handler) HandleCreateLedger(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name             string `json:"name"`
		DefaultCommodity string `json:"default_commodity"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	ledger, err := h.svc.CreateLedger(uid, req.Name, req.DefaultCommodity)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ledger)
}

// HandleUpdateLedger 更新账本名称与默认币种（仅 owner）。
func (h *Handler) HandleUpdateLedger(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ledgerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, domain.Invalidf("无效的账本 id"))
		return
	}
	var req struct {
		Name             string `json:"name"`
		DefaultCommodity string `json:"default_commodity"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	ledger, err := h.svc.UpdateLedger(uid, ledgerID, req.Name, req.DefaultCommodity)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ledger)
}
