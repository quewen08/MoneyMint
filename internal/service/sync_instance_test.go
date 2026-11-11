package service

import (
	"encoding/json"
	"strings"
	"testing"

	"familyledger/internal/db"
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
