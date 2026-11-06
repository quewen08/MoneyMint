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

// AccountBalanceRats 返回某账户按币种精确汇总的余额（big.Rat，禁止浮点）。
// 仅统计账本内未删除交易；账户自身已关闭不影响历史余额计算。
func (s *Store) AccountBalanceRats(ledgerID int64, accountUUID string) (map[string]*big.Rat, error) {
	return accountBalanceRatsOn(s.db, ledgerID, accountUUID)
}

// accountBalanceRatsOn 在给定执行器（db 或事务）上汇总账户余额。
func accountBalanceRatsOn(e execer, ledgerID int64, accountUUID string) (map[string]*big.Rat, error) {
	rows, err := e.Query(
		`SELECT p.commodity, CAST(p.amount AS TEXT)
		 FROM postings p
		 JOIN accounts a ON a.id=p.account_id
		 JOIN transactions t ON t.id=p.transaction_id
		 WHERE a.ledger_id=? AND a.uuid=? AND a.deleted_at IS NULL AND t.deleted_at IS NULL`,
		ledgerID, accountUUID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sums := map[string]*big.Rat{}
	for rows.Next() {
		var comm, amt string
		if err := rows.Scan(&comm, &amt); err != nil {
			return nil, err
		}
		r := new(big.Rat)
		if _, ok := r.SetString(amt); !ok {
			return nil, fmt.Errorf("数据库金额非法: %q", amt)
		}
		if _, ok := sums[comm]; !ok {
			sums[comm] = new(big.Rat)
		}
		sums[comm].Add(sums[comm], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sums, nil
}

// commodityPrecisionMapOn 读取账本币种精度表；未出现的币种由调用方按 2 位兜底。
func commodityPrecisionMapOn(e execer, ledgerID int64) (map[string]int, error) {
	rows, err := e.Query(
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
