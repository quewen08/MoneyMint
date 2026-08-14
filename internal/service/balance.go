package service

import (
	"math/big"

	"familyledger/internal/domain"
)

// PostingInput 是创建/推送交易时的分录输入（金额为有符号定点十进制字符串）。
type PostingInput struct {
	AccountID   int64
	AccountUUID string
	Commodity   string
	Amount      string
}

// validateBalance 校验同一交易内按币种分组的金额之和必须为 0（借贷平衡）。
// 使用 big.Rat 做精确十进制求和，避免浮点误差。
func validateBalance(postings []PostingInput) error {
	sums := map[string]*big.Rat{}
	for _, p := range postings {
		r := new(big.Rat)
		if _, ok := r.SetString(p.Amount); !ok {
			return domain.Invalidf("非法金额 %q", p.Amount)
		}
		if _, ok := sums[p.Commodity]; !ok {
			sums[p.Commodity] = new(big.Rat)
		}
		sums[p.Commodity].Add(sums[p.Commodity], r)
	}
	for c, s := range sums {
		if s.Sign() != 0 {
			return domain.Invalidf("交易不平衡：币种 %s 借贷差为 %s，必须为 0", c, s.RatString())
		}
	}
	return nil
}

// validateAccountType 校验账户类型属于 Beancount 五大类。
func validateAccountType(t string) error {
	switch t {
	case domain.TypeAssets, domain.TypeLiabilities, domain.TypeEquity, domain.TypeIncome, domain.TypeExpenses:
		return nil
	default:
		return domain.Invalidf("非法账户类型 %q", t)
	}
}
