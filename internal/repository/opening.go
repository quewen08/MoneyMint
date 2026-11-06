package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"familyledger/internal/domain"

	"github.com/google/uuid"
)

// 期初交易固定文案（与迁移 012 的回填识别保持一致）。
const (
	openingTxnDescription = "期初余额"
	openingReversalDesc   = "冲正: 期初余额"
)

// OpeningPosting 是期初交易的一条分录快照（冲正时逐条取负）。
// AccountUUID/AccountName 仅用于读取期初时识别目标账户与权益对手方，写入时忽略。
type OpeningPosting struct {
	AccountID   int64
	AccountUUID string
	AccountName string
	Commodity   string
	Amount      string
	Position    int64
}

// OpeningRecord 是某账户当前有效的期初交易（包含全部分录）。
type OpeningRecord struct {
	ID              int64
	UUID            string
	Date            string
	Version         int64
	Commodity       string
	SignedAmount    string
	TargetPostingID int64
	Postings        []OpeningPosting
}

// OpeningApplyInput 是应用期初所需的最小输入（服务层已完成符号/币种/日期/状态校验）。
type OpeningApplyInput struct {
	AccountID      int64
	AccountUUID    string
	SignedAmount   string // 已带账户类型方向的账本符号
	Commodity      string
	Date           string // 账本 opening_date
	Direction      string // hold | owe，仅用于结果回显
	AbsoluteAmount string // 绝对值（服务层按币种精度格式化）
	Precision      int    // 币种展示精度（缺省 2）
}

// OpeningApplyResult 是 ApplyOpening 的返回。
type OpeningApplyResult struct {
	Status          string
	TransactionUUID string
	ReversalUUID    string
	Version         int64
	SignedAmount    string
	Commodity       string
	Date            string
}

// LedgerOpeningDate 读取账本期初日期：优先 012 列，空值时按设计 §7.1 派生。
func (s *Store) LedgerOpeningDate(ledgerID int64) (string, error) {
	var d string
	err := s.db.QueryRow(
		`SELECT COALESCE(
			NULLIF(l.opening_date, ''),
			MIN(
				COALESCE((SELECT MIN(t.date) FROM transactions t
				          WHERE t.ledger_id=l.id AND t.deleted_at IS NULL), '9999-12-31'),
				COALESCE(date(l.created_at, 'localtime'), date('now', 'localtime')),
				date('now', 'localtime')
			)
		) FROM ledgers l WHERE l.id=?`,
		ledgerID,
	).Scan(&d)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return d, err
}

// EarliestNonOpeningTxnDate 返回账本内最早的非期初交易日期；无交易返回空串。
// 期初交易包括原期初与冲正（opening_account_uuid 非空），它们不构成修改上界。
func (s *Store) EarliestNonOpeningTxnDate(ledgerID int64) (string, error) {
	var d sql.NullString
	err := s.db.QueryRow(
		`SELECT MIN(t.date) FROM transactions t
		 WHERE t.ledger_id=? AND t.deleted_at IS NULL
		   AND (t.opening_account_uuid IS NULL OR t.opening_account_uuid = '')`,
		ledgerID,
	).Scan(&d)
	if err != nil {
		return "", err
	}
	if !d.Valid {
		return "", nil
	}
	return d.String, nil
}

// CommodityPrecision 返回账本内某币种的展示精度；未登记的币种按 2 位处理。
func (s *Store) CommodityPrecision(ledgerID int64, symbol string) (int, error) {
	var prec int
	err := s.db.QueryRow(
		`SELECT "precision" FROM commodities WHERE ledger_id=? AND symbol=?`,
		ledgerID, symbol,
	).Scan(&prec)
	if errors.Is(err, sql.ErrNoRows) {
		return 2, nil
	}
	return prec, err
}

// ActiveOpening 读取账户当前有效的期初交易（未冲正、未软删、非冲正本身）。
// 不存在时返回 (OpeningRecord{}, false, nil)。
func (s *Store) ActiveOpening(ledgerID int64, accountUUID string) (OpeningRecord, bool, error) {
	return loadOpeningRecord(s.db, ledgerID, accountUUID)
}

