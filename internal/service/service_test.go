package service

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"familyledger/internal/db"
	"familyledger/internal/domain"
	"familyledger/internal/repository"
)

// newTestService 用临时 SQLite 文件构造真实 Service（走完整迁移），
// 供集成测试使用，无需 mock。
func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return New(repository.New(conn))
}

func registerOwner(t *testing.T, svc *Service) AuthResult {
	t.Helper()
	res, err := svc.Register("alice", "secret123", "Alice")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return res
}

// lid 是测试中首个账本（首个注册用户自动创建）的 id。
const lid = domain.DefaultLedgerID

func accountByID(t *testing.T, svc *Service, id int64) AccountView {
	t.Helper()
	accounts, err := svc.ListAccounts(lid)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	for _, a := range accounts {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("account %d not found", id)
	return AccountView{}
}

func TestRegisterLoginOwner(t *testing.T) {
	svc := newTestService(t)

	res := registerOwner(t, svc)
	if res.User.Role != domain.RoleOwner {
		t.Fatalf("首个用户应为 owner，实际 %q", res.User.Role)
	}
	if res.Token == "" {
		t.Fatal("注册应返回 token")
	}

	// 第二个用户仅建账号，不加入账本
	res2, err := svc.Register("bob", "secret123", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if res2.User.Role != "" {
		t.Fatalf("后续用户 role 应为空，实际 %q", res2.User.Role)
	}

	// 登录成功（多账本后 role 不再随登录返回，恒为空串）
	lr, err := svc.Login("alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if lr.User.Role != "" {
		t.Fatalf("多账本后登录 role 应为空串，实际 %q", lr.User.Role)
	}

	// 错误密码
	if _, err := svc.Login("alice", "wrong"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("错误密码应返回 ErrUnauthorized，实际 %v", err)
	}

	// 重复用户名
	if _, err := svc.Register("alice", "secret123", "A"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("重复用户名应返回 ErrConflict，实际 %v", err)
	}
}

func TestAccountTransactionBalanceExport(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	food, err := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY")
	if err != nil {
		t.Fatalf("create food: %v", err)
	}

	// 重名账户被拒
	if _, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("重名账户应返回 ErrConflict，实际 %v", err)
	}

	// 非法类型被拒
	if _, err := svc.CreateAccount(lid, "X:Bad", "", "Nope", "2026-08-01", ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("非法类型应返回 ErrInvalid，实际 %v", err)
	}

	// 记一笔
	_, err = svc.CreateTransaction(lid, "2026-08-02", "*", "早餐", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "10.00"},
	}, 1)
	if err != nil {
		t.Fatalf("create txn: %v", err)
	}

	if got := accountByID(t, svc, cash.ID).Balances["CNY"]; got != "-10.00" {
		t.Fatalf("cash 余额应为 -10.00，实际 %q", got)
	}
	if got := accountByID(t, svc, food.ID).Balances["CNY"]; got != "10.00" {
		t.Fatalf("food 余额应为 10.00，实际 %q", got)
	}

	// 列表接口
	txns, err := svc.ListTransactions(lid)
	if err != nil {
		t.Fatalf("list txns: %v", err)
	}
	if len(txns) != 1 || len(txns[0].Postings) != 2 {
		t.Fatalf("交易列表应为 1 笔 2 分录，实际 %+v", txns)
	}

	// 导出
	text, err := svc.ExportLedger(lid)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	for _, want := range []string{`option "title"`, "Assets:Cash:CNY", "Expenses:Food:CNY", "早餐", "commodity CNY"} {
		if !strings.Contains(text, want) {
			t.Fatalf("导出应包含 %q，实际：\n%s", want, text)
		}
	}
}

func TestBalanceValidationRejectsUnbalanced(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY")
	food, _ := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY")

	// 借贷不平衡
	_, err := svc.CreateTransaction(lid, "2026-08-02", "*", "坏账", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "9.00"},
	}, 1)
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("不平衡应返回 ErrInvalid，实际 %v", err)
	}

	// 少于 2 条分录
	_, err = svc.CreateTransaction(lid, "2026-08-02", "*", "单分录", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-10.00"},
	}, 1)
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("单分录应返回 ErrInvalid，实际 %v", err)
	}
}

