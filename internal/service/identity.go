package service

import (
	"familyledger/internal/identity"
)

// CheckIdentity 执行单账本恒等式检查（只读，不修改任何数据）。
func (s *Service) CheckIdentity(ledgerID int64) (identity.Result, error) {
	return identity.Check(s.repo.DB(), ledgerID)
}
