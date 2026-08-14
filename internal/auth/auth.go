// Package auth 提供与存储无关的认证纯函数：
//   - 密码 bcrypt 加盐哈希（不落明文）
//   - 随机会话 token 生成
//   - 请求链路中的用户身份传递（context）
//
// 会话的持久化（sessions 表 CRUD）下沉到 repository 层，
// 本包不依赖任何数据库连接，便于在多层复用与单测。
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SessionTTL 是会话有效期（30 天）。家庭 NAS 内网场景，足够长。
const SessionTTL = 30 * 24 * time.Hour

// SQLiteTimeFmt 与 SQLite datetime('now') 产出格式一致，
// 会话过期时间据此序列化后写入 sessions.expires_at。
const SQLiteTimeFmt = "2006-01-02 15:04:05"

// HashPassword 使用 bcrypt 对明文密码加盐哈希。
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword 校验明文密码与存储的 bcrypt 哈希是否匹配。
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// GenerateToken 生成 32 字节随机十六进制会话 token。
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ---- 请求上下文中的用户身份 ----

type ctxKey struct{ name string }

var (
	userIDCtxKey   = ctxKey{"uid"}
	ledgerIDCtxKey = ctxKey{"ledger_id"}
	roleCtxKey     = ctxKey{"role"}
)

// WithUser 把已认证用户 id 注入 context。
func WithUser(ctx context.Context, uid int64) context.Context {
	return context.WithValue(ctx, userIDCtxKey, uid)
}

// UserID 从 context 取出用户 id；未登录时 ok=false。
func UserID(ctx context.Context) (int64, bool) {
	v := ctx.Value(userIDCtxKey)
	if v == nil {
		return 0, false
	}
	uid, ok := v.(int64)
	return uid, ok
}

// WithLedger 把账本 id 与用户在该账本的角色注入 context（多账本 P1-C）。
func WithLedger(ctx context.Context, ledgerID int64, role string) context.Context {
	ctx = context.WithValue(ctx, ledgerIDCtxKey, ledgerID)
	ctx = context.WithValue(ctx, roleCtxKey, role)
	return ctx
}

// LedgerID 从 context 取出账本 id；未解析时 ok=false。
func LedgerID(ctx context.Context) (int64, bool) {
	v := ctx.Value(ledgerIDCtxKey)
	if v == nil {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// Role 从 context 取出用户在该账本的角色；未解析时返回空串。
func Role(ctx context.Context) string {
	v := ctx.Value(roleCtxKey)
	if v == nil {
		return ""
	}
	role, _ := v.(string)
	return role
}
