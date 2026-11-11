package repository

import (
	"fmt"
	"strings"

	"familyledger/internal/beancount"
	"familyledger/internal/domain"

	"github.com/google/uuid"
)

// ImportBeancount 在单个事务内把一份 Beancount 解析结果写入「新账本」：
//
//	建账本（同名自动加序号）→ 补建币种 → 建账户（open/close）→ 建价格 → 建交易与分录，
//	每一步都写 sync_log（create），保证内容可被同步到其他端（AC-34 RK-M2-05）。
//
// 中途任何一步失败整体回滚，绝不留下半个账本（M2-e：「导入在一个大事务内完成」）。
// 账本名按 owner 维度去重（已有同名则追加 " (2)" 等），返回最终使用的账本名。
//
// 必须传入已通过 beancount.Parse 且 Errors 为空的结果——本方法不重复做语法校验。
func (s *Store) ImportBeancount(ownerID int64, p *beancount.ParseResult) (int64, string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = tx.Rollback() }()

	base := p.Title
	if base == "" {
		base = "导入账本"
	}
	name := uniqueLedgerNameTx(tx, ownerID, base)

	res, err := tx.Exec(
		`INSERT INTO ledgers(name, owner_id, default_commodity, opening_date)
		 VALUES(?, ?, NULLIF(?,'') , date('now','localtime'))`,
		name, ownerID, p.Currency,
	)
	if err != nil {
		return 0, "", fmt.Errorf("建账本: %w", err)
	}
	ledgerID, _ := res.LastInsertId()
	if _, err := tx.Exec(
		`INSERT INTO ledger_members(ledger_id, user_id, role, invited_by) VALUES(?, ?, 'owner', ?)`,
		ledgerID, ownerID, ownerID,
	); err != nil {
		return 0, "", fmt.Errorf("建账本成员: %w", err)
	}

	// 币种：显式声明的 + 价格涉及的，全部确保存在（导出 round-trip 需要 commodity 指令）。
	for _, c := range p.Commodities {
		sym := c.Symbol
		if sym == "" {
			sym = c.Name
		}
		if sym == "" {
			continue
		}
		if err := ensureCommodityOn(tx, ledgerID, sym); err != nil {
			return 0, "", err
		}
	}
	for _, pr := range p.Prices {
		if pr.Commodity != "" {
			if err := ensureCommodityOn(tx, ledgerID, pr.Commodity); err != nil {
				return 0, "", err
			}
		}
		if pr.Currency != "" {
			if err := ensureCommodityOn(tx, ledgerID, pr.Currency); err != nil {
				return 0, "", err
			}
		}
	}

	// 账户：建立 name -> id 映射，供后续分录引用。
	acctID := make(map[string]int64, len(p.Accounts))
	for _, a := range p.Accounts {
		auuid := uuid.NewString()
		res, err := tx.Exec(
			`INSERT INTO accounts(ledger_id, uuid, name, type, open_date, close_date, commodity_restriction)
			 VALUES(?, ?, ?, ?, ?, NULLIF(?,'') , NULLIF(?,'') )`,
			ledgerID, auuid, a.Name, accountType(a.Name), a.OpenDate, a.CloseDate, a.Commodity,
		)
		if err != nil {
			return 0, "", fmt.Errorf("建账户 %s: %w", a.Name, err)
		}
		id, _ := res.LastInsertId()
		acctID[a.Name] = id
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityAccount, auuid, domain.OpCreate); err != nil {
			return 0, "", err
		}
	}

	// 价格
	for _, pr := range p.Prices {
		puuid := uuid.NewString()
		if _, err := tx.Exec(
			`INSERT INTO prices(ledger_id, uuid, commodity, currency, date, rate, source)
			 VALUES(?, ?, ?, ?, ?, ?, 'import')`,
			ledgerID, puuid, pr.Commodity, pr.Currency, pr.Date, pr.Amount,
		); err != nil {
			return 0, "", fmt.Errorf("建价格: %w", err)
		}
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityPrice, puuid, domain.OpCreate); err != nil {
			return 0, "", err
		}
	}

	// 交易 + 分录
	for _, t := range p.Transactions {
		// 分录引用的账户必须已声明
		for _, ps := range t.Postings {
			if _, ok := acctID[ps.Account]; !ok {
				return 0, "", fmt.Errorf("交易 %s 引用了未声明的账户 %s", t.Date, ps.Account)
			}
		}
		// 确保各分录币种存在
		for _, ps := range t.Postings {
			if ps.Commodity != "" {
				if err := ensureCommodityOn(tx, ledgerID, ps.Commodity); err != nil {
					return 0, "", err
				}
			}
		}

		tuuid := uuid.NewString()
		desc := t.Narration
		if t.Payee != "" {
			if desc == "" {
				desc = t.Payee
			} else {
				desc = t.Payee + " " + t.Narration
			}
		}
		flag := t.Flag
		if flag == "" {
			flag = "*"
		}
		txn := domain.Transaction{
			UUID:        tuuid,
			Date:        t.Date,
			Flag:        flag,
			Description: desc,
			Tags:        t.Tags,
			CreatedBy:   ownerID,
		}
		postings := make([]domain.Posting, 0, len(t.Postings))
		for i, ps := range t.Postings {
			postings = append(postings, domain.Posting{
				AccountID: acctID[ps.Account],
				Commodity: ps.Commodity,
				Amount:    ps.Amount,
				Position:  i,
			})
		}
		txn.Postings = postings

		txnID, _, err := insertTransactionRowTx(tx, ledgerID, txn, "", "")
		if err != nil {
			return 0, "", fmt.Errorf("建交易 %s: %w", t.Date, err)
		}
		if err := insertPostingRowsTx(tx, txnID, postings); err != nil {
			return 0, "", fmt.Errorf("建分录: %w", err)
		}
		if _, err := appendSyncLogOn(tx, ledgerID, domain.EntityTransaction, tuuid, domain.OpCreate); err != nil {
			return 0, "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, "", err
	}
	return ledgerID, name, nil
}

