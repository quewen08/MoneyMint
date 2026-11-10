package service

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	accounts, err := svc.ListAccounts(lid, false)
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

// TestFirstUserBecomesAdmin 首个注册用户自动成为系统管理员，后续用户不是。
//
// 管理员承担全库级操作（备份下载/恢复），而账本级 owner 只有单账本权限，
// 二者不能混用，否则会出现「A 家庭 owner 能下载 B 家庭账本」的越权。
func TestFirstUserBecomesAdmin(t *testing.T) {
	svc := newTestService(t)

	first := registerOwner(t, svc)
	if !first.User.IsAdmin {
		t.Fatal("首个注册用户应成为系统管理员")
	}

	second, err := svc.Register("bob", "secret123", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if second.User.IsAdmin {
		t.Fatal("第二个用户不应成为系统管理员")
	}

	// Me 也应返回一致的标记（前端据此控制备份入口可见性）
	me, err := svc.Me(first.User.ID)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if !me.IsAdmin {
		t.Fatal("Me 应返回 is_admin=true")
	}
	meBob, err := svc.Me(second.User.ID)
	if err != nil {
		t.Fatalf("me bob: %v", err)
	}
	if meBob.IsAdmin {
		t.Fatal("bob 的 Me 不应返回 is_admin=true")
	}

	// 登录路径同样带回标记
	lr, err := svc.Login("alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !lr.User.IsAdmin {
		t.Fatal("登录响应应包含 is_admin=true")
	}

	// IsAdmin 查询接口
	ok, err := svc.IsAdmin(first.User.ID)
	if err != nil || !ok {
		t.Fatalf("IsAdmin(alice) 期望 true, nil；实际 %v, %v", ok, err)
	}
	ok, err = svc.IsAdmin(second.User.ID)
	if err != nil || ok {
		t.Fatalf("IsAdmin(bob) 期望 false, nil；实际 %v, %v", ok, err)
	}
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

	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	food, err := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create food: %v", err)
	}

	// 重名账户被拒
	if _, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY", "", "", "", ""); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("重名账户应返回 ErrConflict，实际 %v", err)
	}

	// 非法类型被拒
	if _, err := svc.CreateAccount(lid, "X:Bad", "", "Nope", "2026-08-01", "", "", "", "", ""); !errors.Is(err, domain.ErrInvalid) {
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

	cash, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY", "", "", "", "")
	food, _ := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY", "", "", "", "")

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

	cash, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY", "", "", "", "")
	food, _ := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY", "", "", "", "")

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

	// 再记两笔（买菜支出 + 报销回收），使现金账户余额归零后再关闭：
	// 新语义要求 close 前余额必须为 0，否则返回 409 account_balance_not_zero。
	_, _ = svc.CreateTransaction(lid, "2026-08-03", "*", "买菜", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-5.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "5.00"},
	}, 1)
	_, _ = svc.CreateTransaction(lid, "2026-08-04", "*", "报销", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "5.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "-5.00"},
	}, 1)
	if err := svc.DeleteAccount(lid, cash.UUID); err != nil {
		t.Fatalf("close account: %v", err)
	}
	// close 后引用交易应保留（不再连带软删）
	if txns, _ := svc.ListTransactions(lid); len(txns) != 2 {
		t.Fatalf("关账户后其引用交易应保留（close 语义），实际 %d 笔", len(txns))
	}
	// 已关闭账户默认列表不出现，include_closed 才出现且带 close_date
	open, _ := svc.ListAccounts(lid, false)
	for _, a := range open {
		if a.ID == cash.ID {
			t.Fatalf("已关闭账户不应出现在默认列表")
		}
	}
	closed, _ := svc.ListAccounts(lid, true)
	var foundClosed bool
	for _, a := range closed {
		if a.ID == cash.ID {
			if a.CloseDate == "" {
				t.Fatalf("已关闭账户应有 close_date")
			}
			foundClosed = true
		}
	}
	if !foundClosed {
		t.Fatalf("已关闭账户应出现在 include_closed 列表")
	}

	// close 后重建同名应失败（Beancount 不允许 close 后再 open 同名，账户名仍占用）
	_, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-04", "CNY", "", "", "", "")
	if err == nil {
		t.Fatalf("close 后重建同名账户应失败（账户名仍占用）")
	}
}