// ApplyOpening 应用一次期初设置（创建/修正/清空），全部在一个事务内完成：
//   - 缺 active 期初权益账户时同事务补种；
//   - 内容完全一致 → unchanged（不新增交易，天然幂等）；
//   - 存在旧期初 → 先冲正（同日期、分录全部取负），再按需新建；
//   - 新值 > 0 → version = MAX(opening_version)+1，写 opening_key；
//   - 新值 = 0 → 仅冲正，返回 cleared。
func (s *Store) ApplyOpening(ledgerID int64, in OpeningApplyInput) (OpeningApplyResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return OpeningApplyResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	equityID, err := ensureOpeningEquityTx(tx, ledgerID, in.Date)
	if err != nil {
		return OpeningApplyResult{}, err
	}
	if err := ensureCommodityOn(tx, ledgerID, in.Commodity); err != nil {
		return OpeningApplyResult{}, fmt.Errorf("补建期初币种失败: %w", err)
	}

	active, found, err := activeOpeningTx(tx, ledgerID, in.AccountUUID)
	if err != nil {
		return OpeningApplyResult{}, err
	}

	maxVer, err := maxOpeningVersionTx(tx, ledgerID, in.AccountUUID)
	if err != nil {
		return OpeningApplyResult{}, err
	}

	prec := in.Precision
	if prec <= 0 {
		prec = 2
	}
	wantRat, err := parseDecimalRat(in.SignedAmount)
	if err != nil {
		return OpeningApplyResult{}, err
	}
	wantStr := wantRat.FloatString(prec)

	// 内容完全一致：日期、币种、金额均相同 → 幂等返回。
	if found {
		oldRat, err := parseDecimalRat(active.SignedAmount)
		if err != nil {
			return OpeningApplyResult{}, err
		}
		if active.Commodity == in.Commodity && active.Date == in.Date && oldRat.Cmp(wantRat) == 0 {
			if err := tx.Commit(); err != nil {
				return OpeningApplyResult{}, err
			}
			return OpeningApplyResult{
				Status:          "unchanged",
				TransactionUUID: active.UUID,
				Version:         active.Version,
				SignedAmount:    active.SignedAmount,
				Commodity:       active.Commodity,
				Date:            active.Date,
			}, nil
		}
	}

	var reversalUUID string
	if found {
		reversalUUID, err = insertReversalTx(tx, ledgerID, active, in.AccountUUID, openingReversalDesc, prec)
		if err != nil {
			return OpeningApplyResult{}, err
		}
		res, err := tx.Exec(
			`UPDATE transactions SET reversed_by=?, updated_at=datetime('now')
			 WHERE id=? AND (reversed_by IS NULL OR reversed_by='')`,
			reversalUUID, active.ID,
		)
		if err != nil {
			return OpeningApplyResult{}, fmt.Errorf("标记旧期初已冲正: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return OpeningApplyResult{}, domain.Codedf(domain.ErrConflict,
				domain.CodeOpeningConflict, "期初已被并发修改，请重试")
		}
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityTransaction, reversalUUID, domain.OpCreate); err != nil {
			return OpeningApplyResult{}, err
		}
	}

	if wantRat.Sign() == 0 {
		if err := tx.Commit(); err != nil {
			return OpeningApplyResult{}, err
		}
		status := "unchanged"
		if found {
			status = "cleared"
		}
		return OpeningApplyResult{
			Status:       status,
			ReversalUUID: reversalUUID,
			Version:      maxVer,
			SignedAmount: wantStr,
			Commodity:    in.Commodity,
			Date:         in.Date,
		}, nil
	}

	version := maxVer + 1
	openingUUID := uuid.NewString()
	openingKey := fmt.Sprintf("opening:%s:%d", in.AccountUUID, version)
	txnID, err := insertTxnTx(tx, ledgerID, txnInsert{
		UUID:               openingUUID,
		Date:               in.Date,
		Description:        openingTxnDescription,
		Tags:               []string{"opening"},
		OpeningAccountUUID: in.AccountUUID,
		OpeningVersion:     version,
		OpeningKey:         openingKey,
	})
	if err != nil {
		if isUniqueConstraintErr(err) {
			return OpeningApplyResult{}, domain.Codedf(domain.ErrConflict,
				domain.CodeOpeningConflict, "期初设置冲突，请重试")
		}
		return OpeningApplyResult{}, err
	}
	if err := insertPostingsTx(tx, txnID, []OpeningPosting{
		{AccountID: in.AccountID, Commodity: in.Commodity, Amount: wantStr, Position: 0},
		{AccountID: equityID, Commodity: in.Commodity, Amount: negateDecimal(wantStr, prec), Position: 1},
	}); err != nil {
		return OpeningApplyResult{}, err
	}
	if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityTransaction, openingUUID, domain.OpCreate); err != nil {
		return OpeningApplyResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return OpeningApplyResult{}, err
	}
	status := "created"
	if found {
		status = "updated"
	}
	return OpeningApplyResult{
		Status:          status,
		TransactionUUID: openingUUID,
		ReversalUUID:    reversalUUID,
		Version:         version,
		SignedAmount:    wantStr,
		Commodity:       in.Commodity,
		Date:            in.Date,
	}, nil
}

