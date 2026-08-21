package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"familyledger/internal/domain"
)

// ---- 同步协议传输类型 ----

// SyncChangeOut 是 pull 响应中的一条变更。
type SyncChangeOut struct {
	Seq        int64           `json:"seq"`
	EntityType string          `json:"entity_type"`
	Op         string          `json:"op"`
	Entity     json.RawMessage `json:"entity"`
}

// PullResult 是 pull 响应。
type PullResult struct {
	Checkpoint int64           `json:"checkpoint"`
	Changes    []SyncChangeOut `json:"changes"`
}

// PushChange 是 push 请求中的一条变更。
type PushChange struct {
	EntityType string          `json:"entity_type"`
	Op         string          `json:"op"`
	Entity     json.RawMessage `json:"entity"`
}

// PushRequest 是 push 请求。
type PushRequest struct {
	ClientID string       `json:"client_id"`
	Since    int64        `json:"since"`
	Changes  []PushChange `json:"changes"`
}

// PushResult 描述 push 中一条变更的逐条结果。
type PushResult struct {
	Index      int    `json:"index"`
	EntityType string `json:"entity_type"`
	UUID       string `json:"uuid,omitempty"`
	Status     string `json:"status"`
	Accepted   bool   `json:"accepted"`
	ServerUUID string `json:"server_uuid,omitempty"`
	Error      string `json:"error,omitempty"`
}

// PushResultOut 是 push 响应。
type PushResultOut struct {
	Checkpoint int64        `json:"checkpoint"`
	Accepted   int          `json:"accepted"`
	Results    []PushResult `json:"results"`
}

// pushApplyResult 描述一次推送变更的应用结果。
type pushApplyResult struct {
	accepted   bool
	serverUUID string
}

// SyncPull 返回 since 之后的所有变更（水位增量），并记录客户端水位。
func (s *Service) SyncPull(ledgerID, since int64, clientID string) (PullResult, error) {
	entries, err := s.repo.SyncLogSince(ledgerID, since)
	if err != nil {
		return PullResult{}, err
	}

	changes := make([]SyncChangeOut, 0, len(entries))
	var maxSeq int64
	for _, e := range entries {
		if e.Seq > maxSeq {
			maxSeq = e.Seq
		}
		var payload json.RawMessage
		if e.Op == domain.OpDelete {
			payload, _ = json.Marshal(map[string]string{"uuid": e.EntityUUID})
		} else {
			payload, err = s.loadEntity(ledgerID, e.EntityType, e.EntityUUID)
			if errors.Is(err, domain.ErrNotFound) {
				continue // 实体已不存在（如被连带删除），跳过该条避免 pull 失败
			}
			if err != nil {
				return PullResult{}, err
			}
		}
		changes = append(changes, SyncChangeOut{Seq: e.Seq, EntityType: e.EntityType, Op: e.Op, Entity: payload})
	}

	if clientID != "" && maxSeq > 0 {
		if err := s.repo.UpsertCheckpoint(ledgerID, clientID, maxSeq); err != nil {
			return PullResult{}, err
		}
	}
	return PullResult{Checkpoint: maxSeq, Changes: changes}, nil
}

// loadEntity 取出某实体的全量 JSON（引用账户一律用 uuid，便于跨端合并）。
func (s *Service) loadEntity(ledgerID int64, typ, uuid string) (json.RawMessage, error) {
	switch typ {
	case domain.EntityAccount:
		a, err := s.repo.AccountByUUID(ledgerID, uuid)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"uuid":                  a.UUID,
			"name":                  a.Name,
			"display_name":          nullIfEmpty(a.DisplayName),
			"type":                  a.Type,
			"open_date":             a.OpenDate,
			"close_date":            nullIfEmpty(a.CloseDate),
			"commodity_restriction": nullIfEmpty(a.Restriction),
			"icon":                  nullIfEmpty(a.Icon),
			"color":                 nullIfEmpty(a.Color),
			"parent_uuid":           nullIfEmpty(a.ParentUUID),
			"sub_type":              nullIfEmpty(a.SubType),
			"sort_order":            a.SortOrder,
		})
	case domain.EntityCommodity:
		c, err := s.repo.CommodityByUUID(ledgerID, uuid)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"uuid":      c.UUID,
			"symbol":    c.Symbol,
			"name":      nullIfEmpty(c.Name),
			"precision": c.Precision,
		})
	case domain.EntityTransaction:
		t, err := s.repo.TransactionByUUID(ledgerID, uuid)
		if err != nil {
			return nil, err
		}
		postings := make([]map[string]string, 0, len(t.Postings))
		for _, p := range t.Postings {
			postings = append(postings, map[string]string{
				"account_uuid": p.AccountUUID,
				"commodity":    p.Commodity,
				"amount":       p.Amount,
			})
		}
		return json.Marshal(map[string]any{
			"uuid":           t.UUID,
			"date":           t.Date,
			"flag":           t.Flag,
			"description":    t.Description,
			"tags":           t.Tags,
			"created_by_name": nullIfEmpty(t.CreatedByName),
			"postings":       postings,
		})
	default:
		return nil, fmt.Errorf("未知实体类型 %q", typ)
	}
}

