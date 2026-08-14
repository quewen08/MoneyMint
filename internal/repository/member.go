package repository

import (
	"database/sql"
	"errors"

	"familyledger/internal/domain"
)

// ListMembers 列出账本成员及角色，按角色、用户名排序。
func (s *Store) ListMembers(ledgerID int64) ([]domain.Member, error) {
	rows, err := s.db.Query(
		`SELECT lm.user_id, u.username, u.display_name, lm.role
		 FROM ledger_members lm JOIN users u ON u.id=lm.user_id
		 WHERE lm.ledger_id=?
		 ORDER BY CASE lm.role WHEN 'owner' THEN 0 WHEN 'editor' THEN 1 ELSE 2 END, u.username`,
		ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []domain.Member{}
	for rows.Next() {
		var m domain.Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.DisplayName, &m.Role); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// AddMember 把用户加入账本（已存在则更新角色）。
func (s *Store) AddMember(ledgerID, userID int64, role string, invitedBy int64) error {
	_, err := s.db.Exec(
		`INSERT INTO ledger_members(ledger_id, user_id, role, invited_by)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(ledger_id, user_id) DO UPDATE SET role=excluded.role`,
		ledgerID, userID, role, invitedBy,
	)
	return err
}

// UpdateMemberRole 调整成员角色；返回是否命中记录。
func (s *Store) UpdateMemberRole(ledgerID, userID int64, role string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE ledger_members SET role=? WHERE ledger_id=? AND user_id=?`,
		role, ledgerID, userID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RemoveMember 移除成员；返回是否命中记录。
func (s *Store) RemoveMember(ledgerID, userID int64) (bool, error) {
	res, err := s.db.Exec(
		`DELETE FROM ledger_members WHERE ledger_id=? AND user_id=?`,
		ledgerID, userID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// IsMember 判断用户是否为账本成员。
func (s *Store) IsMember(ledgerID, userID int64) (bool, error) {
	var n int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM ledger_members WHERE ledger_id=? AND user_id=?`,
		ledgerID, userID,
	).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// RoleOf 返回用户在账本中的角色；未加入返回空串。
func (s *Store) RoleOf(ledgerID, userID int64) (string, error) {
	var role string
	err := s.db.QueryRow(
		`SELECT role FROM ledger_members WHERE ledger_id=? AND user_id=?`,
		ledgerID, userID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}
