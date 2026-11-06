// Package identity 提供只读的账本恒等式检查（复式记账借贷平衡）。
//
// 恒等式：同一账本内，按 commodity 汇总全部有效 posting 的 amount 必须为 0。
// 全部运算使用 big.Rat 精确累加，禁止浮点。
package identity

import (
	"database/sql"
	"fmt"
	"math/big"
	"sort"
	"time"
)

// Diff 是某币种的恒等式差额（非 0 即失衡）。
type Diff struct {
	Commodity string `json:"commodity"`
	Sum       string `json:"sum"`
}

// Result 是一次恒等式检查的结果。
type Result struct {
	Balanced     bool   `json:"balanced"`
	Diffs        []Diff `json:"diffs"`
	PostingCount int64  `json:"posting_count"`
	CheckedAt    string `json:"checked_at"`
	DurationMS   int64  `json:"duration_ms"`
}

// LedgerResult 是单账本结果（备份告警使用）。
type LedgerResult struct {
	LedgerID int64 `json:"ledger_id"`
	Result
}

// Check 检查单个账本的恒等式。
func Check(db *sql.DB, ledgerID int64) (Result, error) {
	started := time.Now()
	prec, err := precisionMap(db, ledgerID)
	if err != nil {
		return Result{}, err
	}
	rows, err := db.Query(
		`SELECT p.commodity, CAST(p.amount AS TEXT)
		 FROM postings p
		 JOIN transactions t ON t.id=p.transaction_id
		 JOIN accounts a ON a.id=p.account_id
		 WHERE t.ledger_id=? AND t.deleted_at IS NULL AND a.deleted_at IS NULL`,
		ledgerID,
	)
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()

	sums := map[string]*big.Rat{}
	var count int64
	for rows.Next() {
		var commodity, amount string
		if err := rows.Scan(&commodity, &amount); err != nil {
			return Result{}, err
		}
		r := new(big.Rat)
		if _, ok := r.SetString(amount); !ok {
			return Result{}, fmt.Errorf("数据库金额非法: %q", amount)
		}
		if _, ok := sums[commodity]; !ok {
			sums[commodity] = new(big.Rat)
		}
		sums[commodity].Add(sums[commodity], r)
		count++
	}
	if err := rows.Err(); err != nil {
		return Result{}, err
	}

	diffs := make([]Diff, 0)
	for commodity, sum := range sums {
		if sum.Sign() == 0 {
			continue
		}
		p := prec[commodity]
		if p <= 0 {
			p = 2
		}
		diffs = append(diffs, Diff{Commodity: commodity, Sum: sum.FloatString(p)})
	}
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Commodity < diffs[j].Commodity })

	return Result{
		Balanced:     len(diffs) == 0,
		Diffs:        diffs,
		PostingCount: count,
		CheckedAt:    time.Now().Format(time.RFC3339),
		DurationMS:   time.Since(started).Milliseconds(),
	}, nil
}

// CheckAll 检查全部账本；返回按账本 id 升序的结果。
func CheckAll(db *sql.DB) ([]LedgerResult, error) {
	rows, err := db.Query(`SELECT id FROM ledgers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]LedgerResult, 0, len(ids))
	for _, id := range ids {
		res, err := Check(db, id)
		if err != nil {
			return nil, err
		}
		out = append(out, LedgerResult{LedgerID: id, Result: res})
	}
	return out, nil
}

// precisionMap 读取账本币种精度；未登记币种由调用方按 2 位兜底。
func precisionMap(db *sql.DB, ledgerID int64) (map[string]int, error) {
	rows, err := db.Query(
		`SELECT symbol, "precision" FROM commodities WHERE ledger_id=?`, ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var sym string
		var p int
		if err := rows.Scan(&sym, &p); err != nil {
			return nil, err
		}
		out[sym] = p
	}
	return out, rows.Err()
}
