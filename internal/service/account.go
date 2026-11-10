package service

import (
	"time"

	"familyledger/internal/domain"

	"github.com/google/uuid"
)

// AccountView 是账户列表项的返回视图（含按币种余额）。
type AccountView struct {
	ID          int64             `json:"id"`
	UUID        string            `json:"uuid"` // 批 2：列表透出 uuid（前端期初编辑/关户/删除依赖）
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name,omitempty"`
	Type        string            `json:"type"`
	OpenDate    string            `json:"open_date"`
	CloseDate   string            `json:"close_date,omitempty"` // 非空表示已关闭（Beancount close）
	Restriction string            `json:"commodity_restriction,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Color       string            `json:"color,omitempty"`
	ParentUUID  string            `json:"parent_uuid,omitempty"`
	SubType     string            `json:"sub_type,omitempty"`
	SortOrder   int64             `json:"sort_order"` // 0.4-C 分类拖拽排序（同类型内升序）
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
	Icon        string `json:"icon,omitempty"`
	Color       string `json:"color,omitempty"`
	ParentUUID  string `json:"parent_uuid,omitempty"`
	SubType     string `json:"sub_type,omitempty"`
}

// ListAccounts 列出账本内账户及其按币种余额。includeClosed=false 仅未关闭账户。
func (s *Service) ListAccounts(ledgerID int64, includeClosed bool) ([]AccountView, error) {
	accounts, err := s.repo.ListAccounts(ledgerID, includeClosed)
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
			UUID:        a.UUID,
			Name:        a.Name,
			DisplayName: a.DisplayName,
			Type:        a.Type,
			OpenDate:    a.OpenDate,
			CloseDate:   a.CloseDate,
			Restriction: a.Restriction,
			Icon:        a.Icon,
			Color:       a.Color,
			ParentUUID:  a.ParentUUID,
			SubType:     a.SubType,
			SortOrder:   a.SortOrder,
			Balances:    balances[a.ID],
		})
	}
	return out, nil
}

// CreateAccount 创建账户（含币种限制时自动补建币种）。
// icon/color/parentUUID/subType 为展示/层级字段，可选。
// FR-03：未显式传日期时 open_date = min(账本期初日期, 今日)；
// 显式日期不得晚于账本期初日期。
func (s *Service) CreateAccount(ledgerID int64, name, displayName, atype, openDate, commodity, icon, color, parentUUID, subType string) (AccountCreated, error) {
	if name == "" || atype == "" {
		return AccountCreated{}, domain.Invalidf("name 与 type 必填")
	}
	if err := validateAccountType(atype); err != nil {
		return AccountCreated{}, err
	}
	openingDate, err := s.repo.LedgerOpeningDate(ledgerID)
	if err != nil {
		return AccountCreated{}, err
	}
	today := time.Now().Format("2006-01-02")
	if openingDate == "" || openingDate > today {
		openingDate = today
	}
	if openDate == "" {
		openDate = openingDate
	} else {
		if !isISODate(openDate) {
			return AccountCreated{}, domain.Invalidf("open_date 需为 YYYY-MM-DD")
		}
		if openDate > openingDate {
			return AccountCreated{}, domain.Codedf(domain.ErrInvalid, domain.CodeAccountOpenDateAfter,
				"账户开立日期不得晚于账本期初日期 %s", openingDate)
		}
	}

	a := domain.Account{
		UUID:        uuid.NewString(),
		Name:        name,
		DisplayName: displayName,
		Type:        atype,
		OpenDate:    openDate,
		Restriction: commodity,
		Icon:        icon,
		Color:       color,
		ParentUUID:  parentUUID,
		SubType:     subType,
	}
	id, err := s.repo.InsertAccount(ledgerID, a)
	if err != nil {
		return AccountCreated{}, err
	}
	return AccountCreated{
		ID: id, UUID: a.UUID, Name: name, DisplayName: displayName, Type: atype,
		OpenDate: openDate, Icon: icon, Color: color, ParentUUID: parentUUID, SubType: subType,
	}, nil
}

// DeleteAccount 关闭账户（Beancount close 语义，0.4-A 起）。
// 关闭前必须余额为 0（设计决策 §7.3）：非 0 返回 BalanceNotZeroError（409 + 按币种差额）。
// 校验通过则置 close_date，保留全部交易引用；sync_log 写 op=close。
func (s *Service) DeleteAccount(ledgerID int64, uuid string) error {
	closed, diffs, err := s.repo.CloseAccountChecked(ledgerID, uuid)
	if err != nil {
		return err
	}
	if len(diffs) > 0 {
		return &domain.BalanceNotZeroError{AccountUUID: uuid, Balances: diffs}
	}
	if !closed {
		return domain.ErrNotFound
	}
	return nil
}
