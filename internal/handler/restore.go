package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"familyledger/internal/auth"
	"familyledger/internal/restore"
)

// HandleBackupRestorePreview 预检上传的备份文件（只读，不改动数据）。
//
// 仅系统管理员可用。请求体为 multipart/form-data，字段名 file。
// 返回 200 + {valid, backup, current, ...}；预检未过时 valid=false 且带原因
// （属预期输入，不视为异常）。任何非 SQLite / 损坏 / 未来版本 / 超大文件都被拒绝。
func (h *Handler) HandleBackupRestorePreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	uid, _ := auth.UserID(r.Context())

	path, cleanup, ok := h.receiveBackupFile(w, r)
	if !ok {
		return
	}
	defer cleanup()

	prev, err := h.svc.PreviewRestore(uid, path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prev)
}

// HandleBackupRestore 执行还原（全库替换）。仅系统管理员可用。
//
// multipart 字段：file=备份文件，confirm=确认词（须为 "RESTORE"）。
// 成功后返回快照文件名与重写后的库世代；客户端须据此提示用户重新登录。
func (h *Handler) HandleBackupRestore(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	uid, _ := auth.UserID(r.Context())

	path, cleanup, ok := h.receiveBackupFile(w, r)
	if !ok {
		return
	}
	defer cleanup()

	confirm := r.FormValue("confirm")
	res, err := h.svc.ExecuteRestore(uid, path, confirm, h.engine)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// receiveBackupFile 从 multipart 请求中收备份文件，落临时文件，返回路径与清理函数。
//
// 临时文件由调用方 defer cleanup() 负责删除。体积由 MaxBytesReader 硬限在
// restore.MaxBackupBytes 附近，杜绝「上传打爆磁盘」（RK-M2-06）。
// 失败时已写好响应，返回值 ok=false，调用方不应再写响应。
func (h *Handler) receiveBackupFile(w http.ResponseWriter, r *http.Request) (string, func(), bool) {
	r.Body = http.MaxBytesReader(w, r.Body, restore.MaxBackupBytes+1<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "上传文件过大或表单解析失败（上限 " + humanMB(restore.MaxBackupBytes) + "）",
		})
		return "", nil, false
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少上传文件字段 file"})
		return "", nil, false
	}
	defer f.Close()

	tmp, err := os.CreateTemp("", "fambean-restore-*.db")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "创建临时文件失败"})
		return "", nil, false
	}
	if _, err := io.Copy(tmp, f); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "写入临时文件失败"})
		return "", nil, false
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "关闭临时文件失败"})
		return "", nil, false
	}

	cleanup := func() { os.Remove(tmp.Name()) }
	return tmp.Name(), cleanup, true
}

// humanMB 把字节数转成友好 MB 文案（仅用于错误提示）。
func humanMB(b int64) string {
	return fmt.Sprintf("%d MB", b>>20)
}
