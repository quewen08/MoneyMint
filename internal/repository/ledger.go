package repository

import (
	"database/sql"
	"errors"

	"familyledger/internal/domain"
)

// ListLedgersByUser 列出用户所属的全部账本及其角色，按账本 id 排序。
func (s *Store) ListLedgersByUser(userID int64) ([]domain.Ledger, error) {
	rows, err := s.db.Query(
		`SELECT l.id, l.name, l.default_commodity, lm.role
		 FROM ledgers l JOIN ledger_members lm ON lm.ledger_id=l.id
		 WHERE lm.user_id=? ORDER BY l.id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ledgers := []domain.Ledger{}
	for rows.Next() {
		var l domain.Ledger
		var commodity sql.NullString
		if err := rows.Scan(&l.ID, &l.Name, &commodity, &l.Role); err != nil {
			return nil, err
		}
		if commodity.Valid {
			l.DefaultCommodity = commodity.String
		}
		ledgers = append(ledgers, l)
	}
	return ledgers, rows.Err()
}

// LedgerByID 按 id 查询账本基本信息。
func (s *Store) LedgerByID(ledgerID int64) (domain.Ledger, error) {
	var l domain.Ledger
	var commodity sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, default_commodity FROM ledgers WHERE id=?`,
		ledgerID,
	).Scan(&l.ID, &l.Name, &commodity)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Ledger{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Ledger{}, err
	}
	if commodity.Valid {
		l.DefaultCommodity = commodity.String
	}
	return l, nil
}

// CreateLedger 事务内创建账本并让创建者成为 owner，返回新账本 id。
func (s *Store) CreateLedger(ownerID int64, name, defaultCommodity string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO ledgers(name, owner_id, default_commodity) VALUES(?, ?, NULLIF(?,''))`,
		name, ownerID, defaultCommodity,
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

// UpdateLedger 更新账本名称与默认币种；返回是否命中记录。
func (s *Store) UpdateLedger(ledgerID int64, name, defaultCommodity string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE ledgers SET name=?, default_commodity=NULLIF(?,''), updated_at=datetime('now') WHERE id=?`,
		name, defaultCommodity, ledgerID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
