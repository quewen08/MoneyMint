package repository

import (
	"fmt"
	"math/big"
)

// ComputeBalances 返回 map[accountID]map[commodity]定点十进制字符串。
// 使用 big.Rat 精确累加，避免浮点误差。
func (s *Store) ComputeBalances(ledgerID int64) (map[int64]map[string]string, error) {
	rows, err := s.db.Query(
		`SELECT p.account_id, p.commodity, CAST(p.amount AS TEXT)
		 FROM postings p
		 JOIN accounts a ON a.id=p.account_id
		 JOIN transactions t ON t.id=p.transaction_id
		 WHERE a.ledger_id=? AND a.deleted_at IS NULL AND t.deleted_at IS NULL`,
		ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	acc := map[int64]map[string]*big.Rat{}
	for rows.Next() {
		var aid int64
		var comm, amt string
		if err := rows.Scan(&aid, &comm, &amt); err != nil {
			return nil, err
		}
		if _, ok := acc[aid]; !ok {
			acc[aid] = map[string]*big.Rat{}
		}
		r := new(big.Rat)
		if _, ok := r.SetString(amt); !ok {
			return nil, fmt.Errorf("数据库金额非法: %q", amt)
		}
		if _, ok := acc[aid][comm]; !ok {
			acc[aid][comm] = new(big.Rat)
		}
		acc[aid][comm].Add(acc[aid][comm], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[int64]map[string]string{}
	for aid, m := range acc {
		mm := map[string]string{}
		for c, v := range m {
			mm[c] = v.FloatString(2)
		}
		out[aid] = mm
	}
	return out, nil
}
