package repository

import (
	"database/sql"
	"errors"

	"familyledger/internal/domain"
)

// CreateUser 创建账号；用户名已存在返回 domain.ErrConflict。
func (s *Store) CreateUser(username, passwordHash, displayName string) (int64, error) {
	exists, err := s.UsernameExists(username)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, domain.ErrConflict
	}
	res, err := s.db.Exec(
		`INSERT INTO users(username, password_hash, display_name) VALUES(?, ?, ?)`,
		username, passwordHash, displayName,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UserByUsername 按用户名查询用户。
func (s *Store) UserByUsername(username string) (domain.User, error) {
	var u domain.User
	err := s.db.QueryRow(
		`SELECT id, username, display_name, password_hash FROM users WHERE username=?`,
		username,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return u, err
}

// UserByID 按 id 查询用户。
func (s *Store) UserByID(id int64) (domain.User, error) {
	var u domain.User
	err := s.db.QueryRow(
		`SELECT id, username, display_name, password_hash FROM users WHERE id=?`,
		id,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return u, err
}

// UpdateUserPassword 更新用户密码哈希；返回是否命中记录。
func (s *Store) UpdateUserPassword(userID int64, hash string) (bool, error) {
	res, err := s.db.Exec(`UPDATE users SET password_hash=? WHERE id=?`, hash, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UsernameExists 判断用户名是否已存在。
func (s *Store) UsernameExists(username string) (bool, error) {
	var n int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM users WHERE username=?`, username,
	).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// LedgerCount 返回账本总数（用于判断是否首个注册用户）。
func (s *Store) LedgerCount() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM ledgers`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CreateLedgerForOwner 事务内创建账本并写入 owner 成员关系（首个注册用户），
// 返回新账本 id（不再硬编码 id=1，由 AUTOINCREMENT 分配）。
func (s *Store) CreateLedgerForOwner(name string, ownerID int64) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO ledgers(name, owner_id) VALUES(?, ?)`,
		name, ownerID,
	)
	if err != nil {
		return 0, err
	}
	ledgerID, _ := res.LastInsertId()
	if _, err := tx.Exec(
		`INSERT INTO ledger_members(ledger_id, user_id, role, invited_by) VALUES(?, ?, 'owner', ?)`,
		ledgerID, ownerID, ownerID,
	); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return ledgerID, nil
}
