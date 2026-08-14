// Package handler 是 HTTP 适配层：解析请求、调用 service、写出响应。
//
// 本层保持「薄」——不含业务规则，只做 JSON 编解码、路径/查询参数提取
// 与错误到 HTTP 状态码的映射。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"familyledger/internal/domain"
	"familyledger/internal/service"
)

// Handler 持有 service 依赖，暴露各 HTTP 端点方法。
type Handler struct {
	svc *service.Service
}

// New 构造 Handler。
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// ---------- 响应辅助 ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// writeError 把领域/业务错误映射为 HTTP 状态码并输出 {"error": ...}。
func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, statusOf(err), map[string]string{"error": err.Error()})
}

// statusOf 将哨兵错误映射为 HTTP 状态码，未知错误按 400 处理。
func statusOf(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}
