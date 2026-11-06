package service

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
	"time"

	"familyledger/internal/domain"
	"familyledger/internal/repository"
)

// OpeningInput 是设置期初的请求体（前端只传绝对值 + 方向，负号一律由后端推导）。
type OpeningInput struct {
	Amount    string `json:"amount"`    // 非负定点十进制字符串
	Direction string `json:"direction"` // hold（持有）| owe（欠款）
	Commodity string `json:"commodity"` // 可选；必须与账户/账本解析结果一致
	Date      string `json:"date"`      // 可选；必须等于账本期初日期
}

// OpeningView 是查询期初的返回。
type OpeningView struct {
	HasOpening      bool   `json:"has_opening"`
	AccountUUID     string `json:"account_uuid"`
	Amount          string `json:"amount,omitempty"`        // 绝对值
	Direction       string `json:"direction,omitempty"`     // hold | owe
	SignedAmount    string `json:"signed_amount,omitempty"` // 账本符号（欠款为负）
	Commodity       string `json:"commodity,omitempty"`
	Version         int64  `json:"version,omitempty"`
	Date            string `json:"date,omitempty"`
	TransactionUUID string `json:"transaction_uuid,omitempty"`
}

// OpeningResult 是设置期初的返回。
type OpeningResult struct {
	Status          string `json:"status"` // created | updated | cleared | unchanged
	AccountUUID     string `json:"account_uuid"`
	TransactionUUID string `json:"transaction_uuid,omitempty"`
	ReversalUUID    string `json:"reversal_uuid,omitempty"`
	Version         int64  `json:"version"`
	SignedAmount    string `json:"signed_amount"`
	Commodity       string `json:"commodity"`
	Date            string `json:"date"`
}

var (
	isoDateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	amountRe   = regexp.MustCompile(`^\d+(?:\.\d+)?$`)
)

// GetOpening 读取账户当前有效期的期初（不存在时 has_opening=false）。
func (s *Service) GetOpening(ledgerID int64, accountUUID string) (OpeningView, error) {
	acc, err := s.repo.AccountByUUID(ledgerID, accountUUID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return OpeningView{}, domain.Codedf(domain.ErrNotFound, domain.CodeAccountNotFound, "账户不存在")
		}
		return OpeningView{}, err
	}
	if acc.Type != "Assets" && acc.Type != "Liabilities" {
		return OpeningView{HasOpening: false, AccountUUID: accountUUID}, nil
	}
	rec, found, err := s.repo.ActiveOpening(ledgerID, accountUUID)
	if err != nil {
		return OpeningView{}, err
	}
	if !found {
		return OpeningView{HasOpening: false, AccountUUID: accountUUID}, nil
	}
	prec, err := s.repo.CommodityPrecision(ledgerID, rec.Commodity)
	if err != nil {
		return OpeningView{}, err
	}
	signed, err := parseRat(rec.SignedAmount)
	if err != nil {
		return OpeningView{}, err
	}
	amount := new(big.Rat).Abs(signed).FloatString(prec)
	direction := domain.OpeningDirectionHold
	if signed.Sign() < 0 {
		direction = domain.OpeningDirectionOwe
	}
	return OpeningView{
		HasOpening:      true,
		AccountUUID:     accountUUID,
		Amount:          amount,
		Direction:       direction,
		SignedAmount:    signed.FloatString(prec),
		Commodity:       rec.Commodity,
		Version:         rec.Version,
		Date:            rec.Date,
		TransactionUUID: rec.UUID,
	}, nil
}

