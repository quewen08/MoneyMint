package service

import (
	"time"

	"familyledger/internal/domain"

	"github.com/google/uuid"
)

// AccountView 是账户列表项的返回视图（含按币种余额）。
type AccountView struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name,omitempty"`
	Type        string            `json:"type"`
	OpenDate    string            `json:"open_date"`
	Restriction string            `json:"commodity_restriction,omitempty"`
	Balances    map[string]string `json:"balances,omitempty"`
}

// AccountCreated 是创建账户的返回。
type AccountCreated struct {
	ID          int64  `json:"id"`
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Type        string `json:"type"`
	OpenDate    string `json:"open_date"`
}

// ListAccounts 列出账本内所有账户及其按币种余额。
func (s *Service) ListAccounts(ledgerID int64) ([]AccountView, error) {
	accounts, err := s.repo.ListAccounts(ledgerID)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.ComputeBalances(ledgerID)
	if err != nil {
		return nil, err
	}

	out := make([]AccountView, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, AccountView{
			ID:          a.ID,
			Name:        a.Name,
			DisplayName: a.DisplayName,
			Type:        a.Type,
			OpenDate:    a.OpenDate,
			Restriction: a.Restriction,
			Balances:    balances[a.ID],
		})
	}
	return out, nil
}

// CreateAccount 创建账户（含币种限制时自动补建币种）。
func (s *Service) CreateAccount(ledgerID int64, name, displayName, atype, openDate, commodity string) (AccountCreated, error) {
	if name == "" || atype == "" {
		return AccountCreated{}, domain.Invalidf("name 与 type 必填")
	}
	if err := validateAccountType(atype); err != nil {
		return AccountCreated{}, err
	}
	if openDate == "" {
		openDate = time.Now().Format("2006-01-02")
	}

	a := domain.Account{
		UUID:        uuid.NewString(),
		Name:        name,
		DisplayName: displayName,
		Type:        atype,
		OpenDate:    openDate,
		Restriction: commodity,
	}
	id, err := s.repo.InsertAccount(ledgerID, a)
	if err != nil {
		return AccountCreated{}, err
	}
	return AccountCreated{ID: id, UUID: a.UUID, Name: name, DisplayName: displayName, Type: atype, OpenDate: openDate}, nil
}

// DeleteAccount 软删账户（连带其引用交易）。
func (s *Service) DeleteAccount(ledgerID int64, uuid string) error {
	deleted, err := s.repo.SoftDeleteAccount(ledgerID, uuid)
	if err != nil {
		return err
	}
	if !deleted {
		return domain.ErrNotFound
	}
	return nil
}
