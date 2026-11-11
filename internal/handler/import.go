package handler

import (
	"io"
	"net/http"
	"strings"
)

// 导入端点（Beancount → 新账本）。与还原不同：导入是「按当前用户」维度操作，
// 把一份自家导出的 .beancount 文本新建为一个独立账本，属于该用户私有数据，
// 因此用 requireUserID（登录即可），而非 requireAdmin（全局管理员）。
//
// 两种提交形态都支持，方便前端「粘贴文本」与「上传文件」两种交互：
//   - Content-Type: application/json 且体为 {"text": "..."}
//   - multipart/form-data 且字段 file 为 .beancount 文件
//   - 其他：原始文本请求体

// HandleImportPreview 解析 Beancount 文本并给出导入预览（只读，不写库）。
//
// 返回 200 + {valid, title, ledger_name, accounts, transactions, prices,
// commodities, duplicate, errors?}。valid=false 且 errors 非空表示文件含
// 不支持语法，整体拒绝（列出全部错误行号），属预期输入、不视为异常。
func (h *Handler) HandleImportPreview(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	text, ok := h.receiveBeancountText(w, r)
	if !ok {
		return
	}
	prev, err := h.svc.PreviewImport(uid, text)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prev)
}

// HandleImport 执行导入：解析 → 校验 → 单一大事务写入新账本。
//
// 成功后返回 {ledger_id, ledger_name, accounts, transactions}。同名账本自动加
// 序号，绝不覆盖既有数据（RK-M2-05：导入是「新建」，不是「替换」）。
func (h *Handler) HandleImport(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUserID(w, r)
	if !ok {
		return
	}
	text, ok := h.receiveBeancountText(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ExecuteImport(uid, text)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// receiveBeancountText 从 JSON 体 / multipart 文件 / 原始体中取出 Beancount 文本。
// 失败时已写好响应，返回值 ok=false，调用方不应再写响应。
func (h *Handler) receiveBeancountText(w http.ResponseWriter, r *http.Request) (string, bool) {
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "application/json"):
		var body struct {
			Text string `json:"text"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "请求体不是合法 JSON（需 {\"text\": \"...\"}）",
			})
			return "", false
		}
		if strings.TrimSpace(body.Text) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text 不能为空"})
			return "", false
		}
		return body.Text, true
	case strings.HasPrefix(ct, "multipart/form-data"):
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "表单解析失败（上限 32 MB）"})
			return "", false
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少上传文件字段 file"})
			return "", false
		}
		defer f.Close()
		raw, err := io.ReadAll(f)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "读取上传文件失败"})
			return "", false
		}
		if strings.TrimSpace(string(raw)) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "上传文件内容为空"})
			return "", false
		}
		return string(raw), true
	default:
		// 原始文本请求体（如 curl 直接 POST 文件内容）
		raw, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "读取请求体失败"})
			return "", false
		}
		if strings.TrimSpace(string(raw)) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体不能为空"})
			return "", false
		}
		return string(raw), true
	}
}
