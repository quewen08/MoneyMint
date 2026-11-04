package repository

import (
	"database/sql"
	"errors"
	"time"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// CreateSession 为用户写入一条新会话，返回 token。
//
// 设备信息（ip / user_agent）不在这里写：service 层签发 token 时没有请求上下文，
// 由 handler 在响应前调用 AttachSessionMeta 补齐（见 M3 会话列表）。
func (s *Store) CreateSession(userID int64) (string, error) {
	tok, err := auth.GenerateToken()
	if err != nil {
		return "", err
	}
	// 统一用 UTC 存储：last_seen_at 由 datetime('now')（UTC）写入，
	// 若 created_at 用服务器本地时间，同一行里两列时区不一致，
	// 界面就会显示「登录时间晚于最近活跃」这种自相矛盾的数据。
	// 注意必须格式化后传参——直接把 time.Time 交给驱动会写成
	// `2026-11-04 09:02:43.1671709 +0000 UTC`，与 datetime('now') 无法字符串比较。
	now := time.Now().UTC()
	nowStr := now.Format(auth.SQLiteTimeFmt)
	expStr := now.Add(auth.SessionTTL).Format(auth.SQLiteTimeFmt)
	if _, err := s.db.Exec(
		`INSERT INTO sessions(token, user_id, created_at, expires_at, last_seen_at)
		 VALUES(?, ?, ?, ?, ?)`,
		tok, userID, nowStr, expStr, nowStr,
	); err != nil {
		return "", err
	}
	return tok, nil
}

// AttachSessionMeta 为会话补写设备信息（登录/注册/refresh 后由 handler 调用）。
func (s *Store) AttachSessionMeta(token, ip, userAgent string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET ip=?, user_agent=?, last_seen_at=? WHERE token=?`,
		ip, userAgent, time.Now().UTC().Format(auth.SQLiteTimeFmt), token,
	)
	return err
}

// TouchSession 刷新某会话的最近活跃时间（节流由调用方控制，避免每请求写盘）。
func (s *Store) TouchSession(token string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET last_seen_at=? WHERE token=?`,
		time.Now().UTC().Format(auth.SQLiteTimeFmt), token)
	return err
}

// DeleteSession 注销指定 token。
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token=?`, token)
	return err
}

// UserIDByToken 解析 token 对应的用户；无效或已过期返回 domain.ErrUnauthorized。
func (s *Store) UserIDByToken(token string) (int64, error) {
	if token == "" {
		return 0, domain.ErrUnauthorized
	}
	var uid int64
	var exp string
	if err := s.db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE token=?`, token,
	).Scan(&uid, &exp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.ErrUnauthorized
		}
		return 0, err
	}
	if t, err := time.ParseInLocation(auth.SQLiteTimeFmt, exp, time.UTC); err == nil && t.Before(time.Now().UTC()) {
		return 0, domain.ErrUnauthorized
	}
	return uid, nil
}

// ListSessionsByUser 列出某用户全部未过期会话，按最近活跃倒序。
func (s *Store) ListSessionsByUser(userID int64) ([]domain.Session, error) {
	rows, err := s.db.Query(
		`SELECT token, user_id, created_at, expires_at,
		        COALESCE(last_seen_at, ''), COALESCE(user_agent, ''), COALESCE(ip, '')
		 FROM sessions
		 WHERE user_id=? AND expires_at > datetime('now')
		 ORDER BY COALESCE(NULLIF(last_seen_at, ''), created_at) DESC, created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Session
	for rows.Next() {
		var sess domain.Session
		if err := rows.Scan(&sess.Token, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt,
			&sess.LastSeenAt, &sess.UserAgent, &sess.IP); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// DeleteOtherSessions 删除该用户除 keepToken 之外的全部会话，返回删除条数。
func (s *Store) DeleteOtherSessions(userID int64, keepToken string) (int, error) {
	res, err := s.db.Exec(
		`DELETE FROM sessions WHERE user_id=? AND token<>?`, userID, keepToken)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// DeleteSessionsByUser 使某用户全部会话失效（改密/重置后调用）。
func (s *Store) DeleteSessionsByUser(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}