func TestForcedPasswordChange(t *testing.T) {
	svc := newTestService(t)
	owner := registerOwner(t, svc)

	// 自己注册的账号不置位（密码是自己选的）
	if me, _ := svc.Me(owner.User.ID); me.MustChangePassword {
		t.Fatalf("自己注册的账号不应要求强制改密")
	}

	// 造一个待重置的成员
	bob, err := svc.Register("bobby", "bob123456", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if _, _, err := svc.AddMember(lid, owner.User.ID, "bobby", domain.RoleEditor); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// owner 重置成员密码 → 该成员被要求下次登录先改密
	if err := svc.ResetMemberPassword(lid, owner.User.ID, bob.User.ID, "temp123456"); err != nil {
		t.Fatalf("reset member password: %v", err)
	}
	if me, _ := svc.Me(bob.User.ID); !me.MustChangePassword {
		t.Fatalf("管理员重置密码后应要求强制改密")
	}

	// 登录 → 会话主体也带该标记（中间件据此 428）
	login, err := svc.Login("bobby", "temp123456")
	if err != nil {
		t.Fatalf("login bob: %v", err)
	}
	sess, err := svc.Authenticate(login.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if !sess.MustChangePassword {
		t.Fatalf("会话主体应带 must_change_password 标记")
	}

	// 自己改密 → 标记解除，新会话不再被拦
	if _, err := svc.ChangePassword(bob.User.ID, "temp123456", "ownpass123"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if me, _ := svc.Me(bob.User.ID); me.MustChangePassword {
		t.Fatalf("自己改密后应清除强制改密标记")
	}
	login2, _ := svc.Login("bobby", "ownpass123")
	sess2, _ := svc.Authenticate(login2.Token)
	if sess2.MustChangePassword {
		t.Fatalf("改密后的新会话不应再要求改密")
	}

	// 预设 owner / CLI 重置走的是同一条标记入口
	if err := svc.MarkPasswordChangeRequired("bobby"); err != nil {
		t.Fatalf("mark required: %v", err)
	}
	if me, _ := svc.Me(bob.User.ID); !me.MustChangePassword {
		t.Fatalf("MarkPasswordChangeRequired 应置位")
	}
	if err := svc.MarkPasswordChangeRequired("nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("标记不存在的用户应返回 ErrNotFound，实际 %v", err)
	}
}

func TestTrashRestore(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY", "", "", "", "")
	food, _ := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY", "", "", "", "")
	created, _ := svc.CreateTransaction(lid, "2026-08-02", "*", "早餐", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "10.00"},
	}, 1)

	// 未删除时回收站为空
	if trash, err := svc.ListDeletedTransactions(lid); err != nil || len(trash) != 0 {
		t.Fatalf("未删除时回收站应为空，实际 %d（err=%v）", len(trash), err)
	}

	if err := svc.DeleteTransaction(lid, created.UUID); err != nil {
		t.Fatalf("delete txn: %v", err)
	}
	trash, err := svc.ListDeletedTransactions(lid)
	if err != nil {
		t.Fatalf("list trash: %v", err)
	}
	if len(trash) != 1 {
		t.Fatalf("回收站应有 1 条，实际 %d", len(trash))
	}
	if trash[0].UUID != created.UUID || trash[0].DeletedAt == "" {
		t.Fatalf("回收站条目应带 uuid 与删除时间，实际 %+v", trash[0])
	}
	if len(trash[0].Postings) != 2 {
		t.Fatalf("回收站条目应保留分录，实际 %d 条", len(trash[0].Postings))
	}
	if txns, _ := svc.ListTransactions(lid); len(txns) != 0 {
		t.Fatalf("已删除交易不应出现在正常列表，实际 %d", len(txns))
	}

	// 恢复：回到正常列表、回收站清空、重复恢复幂等返回 NotFound
	if err := svc.RestoreTransaction(lid, created.UUID); err != nil {
		t.Fatalf("restore txn: %v", err)
	}
	if txns, _ := svc.ListTransactions(lid); len(txns) != 1 {
		t.Fatalf("恢复后正常列表应有 1 条，实际 %d", len(txns))
	}
	if trash, _ := svc.ListDeletedTransactions(lid); len(trash) != 0 {
		t.Fatalf("恢复后回收站应为空，实际 %d", len(trash))
	}
	if err := svc.RestoreTransaction(lid, created.UUID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("重复恢复应返回 ErrNotFound，实际 %v", err)
	}

	// 恢复必须对其他设备可见：sync_log 里应有一条 transaction/create 指向该 uuid
	pull, err := svc.SyncPull(lid, 0, "trash-test")
	if err != nil {
		t.Fatalf("sync pull: %v", err)
	}
	var restoreSeen bool
	for _, c := range pull.Changes {
		if c.EntityType == domain.EntityTransaction && c.Op == domain.OpCreate &&
			strings.Contains(string(c.Entity), created.UUID) {
			restoreSeen = true
		}
	}
	if !restoreSeen {
		t.Fatalf("恢复后应写入 sync_log op=create（否则其他设备拉不回这笔交易）")
	}

	// 引用已关闭账户的交易不允许恢复（否则账本内出现不可再创建的脏数据）
	if err := svc.DeleteTransaction(lid, created.UUID); err != nil {
		t.Fatalf("delete again: %v", err)
	}
	if err := svc.DeleteAccount(lid, cash.UUID); err != nil {
		t.Fatalf("close account: %v", err)
	}
	err = svc.RestoreTransaction(lid, created.UUID)
	if err == nil {
		t.Fatalf("引用已关闭账户的交易不应允许恢复")
	}
	if !strings.Contains(err.Error(), "已关闭") {
		t.Fatalf("错误信息应说明账户已关闭，实际 %v", err)
	}
}

func TestSyncPushPullAndNaturalKeyMerge(t *testing.T) {
	svc := newTestService(t)
	owner := registerOwner(t, svc)

	// 在线先建一个「先到者」账户
	existing, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY", "", "", "", "")

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
	res, err := svc.SyncPush(lid, owner.User.ID, req)
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

// TestTransactionCreatedByName 验证：push 端交易由服务端按 token 用户填 created_by，
// pull 返回的交易实体含 created_by_name（JOIN users.display_name）。
func TestTransactionCreatedByName(t *testing.T) {
	svc := newTestService(t)
	owner := registerOwner(t, svc)

	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	food, err := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create food: %v", err)
	}

	req := PushRequest{
		ClientID: "devB",
		Changes: []PushChange{
			{EntityType: domain.EntityTransaction, Entity: jsonRaw(t, map[string]any{
				"uuid": "loc-txn2", "date": "2026-08-03", "flag": "*", "description": "午餐",
				"postings": []map[string]string{
					{"account_uuid": food.UUID, "commodity": "CNY", "amount": "-20.00"},
					{"account_uuid": cash.UUID, "commodity": "CNY", "amount": "20.00"},
				},
			})},
		},
	}
	if _, err := svc.SyncPush(lid, owner.User.ID, req); err != nil {
		t.Fatalf("sync push: %v", err)
	}

	// GET /api/transactions 视图也应含 created_by_name
	views, err := svc.ListTransactions(lid)
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	var found bool
	for _, v := range views {
		if v.Description == "午餐" {
			found = true
			if v.CreatedByName != "Alice" {
				t.Fatalf("列表 created_by_name 应为 Alice，实际 %q", v.CreatedByName)
			}
		}
	}
	if !found {
		t.Fatalf("列表应包含午餐交易")
	}

	// pull 实体 JSON 应含 created_by_name: Alice
	pull, err := svc.SyncPull(lid, 0, "devB")
	if err != nil {
		t.Fatalf("sync pull: %v", err)
	}
	joined := ""
	for _, c := range pull.Changes {
		joined += string(c.Entity)
	}
	if !strings.Contains(joined, `"created_by_name":"Alice"`) {
		t.Fatalf("pull 应含 created_by_name: Alice，实际 %s", joined)
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

func TestSessionListAndRevoke(t *testing.T) {
	svc := newTestService(t)
	owner := registerOwner(t, svc)

	// 第二个会话（模拟另一台设备），并补上设备信息
	secondRes, err := svc.Login("alice", "secret123")
	if err != nil {
		t.Fatalf("second login: %v", err)
	}
	secondTok := secondRes.Token
	svc.AttachSessionMeta(owner.Token, "10.0.0.5",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36")

	list, err := svc.ListSessions(owner.User.ID, owner.Token)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 个会话，实际 %d", len(list))
	}

	var current, other SessionView
	for _, s := range list {
		if s.Current {
			current = s
		} else {
			other = s
		}
	}
	if current.ID == "" || other.ID == "" {
		t.Fatalf("应恰好有一个当前会话，list=%+v", list)
	}
	if current.Device != "Chrome · Windows" {
		t.Fatalf("UA 应解析为 Chrome · Windows，实际 %q", current.Device)
	}
	// 时间格式必须与 datetime('now') 同构：带时区后缀（time.Time 默认 String）
	// 会同时破坏字符串比较排序与前端解析，这条断言专门钉住它。
	for _, ts := range []string{current.CreatedAt, current.LastSeenAt, current.ExpiresAt} {
		if len(ts) != 19 || strings.ContainsAny(ts, "+UTC") {
			t.Fatalf("会话时间应为 YYYY-MM-DD HH:MM:SS（UTC），实际 %q", ts)
		}
	}
	if current.IP != "10.0.0.5" {
		t.Fatalf("应回显登录 IP，实际 %q", current.IP)
	}
	// SID 不能等于 token 本身（否则等于把凭据发给前端）
	if current.ID == owner.Token || other.ID == secondTok {
		t.Fatalf("SID 不能是 token 原文")
	}

	// 吊销其他设备
	if err := svc.RevokeSession(owner.User.ID, other.ID, owner.Token); err != nil {
		t.Fatalf("revoke other: %v", err)
	}
	if _, err := svc.Authenticate(secondTok); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("被吊销的会话应失效")
	}
	if err := svc.RevokeSession(owner.User.ID, other.ID, owner.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("重复吊销应返回 ErrNotFound，实际 %v", err)
	}

	// 退出其他设备（此时只剩当前，吊销 0 条）
	n, err := svc.RevokeOtherSessions(owner.User.ID, owner.Token)
	if err != nil || n != 0 {
		t.Fatalf("只剩当前会话时应吊销 0 条，实际 n=%d err=%v", n, err)
	}

	// 吊销当前会话 = 退出登录
	if err := svc.RevokeSession(owner.User.ID, current.ID, owner.Token); err != nil {
		t.Fatalf("revoke current: %v", err)
	}
	if _, err := svc.Authenticate(owner.Token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("吊销当前会话后 token 应失效")
	}
}

func TestLoginRateLimit(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)
	svc.ConfigureLoginLimit(3, 15*time.Minute, 10*time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := svc.LoginFrom("alice", "wrongpass", "1.2.3.4"); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("第 %d 次错误密码应返回 ErrUnauthorized，实际 %v", i+1, err)
		}
	}

	// 超限后即使密码正确也被挡（锁定优先于凭证校验）
	var rl *domain.RateLimitError
	_, err := svc.LoginFrom("alice", "secret123", "1.2.3.4")
	if !errors.As(err, &rl) {
		t.Fatalf("超限后应返回 RateLimitError，实际 %v", err)
	}
	if rl.RetryAfter <= 0 {
		t.Fatalf("应给出剩余等待时长，实际 %v", rl.RetryAfter)
	}

	// 换 IP 也不能绕过（用户名维度同样计数）
	if _, err := svc.LoginFrom("alice", "secret123", "9.9.9.9"); !errors.As(err, &rl) {
		t.Fatalf("换 IP 仍应被用户名维度的锁定挡住，实际 %v", err)
	}

	// 同 IP 的其他用户不受牵连：IP 维度阈值放宽到 4 倍（3 次只用掉配额的一部分），
	// 第 4 次失败仍应返回凭证错误而不是限流。
	_, errBob := svc.LoginFrom("bob", "whatever", "1.2.3.4")
	if errors.As(errBob, &rl) {
		t.Fatalf("同 IP 的其他用户不应被牵连，实际 %v", errBob)
	}
	if !errors.Is(errBob, domain.ErrUnauthorized) {
		t.Fatalf("bob 应拿到凭证错误，实际 %v", errBob)
	}
}