// SyncPush 逐条幂等提交离线变更：单条失败不阻塞其余，响应逐条结果。
// userID 为当前请求用户 id，用作离线推送交易的 created_by（服务端权威归因，不信任客户端）。
func (s *Service) SyncPush(ledgerID, userID int64, req PushRequest) (PushResultOut, error) {
	results := make([]PushResult, 0, len(req.Changes))
	accepted := 0

	apply := func(idx int, c PushChange) {
		op := c.Op
		if op == "" {
			op = domain.OpCreate
		}
		pr := PushResult{Index: idx, EntityType: c.EntityType}
		if op != domain.OpCreate && op != domain.OpDelete && op != domain.OpClose && op != domain.OpUpdate {
			pr.Status = "error"
			pr.Error = fmt.Sprintf("非法 op %q", op)
			results = append(results, pr)
			return
		}
		var idField struct {
			UUID string `json:"uuid"`
		}
		_ = json.Unmarshal(c.Entity, &idField)
		pr.UUID = idField.UUID

		res, err := s.applyPushEntity(ledgerID, userID, c.EntityType, op, c.Entity)
		if err != nil {
			pr.Status = "error"
			pr.Error = err.Error()
			results = append(results, pr)
			return
		}
		pr.Status = "ok"
		pr.Accepted = res.accepted
		pr.ServerUUID = res.serverUUID
		if res.accepted {
			accepted++
		}
		results = append(results, pr)
	}

	// 分拣顺序：先所有 create（被引用实体先落库），再所有 delete（交易），
	// 最后所有 close（账户关闭 = Beancount close，0.4-A 起）。
	for _, pass := range []struct{ op, typ string }{
		{domain.OpCreate, domain.EntityCommodity},
		{domain.OpCreate, domain.EntityAccount},
		{domain.OpCreate, domain.EntityTransaction},
		{domain.OpDelete, domain.EntityTransaction},
		{domain.OpClose, domain.EntityAccount},
		{domain.OpUpdate, domain.EntityAccount}, // 0.4-C 账户排序等字段更新
	} {
		for i, c := range req.Changes {
			op := c.Op
			if op == "" {
				op = domain.OpCreate
			}
			if op == pass.op && c.EntityType == pass.typ {
				apply(i, c)
			}
		}
	}

	// results 按请求原始顺序返回
	sort.Slice(results, func(i, j int) bool { return results[i].Index < results[j].Index })

	maxSeq, err := s.repo.MaxSyncSeq(ledgerID)
	if err != nil {
		return PushResultOut{}, err
	}
	if req.ClientID != "" {
		if err := s.repo.UpsertCheckpoint(ledgerID, req.ClientID, maxSeq); err != nil {
			return PushResultOut{}, err
		}
	}
	return PushResultOut{Checkpoint: maxSeq, Accepted: accepted, Results: results}, nil
}

