package service

import (
	"encoding/json"
	"strings"
	"testing"

	"familyledger/internal/db"
	"familyledger/internal/domain"
)

// TestSyncResponsesCarryInstanceID 验证迁移 014 库世代在同步三通道透出
// （next-phase 方案 §621：health / pull / push 响应追加 instance_id，additive）。
//
// restore 会重写 instance_id（restore.Execute → db.RotateInstanceID），
// 客户端比对该字段不一致时必须清空该账本本地实体并全量重新 pull，
// 否则会沿用旧水位、静默丢失 restore 带来的数据变更。
func TestSyncResponsesCarryInstanceID(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	want, err := db.GetInstanceID(svc.repo.DB())
	if err != nil {
		t.Fatalf("读取 instance_id: %v", err)
	}

	// pull：instance_id 与库内一致。
	pull, err := svc.SyncPull(lid, 0, "instance-test")
	if err != nil {
		t.Fatalf("SyncPull: %v", err)
	}
	if pull.InstanceID != want {
		t.Fatalf("pull.instance_id = %q, want %q", pull.InstanceID, want)
	}

	// push：同样携带 instance_id（空 push 也必须带，客户端首连即取世代）。
	push, err := svc.SyncPush(lid, 1, PushRequest{ClientID: "instance-test"})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if push.InstanceID != want {
		t.Fatalf("push.instance_id = %q, want %q", push.InstanceID, want)
	}
}

// TestPullJSONShapeHasInstanceID 用 JSON 序列化断言 wire 格式确实含
// instance_id 字段（防「字段加了但 tag 漏了」的静默回归）。
func TestPullJSONShapeHasInstanceID(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	pull, err := svc.SyncPull(lid, 0, "")
	if err != nil {
		t.Fatalf("SyncPull: %v", err)
	}
	raw, err := json.Marshal(pull)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"instance_id"`) {
		t.Fatalf("pull 响应 JSON 缺少 instance_id 字段: %s", raw)
	}
}

// TestPushAdjudicationCodes 验证 push 裁决的稳定错误码（§6.6.1 #1/#5，
// 客户端死信分类依赖这张码表）：
//   - 分录引用账户不存在 → code=account_not_found（客户端 pull 后重映射重试）
//   - 分录引用已关闭账户 → code=account_closed（Beancount close 语义，转 dead）
func TestPushAdjudicationCodes(t *testing.T) {
	svc := newTestService(t)
	res := registerOwner(t, svc)

	accounts, err := svc.ListAccounts(lid, false)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) < 1 {
		t.Fatalf("默认账户未种子化")
	}
	cash := accounts[0]

	// ① 引用不存在的账户 → account_not_found
	push, err := svc.SyncPush(lid, res.User.ID, PushRequest{
		ClientID: "adjudication",
		Changes: []PushChange{{
			EntityType: "transaction",
			Entity: mustJSON(t, map[string]any{
				"uuid": "txn-missing-acc", "date": "2026-01-01", "flag": "*",
				"postings": []map[string]any{
					{"account_uuid": "no-such-uuid", "commodity": "CNY", "amount": "1.00"},
					{"account_uuid": cash.UUID, "commodity": "CNY", "amount": "-1.00"},
				},
			}),
		}},
	})
	if err != nil {
		t.Fatalf("SyncPush(不存在账户): %v", err)
	}
	if push.Results[0].Status != "error" || push.Results[0].Code != domain.CodeAccountNotFound {
		t.Fatalf("应返回 account_not_found，实际 %+v", push.Results[0])
	}

	// ② 关闭现金账户（余额 0 可关）后记账 → account_closed
	_, err = svc.SyncPush(lid, res.User.ID, PushRequest{
		ClientID: "adjudication",
		Changes: []PushChange{{EntityType: "account", Op: "close", Entity: mustJSON(t, map[string]any{"uuid": cash.UUID})}},
	})
	if err != nil {
		t.Fatalf("SyncPush(close): %v", err)
	}
	push2, err := svc.SyncPush(lid, res.User.ID, PushRequest{
		ClientID: "adjudication",
		Changes: []PushChange{{
			EntityType: "transaction",
			Entity: mustJSON(t, map[string]any{
				"uuid": "txn-closed-acc", "date": "2026-01-01", "flag": "*",
				"postings": []map[string]any{
					{"account_uuid": cash.UUID, "commodity": "CNY", "amount": "1.00"},
					{"account_uuid": accounts[1].UUID, "commodity": "CNY", "amount": "-1.00"},
				},
			}),
		}},
	})
	if err != nil {
		t.Fatalf("SyncPush(已关闭账户): %v", err)
	}
	if push2.Results[0].Status != "error" || push2.Results[0].Code != domain.CodeAccountClosed {
		t.Fatalf("应返回 account_closed，实际 %+v", push2.Results[0])
	}
}

// mustJSON 序列化测试实体；失败直接 Fatal。
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}
