// Package handler 是 HTTP 适配层：解析请求、调用 service、写出响应。
//
// 本层保持「薄」——不含业务规则，只做 JSON 编解码、路径/查询参数提取
// 与错误到 HTTP 状态码的映射。
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"familyledger/internal/backup"
	"familyledger/internal/domain"
	"familyledger/internal/service"
)

// Handler 持有 service 依赖，暴露各 HTTP 端点方法。
type Handler struct {
	svc *service.Service
	// engine 为备份引擎；为 nil 表示备份功能未启用（初始化失败或显式关闭），
	// 此时备份端点统一返回 503，而不是伪装成功。
	engine *backup.Engine
	// auth 是会话凭据的下发/读取选项（D-7）。默认兼容期形态：
	// Cookie 开启、内网 HTTP 不设 Secure、响应体保留 token（旧版 /legacy/ 仍用 Bearer）。
	auth AuthOptions
}

// New 构造 Handler。默认按兼容期（D-7 观察期）配置会话凭据，
// 组合根如需改写，调用 ConfigureAuth。
func New(svc *service.Service, engine *backup.Engine) *Handler {
	return &Handler{svc: svc, engine: engine, auth: DefaultAuthOptions()}
}

// ConfigureAuth 覆盖会话凭据选项（由组合根按环境变量注入）。
func (h *Handler) ConfigureAuth(opts AuthOptions) { h.auth = opts }

// AuthOptions 返回当前会话凭据选项（供鉴权中间件判断 Cookie / CSRF 策略）。
func (h *Handler) AuthOptions() AuthOptions { return h.auth }

// WriteError 把错误按统一契约写出（供 server 中间件复用，避免两套错误格式）。
func WriteError(w http.ResponseWriter, err error) { writeError(w, err) }

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

// writeError 把领域/业务错误映射为 HTTP 状态码并输出结构化 JSON。
// 新业务错误优先携带稳定错误码（code）与可选明细（如 balances）。
func writeError(w http.ResponseWriter, err error) {
	// 限流是独立语义：429 + Retry-After，且要给出「等多久」让前端能显示倒计时。
	var rl *domain.RateLimitError
	if errors.As(err, &rl) {
		secs := int(rl.RetryAfter.Seconds() + 0.999)
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":               fmt.Sprintf("尝试次数过多，请约 %d 分钟后再试", (secs+59)/60),
			"code":                "too_many_attempts",
			"retry_after_seconds": secs,
		})
		return
	}

	// 关闭账户余额非 0：409 + 按币种差额明细（设计 §3.4.6）。
	var bz *domain.BalanceNotZeroError
	if errors.As(err, &bz) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":        bz.Error(),
			"code":         domain.CodeAccountBalanceNotZero,
			"account_uuid": bz.AccountUUID,
			"balances":     bz.Balances,
		})
		return
	}

	// 带错误码的业务错误：code 平铺 details，便于前端直接读取。
	var ce *domain.CodedError
	if errors.As(err, &ce) {
		body := map[string]any{"error": ce.Message}
		if ce.Code != "" {
			body["code"] = ce.Code
		}
		if ce.Details != nil {
			if m, ok := ce.Details.(map[string]any); ok {
				for k, v := range m {
					body[k] = v
				}
			} else {
				body["details"] = ce.Details
			}
		}
		writeJSON(w, statusForCoded(ce), body)
		return
	}

	writeJSON(w, statusOf(err), map[string]string{"error": err.Error()})
}

// statusForCoded 决定 CodedError 的 HTTP 状态：优先哨兵语义，其次错误码。
func statusForCoded(e *domain.CodedError) int {
	if e.Sentinel != nil {
		return statusOf(e)
	}
	switch e.Code {
	case domain.CodeAccountNotFound:
		return http.StatusNotFound
	case domain.CodeAccountBalanceNotZero, domain.CodeAccountClosed, domain.CodeOpeningConflict:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
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