// RebuildAllOpenings 修改账本期初日期并在同一事务内重建该账本全部有效期初：
// 对每条有效期初生成冲正（旧日期）与新期初（新日期，version+1），
// 然后更新 ledgers.opening_date，只下调未关闭账户的 open_date，写 sync_log。
// 返回实际重建的期初条数（旧日期等于新日期时跳过，保证幂等）。
//
// BUG-01：已关闭账户不参与重建——其 open_date 按 FR-23 保持不变，
// 若把期初交易前移到 open_date 之前，导出会出现「交易早于开户日」导致
// bean-check 报 inactive account。因此重建范围只包含未删除且未关闭的账户。
func (s *Store) RebuildAllOpenings(ledgerID int64, newDate string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	type activeRow struct {
		id        int64
		uuid      string
		date      string
		version   int64
		account   string
		accountID int64
	}
	rows, err := tx.Query(
		`SELECT t.id, t.uuid, t.date, t.opening_version, t.opening_account_uuid, a.id
		 FROM transactions t JOIN accounts a
		   ON a.ledger_id=t.ledger_id AND a.uuid=t.opening_account_uuid
		 WHERE t.ledger_id=?
		   AND t.opening_account_uuid IS NOT NULL AND t.opening_account_uuid <> ''
		   AND t.reversal_of IS NULL
		   AND (t.reversed_by IS NULL OR t.reversed_by='')
		   AND t.deleted_at IS NULL
		   AND a.deleted_at IS NULL
		   AND a.close_date IS NULL
		 ORDER BY t.id`,
		ledgerID,
	)
	if err != nil {
		return 0, err
	}
	var actives []activeRow
	for rows.Next() {
		var a activeRow
		if err := rows.Scan(&a.id, &a.uuid, &a.date, &a.version, &a.account, &a.accountID); err != nil {
			rows.Close()
			return 0, err
		}
		actives = append(actives, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	rebuilt := 0
	for _, a := range actives {
		if a.date == newDate {
			continue
			// 日期未变，无需冲正重建；保证 PATCH 同值幂等。
		}
		rec, found, err := activeOpeningTx(tx, ledgerID, a.account)
		if err != nil {
			return 0, err
		}
		if !found || len(rec.Postings) < 2 {
			continue // 数据异常或无有效分录：跳过，不影响其余账户
		}
		prec, err := commodityPrecisionTx(tx, ledgerID, rec.Commodity)
		if err != nil {
			return 0, err
		}

		revUUID, err := insertReversalTx(tx, ledgerID, rec, a.account, openingReversalDesc, prec)
		if err != nil {
			return 0, err
		}
		res, err := tx.Exec(
			`UPDATE transactions SET reversed_by=?, updated_at=datetime('now')
			 WHERE id=? AND (reversed_by IS NULL OR reversed_by='')`,
			revUUID, rec.ID,
		)
		if err != nil {
			return 0, fmt.Errorf("标记期初 %s 已冲正: %w", rec.UUID, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return 0, domain.Codedf(domain.ErrConflict,
				domain.CodeOpeningConflict, "期初已被并发修改，请重试")
		}
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityTransaction, revUUID, domain.OpCreate); err != nil {
			return 0, err
		}

		version := a.version + 1
		if max, err := maxOpeningVersionTx(tx, ledgerID, a.account); err != nil {
			return 0, err
		} else if max+1 > version {
			version = max + 1
		}
		newUUID := uuid.NewString()
		key := fmt.Sprintf("opening:%s:%d", a.account, version)
		newID, err := insertTxnTx(tx, ledgerID, txnInsert{
			UUID:               newUUID,
			Date:               newDate,
			Description:        openingTxnDescription,
			Tags:               []string{"opening"},
			OpeningAccountUUID: a.account,
			OpeningVersion:     version,
			OpeningKey:         key,
		})
		if err != nil {
			return 0, err
		}
		if err := insertPostingsTx(tx, newID, rec.Postings); err != nil {
			return 0, err
		}
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityTransaction, newUUID, domain.OpCreate); err != nil {
			return 0, err
		}
		rebuilt++
	}

	if _, err := tx.Exec(
		`UPDATE ledgers SET opening_date=?, updated_at=datetime('now') WHERE id=?`,
		newDate, ledgerID,
	); err != nil {
		return 0, fmt.Errorf("更新账本期初日期: %w", err)
	}

	// 只下调未关闭账户的 open_date（FR-23：已关闭账户完全不动）。
	arows, err := tx.Query(
		`SELECT id, uuid, COALESCE(open_date, '')
		 FROM accounts
		 WHERE ledger_id=? AND deleted_at IS NULL AND close_date IS NULL
		   AND (open_date IS NULL OR open_date='' OR open_date > ?)`,
		ledgerID, newDate,
	)
	if err != nil {
		return 0, err
	}
	type accountRow struct {
		id     int64
		uuid   string
		oldDay string
	}
	var accounts []accountRow
	for arows.Next() {
		var a accountRow
		if err := arows.Scan(&a.id, &a.uuid, &a.oldDay); err != nil {
			arows.Close()
			return 0, err
		}
		accounts = append(accounts, a)
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return 0, err
	}
	for _, a := range accounts {
		res, err := tx.Exec(
			`UPDATE accounts SET open_date=?, updated_at=datetime('now')
			 WHERE id=? AND deleted_at IS NULL AND close_date IS NULL
			   AND (open_date IS NULL OR open_date='' OR open_date > ?)`,
			newDate, a.id, newDate,
		)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityAccount, a.uuid, domain.OpUpdate); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return rebuilt, nil
}

// ---- 内部辅助 ----

// txnInsert 是期初/冲正交易的最小写入参数。
type txnInsert struct {
	UUID               string
	Date               string
	Description        string
	Tags               []string
	ReversalOf         string
	OpeningAccountUUID string
	OpeningVersion     int64
	OpeningKey         string
}

// insertTxnTx 插入一条期初/冲正交易（不含分录）。
func insertTxnTx(tx execer, ledgerID int64, t txnInsert) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO transactions
		 (ledger_id, uuid, date, flag, description, tags,
		  opening_account_uuid, opening_version, opening_key, reversal_of)
		 VALUES (?, ?, ?, '*', ?, ?, NULLIF(?,''), ?, NULLIF(?,''), NULLIF(?,''))`,
		ledgerID, t.UUID, t.Date, t.Description, marshalTags(t.Tags),
		t.OpeningAccountUUID, t.OpeningVersion, t.OpeningKey, t.ReversalOf,
	)
	if err != nil {
		return 0, fmt.Errorf("写入期初交易: %w", err)
	}
	return res.LastInsertId()
}

// insertPostingsTx 批量插入分录。
func insertPostingsTx(tx execer, txnID int64, posts []OpeningPosting) error {
	for _, p := range posts {
		if _, err := tx.Exec(
			`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
			 VALUES(?, ?, ?, ?, ?)`,
			txnID, p.AccountID, p.Commodity, p.Amount, p.Position,
		); err != nil {
			return fmt.Errorf("写入期初分录(账户 %d): %w", p.AccountID, err)
		}
	}
	return nil
}

// insertReversalTx 为一条有效期初生成冲正交易：同日期、分录全部取负、
// reversal_of=原 uuid，opening_account_uuid/version 沿用原交易（仅溯源）。
func insertReversalTx(tx execer, ledgerID int64, rec OpeningRecord, accountUUID, desc string, precision int) (string, error) {
	revUUID := uuid.NewString()
	revID, err := insertTxnTx(tx, ledgerID, txnInsert{
		UUID:               revUUID,
		Date:               rec.Date,
		Description:        desc,
		Tags:               []string{"opening", "reversal"},
		ReversalOf:         rec.UUID,
		OpeningAccountUUID: accountUUID,
		OpeningVersion:     rec.Version,
	})
	if err != nil {
		return "", err
	}
	posts := make([]OpeningPosting, 0, len(rec.Postings))
	for _, p := range rec.Postings {
		neg, err := parseDecimalRat(p.Amount)
		if err != nil {
			return "", fmt.Errorf("冲正金额非法 %q: %w", p.Amount, err)
		}
		posts = append(posts, OpeningPosting{
			AccountID: p.AccountID,
			Commodity: p.Commodity,
			Amount:    neg.Neg(neg).FloatString(precision),
			Position:  p.Position,
		})
	}
	if err := insertPostingsTx(tx, revID, posts); err != nil {
		return "", err
	}
	return revUUID, nil
}

// activeOpeningTx 在事务内读取账户有效期初。
func activeOpeningTx(tx execer, ledgerID int64, accountUUID string) (OpeningRecord, bool, error) {
	return loadOpeningRecord(tx, ledgerID, accountUUID)
}

// loadOpeningRecord 读取有效期初交易与全部分录（含账户 uuid/名称，用于按权益对手方解析金额）。
func loadOpeningRecord(e execer, ledgerID int64, accountUUID string) (OpeningRecord, bool, error) {
	var rec OpeningRecord
	err := e.QueryRow(
		`SELECT id, uuid, date, opening_version
		 FROM transactions
		 WHERE ledger_id=? AND opening_account_uuid=?
		   AND reversal_of IS NULL
		   AND (reversed_by IS NULL OR reversed_by='')
		   AND deleted_at IS NULL
		 ORDER BY id DESC LIMIT 1`,
		ledgerID, accountUUID,
	).Scan(&rec.ID, &rec.UUID, &rec.Date, &rec.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return OpeningRecord{}, false, nil
	}
	if err != nil {
		return OpeningRecord{}, false, err
	}

	rows, err := e.Query(
		`SELECT p.account_id, COALESCE(a.uuid, ''), a.name,
		        p.commodity, CAST(p.amount AS TEXT), p.position
		 FROM postings p JOIN accounts a ON a.id=p.account_id
		 WHERE p.transaction_id=? ORDER BY p.position, p.id`,
		rec.ID,
	)
	if err != nil {
		return OpeningRecord{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var p OpeningPosting
		if err := rows.Scan(&p.AccountID, &p.AccountUUID, &p.AccountName,
			&p.Commodity, &p.Amount, &p.Position); err != nil {
			return OpeningRecord{}, false, err
		}
		rec.Postings = append(rec.Postings, p)
	}
	if err := rows.Err(); err != nil {
		return OpeningRecord{}, false, err
	}
	resolveOpeningAmount(&rec, accountUUID)
	return rec, true, nil
}

// resolveOpeningAmount 解析期初目标账户金额，不依赖 postings.position 顺序。
//
// 规则（BUG-02）：目标账户必须由 accountUUID 定位，金额按「对手方为
// Equity:Opening-Balances 的分录」反推——账户金额 = -权益金额（按币种配对）。
// 这样即使历史数据把权益分录排在前、目标分录排在后，符号也不会反转；
// 相同金额重复保存时能命中 unchanged 分支，不产生多余冲正。
// 找不到权益分录时退化为读取目标分录本身（兼容异常/手工数据）。
func resolveOpeningAmount(rec *OpeningRecord, accountUUID string) {
	equityName := domain.OpeningEquityName

	// 1) 定位目标账户分录：优先 accountUUID，缺失时取任一非权益分录。
	targetIdx := -1
	for i := range rec.Postings {
		if rec.Postings[i].AccountName == equityName {
			continue
		}
		if accountUUID != "" && rec.Postings[i].AccountUUID == accountUUID {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		for i := range rec.Postings {
			if rec.Postings[i].AccountName == equityName {
				continue
			}
			targetIdx = i
			break
		}
	}
	if targetIdx >= 0 {
		rec.TargetPostingID = rec.Postings[targetIdx].AccountID
	}

	// 2) 定位权益分录：优先与目标分录同币种，找不到再退化为第一条权益分录。
	equityIdx := -1
	for i := range rec.Postings {
		if rec.Postings[i].AccountName != equityName {
			continue
		}
		if equityIdx < 0 {
			equityIdx = i
		}
		if targetIdx >= 0 && rec.Postings[i].Commodity == rec.Postings[targetIdx].Commodity {
			equityIdx = i
			break
		}
	}

	// 3) 账户金额 = -权益金额；权益缺失时用目标分录自身金额。
	if equityIdx >= 0 {
		if neg, err := negateAmountString(rec.Postings[equityIdx].Amount); err == nil {
			rec.SignedAmount = neg
			rec.Commodity = rec.Postings[equityIdx].Commodity
			return
		}
	}
	if targetIdx >= 0 {
		rec.SignedAmount = rec.Postings[targetIdx].Amount
		rec.Commodity = rec.Postings[targetIdx].Commodity
	}
}

// negateAmountString 翻转定点十进制字符串的符号并保留原有精度与小数位。
// 例："10.00" → "-10.00"、"-0.50" → "0.50"、"-0.00" → "0.00"。
func negateAmountString(s string) (string, error) {
	t := strings.TrimSpace(s)
	r, err := parseDecimalRat(t)
	if err != nil {
		return "", err
	}
	if r.Sign() == 0 {
		return strings.TrimLeft(t, "+-"), nil
	}
	switch {
	case strings.HasPrefix(t, "-"):
		return t[1:], nil
	case strings.HasPrefix(t, "+"):
		return "-" + t[1:], nil
	default:
		return "-" + t, nil
	}
}

// maxOpeningVersionTx 返回某账户历史最大 opening_version（含已冲正版本）。
func maxOpeningVersionTx(tx execer, ledgerID int64, accountUUID string) (int64, error) {
	var v int64
	err := tx.QueryRow(
		`SELECT COALESCE(MAX(opening_version), 0) FROM transactions
		 WHERE ledger_id=? AND opening_account_uuid=?`,
		ledgerID, accountUUID,
	).Scan(&v)
	return v, err
}

// ensureOpeningEquityTx 返回 active 的期初权益账户 id；缺失时同事务补种。
// 同名账户若存在但已关闭，返回 account_closed（唯一索引不允许同名重建）。
//
// BUG-04 修复：006 起账户名在账本内全局唯一（含已删除/已关闭账户），
// 因此同名账户被软删（deleted_at 非空）后无法再插入同名新账户。旧实现
// 在唯一索引冲突时递归自调用，而软删账户永远查不到，最终栈溢出。
// 现改为分支 + 有界循环（最多两轮），任何路径都不再自调用：
//   - 存在未删除同名账户：直接复用（已关闭则报 account_closed）；
//   - 仅存在同名软删账户：就地恢复，复用原 id/uuid 与历史分录，写 sync_log(update)；
//   - 完全不存在同名记录：才新建。
func ensureOpeningEquityTx(tx execer, ledgerID int64, openingDate string) (int64, error) {
	// 最多两轮：第一轮可能因并发写入撞唯一索引，第二轮重读后必然收敛到
	// 「复用已有账户」分支。循环上界同时是防自调用成环的硬保护。
	for attempt := 0; attempt < 2; attempt++ {
		activeID, closeDate, found, err := findActiveOpeningEquityTx(tx, ledgerID)
		if err != nil {
			return 0, err
		}
		if found {
			if closeDate.Valid {
				return 0, domain.Codedf(domain.ErrConflict, domain.CodeAccountClosed,
					"期初余额账户已关闭，请先恢复")
			}
			return activeID, nil
		}

		restoredID, restored, err := restoreSoftDeletedOpeningEquityTx(tx, ledgerID)
		if err != nil {
			return 0, err
		}
		if restored {
			return restoredID, nil
			// 并发下若恢复未命中（RowsAffected=0），继续走新建分支；
			// 新建撞唯一索引时进入下一轮重读，不会递归。
		}

		equityUUID := uuid.NewString()
		res, err := tx.Exec(
			`INSERT INTO accounts
			 (ledger_id, uuid, name, display_name, type, open_date,
			  close_date, commodity_restriction, icon, color, parent_uuid, sub_type, sort_order,
			  deleted_at, created_at, updated_at)
			 VALUES (?, ?, ?, '期初余额权益', 'Equity', ?,
			         NULL, NULL, '⚖️', '#868E96', NULL, NULL, 0,
			         NULL, datetime('now'), datetime('now'))`,
			ledgerID, equityUUID, domain.OpeningEquityName, openingDate,
		)
		if err != nil {
			if isUniqueConstraintErr(err) {
				// 并发补种或软删记录被他人同时恢复：不递归，交给下一轮重读。
				continue
			}
			return 0, fmt.Errorf("补种期初权益账户: %w", err)
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("读取补种期初权益账户 id: %w", err)
		}
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityAccount, equityUUID, domain.OpCreate); err != nil {
			return 0, err
		}
		return newID, nil
	}
	return 0, fmt.Errorf("补种期初权益账户: 并发冲突重试后仍未收敛")
}

// findActiveOpeningEquityTx 查询未删除的同名期初权益账户。
func findActiveOpeningEquityTx(tx execer, ledgerID int64) (int64, sql.NullString, bool, error) {
	var (
		id        int64
		closeDate sql.NullString
	)
	err := tx.QueryRow(
		`SELECT id, close_date FROM accounts
		 WHERE ledger_id=? AND name=? AND deleted_at IS NULL ORDER BY id LIMIT 1`,
		ledgerID, domain.OpeningEquityName,
	).Scan(&id, &closeDate)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, sql.NullString{}, false, nil
	}
	if err != nil {
		return 0, sql.NullString{}, false, err
	}
	return id, closeDate, true, nil
}

// restoreSoftDeletedOpeningEquityTx 就地恢复同名软删的期初权益账户：
// 清空 deleted_at/close_date、刷新 updated_at，并追加 sync_log(account/update)。
// uuid 保持不变，历史分录通过 account_id 继续引用该账户。
// 无同名软删记录时返回 (0, false, nil)；并发下已被他人恢复同样返回 (0, false, nil)。
func restoreSoftDeletedOpeningEquityTx(tx execer, ledgerID int64) (int64, bool, error) {
	var (
		id   int64
		uuid string
	)
	err := tx.QueryRow(
		`SELECT id, COALESCE(uuid, '') FROM accounts
		 WHERE ledger_id=? AND name=? AND deleted_at IS NOT NULL ORDER BY id LIMIT 1`,
		ledgerID, domain.OpeningEquityName,
	).Scan(&id, &uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	res, err := tx.Exec(
		`UPDATE accounts
		 SET deleted_at=NULL, close_date=NULL, updated_at=datetime('now')
		 WHERE id=? AND deleted_at IS NOT NULL`,
		id,
	)
	if err != nil {
		return 0, false, fmt.Errorf("恢复期初权益账户: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// 并发已恢复：调用方重读 active 分支。
		return 0, false, nil
	}
	if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityAccount, uuid, domain.OpUpdate); err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// parseDecimalRat 把定点十进制字符串解析为精确有理数（禁止浮点）。
func parseDecimalRat(s string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return nil, fmt.Errorf("非法金额 %q", s)
	}
	return r, nil
}

// commodityPrecisionTx 在事务内读取币种精度；未登记按 2 位。
func commodityPrecisionTx(tx execer, ledgerID int64, symbol string) (int, error) {
	var prec int
	err := tx.QueryRow(
		`SELECT "precision" FROM commodities WHERE ledger_id=? AND symbol=?`,
		ledgerID, symbol,
	).Scan(&prec)
	if errors.Is(err, sql.ErrNoRows) {
		return 2, nil
	}
	return prec, err
}

// negateDecimal 取负并按给定精度输出。
func negateDecimal(s string, precision int) string {
	if precision <= 0 {
		precision = 2
	}
	r, err := parseDecimalRat(s)
	if err != nil {
		return "0." + strings.Repeat("0", precision)
	}
	return r.Neg(r).FloatString(precision)
}

// isUniqueConstraintErr 识别 SQLite 唯一约束冲突（modernc 驱动文案）。
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "unique constraint") || strings.Contains(m, "constraint failed")
}

// sortDiffByCommodity 让关闭账户差额按币种稳定排序。
func sortDiffByCommodity(diffs []domain.BalanceDiff) {
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Commodity < diffs[j].Commodity })
}
