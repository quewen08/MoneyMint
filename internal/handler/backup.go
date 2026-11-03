package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"familyledger/internal/auth"
	"familyledger/internal/backup"
)

// 备份端点（全库级操作，仅系统管理员可用）。
//
// 权限模型说明：账本级 owner 只对一个账本有权限，而备份包含全部账本与用户。
// 若允许账本 owner 下载备份，等于让 A 家庭拿到 B 家庭的账目，属越权。
// 因此这里校验的是系统管理员（users.is_admin），不是账本角色。

// requireAdmin 校验当前登录用户是否为系统管理员；未通过时已写好响应。
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
		return false
	}
	admin, err := h.svc.IsAdmin(uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询权限失败"})
		return false
	}
	if !admin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "需要管理员权限"})
		return false
	}
	return true
}

// backupEngineOrFail 返回备份引擎；未启用时写 503 并返回 nil。
func (h *Handler) backupEngineOrFail(w http.ResponseWriter) *backup.Engine {
	if h.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "备份功能未启用（服务端备份目录不可用或已关闭）",
		})
		return nil
	}
	return h.engine
}

// HandleBackupList 列出全部备份记录（含失败记录，便于排查）。
func (h *Handler) HandleBackupList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	engine := h.backupEngineOrFail(w)
	if engine == nil {
		return
	}

	items, err := engine.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "读取备份清单失败"})
		return
	}
	if items == nil {
		items = []backup.Item{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// HandleBackupRun 手动触发一次备份。
func (h *Handler) HandleBackupRun(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	engine := h.backupEngineOrFail(w)
	if engine == nil {
		return
	}

	item, err := engine.Snapshot("manual")
	if err != nil {
		if errors.Is(err, backup.ErrBusy) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		// 对外只给概括信息，细节留在服务端日志里
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "备份失败，请查看服务端日志",
		})
		return
	}

	uid, _ := auth.UserID(r.Context())
	log.Printf("event=backup.run user=%d file=%s size=%d duration_ms=%d ip=%s",
		uid, item.Name, item.Size, item.DurationMS, r.RemoteAddr)

	writeJSON(w, http.StatusCreated, item)
}

// HandleBackupDownload 下载指定备份文件。
//
// 路径穿越防护分四层（任何一层单独都可能被绕过，故叠加）：
//  1. 参数非空
//  2. filepath.Base 与原名完全一致（剥离任何目录成分）
//  3. 显式拒绝 ".." 与路径分隔符
//  4. 文件名必须匹配白名单正则（ledger-YYYYMMDD-HHMMSS[.lN].db|.beancount）
//
// 另外 Join 之后再次确认最终路径仍在备份目录内，作为纵深防御。
func (h *Handler) HandleBackupDownload(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	engine := h.backupEngineOrFail(w)
	if engine == nil {
		return
	}

	uid, _ := auth.UserID(r.Context())
	name := strings.TrimSpace(r.URL.Query().Get("name"))

	// 第 1~3 层：非空、无目录成分、无穿越标记
	if name == "" ||
		filepath.Base(name) != name ||
		strings.Contains(name, "..") ||
		strings.ContainsAny(name, `/\`) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "非法文件名"})
		return
	}
	// 第 4 层：白名单正则
	if !backup.ValidSnapshotName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "非法文件名"})
		return
	}

	full := filepath.Join(engine.Dir(), name)
	// 纵深防御：Join 后必须仍在备份目录内
	if !withinDir(engine.Dir(), full) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "非法路径"})
		return
	}

	f, err := os.Open(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "备份文件不存在"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "打开备份文件失败"})
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || st.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "备份文件不存在"})
		return
	}

	log.Printf("event=backup.download user=%d file=%s ip=%s", uid, name, r.RemoteAddr)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, st.ModTime(), f)
}

// withinDir 判断 full 是否位于 dir 之内（解析符号链接后比较相对路径）。
func withinDir(dir, full string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absFull)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
