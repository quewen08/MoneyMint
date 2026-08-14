package handler

import (
	"archive/zip"
	"bytes"
	"net/http"
	"sort"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// HandleExport 导出当前账本为单文件 Beancount 文本（附件下载）。
func (h *Handler) HandleExport(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	text, err := h.svc.ExportLedger(ledgerID)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="family.beancount"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

// HandleExportArchive 导出当前账本为多文件目录（main.bean + accounts/ + date/），打包为 zip 下载。
func (h *Handler) HandleExportArchive(w http.ResponseWriter, r *http.Request) {
	ledgerID, ok := auth.LedgerID(r.Context())
	if !ok {
		writeError(w, domain.ErrForbidden)
		return
	}
	files, err := h.svc.ExportLedgerFiles(ledgerID)
	if err != nil {
		writeError(w, err)
		return
	}

	// 按路径排序，保证 zip 内文件顺序稳定。
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range paths {
		f, err := zw.Create(p)
		if err != nil {
			writeError(w, err)
			return
		}
		if _, err := f.Write(files[p]); err != nil {
			writeError(w, err)
			return
		}
	}
	if err := zw.Close(); err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="ledger-beancount.zip"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}