// applyPushEntity 按 op 应用一条推送：create 幂等写入（账户/币种按自然键合并），
// delete 软删（仅交易），close 关闭账户（置 close_date，0.4-A）。
func (s *Service) applyPushEntity(ledgerID, userID int64, typ, op string, raw json.RawMessage) (pushApplyResult, error) {
	if op == domain.OpDelete {
		return s.applyDeleteEntity(ledgerID, typ, raw)
	}
	if op == domain.OpClose {
		return s.applyCloseEntity(ledgerID, typ, raw)
	}
	if op == domain.OpUpdate {
		return s.applyUpdateEntity(ledgerID, typ, raw)
	}
	switch typ {
	case domain.EntityAccount:
		var e struct {
			UUID        string `json:"uuid"`
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
			OpenDate    string `json:"open_date"`
			Restriction string `json:"commodity_restriction"`
			Icon        string `json:"icon"`
			Color       string `json:"color"`
			ParentUUID  string `json:"parent_uuid"`
			SubType     string `json:"sub_type"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return pushApplyResult{}, err
		}
		if e.UUID == "" || e.Name == "" || e.Type == "" {
			return pushApplyResult{}, domain.Invalidf("账户推送缺少 uuid/name/type")
		}
		// ① 同 uuid 已存在：幂等跳过。
		exists, err := s.repo.AccountUUIDExists(ledgerID, e.UUID)
		if err != nil {
			return pushApplyResult{}, err
		}
		if exists {
			return pushApplyResult{}, nil
		}
		// ② 自然键合并：同账本存在未关闭同名账户 → 返回先到者 uuid。
		//    若同名账户已关闭，不合并（关闭账户不可再记账），走 ③ INSERT（触发同名唯一约束冲突）。
		existing, err := s.repo.AccountNameExists(ledgerID, e.Name)
		if err != nil {
			return pushApplyResult{}, err
		}
		if existing != "" {
			acc, err := s.repo.AccountByUUID(ledgerID, existing)
			if err != nil {
				return pushApplyResult{}, err
			}
			if acc.CloseDate == "" {
				return pushApplyResult{serverUUID: existing}, nil
			}
			// 同名账户已关闭：不合并，继续走 INSERT（将因同名唯一约束失败而报错，符合预期）。
		}
		// ③ 真正插入。
		if _, err := s.repo.InsertAccount(ledgerID, domain.Account{
			UUID:        e.UUID,
			Name:        e.Name,
			DisplayName: e.DisplayName,
			Type:        e.Type,
			OpenDate:    e.OpenDate,
			Restriction: e.Restriction,
			Icon:        e.Icon,
			Color:       e.Color,
			ParentUUID:  e.ParentUUID,
			SubType:     e.SubType,
		}); err != nil {
			return pushApplyResult{}, err
		}
		return pushApplyResult{accepted: true}, nil

	case domain.EntityCommodity:
		var e struct {
			UUID      string `json:"uuid"`
			Symbol    string `json:"symbol"`
			Name      string `json:"name"`
			Precision int    `json:"precision"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return pushApplyResult{}, err
		}
		if e.UUID == "" || e.Symbol == "" {
			return pushApplyResult{}, domain.Invalidf("币种推送缺少 uuid/symbol")
		}
		exists, err := s.repo.CommodityUUIDExists(ledgerID, e.UUID)
		if err != nil {
			return pushApplyResult{}, err
		}
		if exists {
			return pushApplyResult{}, nil
		}
		existing, err := s.repo.CommoditySymbolExists(ledgerID, e.Symbol)
		if err != nil {
			return pushApplyResult{}, err
		}
		if existing != "" {
			return pushApplyResult{serverUUID: existing}, nil
		}
		if err := s.repo.InsertCommodity(ledgerID, domain.Commodity{
			UUID:      e.UUID,
			Symbol:    e.Symbol,
			Name:      e.Name,
			Precision: e.Precision,
		}); err != nil {
			return pushApplyResult{}, err
		}
		return pushApplyResult{accepted: true}, nil

	case domain.EntityTransaction:
		var e struct {
			UUID        string   `json:"uuid"`
			Date        string   `json:"date"`
			Flag        string   `json:"flag"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
			Postings    []struct {
				AccountUUID string `json:"account_uuid"`
				Commodity   string `json:"commodity"`
				Amount      string `json:"amount"`
			} `json:"postings"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return pushApplyResult{}, err
		}
		if e.UUID == "" || len(e.Postings) < 2 {
			return pushApplyResult{}, domain.Invalidf("交易推送缺少 uuid 或分录不足")
		}
		// 借贷平衡（服务端权威校验）。
		inputs := make([]PostingInput, 0, len(e.Postings))
		for _, p := range e.Postings {
			inputs = append(inputs, PostingInput{Commodity: p.Commodity, Amount: p.Amount})
		}
		if err := validateBalance(inputs); err != nil {
			return pushApplyResult{}, err
		}

		flag := e.Flag
		if flag == "" {
			flag = "*"
		}
		date := e.Date
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		txn := domain.Transaction{UUID: e.UUID, Date: date, Flag: flag, Description: e.Description, Tags: e.Tags, CreatedBy: userID}
		for _, p := range e.Postings {
			accID, err := s.repo.AccountIDByUUID(ledgerID, p.AccountUUID)
			if err != nil {
				return pushApplyResult{}, fmt.Errorf("推送交易引用的账户 %s 不存在: %w", p.AccountUUID, err)
			}
			txn.Postings = append(txn.Postings, domain.Posting{
				AccountID: accID,
				Commodity: p.Commodity,
				Amount:    p.Amount,
			})
		}
		created, err := s.repo.SavePushTransaction(ledgerID, txn)
		if err != nil {
			return pushApplyResult{}, err
		}
		return pushApplyResult{accepted: created}, nil

	default:
		return pushApplyResult{}, fmt.Errorf("未知实体类型 %q", typ)
	}
}

