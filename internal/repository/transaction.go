package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"familyledger/internal/domain"
)

// ListTransactions 列出账本内所有未删除交易（含分录），按日期、id 倒序。
func (s *Store) ListTransactions(ledgerID int64) ([]domain.Transaction, error) {
	rows, err := s.db.Query(
		`SELECT id, uuid, date, flag, description FROM transactions
		 WHERE ledger_id=? AND deleted_at IS NULL ORDER BY date DESC, id DESC`,
		ledgerID,
	)
	if err != nil {
		return nil, err
	}
	txns := []domain.Transaction{}
	ids := []int64{}
	for rows.Next() {
		var t domain.Transaction
		if err := rows.Scan(&t.ID, &t.UUID, &t.Date, &t.Flag, &t.Description); err != nil {
			rows.Close()
			return nil, err
		}
		txns = append(txns, t)
		ids = append(ids, t.ID)
	}
	rows.Close()

	if len(ids) == 0 {
		return txns, nil
	}

	query, args, err := sqlArgsIn(ids)
	if err != nil {
		return nil, err
	}
	prows, err := s.db.Query(
		`SELECT p.transaction_id, p.account_id, a.uuid, a.name, p.commodity, CAST(p.amount AS TEXT), p.position
		 FROM postings p JOIN accounts a ON a.id=p.account_id
		 WHERE p.transaction_id `+query+` ORDER BY p.transaction_id, p.position`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer prows.Close()

	byTxn := map[int64][]domain.Posting{}
	for prows.Next() {
		var tid int64
		var p domain.Posting
		if err := prows.Scan(&tid, &p.AccountID, &p.AccountUUID, &p.AccountName, &p.Commodity, &p.Amount, &p.Position); err != nil {
			return nil, err
		}
		byTxn[tid] = append(byTxn[tid], p)
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}
	for i := range txns {
		txns[i].Postings = byTxn[txns[i].ID]
	}
	return txns, nil
}

// TransactionByUUID 按 uuid 查询单笔未删除交易（含分录，引用账户 uuid）。
func (s *Store) TransactionByUUID(ledgerID int64, uuid string) (domain.Transaction, error) {
	var t domain.Transaction
	err := s.db.QueryRow(
		`SELECT id, uuid, date, flag, description FROM transactions
		 WHERE ledger_id=? AND uuid=? AND deleted_at IS NULL`,
		ledgerID, uuid,
	).Scan(&t.ID, &t.UUID, &t.Date, &t.Flag, &t.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Transaction{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Transaction{}, err
	}

	prows, err := s.db.Query(
		`SELECT p.account_id, a.uuid, a.name, p.commodity, CAST(p.amount AS TEXT), p.position
		 FROM postings p JOIN accounts a ON a.id=p.account_id
		 WHERE p.transaction_id=? ORDER BY p.position`, t.ID,
	)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer prows.Close()
	for prows.Next() {
		var p domain.Posting
		if err := prows.Scan(&p.AccountID, &p.AccountUUID, &p.AccountName, &p.Commodity, &p.Amount, &p.Position); err != nil {
			return domain.Transaction{}, err
		}
		t.Postings = append(t.Postings, p)
	}
	return t, prows.Err()
}

// CreateTransaction 在线创建一笔不可变交易：在事务内补建币种、写交易与分录、追加 sync_log。
// 返回新交易 id。uuid 由 service 层预生成（保证全局唯一）。
func (s *Store) CreateTransaction(ledgerID int64, txn domain.Transaction) (int64, error) {
	id, _, err := s.saveTransaction(ledgerID, txn, false)
	return id, err
}

// SavePushTransaction 幂等保存离线推送的交易：uuid 已存在则跳过（created=false）。
func (s *Store) SavePushTransaction(ledgerID int64, txn domain.Transaction) (bool, error) {
	_, created, err := s.saveTransaction(ledgerID, txn, true)
	return created, err
}

// saveTransaction 是 CreateTransaction / SavePushTransaction 的公共实现。
func (s *Store) saveTransaction(ledgerID int64, txn domain.Transaction, idempotent bool) (int64, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()

	if idempotent {
		var n int
		if err := tx.QueryRow(
			`SELECT count(*) FROM transactions WHERE ledger_id=? AND uuid=?`,
			ledgerID, txn.UUID,
		).Scan(&n); err != nil {
			return 0, false, err
		}
		if n > 0 {
			_ = tx.Commit()
			return 0, false, nil // 已存在（幂等）
		}
	}

	for _, p := range txn.Postings {
		if err := ensureCommodityOn(tx, ledgerID, p.Commodity); err != nil {
			return 0, false, err
		}
	}

	var createdBy any
	if txn.CreatedBy != 0 {
		createdBy = txn.CreatedBy
	}
	res, err := tx.Exec(
		`INSERT INTO transactions(ledger_id, uuid, date, flag, description, created_by)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		ledgerID, txn.UUID, txn.Date, txn.Flag, txn.Description, createdBy,
	)
	if err != nil {
		return 0, false, fmt.Errorf("写入交易失败: %w", err)
	}
	txnID, _ := res.LastInsertId()

	for i, p := range txn.Postings {
		if _, err := tx.Exec(
			`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
			 VALUES(?, ?, ?, ?, ?)`,
			txnID, p.AccountID, p.Commodity, p.Amount, i,
		); err != nil {
			return 0, false, fmt.Errorf("写入分录失败(账户 %d): %w", p.AccountID, err)
		}
	}

	if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityTransaction, txn.UUID, domain.OpCreate); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return txnID, true, nil
}

// SoftDeleteTransaction 软删一笔交易并写 sync_log delete。幂等：已删/不存在返回 false。
func (s *Store) SoftDeleteTransaction(ledgerID int64, uuid string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE transactions SET deleted_at=datetime('now')
		 WHERE ledger_id=? AND uuid=? AND deleted_at IS NULL`,
		ledgerID, uuid,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	if _, err := appendSyncLogOn(s.db, ledgerID, domain.EntityTransaction, uuid, domain.OpDelete); err != nil {
		return false, err
	}
	return true, nil
}

// SoftDeleteAccount 软删账户及其引用交易（事务内），并写各自的 delete 事件。
// 幂等：不存在/已删返回 false。
func (s *Store) SoftDeleteAccount(ledgerID int64, uuid string) (bool, error) {
	var aid int64
	err := s.db.QueryRow(
		`SELECT id FROM accounts WHERE ledger_id=? AND uuid=? AND deleted_at IS NULL`,
		ledgerID, uuid,
	).Scan(&aid)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// 先软删引用该账户的交易，RETURNING 收集本次实际软删的交易 uuid。
	rows, err := tx.Query(
		`UPDATE transactions SET deleted_at=datetime('now')
		 WHERE ledger_id=? AND deleted_at IS NULL AND id IN (
			SELECT DISTINCT p.transaction_id FROM postings p
			JOIN accounts a ON a.id=p.account_id
			WHERE a.ledger_id=? AND a.uuid=?)
		 RETURNING uuid`,
		ledgerID, ledgerID, uuid,
	)
	if err != nil {
		return false, err
	}
	txnUUIDs := []string{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			rows.Close()
			return false, err
		}
		txnUUIDs = append(txnUUIDs, u)
	}
	rows.Close()

	if _, err := tx.Exec(
		`UPDATE accounts SET deleted_at=datetime('now') WHERE ledger_id=? AND uuid=? AND deleted_at IS NULL`,
		ledgerID, uuid,
	); err != nil {
		return false, err
	}

	if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityAccount, uuid, domain.OpDelete); err != nil {
		return false, err
	}
	for _, u := range txnUUIDs {
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityTransaction, u, domain.OpDelete); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// sqlArgsIn 为 IN 子句生成占位符与参数。
func sqlArgsIn(ids []int64) (string, []any, error) {
	if len(ids) == 0 {
		return "", nil, errors.New("empty ids")
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return "IN (" + strings.Join(ph, ",") + ")", args, nil
}
