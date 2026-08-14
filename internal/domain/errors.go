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
