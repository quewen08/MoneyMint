package service

import (
	"time"

	"familyledger/internal/domain"
)

// LedgerView 是账本列表/详情的返回视图（含当前用户角色与期初日期）。
type LedgerView struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	DefaultCommodity string `json:"default_commodity,omitempty"`
	OpeningDate      string `json:"opening_date"`
	Role             string `json:"role"`
}

// ListLedgers 列出当前用户所属的全部账本及其角色。
func (s *Service) ListLedgers(userID int64) ([]LedgerView, error) {
	ledgers, err := s.repo.ListLedgersByUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]LedgerView, 0, len(ledgers))
	for _, l := range ledgers {
		out = append(out, LedgerView{
			ID:               l.ID,
			Name:             l.Name,
			DefaultCommodity: l.DefaultCommodity,
			OpeningDate:      l.OpeningDate,
			Role:             l.Role,
		})
	}
	return out, nil
}

// CreateLedger 创建新账本，创建者成为 owner；期初日期默认今日（FR-01）。
func (s *Service) CreateLedger(userID int64, name, defaultCommodity string) (LedgerView, error) {
	if name == "" {
		return LedgerView{}, domain.Invalidf("账本名称必填")
	}
	id, err := s.repo.CreateLedger(userID, name, defaultCommodity)
	if err != nil {
		return LedgerView{}, err
	}
	// 建账成功后写入默认账户（支出/收入/资产/负债/权益），失败不阻断建账。
	if err := s.repo.SeedDefaultAccounts(id); err != nil {
		return LedgerView{}, err
	}
	l, err := s.repo.LedgerByID(id)
	if err != nil {
		return LedgerView{}, err
	}
	return LedgerView{
		ID:               id,
		Name:             name,
		DefaultCommodity: defaultCommodity,
		OpeningDate:      l.OpeningDate,
		Role:             domain.RoleOwner,
	}, nil
}

// UpdateLedger 更新账本名称、默认币种与可选的期初日期（仅 owner）。
//
// openingDate 非 nil 时（PATCH 显式传入）：
//   - 必须是合法 YYYY-MM-DD，且 ≤ 今日、不晚于账本最早非期初交易；
//   - 保存后单事务重建全部有效期初（冲正旧 + 新建新日期），并只下调未关闭账户的 open_date。
func (s *Service) UpdateLedger(userID, ledgerID int64, name, defaultCommodity string, openingDate *string) (LedgerView, error) {
	if name == "" {
		return LedgerView{}, domain.Invalidf("账本名称必填")
	}
	if err := s.requireOwner(ledgerID, userID); err != nil {
		return LedgerView{}, err
	}
	if openingDate != nil {
		d := *openingDate
		if !isISODate(d) {
			return LedgerView{}, domain.Codedf(domain.ErrInvalid, domain.CodeLedgerOpeningDateInvalid,
				"opening_date 需为 YYYY-MM-DD")
		}
		today := time.Now().Format("2006-01-02")
		if d > today {
			return LedgerView{}, domain.Codedf(domain.ErrInvalid, domain.CodeLedgerOpeningDateInvalid,
				"opening_date 不能晚于今日 %s", today)
		}
		earliest, err := s.repo.EarliestNonOpeningTxnDate(ledgerID)
		if err != nil {
			return LedgerView{}, err
		}
		if earliest != "" && d > earliest {
			return LedgerView{}, domain.Codedf(domain.ErrInvalid, domain.CodeLedgerOpeningDateAfterTxn,
				"opening_date 不能晚于账本最早交易 %s", earliest)
		}
	}

	ok, err := s.repo.UpdateLedger(ledgerID, name, defaultCommodity)
	if err != nil {
		return LedgerView{}, err
	}
	if !ok {
		return LedgerView{}, domain.ErrNotFound
	}
	if openingDate != nil {
		if _, err := s.RebuildOpenings(ledgerID, *openingDate); err != nil {
			return LedgerView{}, err
		}
	}

	l, err := s.repo.LedgerByID(ledgerID)
	if err != nil {
		return LedgerView{}, err
	}
	return LedgerView{
		ID:               ledgerID,
		Name:             name,
		DefaultCommodity: l.DefaultCommodity,
		OpeningDate:      l.OpeningDate,
		Role:             domain.RoleOwner,
	}, nil
}
