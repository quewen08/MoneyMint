package service

import (
	"strings"
	"time"

	"familyledger/internal/domain"

	"github.com/google/uuid"
)

// PostingView 是交易列表项中分录的返回视图。
type PostingView struct {
	AccountID int64  `json:"account_id"`
	Account   string `json:"account"`
	Commodity string `json:"commodity"`
	Amount    string `json:"amount"`
}

// TransactionView 是交易列表项的返回视图。
type TransactionView struct {
	ID            int64         `json:"id"`
	Date          string        `json:"date"`
	Flag          string        `json:"flag"`
	Description   string        `json:"description"`
	CreatedByName string        `json:"created_by_name"` // 记账人显示名；无创建者时为空串
	Postings      []PostingView `json:"postings"`
}

// TransactionCreated 是创建交易的返回。
type TransactionCreated struct {
	ID       int64  `json:"id"`
	UUID     string `json:"uuid"`
	Date     string `json:"date"`
	Postings int    `json:"postings"`
}

// ListTransactions 列出账本内所有未删除交易（含分录，内联账户名）。
func (s *Service) ListTransactions(ledgerID int64) ([]TransactionView, error) {
	txns, err := s.repo.ListTransactions(ledgerID)
	if err != nil {
		return nil, err
	}
	out := make([]TransactionView, 0, len(txns))
	for _, t := range txns {
		v := TransactionView{
			ID:            t.ID,
			Date:          t.Date,
			Flag:          t.Flag,
			Description:   t.Description,
			CreatedByName: t.CreatedByName,
			Postings:      make([]PostingView, 0, len(t.Postings)),
		}
		for _, p := range t.Postings {
			v.Postings = append(v.Postings, PostingView{
				AccountID: p.AccountID,
				Account:   p.AccountName,
				Commodity: p.Commodity,
				Amount:    p.Amount,
			})
		}
		out = append(out, v)
	}
	return out, nil
}

// CreateTransaction 创建一笔不可变交易（在线，postings 用 account_id 引用）。
// 先做借贷平衡校验，再在事务内落库并补建币种。
// tags 中禁止出现 opening：期初只能走专用端点，防止绕过方向/币种/唯一性校验。
func (s *Service) CreateTransaction(ledgerID int64, date, flag, description string, postings []PostingInput, createdBy int64, tags ...string) (TransactionCreated, error) {
	if containsTag(tags, "opening") {
		return TransactionCreated{}, domain.Codedf(domain.ErrInvalid, domain.CodeOpeningViaAPIOnly,
			"期初余额请通过账户期初专用端点设置")
	}
	if len(postings) < 2 {
		return TransactionCreated{}, domain.Invalidf("一笔交易至少需要 2 条分录")
	}
	if err := validateBalance(postings); err != nil {
		return TransactionCreated{}, err
	}
	if flag == "" {
		flag = "*"
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	txn := domain.Transaction{
		UUID:        uuid.NewString(),
		Date:        date,
		Flag:        flag,
		Description: description,
		Tags:        append([]string(nil), tags...),
		CreatedBy:   createdBy,
		Postings:    make([]domain.Posting, 0, len(postings)),
	}
	for i, p := range postings {
		txn.Postings = append(txn.Postings, domain.Posting{
			AccountID: p.AccountID,
			Commodity: p.Commodity,
			Amount:    p.Amount,
			Position:  i,
		})
	}

	id, err := s.repo.CreateTransaction(ledgerID, txn)
	if err != nil {
		return TransactionCreated{}, err
	}
	return TransactionCreated{ID: id, UUID: txn.UUID, Date: date, Postings: len(postings)}, nil
}

// TrashPostingView 是回收站分录视图：account 保留 Beancount 全名，
// account_display 是给人看的账户名（「现金」）。
type TrashPostingView struct {
	Account        string `json:"account"`
	AccountDisplay string `json:"account_display"`
	Commodity      string `json:"commodity"`
	Amount         string `json:"amount"`
}

// TrashItemView 是回收站条目视图（在普通交易视图上加删除时间）。
type TrashItemView struct {
	UUID          string             `json:"uuid"`
	Date          string             `json:"date"`
	Flag          string             `json:"flag"`
	Description   string             `json:"description"`
	Tags          []string           `json:"tags"`
	DeletedAt     string             `json:"deleted_at"`
	CreatedByName string             `json:"created_by_name"`
	Postings      []TrashPostingView `json:"postings"`
}

// ListDeletedTransactions 列出回收站（已软删交易），按删除时间倒序。
func (s *Service) ListDeletedTransactions(ledgerID int64) ([]TrashItemView, error) {
	txns, err := s.repo.ListDeletedTransactions(ledgerID)
	if err != nil {
		return nil, err
	}
	out := make([]TrashItemView, 0, len(txns))
	for _, t := range txns {
		v := TrashItemView{
			UUID:          t.UUID,
			Date:          t.Date,
			Flag:          t.Flag,
			Description:   t.Description,
			Tags:          t.Tags,
			DeletedAt:     t.DeletedAt,
			CreatedByName: t.CreatedByName,
			Postings:      make([]TrashPostingView, 0, len(t.Postings)),
		}
		for _, p := range t.Postings {
			display := p.AccountDisplay
			if display == "" {
				display = p.AccountName
			}
			v.Postings = append(v.Postings, TrashPostingView{
				Account:        p.AccountName,
				AccountDisplay: display,
				Commodity:      p.Commodity,
				Amount:         p.Amount,
			})
		}
		out = append(out, v)
	}
	return out, nil
}

// RestoreTransaction 从回收站恢复一笔交易。
//
// 恢复前校验引用账户：已关闭账户不允许再被记账引用，若交易的任一科目已关闭，
// 直接拒绝并告知是哪个账户，而不是恢复出一条账本内不可再创建的脏数据。
func (s *Service) RestoreTransaction(ledgerID int64, uuid string) error {
	closed, err := s.repo.ClosedAccountsOfTransaction(ledgerID, uuid)
	if err != nil {
		return err
	}
	if len(closed) > 0 {
		return domain.Invalidf("交易引用的账户「%s」已关闭，无法恢复（可先重新开启该账户或改为冲正）", strings.Join(closed, "」「"))
	}
	restored, err := s.repo.RestoreTransaction(ledgerID, uuid)
	if err != nil {
		return err
	}
	if !restored {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteTransaction 软删一笔交易。
func (s *Service) DeleteTransaction(ledgerID int64, uuid string) error {
	deleted, err := s.repo.SoftDeleteTransaction(ledgerID, uuid)
	if err != nil {
		return err
	}
	if !deleted {
		return domain.ErrNotFound
	}
	return nil
}
