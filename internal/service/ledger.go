package service

import (
	"familyledger/internal/domain"
)

// LedgerView 是账本列表/详情的返回视图（含当前用户角色）。
type LedgerView struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	DefaultCommodity string `json:"default_commodity,omitempty"`
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
			Role:             l.Role,
		})
	}
	return out, nil
}

// CreateLedger 创建新账本，创建者成为 owner。
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
	return LedgerView{ID: id, Name: name, DefaultCommodity: defaultCommodity, Role: domain.RoleOwner}, nil
}

// UpdateLedger 更新账本名称与默认币种（仅 owner）。
func (s *Service) UpdateLedger(userID, ledgerID int64, name, defaultCommodity string) (LedgerView, error) {
	if name == "" {
		return LedgerView{}, domain.Invalidf("账本名称必填")
	}
	if err := s.requireOwner(ledgerID, userID); err != nil {
		return LedgerView{}, err
	}
	ok, err := s.repo.UpdateLedger(ledgerID, name, defaultCommodity)
	if err != nil {
		return LedgerView{}, err
	}
	if !ok {
		return LedgerView{}, domain.ErrNotFound
	}
	return LedgerView{ID: ledgerID, Name: name, DefaultCommodity: defaultCommodity, Role: domain.RoleOwner}, nil
}
