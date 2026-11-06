package domain

import (
	"errors"
	"fmt"
)

// 哨兵错误：service 层把数据访问/校验结果归一化为这些错误，
// handler 层据此映射 HTTP 状态码，避免各层直接依赖 sql.ErrNoRows。
var (
	// ErrNotFound 表示目标资源不存在（或已被软删）。
	ErrNotFound = errors.New("资源不存在")
	// ErrConflict 表示违反唯一约束（如用户名已存在、账户重名）。
	ErrConflict = errors.New("资源冲突")
	// ErrUnauthorized 表示未登录或凭证错误。
	ErrUnauthorized = errors.New("未认证或凭证错误")
	// ErrForbidden 表示已登录但无权限（非账本成员 / 非 owner）。
	ErrForbidden = errors.New("无权限")
	// ErrInvalid 表示请求参数/业务规则校验失败。
	ErrInvalid = errors.New("参数或业务规则校验失败")
)

// Invalidf 构造一条携带具体原因的 ErrInvalid（保留 errors.Is(err, ErrInvalid) 语义）。
func Invalidf(format string, args ...any) error {
	return &invalidError{msg: fmt.Sprintf(format, args...)}
}

type invalidError struct{ msg string }

func (e *invalidError) Error() string { return e.msg }
func (e *invalidError) Unwrap() error { return ErrInvalid }

// 稳定错误码（与设计 §16 对齐）。handler 依码表映射 HTTP 状态，
// 前端可据此做结构化处理，避免依赖易变的中文文案。
const (
	CodeAccountBalanceNotZero    = "account_balance_not_zero"
	CodeAccountNotFound          = "account_not_found"
	CodeAccountClosed            = "account_closed"
	CodeAccountOpenDateAfter     = "account_open_date_after_ledger"
	CodeOpeningAmountInvalid     = "opening_amount_invalid"
	CodeOpeningDirectionInvalid  = "opening_direction_invalid"
	CodeOpeningCommodityMismatch = "opening_commodity_mismatch"
	CodeOpeningAccountNotOpenable = "opening_account_not_openable"
	CodeOpeningDateMismatch      = "opening_date_mismatch"
	CodeOpeningConflict          = "opening_conflict"
	CodeOpeningViaAPIOnly        = "opening_via_api_only"
	CodeLedgerOpeningDateInvalid = "ledger_opening_date_invalid"
	CodeLedgerOpeningDateAfterTxn = "ledger_opening_date_after_first_txn"
)

// CodedError 是携带稳定错误码与结构化明细的业务错误。
// Details 若为 map[string]any，handler 会将其键值平铺到响应体顶层
// （如 account_uuid / balances），便于前端直接读取。
type CodedError struct {
	Code     string
	Message  string
	Details  any
	Sentinel error // 可选，保留 errors.Is(err, Sentinel) 语义
}

// Error 实现 error 接口。
func (e *CodedError) Error() string { return e.Message }

// Unwrap 返回可选的哨兵错误。
func (e *CodedError) Unwrap() error { return e.Sentinel }

// Codedf 构造一条带错误码的 CodedError。
func Codedf(sentinel error, code, format string, args ...any) error {
	return &CodedError{Code: code, Message: fmt.Sprintf(format, args...), Sentinel: sentinel}
}

// BalanceDiff 是某币种的账户余额差额（关闭账户校验用）。
type BalanceDiff struct {
	Commodity string `json:"commodity"`
	Amount    string `json:"amount"`
}

// BalanceNotZeroError 表示关闭账户时仍有非 0 余额。
// handler 将其映射为 409 account_balance_not_zero，并输出 balances 明细。
type BalanceNotZeroError struct {
	AccountUUID string
	Balances    []BalanceDiff
}

// Error 实现 error 接口。
func (e *BalanceNotZeroError) Error() string { return "账户余额不为 0，请先转出" }

// Unwrap 保留 ErrConflict 语义（409）。
func (e *BalanceNotZeroError) Unwrap() error { return ErrConflict }
