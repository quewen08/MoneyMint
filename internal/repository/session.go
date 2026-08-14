package repository

import (
	"database/sql"
	"errors"
	"time"

	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// CreateSession 为用户写入一条新会话，返回 token。
func (s *Store) CreateSession(userID int64) (string, error) {
	tok, err := auth.GenerateToken()
	if err != nil {
		return "", err
	}
	exp := time.Now().Add(auth.SessionTTL).Format(auth.SQLiteTimeFmt)
	if _, err := s.db.Exec(
		`INSERT INTO sessions(token, user_id, expires_at) VALUES(?, ?, ?)`,
		tok, userID, exp,
	); err != nil {
		return "", err
	}
	return tok, nil
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
	if t, err := time.Parse(auth.SQLiteTimeFmt, exp); err == nil && t.Before(time.Now()) {
		return 0, domain.ErrUnauthorized
	}
	return uid, nil
}

// DeleteSessionsByUser 使某用户全部会话失效（改密/重置后调用）。
func (s *Store) DeleteSessionsByUser(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}