// TestCreateTransactionRejectsClosedAccount AC-28 服务端半边（批 2 追加授权）：
// 在线记账对已关户账户返回 409 account_closed（错误构造与期初路径同款）。
// 离线 sync push 走 SavePushTransaction 不经过本路径（离线优先语义，
// 离线记账不受阻，补录由 push 独立裁决）。
func TestCreateTransactionRejectsClosedAccount(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "2026-08-01", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	food, err := svc.CreateAccount(lid, "Expenses:Food:CNY", "", "Expenses", "2026-08-01", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create food: %v", err)
	}

	// close 要求余额为 0：cash/food 初始余额均为 0，可直接关户。
	if err := svc.DeleteAccount(lid, cash.UUID); err != nil {
		t.Fatalf("close account: %v", err)
	}

	// 关户后在线记账 → 409 account_closed。
	_, err = svc.CreateTransaction(lid, "2026-08-03", "*", "关户后记账", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-5.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "5.00"},
	}, 1)
	if code := codedCode(t, err); code != domain.CodeAccountClosed {
		t.Fatalf("已关户账户在线记账应返回 account_closed，实际 %s (%v)", code, err)
	}

	// 关户校验必须拦在落库之前：交易不得写入。
	txns, err := svc.ListTransactions(lid)
	if err != nil {
		t.Fatalf("list txns: %v", err)
	}
	if len(txns) != 0 {
		t.Fatalf("关户校验应拦下交易，实际列表 %d 笔", len(txns))
	}
}
