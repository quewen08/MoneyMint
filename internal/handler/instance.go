package handler

import "net/http"

// HandleDBInstance 返回当前数据库世代 UUID。
//
// 任何已登录用户可读（instance_id 是只读、不敏感的库标识，不泄露任何业务数据）。
// 鉴权由全局 authMiddleware 统一拦截未登录请求（返回 401）。
func (h *Handler) HandleDBInstance(w http.ResponseWriter, r *http.Request) {
	id, err := h.svc.GetInstanceID()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"instance_id": id})
}
