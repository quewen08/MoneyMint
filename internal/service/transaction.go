package service

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"
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
//
// Idempotent=true 表示本次请求命中了既有幂等键、没有新增交易：
// handler 据此返回 200（新建为 201），uuid/id 与首次成功时一致，
// 客户端可安全重放（AC-23）。为兼容旧前端，该字段作为附加字段输出。
type TransactionCreated struct {
	ID         int64  `json:"id"`
	UUID       string `json:"uuid"`
	Date       string `json:"date"`
	Postings   int    `json:"postings"`
	Idempotent bool   `json:"idempotent"`
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
//
// 不带幂等键的旧调用路径（离线 push 走 sync 包，不经过这里）。
func (s *Service) CreateTransaction(ledgerID int64, date, flag, description string, postings []PostingInput, createdBy int64, tags ...string) (TransactionCreated, error) {
	return s.createTransaction(ledgerID, "", date, flag, description, postings, createdBy, tags)
}

// CreateTransactionIdempotent 在线创建交易，带可选客户端幂等键（G-04 / AC-23）。
//
// 幂等键来源：HTTP 层优先取 Idempotency-Key 头，其次 body 的 client_request_id；
// 空串表示调用方不要求幂等（保持旧行为）。
//
// 语义：
//   - 首次成功 → 新建（handler 返回 201）；
//   - 重复提交且未删除、内容一致 → 返回既有 uuid/id 且 Idempotent=true（200）；
//   - 命中已软删交易 → 409 idempotency_key_deleted（换新键重提）；
//   - 同键不同内容 → 409 idempotency_payload_mismatch。
func (s *Service) CreateTransactionIdempotent(
	ledgerID int64, clientRequestID, date, flag, description string,
	postings []PostingInput, createdBy int64, tags ...string,
) (TransactionCreated, error) {
	key, err := normalizeClientRequestID(clientRequestID)
	if err != nil {
		return TransactionCreated{}, err
	}
	return s.createTransaction(ledgerID, key, date, flag, description, postings, createdBy, tags)
}

// createTransaction 是 CreateTransaction / CreateTransactionIdempotent 的公共实现。
func (s *Service) createTransaction(
	ledgerID int64, clientRequestID, date, flag, description string,
	postings []PostingInput, createdBy int64, tags []string,
) (TransactionCreated, error) {
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

	payloadHash := transactionPayloadHash(date, flag, description, postings)
	res, err := s.repo.CreateTransactionIdempotent(ledgerID, txn, clientRequestID, payloadHash)
	if err != nil {
		return TransactionCreated{}, err
	}
	if res.Created {
		return TransactionCreated{
			ID: res.ID, UUID: res.UUID, Date: res.Date,
			Postings: res.Postings, Idempotent: false,
		}, nil
	}
	// 命中既有幂等键：先判软删，再判内容是否一致。
	if res.Deleted {
		return TransactionCreated{}, domain.Codedf(domain.ErrConflict, domain.CodeIdempotencyKeyDeleted,
			"该幂等键对应的交易已被删除，请生成新的幂等键后重试")
	}
	if res.Hash != "" && res.Hash != payloadHash {
		return TransactionCreated{}, domain.Codedf(domain.ErrConflict, domain.CodeIdempotencyPayloadMismatch,
			"该幂等键已用于另一笔内容不同的交易，请生成新的幂等键后重试")
	}
	return TransactionCreated{
		ID: res.ID, UUID: res.UUID, Date: res.Date,
		Postings: res.Postings, Idempotent: true,
	}, nil
}

// maxClientRequestIDLen 是幂等键的长度上限（防止把任意长文本写进索引）。
const maxClientRequestIDLen = 200

// normalizeClientRequestID 规范化并校验客户端幂等键。
// 去掉首尾空白；空串合法（表示不启用幂等）；控制字符与超长直接拒绝。
func normalizeClientRequestID(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", nil
	}
	if len(key) > maxClientRequestIDLen {
		return "", domain.Invalidf("client_request_id 不能超过 %d 个字符", maxClientRequestIDLen)
	}
	for _, r := range key {
		if r < 0x20 || r == 0x7f {
			return "", domain.Invalidf("client_request_id 不能包含控制字符")
		}
	}
	return key, nil
}

// transactionPayloadHash 计算规范化后的请求内容哈希，用于识别「同键不同内容」。
//
// 规范化范围：date / flag / description / 分录（账户 id、币种、金额）。
// 金额用 big.Rat 转成精确规范值再参与哈希，避免 "10.00" 与 "10.0" 被误判为不同内容；
// 该值仅用于比较，不参与存储与展示（金额仍以原始十进制字符串落库）。
func transactionPayloadHash(date, flag, description string, postings []PostingInput) string {
	var b strings.Builder
	b.WriteString("fambean-txn-v1\x00")
	b.WriteString(date)
	b.WriteByte(0)
	b.WriteString(flag)
	b.WriteByte(0)
	b.WriteString(description)
	b.WriteByte(0)
	for _, p := range postings {
		b.WriteString(strconv.FormatInt(p.AccountID, 10))
		b.WriteByte(0)
		b.WriteString(p.Commodity)
		b.WriteByte(0)
		b.WriteString(normalizeAmountForHash(p.Amount))
		b.WriteByte(0)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// normalizeAmountForHash 把十进制金额字符串转成精确规范形式；解析失败时退回去空白原值。
func normalizeAmountForHash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if r, ok := new(big.Rat).SetString(s); ok {
		return r.RatString()
	}
	return s
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
