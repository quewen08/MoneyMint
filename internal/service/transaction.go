package service

import (
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
	ID          int64         `json:"id"`
	Date        string        `json:"date"`
	Flag        string        `json:"flag"`
	Description string        `json:"description"`
	Postings    []PostingView `json:"postings"`
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
			ID:          t.ID,
			Date:        t.Date,
			Flag:        t.Flag,
			Description: t.Description,
			Postings:    make([]PostingView, 0, len(t.Postings)),
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
func (s *Service) CreateTransaction(ledgerID int64, date, flag, description string, postings []PostingInput, createdBy int64) (TransactionCreated, error) {
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