func TestDeleteSemantics(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY")
	food, _ := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY")

	created, _ := svc.CreateTransaction(lid, "2026-08-02", "*", "早餐", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "10.00"},
	}, 1)

	// 删除交易（幂等）
	if err := svc.DeleteTransaction(lid, created.UUID); err != nil {
		t.Fatalf("delete txn: %v", err)
	}
	if err := svc.DeleteTransaction(lid, created.UUID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("重复删除应返回 ErrNotFound，实际 %v", err)
	}
	if txns, _ := svc.ListTransactions(lid); len(txns) != 0 {
		t.Fatalf("删除后交易应为 0，实际 %d", len(txns))
	}

	// 再记一笔，然后删除账户：其引用交易应被连带删除
	_, _ = svc.CreateTransaction(lid, "2026-08-03", "*", "买菜", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-5.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "5.00"},
	}, 1)
	if err := svc.DeleteAccount(lid, cash.UUID); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if txns, _ := svc.ListTransactions(lid); len(txns) != 0 {
		t.Fatalf("删账户后其引用交易应被连带删除，剩余 %d 笔", len(txns))
	}

	// 软删后可重建同名账户
	rebuilt, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-04", "CNY")
	if err != nil {
		t.Fatalf("软删后重建同名账户应成功，实际 %v", err)
	}
	if rebuilt.UUID == cash.UUID {
		t.Fatalf("重建账户应有新 uuid")
	}
}

func TestSyncPushPullAndNaturalKeyMerge(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	// 在线先建一个「先到者」账户
	existing, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY")

	// push：同名账户合并 + 引用缺失的交易失败
	req := PushRequest{
		ClientID: "devA",
		Changes: []PushChange{
			{EntityType: domain.EntityAccount, Entity: jsonRaw(t, map[string]any{
				"uuid": "loc-cash", "name": "Assets:Cash:CNY", "type": "Assets", "open_date": "2026-08-01",
			})},
			{EntityType: domain.EntityTransaction, Entity: jsonRaw(t, map[string]any{
				"uuid": "loc-txn", "date": "2026-08-02", "flag": "*", "description": "早餐",
				"postings": []map[string]string{
					{"account_uuid": "loc-food", "commodity": "CNY", "amount": "-10.00"},
					{"account_uuid": "nope", "commodity": "CNY", "amount": "10.00"},
				},
			})},
			{EntityType: domain.EntityAccount, Entity: jsonRaw(t, map[string]any{
				"uuid": "loc-food", "name": "Expenses:Coffee", "type": "Expenses", "open_date": "2026-08-01",
			})},
		},
	}
	res, err := svc.SyncPush(lid, req)
	if err != nil {
		t.Fatalf("sync push: %v", err)
	}
	if res.Accepted != 1 {
		t.Fatalf("accepted 应为 1（仅 loc-food 真正插入），实际 %d", res.Accepted)
	}
	// 结果按请求顺序：loc-cash 合并、loc-txn 失败、loc-food 创建
	if res.Results[0].ServerUUID != existing.UUID {
		t.Fatalf("同名账户应合并返回先到者 uuid %q，实际 %q", existing.UUID, res.Results[0].ServerUUID)
	}
	if res.Results[1].Status != "error" {
		t.Fatalf("引用缺失交易应 error，实际 %q", res.Results[1].Status)
	}
	if res.Results[2].Status != "ok" || !res.Results[2].Accepted {
		t.Fatalf("loc-food 应创建成功，实际 %+v", res.Results[2])
	}

	// pull 应包含 loc-food 与后续 create 事件，不含 loc-cash（合并未落库）
	pull, err := svc.SyncPull(lid, 0, "devA")
	if err != nil {
		t.Fatalf("sync pull: %v", err)
	}
	joined := ""
	for _, c := range pull.Changes {
		joined += string(c.Entity)
	}
	if strings.Contains(joined, "loc-cash") {
		t.Fatalf("pull 不应含 loc-cash（合并未落库）")
	}
	if !strings.Contains(joined, "loc-food") {
		t.Fatalf("pull 应含 loc-food create")
	}
}

func TestMemberManagement(t *testing.T) {
	svc := newTestService(t)
	owner := registerOwner(t, svc)
	bob, _ := svc.Register("bob", "secret123", "Bob")

	// bob 非成员：AuthorizeLedger 拒绝
	if err := svc.AuthorizeLedger(bob.User.ID, lid); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("非成员应 ErrForbidden，实际 %v", err)
	}

	// owner 添加成员
	if _, _, err := svc.AddMember(lid, owner.User.ID, "bob", domain.RoleEditor); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := svc.AuthorizeLedger(bob.User.ID, lid); err != nil {
		t.Fatalf("添加后应为成员，实际 %v", err)
	}

	// 改角色
	if err := svc.UpdateMemberRole(lid, owner.User.ID, bob.User.ID, domain.RoleViewer); err != nil {
		t.Fatalf("update role: %v", err)
	}

	// 非 owner 不能管理成员
	if _, err := svc.ListMembers(lid, bob.User.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("非 owner 应 ErrForbidden，实际 %v", err)
	}

	members, err := svc.ListMembers(lid, owner.User.ID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("成员数应为 2，实际 %d", len(members))
	}

	// 移除成员
	if err := svc.RemoveMember(lid, owner.User.ID, bob.User.ID); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if err := svc.AuthorizeLedger(bob.User.ID, lid); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("移除后应非成员，实际 %v", err)
	}
}

// jsonRaw 把对象序列化为 push 实体 JSON。
func jsonRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