// applyDeleteEntity 处理 op=delete 的推送（实体只需 uuid）。
// 0.4-A 起账户不再支持 delete（改为 close）；此处仅处理交易软删。
// 兼容：旧客户端若对账户发 op=delete，按 close 语义处理（关闭账户）。
func (s *Service) applyDeleteEntity(ledgerID int64, typ string, raw json.RawMessage) (pushApplyResult, error) {
	var e struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return pushApplyResult{}, err
	}
	if e.UUID == "" {
		return pushApplyResult{}, domain.Invalidf("删除变更缺少 uuid")
	}
	switch typ {
	case domain.EntityAccount:
		// 向前兼容：旧客户端 op=delete account → 按 close 处理。
		closed, err := s.repo.CloseAccount(ledgerID, e.UUID)
		return pushApplyResult{accepted: closed}, err
	case domain.EntityTransaction:
		deleted, err := s.repo.SoftDeleteTransaction(ledgerID, e.UUID)
		return pushApplyResult{accepted: deleted}, err
	default:
		return pushApplyResult{}, fmt.Errorf("不支持删除实体类型 %q", typ)
	}
}

// applyCloseEntity 处理 op=close 的推送（账户关闭，置 close_date）。
func (s *Service) applyCloseEntity(ledgerID int64, typ string, raw json.RawMessage) (pushApplyResult, error) {
	var e struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return pushApplyResult{}, err
	}
	if e.UUID == "" {
		return pushApplyResult{}, domain.Invalidf("关闭变更缺少 uuid")
	}
	switch typ {
	case domain.EntityAccount:
		closed, err := s.repo.CloseAccount(ledgerID, e.UUID)
		return pushApplyResult{accepted: closed}, err
	default:
		return pushApplyResult{}, fmt.Errorf("不支持关闭实体类型 %q", typ)
	}
}

// applyUpdateEntity 处理 op=update 的推送（0.4-C 起，仅账户排序等字段更新）。
// 前端已知账户 uuid，无需自然键合并；只更新已存在账户的 sort_order（当前唯一可更新字段）。
func (s *Service) applyUpdateEntity(ledgerID int64, typ string, raw json.RawMessage) (pushApplyResult, error) {
	var e struct {
		UUID      string `json:"uuid"`
		SortOrder int64  `json:"sort_order"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return pushApplyResult{}, err
	}
	if e.UUID == "" {
		return pushApplyResult{}, domain.Invalidf("更新变更缺少 uuid")
	}
	switch typ {
	case domain.EntityAccount:
		if err := s.repo.UpdateAccountSortOrder(ledgerID, e.UUID, e.SortOrder); err != nil {
			return pushApplyResult{}, err
		}
		return pushApplyResult{accepted: true}, nil
	default:
		return pushApplyResult{}, fmt.Errorf("不支持更新实体类型 %q", typ)
	}
}

// nullIfEmpty 空串转 nil（与旧 loadEntity 的 nullToNil 语义一致，JSON 输出 null）。
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
