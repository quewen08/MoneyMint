package service

import (
	"fmt"

	"familyledger/internal/beancount"
	"familyledger/internal/domain"
)

// ImportPreview 是导入预检结果（只读，不写库）。
//
// Valid=false 且 Errors 非空：文件含不支持语法，整体拒绝（列出全部错误行号）。
// Duplicate=true：疑似重复导入（同名账本 + 相同交易数），仅提示，不阻断。
type ImportPreview struct {
	Valid        bool                   `json:"valid"`
	Title        string                 `json:"title"`
	LedgerName   string                 `json:"ledger_name"` // 实际将使用的账本名（去重后）
	Accounts     int                    `json:"accounts"`
	Transactions int                    `json:"transactions"`
	Prices       int                    `json:"prices"`
	Commodities  int                    `json:"commodities"`
	Duplicate    bool                   `json:"duplicate"`
	Errors       []beancount.ParseError `json:"errors,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

// ImportResult 是导入执行结果。
type ImportResult struct {
	LedgerID     int64  `json:"ledger_id"`
	LedgerName   string `json:"ledger_name"`
	Accounts     int    `json:"accounts"`
	Transactions int    `json:"transactions"`
}

// PreviewImport 解析 Beancount 文本并给出导入预览（不写库）。
func (s *Service) PreviewImport(userID int64, text string) (*ImportPreview, error) {
	p, err := beancount.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalid, err.Error())
	}
	prev := &ImportPreview{
		Title:        p.Title,
		Accounts:     len(p.Accounts),
		Transactions: len(p.Transactions),
		Prices:       len(p.Prices),
		Commodities:  len(p.Commodities),
	}
	if len(p.Errors) > 0 {
		prev.Valid = false
		prev.Errors = p.Errors
		prev.Error = "文件含不支持的语法，已整体拒绝"
		return prev, nil
	}
	prev.Valid = true
	base := p.Title
	if base == "" {
		base = "导入账本"
	}
	prev.LedgerName = s.repo.UniqueLedgerName(userID, base)
	dup, err := s.repo.IsDuplicateImport(userID, base, len(p.Transactions))
	if err == nil {
		prev.Duplicate = dup
	}
	return prev, nil
}

// ExecuteImport 执行导入：解析 → 校验 → 单一大事务写入新账本。
func (s *Service) ExecuteImport(userID int64, text string) (*ImportResult, error) {
	p, err := beancount.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalid, err.Error())
	}
	if len(p.Errors) > 0 {
		return nil, domain.Invalidf("文件含不支持的语法，已整体拒绝")
	}
	ledgerID, name, err := s.repo.ImportBeancount(userID, p)
	if err != nil {
		return nil, err
	}
	return &ImportResult{
		LedgerID:     ledgerID,
		LedgerName:   name,
		Accounts:     len(p.Accounts),
		Transactions: len(p.Transactions),
	}, nil
}