// SetOpening 设置/修正/清空账户期初（在线专用）。
//   - 仅 Assets / Liabilities 可设，且账户必须处于打开态；
//   - 金额只接受非负值，方向由服务层映射为账本符号（欠款为负）；
//   - 币种优先级：账户限制 → 账本默认 → CNY；
//   - 日期固定取 ledgers.opening_date；
//   - 编辑时「冲正旧 + 新建」同事务，任一失败整体回滚。
func (s *Service) SetOpening(ledgerID int64, accountUUID string, in OpeningInput) (OpeningResult, error) {
	acc, err := s.repo.AccountByUUID(ledgerID, accountUUID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return OpeningResult{}, domain.Codedf(domain.ErrNotFound, domain.CodeAccountNotFound, "账户不存在")
		}
		return OpeningResult{}, err
	}
	if acc.CloseDate != "" {
		return OpeningResult{}, domain.Codedf(domain.ErrConflict, domain.CodeAccountClosed,
			"账户已关闭，无法设置期初")
	}
	if acc.Type != "Assets" && acc.Type != "Liabilities" {
		return OpeningResult{}, domain.Codedf(domain.ErrInvalid, domain.CodeOpeningAccountNotOpenable,
			"仅资产/负债账户可设置期初余额")
	}

	ledger, err := s.repo.LedgerByID(ledgerID)
	if err != nil {
		return OpeningResult{}, err
	}
	commodity := resolvedCommodity(acc, ledger.DefaultCommodity)
	if in.Commodity != "" && in.Commodity != commodity {
		return OpeningResult{}, domain.Codedf(domain.ErrInvalid, domain.CodeOpeningCommodityMismatch,
			"币种必须为 %s（账户限制/账本默认），不接受 %s", commodity, in.Commodity)
	}

	direction := strings.TrimSpace(in.Direction)
	if direction != domain.OpeningDirectionHold && direction != domain.OpeningDirectionOwe {
		return OpeningResult{}, domain.Codedf(domain.ErrInvalid, domain.CodeOpeningDirectionInvalid,
			"direction 仅支持 hold 或 owe")
	}
	raw := strings.TrimSpace(in.Amount)
	if raw == "" || !amountRe.MatchString(raw) {
		return OpeningResult{}, domain.Codedf(domain.ErrInvalid, domain.CodeOpeningAmountInvalid,
			"金额必须为非负的定点十进制字符串（如 100 或 100.50）")
	}
	r, ok := new(big.Rat).SetString(raw)
	if !ok {
		return OpeningResult{}, domain.Codedf(domain.ErrInvalid, domain.CodeOpeningAmountInvalid,
			"金额格式非法")
	}

	openingDate, err := s.repo.LedgerOpeningDate(ledgerID)
	if err != nil {
		return OpeningResult{}, err
	}
	if in.Date != "" && in.Date != openingDate {
		return OpeningResult{}, domain.Codedf(domain.ErrInvalid, domain.CodeOpeningDateMismatch,
			"期初日期必须为账本期初日期 %s", openingDate)
	}
	prec, err := s.repo.CommodityPrecision(ledgerID, commodity)
	if err != nil {
		return OpeningResult{}, err
	}
	absStr := r.FloatString(prec)
	signed := absStr
	if direction == domain.OpeningDirectionOwe && r.Sign() != 0 {
		signed = "-" + absStr
	}

	applied, err := s.repo.ApplyOpening(ledgerID, repository.OpeningApplyInput{
		AccountID:      acc.ID,
		AccountUUID:    accountUUID,
		SignedAmount:   signed,
		Commodity:      commodity,
		Date:           openingDate,
		Direction:      direction,
		AbsoluteAmount: absStr,
		Precision:      prec,
	})
	if err != nil {
		return OpeningResult{}, err
	}
	return OpeningResult{
		Status:          applied.Status,
		AccountUUID:     accountUUID,
		TransactionUUID: applied.TransactionUUID,
		ReversalUUID:    applied.ReversalUUID,
		Version:         applied.Version,
		SignedAmount:    applied.SignedAmount,
		Commodity:       applied.Commodity,
		Date:            applied.Date,
	}, nil
}

// RebuildOpenings 在单事务内平移账本期初日期并重建全部有效期初。
func (s *Service) RebuildOpenings(ledgerID int64, newDate string) (int, error) {
	if !isISODate(newDate) {
		return 0, domain.Codedf(domain.ErrInvalid, domain.CodeLedgerOpeningDateInvalid,
			"opening_date 需为 YYYY-MM-DD")
	}
	return s.repo.RebuildAllOpenings(ledgerID, newDate)
}

// isISODate 严格校验 YYYY-MM-DD（拒绝 2026-02-30 之类）。
func isISODate(s string) bool {
	if !isoDateRe.MatchString(s) {
		return false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return false
	}
	return t.Format("2006-01-02") == s
}

// containsTag 大小写不敏感判断标签是否包含指定值。
func containsTag(tags []string, want string) bool {
	for _, t := range tags {
		if strings.EqualFold(strings.TrimSpace(t), want) {
			return true
		}
	}
	return false
}

// resolvedCommodity 按「账户限制 → 账本默认 → CNY」解析期初币种。
func resolvedCommodity(acc domain.Account, ledgerDefault string) string {
	if acc.Restriction != "" {
		return acc.Restriction
	}
	if ledgerDefault != "" {
		return ledgerDefault
	}
	return "CNY"
}

// parseRat 把定点十进制字符串解析为精确有理数。
func parseRat(s string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return nil, domain.Invalidf("金额格式非法")
	}
	return r, nil
}