// UniqueLedgerName 返回 owner 维度下去重后的账本名（只读，供预检展示）。
func (s *Store) UniqueLedgerName(ownerID int64, base string) string {
	if base == "" {
		base = "导入账本"
	}
	return uniqueLedgerNameTx(s.db, ownerID, base)
}

// IsDuplicateImport 判断是否存在「同名账本 + 相同交易数」，用作疑似重复导入提示。
// 不阻断导入（用户可能确实想导入两次）。
func (s *Store) IsDuplicateImport(ownerID int64, base string, txnCount int) (bool, error) {
	if base == "" {
		base = "导入账本"
	}
	var n int
	err := s.db.QueryRow(
		`SELECT count(*) FROM ledgers l
		 JOIN (SELECT ledger_id, count(*) c FROM transactions GROUP BY ledger_id) t
		   ON t.ledger_id = l.id
		 WHERE l.owner_id=? AND l.name=? AND t.c=?`,
		ownerID, base, txnCount,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// uniqueLedgerNameTx 在事务/连接上计算去重账本名：base 已存在则追加 " (2)"、" (3)"…
func uniqueLedgerNameTx(e execer, ownerID int64, base string) string {
	name := base
	n := 2
	for {
		var cnt int
		if err := e.QueryRow(
			`SELECT count(*) FROM ledgers WHERE owner_id=? AND name=?`, ownerID, name,
		).Scan(&cnt); err != nil || cnt == 0 {
			return name
		}
		name = fmt.Sprintf("%s (%d)", base, n)
		n++
	}
}

// accountType 从 Beancount 账户名推断五大类（前缀匹配，默认 Expenses）。
func accountType(name string) string {
	switch {
	case strings.HasPrefix(name, "Assets:"):
		return domain.TypeAssets
	case strings.HasPrefix(name, "Liabilities:"):
		return domain.TypeLiabilities
	case strings.HasPrefix(name, "Equity:"):
		return domain.TypeEquity
	case strings.HasPrefix(name, "Income:"):
		return domain.TypeIncome
	default:
		return domain.TypeExpenses
	}
}
