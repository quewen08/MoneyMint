package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"familyledger/internal/domain"
)

// ListTransactions 列出账本内所有未删除交易（含分录），按日期、id 倒序。
func (s *Store) ListTransactions(ledgerID int64) ([]domain.Transaction, error) {
	return s.listTransactions(ledgerID, false)
}

// ListDeletedTransactions 列出回收站内的交易（已软删），按删除时间、id 倒序。
//
// 与 ListTransactions 共用同一套装载逻辑，只是筛选条件相反——回收站不引入新表，
// 依赖 002_delete_semantics 引入的 deleted_at。
func (s *Store) ListDeletedTransactions(ledgerID int64) ([]domain.Transaction, error) {
	return s.listTransactions(ledgerID, true)
}

// listTransactions 是两条列表查询的共同实现（deleted=false 取未删、true 取已删）。
func (s *Store) listTransactions(ledgerID int64, deleted bool) ([]domain.Transaction, error) {
	where := "t.deleted_at IS NULL"
	order := "t.date DESC, t.id DESC"
	if deleted {
		where = "t.deleted_at IS NOT NULL"
		order = "t.deleted_at DESC, t.id DESC"
	}
	rows, err := s.db.Query(
		`SELECT t.id, t.uuid, t.date, t.flag, t.description, t.tags, t.created_by,
		        u.display_name AS created_by_name, t.deleted_at
		 FROM transactions t
		 LEFT JOIN users u ON t.created_by = u.id
		 WHERE t.ledger_id=? AND `+where+` ORDER BY `+order,
		ledgerID,
	)
	if err != nil {
		return nil, err
	}
	txns := []domain.Transaction{}
	ids := []int64{}
	for rows.Next() {
		var t domain.Transaction
		var tags sql.NullString
		var deletedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.UUID, &t.Date, &t.Flag, &t.Description, &tags, &t.CreatedBy, &t.CreatedByName, &deletedAt); err != nil {
			rows.Close()
			return nil, err
		}
		t.Tags = parseTags(tags)
		if deletedAt.Valid {
			t.DeletedAt = deletedAt.String
		}
		txns = append(txns, t)
		ids = append(ids, t.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return txns, nil
	}

	query, args, err := sqlArgsIn(ids)
	if err != nil {
		return nil, err
	}
	prows, err := s.db.Query(
		`SELECT p.transaction_id, p.account_id, a.uuid, a.name,
		        COALESCE(NULLIF(a.display_name, ''), a.name) AS display,
		        p.commodity, CAST(p.amount AS TEXT), p.position
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
		if err := prows.Scan(&tid, &p.AccountID, &p.AccountUUID, &p.AccountName, &p.AccountDisplay, &p.Commodity, &p.Amount, &p.Position); err != nil {
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
	var tags sql.NullString
	err := s.db.QueryRow(
		`SELECT t.id, t.uuid, t.date, t.flag, t.description, t.tags, t.created_by,
		        u.display_name AS created_by_name
		 FROM transactions t
		 LEFT JOIN users u ON t.created_by = u.id
		 WHERE t.ledger_id=? AND t.uuid=? AND t.deleted_at IS NULL`,
		ledgerID, uuid,
	).Scan(&t.ID, &t.UUID, &t.Date, &t.Flag, &t.Description, &tags, &t.CreatedBy, &t.CreatedByName)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Transaction{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Transaction{}, err
	}
	t.Tags = parseTags(tags)

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

// parseTags 把 tags 列的 JSON 字符串解析为标签切片（NULL/空/非法 → 空切片）。
func parseTags(tags sql.NullString) []string {
	if !tags.Valid || tags.String == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(tags.String), &out); err != nil || out == nil {
		return nil
	}
	return out
}

// marshalTags 把标签切片序列化为 JSON 字符串（空/nil → NULL）。
func marshalTags(tags []string) any {
	if len(tags) == 0 {
		return nil
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return nil
	}
	return string(b)
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
		`INSERT INTO transactions(ledger_id, uuid, date, flag, description, tags, created_by)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		ledgerID, txn.UUID, txn.Date, txn.Flag, txn.Description, marshalTags(txn.Tags), createdBy,
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

// RestoreTransaction 从回收站恢复一笔交易：清空 deleted_at，并写 sync_log op=create
// 让其他设备重新拉回这笔交易（客户端只认 create/delete/update/close 四种 op，
// 恢复复用 create 语义，不引入新 op，旧客户端也能正确合并）。
// 幂等：不存在或未删除返回 false。
func (s *Store) RestoreTransaction(ledgerID int64, uuid string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE transactions SET deleted_at=NULL
		 WHERE ledger_id=? AND uuid=? AND deleted_at IS NOT NULL`,
		ledgerID, uuid,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	if _, err := appendSyncLogOn(s.db, ledgerID, domain.EntityTransaction, uuid, domain.OpCreate); err != nil {
		return false, err
	}
	return true, nil
}

// ClosedAccountsOfTransaction 返回该交易分录引用到的「已关闭」账户名。
//
// 恢复前必须检查：记账不允许引用已关闭账户（0.4-A 起的约束），
// 否则恢复出来的交易会是一条账本内无法新建、却能通过导出校验的脏数据。
// 返回展示名优先（「现金」比 `Assets:Cash` 更适合直接弹给用户看）。
func (s *Store) ClosedAccountsOfTransaction(ledgerID int64, uuid string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT COALESCE(NULLIF(a.display_name, ''), a.name) AS label
		 FROM postings p
		 JOIN transactions t ON t.id = p.transaction_id
		 JOIN accounts a ON a.id = p.account_id
		 WHERE t.ledger_id=? AND t.uuid=? AND a.close_date IS NOT NULL
		 ORDER BY label`,
		ledgerID, uuid,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
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

// CloseAccount 关闭账户（Beancount close 语义）：置 close_date，保留全部交易引用。
// 不再软删账户、不连带删除引用交易（0.4-A 起，原 SoftDeleteAccount 语义废弃）。
// 写 sync_log op=close。幂等：不存在/已关闭返回 false。
func (s *Store) CloseAccount(ledgerID int64, uuid string) (bool, error) {
	// 仅匹配未关闭账户（close_date IS NULL）；deleted_at 兼容旧库（0.4 起不再写入）。
	// close_date 取 date('now')：Beancount close 指令要求 YYYY-MM-DD 格式。
	res, err := s.db.Exec(
		`UPDATE accounts SET close_date=date('now'), updated_at=datetime('now')
		 WHERE ledger_id=? AND uuid=? AND close_date IS NULL AND deleted_at IS NULL`,
		ledgerID, uuid,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	if _, err := appendSyncLogOn(s.db, ledgerID, domain.EntityAccount, uuid, domain.OpClose); err != nil {
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
