package server

// 期初重构（012）的 HTTP 层集成测试：路由 + 鉴权中间件 + handler + service + SQLite
// 全部走真实装配，不 mock，覆盖 PRD TC-01~TC-25 中依赖端点行为的部分。

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familyledger/internal/db"
	"familyledger/internal/handler"
	"familyledger/internal/repository"
	"familyledger/internal/service"
	"familyledger/internal/testutil"
)

type openingHTTPEnv struct {
	t          *testing.T
	srv        *httptest.Server
	svc        *service.Service
	conn       *sql.DB
	ownerToken string
	ownerID    int64
}

func newOpeningHTTPEnv(t *testing.T) *openingHTTPEnv {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	svc := service.New(repository.New(conn))
	owner, err := svc.Register("alice", "secret123", "Alice")
	if err != nil {
		t.Fatalf("register owner: %v", err)
	}
	h := handler.New(svc, nil)
	e := &openingHTTPEnv{
		t:          t,
		srv:        httptest.NewServer(newRouter(h, svc, "")),
		svc:        svc,
		conn:       conn,
		ownerToken: owner.Token,
		ownerID:    owner.User.ID,
	}
	t.Cleanup(e.srv.Close)
	return e
}

// do 发起一次请求；body 为 nil 表示无请求体。返回状态码、响应头与响应体。
func (e *openingHTTPEnv) do(method, path, token string, ledgerID int64, body any) (int, http.Header, []byte) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal body: %v", err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, rd)
	if err != nil {
		e.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ledgerID > 0 {
		req.Header.Set("X-Ledger-Id", jsonNumber(ledgerID))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, resp.Header, raw
}

func jsonNumber(v int64) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// decodeMap 把响应 JSON 解为 map 并断言状态码。
func (e *openingHTTPEnv) decodeMap(wantStatus int, status int, raw []byte) map[string]any {
	e.t.Helper()
	if status != wantStatus {
		e.t.Fatalf("状态码期望 %d，实际 %d，响应体: %s", wantStatus, status, raw)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		e.t.Fatalf("解析 JSON 失败: %v，响应体: %s", err, raw)
	}
	return m
}

// createAccount 通过 HTTP 建账户，返回 id/uuid。
func (e *openingHTTPEnv) createAccount(ledgerID int64, name, atype, commodity string) (int64, string) {
	e.t.Helper()
	status, _, raw := e.do(http.MethodPost, "/api/accounts", e.ownerToken, ledgerID, map[string]any{
		"name": name, "type": atype, "commodity": commodity,
	})
	if status != http.StatusCreated {
		e.t.Fatalf("建账户 %s 期望 201，实际 %d: %s", name, status, raw)
	}
	var out struct {
		ID   int64  `json:"id"`
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		e.t.Fatalf("解析账户响应: %v", err)
	}
	return out.ID, out.UUID
}

// createTxn 通过 HTTP 记一笔（金额为账本符号定点十进制）。
func (e *openingHTTPEnv) createTxn(ledgerID int64, date, desc string, postings []map[string]any) {
	e.t.Helper()
	status, _, raw := e.do(http.MethodPost, "/api/transactions", e.ownerToken, ledgerID, map[string]any{
		"date": date, "flag": "*", "description": desc, "postings": postings,
	})
	if status != http.StatusCreated {
		e.t.Fatalf("记账 %s 期望 201，实际 %d: %s", desc, status, raw)
	}
}

// TestHTTPOpeningDirectionAndLedgerDate 覆盖 TC-01/02/03/05/06/24：
// 正负期初与方向映射、GET 期初、账本日期回显、owner PATCH 与重建、日期约束。
func TestHTTPOpeningDirectionAndLedgerDate(t *testing.T) {
	e := newOpeningHTTPEnv(t)
	today := time.Now().Format("2006-01-02")

	cashID, cashUUID := e.createAccount(1, "Assets:Cash:CNY", "Assets", "CNY")
	_, cardUUID := e.createAccount(1, "Liabilities:TestCard", "Liabilities", "CNY")
	_, foodUUID := e.createAccount(1, "Expenses:QA:Food", "Expenses", "CNY")

	// TC-01 资产正期初
	status, _, raw := e.do(http.MethodPost, "/api/accounts/"+cashUUID+"/opening", e.ownerToken, 1,
		map[string]any{"amount": "100", "direction": "hold"})
	res := e.decodeMap(http.StatusOK, status, raw)
	if res["status"] != "created" || res["signed_amount"] != "100.00" || res["commodity"] != "CNY" {
		t.Fatalf("资产期初结果不符: %+v", res)
	}
	if res["date"] != today {
		t.Fatalf("期初日期应等于账本期初 %s，实际 %v", today, res["date"])
	}

	// TC-02 负债欠款为负
	status, _, raw = e.do(http.MethodPost, "/api/accounts/"+cardUUID+"/opening", e.ownerToken, 1,
		map[string]any{"amount": "200", "direction": "owe"})
	res = e.decodeMap(http.StatusOK, status, raw)
	if res["signed_amount"] != "-200.00" {
		t.Fatalf("负债欠款应为 -200.00，实际 %v", res["signed_amount"])
	}

	// TC-03 GET 期初：方向与绝对值反推正确。
	status, _, raw = e.do(http.MethodGet, "/api/accounts/"+cashUUID+"/opening", e.ownerToken, 1, nil)
	view := e.decodeMap(http.StatusOK, status, raw)
	if view["has_opening"] != true || view["direction"] != "hold" || view["amount"] != "100.00" {
		t.Fatalf("GET 期初视图不符: %+v", view)
	}

	// TC-24 GET /api/ledgers 返回 opening_date。
	status, _, raw = e.do(http.MethodGet, "/api/ledgers", e.ownerToken, 0, nil)
	ledgers := e.decodeMap(http.StatusOK, status, raw)
	list, _ := ledgers["ledgers"].([]any)
	if len(list) != 1 {
		t.Fatalf("应只有 1 个账本，实际 %d", len(list))
	}
	first, _ := list[0].(map[string]any)
	if first["opening_date"] != today {
		t.Fatalf("GET /api/ledgers 应返回 opening_date=%s，实际 %v", today, first["opening_date"])
	}

	// owner PATCH：平移期初日期（昨天），期初交易重建且一致。
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	status, _, raw = e.do(http.MethodPatch, "/api/ledgers/1", e.ownerToken, 0, map[string]any{
		"name": "家庭账本", "default_commodity": "CNY", "opening_date": yesterday,
	})
	patched := e.decodeMap(http.StatusOK, status, raw)
	if patched["opening_date"] != yesterday {
		t.Fatalf("PATCH 后 opening_date 应为 %s，实际 %v", yesterday, patched["opening_date"])
	}
	status, _, raw = e.do(http.MethodGet, "/api/accounts/"+cashUUID+"/opening", e.ownerToken, 1, nil)
	view = e.decodeMap(http.StatusOK, status, raw)
	if view["date"] != yesterday || view["signed_amount"] != "100.00" {
		t.Fatalf("重建后资产期初应保持 100.00 且日期 %s: %+v", yesterday, view)
	}
	status, _, raw = e.do(http.MethodGet, "/api/accounts/"+cardUUID+"/opening", e.ownerToken, 1, nil)
	view = e.decodeMap(http.StatusOK, status, raw)
	if view["date"] != yesterday || view["signed_amount"] != "-200.00" {
		t.Fatalf("重建后负债期初应保持 -200.00 且日期 %s: %+v", yesterday, view)
	}

	// 未来日期 → 400 + 稳定错误码。
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	status, _, raw = e.do(http.MethodPatch, "/api/ledgers/1", e.ownerToken, 0, map[string]any{
		"name": "家庭账本", "default_commodity": "CNY", "opening_date": tomorrow,
	})
	future := e.decodeMap(http.StatusBadRequest, status, raw)
	if future["code"] != "ledger_opening_date_invalid" {
		t.Fatalf("未来日期错误码不符: %+v", future)
	}

	// 同值 PATCH 幂等：opening_date 不变，期初条数不增。
	status, _, raw = e.do(http.MethodPatch, "/api/ledgers/1", e.ownerToken, 0, map[string]any{
		"name": "家庭账本", "default_commodity": "CNY", "opening_date": yesterday,
	})
	if status != http.StatusOK {
		t.Fatalf("同值 PATCH 应幂等成功，实际 %d: %s", status, raw)
	}
	var active int
	if err := e.conn.QueryRow(
		`SELECT count(*) FROM transactions WHERE ledger_id=1 AND opening_account_uuid<>'' AND reversal_of IS NULL
		   AND (reversed_by IS NULL OR reversed_by='') AND deleted_at IS NULL`,
	).Scan(&active); err != nil {
		t.Fatalf("统计有效期初: %v", err)
	}
	if active != 2 {
		t.Fatalf("同值 PATCH 后有效期初应为 2 条，实际 %d", active)
	}

	// 不晚于最早交易：记一笔 2026-01-01，再把期初挪到 2026-02-01 → 400。
	e.createTxn(1, "2026-01-01", "旧交易", []map[string]any{
		{"account_id": cashID, "commodity": "CNY", "amount": "-10.00"},
		{"account_id": mustAccountID(t, e, foodUUID), "commodity": "CNY", "amount": "10.00"},
	})
	status, _, raw = e.do(http.MethodPatch, "/api/ledgers/1", e.ownerToken, 0, map[string]any{
		"name": "家庭账本", "default_commodity": "CNY", "opening_date": "2026-02-01",
	})
	after := e.decodeMap(http.StatusBadRequest, status, raw)
	if after["code"] != "ledger_opening_date_after_first_txn" {
		t.Fatalf("晚于最早交易错误码不符: %+v", after)
	}
}

func mustAccountID(t *testing.T, e *openingHTTPEnv, uuid string) int64 {
	t.Helper()
	var id int64
	if err := e.conn.QueryRow(`SELECT id FROM accounts WHERE uuid=?`, uuid).Scan(&id); err != nil {
		t.Fatalf("查询账户 id: %v", err)
	}
	return id
}

// TestHTTPViewerAndCrossLedgerIsolation 覆盖 TC-07/08/25：viewer 只读、owner-only、跨账本隔离。
func TestHTTPViewerAndCrossLedgerIsolation(t *testing.T) {
	e := newOpeningHTTPEnv(t)

	cashID, cashUUID := e.createAccount(1, "Assets:Cash:CNY", "Assets", "CNY")

	bob, err := e.svc.Register("bob", "secret123", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if _, _, err := e.svc.AddMember(1, e.ownerID, "bob", "viewer"); err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	// viewer 读允许。
	status, _, raw := e.do(http.MethodGet, "/api/accounts", bob.Token, 1, nil)
	if status != http.StatusOK {
		t.Fatalf("viewer GET 应 200，实际 %d: %s", status, raw)
	}

	// viewer 写期初 → 403。
	status, _, raw = e.do(http.MethodPost, "/api/accounts/"+cashUUID+"/opening", bob.Token, 1,
		map[string]any{"amount": "10", "direction": "hold"})
	if status != http.StatusForbidden {
		t.Fatalf("viewer 设置期初应 403，实际 %d: %s", status, raw)
	}
	// viewer 改账本日期 → 403（service 层 owner 校验）。
	status, _, raw = e.do(http.MethodPatch, "/api/ledgers/1", bob.Token, 0, map[string]any{
		"name": "bob 改账本", "default_commodity": "CNY", "opening_date": "2025-01-01",
	})
	if status != http.StatusForbidden {
		t.Fatalf("viewer PATCH 账本应 403，实际 %d: %s", status, raw)
	}
	// viewer 记账 → 403。
	status, _, _ = e.do(http.MethodPost, "/api/transactions", bob.Token, 1, map[string]any{
		"date": "2026-01-01", "description": "x", "postings": []map[string]any{
			{"account_id": cashID, "commodity": "CNY", "amount": "-1.00"},
			{"account_id": cashID, "commodity": "CNY", "amount": "1.00"},
		},
	})
	if status != http.StatusForbidden {
		t.Fatalf("viewer 记账应 403，实际 %d", status)
	}

	// 跨账本：第二个账本中设置期初，用错误账本上下文读取必须 404 account_not_found。
	status, _, raw = e.do(http.MethodPost, "/api/ledgers", e.ownerToken, 0, map[string]any{
		"name": "旅行账本", "default_commodity": "USD",
	})
	if status != http.StatusCreated {
		t.Fatalf("建账本应 201，实际 %d: %s", status, raw)
	}
	var l2 struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &l2); err != nil || l2.ID == 0 {
		t.Fatalf("解析账本响应: %v %s", err, raw)
	}
	_, usdUUID := e.createAccount(l2.ID, "Assets:Travel:USD", "Assets", "USD")
	status, _, raw = e.do(http.MethodPost, "/api/accounts/"+usdUUID+"/opening", e.ownerToken, l2.ID,
		map[string]any{"amount": "50", "direction": "hold"})
	res := e.decodeMap(http.StatusOK, status, raw)
	if res["commodity"] != "USD" {
		t.Fatalf("账本 2 期初币种应取 USD，实际 %v", res["commodity"])
	}

	status, _, raw = e.do(http.MethodGet, "/api/accounts/"+usdUUID+"/opening", e.ownerToken, l2.ID, nil)
	if status != http.StatusOK {
		t.Fatalf("账本 2 读取期初应 200，实际 %d: %s", status, raw)
	}
	status, _, raw = e.do(http.MethodGet, "/api/accounts/"+usdUUID+"/opening", e.ownerToken, 1, nil)
	if status != http.StatusNotFound {
		t.Fatalf("跨账本读取应 404，实际 %d: %s", status, raw)
	}
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	if body["code"] != "account_not_found" {
		t.Fatalf("跨账本读取错误码不符: %+v", body)
	}
	// bob 非账本 2 成员 → 403。
	status, _, _ = e.do(http.MethodGet, "/api/accounts", bob.Token, l2.ID, nil)
	if status != http.StatusForbidden {
		t.Fatalf("非成员访问账本 2 应 403，实际 %d", status)
	}
}

// TestHTTPExportIdentityHeaders 覆盖 TC-17/18：失衡导出仍 200，且响应头带差额。
func TestHTTPExportIdentityHeaders(t *testing.T) {
	e := newOpeningHTTPEnv(t)

	cashID, _ := e.createAccount(1, "Assets:Cash:CNY", "Assets", "CNY")
	e.createTxn(1, "2026-01-02", "早餐", []map[string]any{
		{"account_id": cashID, "commodity": "CNY", "amount": "-10.00"},
		{"account_id": mustAccountIDByName(t, e, "Expenses:Food"), "commodity": "CNY", "amount": "10.00"},
	})

	// 先确认平衡态导出头。
	status, hdr, raw := e.do(http.MethodGet, "/api/export", e.ownerToken, 1, nil)
	if status != http.StatusOK {
		t.Fatalf("平衡账本导出应 200，实际 %d: %s", status, raw)
	}
	if hdr.Get("X-Ledger-Balanced") != "true" {
		t.Fatalf("平衡账本 X-Ledger-Balanced 应为 true，实际 %q", hdr.Get("X-Ledger-Balanced"))
	}
	if len(raw) == 0 {
		t.Fatal("导出内容不应为空")
	}

	// 直接插入单边分录制造 CNY 差 0.01。
	if _, err := e.conn.Exec(`INSERT INTO transactions(ledger_id, uuid, date, flag, description)
		VALUES(1, 'bad-txn', '2026-01-03', '*', '单边')`); err != nil {
		t.Fatalf("insert bad txn: %v", err)
	}
	if _, err := e.conn.Exec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		SELECT id, ?, 'CNY', '0.01', 0 FROM transactions WHERE uuid='bad-txn'`, cashID); err != nil {
		t.Fatalf("insert bad posting: %v", err)
	}

	status, hdr, raw = e.do(http.MethodGet, "/api/export", e.ownerToken, 1, nil)
	if status != http.StatusOK {
		t.Fatalf("失衡导出仍应 200，实际 %d: %s", status, raw)
	}
	if hdr.Get("X-Ledger-Balanced") != "false" {
		t.Fatalf("失衡 X-Ledger-Balanced 应为 false，实际 %q", hdr.Get("X-Ledger-Balanced"))
	}
	if got := hdr.Get("X-Ledger-Diffs"); got != "CNY=0.01" {
		t.Fatalf("X-Ledger-Diffs 应为 CNY=0.01，实际 %q", got)
	}

	// 恒等式检查端点与导出头同源结论。
	status, _, raw = e.do(http.MethodGet, "/api/identity/check", e.ownerToken, 1, nil)
	idc := e.decodeMap(http.StatusOK, status, raw)
	if idc["balanced"] != false {
		t.Fatalf("identity/check 应报告不平衡: %+v", idc)
	}
	diffs, _ := idc["diffs"].([]any)
	if len(diffs) != 1 {
		t.Fatalf("identity/check 差额数量不符: %+v", idc["diffs"])
	}
	d0, _ := diffs[0].(map[string]any)
	if d0["commodity"] != "CNY" || d0["sum"] != "0.01" {
		t.Fatalf("identity/check 差额明细不符: %+v", d0)
	}

	// 目录 zip 同样 200 且带头。
	status, hdr, raw = e.do(http.MethodGet, "/api/export/archive", e.ownerToken, 1, nil)
	if status != http.StatusOK {
		t.Fatalf("失衡目录导出仍应 200，实际 %d: %s", status, raw)
	}
	if hdr.Get("X-Ledger-Balanced") != "false" || hdr.Get("X-Ledger-Diffs") != "CNY=0.01" {
		t.Fatalf("目录导出缺少恒等式头: %q / %q", hdr.Get("X-Ledger-Balanced"), hdr.Get("X-Ledger-Diffs"))
	}
}

func mustAccountIDByName(t *testing.T, e *openingHTTPEnv, name string) int64 {
	t.Helper()
	var id int64
	if err := e.conn.QueryRow(`SELECT id FROM accounts WHERE ledger_id=1 AND name=?`, name).Scan(&id); err != nil {
		t.Fatalf("查询账户 %s: %v", name, err)
	}
	return id
}

// TestHTTPCloseAccountConflict 覆盖 TC-19/20/21：余额非 0 关闭 409、结构化差额、结清后关闭成功。
func TestHTTPCloseAccountConflict(t *testing.T) {
	e := newOpeningHTTPEnv(t)

	cashID, cashUUID := e.createAccount(1, "Assets:Cash:CNY", "Assets", "CNY")
	foodID := mustAccountIDByName(t, e, "Expenses:Food")
	e.createTxn(1, "2026-01-02", "早餐", []map[string]any{
		{"account_id": cashID, "commodity": "CNY", "amount": "-10.00"},
		{"account_id": foodID, "commodity": "CNY", "amount": "10.00"},
	})

	status, _, raw := e.do(http.MethodDelete, "/api/accounts/"+cashUUID, e.ownerToken, 1, nil)
	body := e.decodeMap(http.StatusConflict, status, raw)
	if body["code"] != "account_balance_not_zero" {
		t.Fatalf("关闭余额非 0 错误码不符: %+v", body)
	}
	bals, _ := body["balances"].([]any)
	if len(bals) != 1 {
		t.Fatalf("应返回 1 个币种差额，实际 %+v", body["balances"])
	}
	b0, _ := bals[0].(map[string]any)
	if b0["commodity"] != "CNY" || b0["amount"] != "-10.00" {
		t.Fatalf("差额明细不符: %+v", b0)
	}

	// 账户保持打开。
	status, _, raw = e.do(http.MethodGet, "/api/accounts", e.ownerToken, 1, nil)
	if status != http.StatusOK {
		t.Fatalf("账户列表应 200，实际 %d: %s", status, raw)
	}
	var accounts struct {
		Accounts []struct {
			UUID      string `json:"uuid"`
			CloseDate string `json:"close_date"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &accounts); err != nil {
		t.Fatalf("解析账户列表: %v", err)
	}
	for _, a := range accounts.Accounts {
		if a.UUID == cashUUID && a.CloseDate != "" {
			t.Fatalf("校验失败时账户不应被关闭: %+v", a)
		}
	}

	// 结清后关闭成功；再次关闭幂等 404。
	e.createTxn(1, "2026-01-03", "报销", []map[string]any{
		{"account_id": cashID, "commodity": "CNY", "amount": "10.00"},
		{"account_id": foodID, "commodity": "CNY", "amount": "-10.00"},
	})
	status, _, raw = e.do(http.MethodDelete, "/api/accounts/"+cashUUID, e.ownerToken, 1, nil)
	if status != http.StatusOK {
		t.Fatalf("余额为 0 关闭应 200，实际 %d: %s", status, raw)
	}
	status, _, _ = e.do(http.MethodDelete, "/api/accounts/"+cashUUID, e.ownerToken, 1, nil)
	if status != http.StatusNotFound {
		t.Fatalf("重复关闭应幂等 404，实际 %d", status)
	}
}

// TestOpeningExportPassesBeanCheck 端到端：正负期初 + 普通交易 + 关闭后导出，
// 单文件与目录 zip 都必须能被 bean-check 校验通过（退出码 0）。
func TestOpeningExportPassesBeanCheck(t *testing.T) {
	e := newOpeningHTTPEnv(t)

	cashID, cashUUID := e.createAccount(1, "Assets:Cash:CNY", "Assets", "CNY")
	_, cardUUID := e.createAccount(1, "Liabilities:TestCard", "Liabilities", "CNY")

	// 资产 100 / 负债欠款 200，方向映射后为 +100 / -200。
	status, _, raw := e.do(http.MethodPost, "/api/accounts/"+cashUUID+"/opening", e.ownerToken, 1,
		map[string]any{"amount": "100", "direction": "hold"})
	e.decodeMap(http.StatusOK, status, raw)
	status, _, raw = e.do(http.MethodPost, "/api/accounts/"+cardUUID+"/opening", e.ownerToken, 1,
		map[string]any{"amount": "200", "direction": "owe"})
	e.decodeMap(http.StatusOK, status, raw)

	// 一笔正常支出；日期必须不早于账户开立日（默认取账本期初日=今日），
	// 否则 Beancount 会判定「引用未激活账户」导致 bean-check 失败。
	e.createTxn(1, time.Now().Format("2006-01-02"), "早餐", []map[string]any{
		{"account_id": cashID, "commodity": "CNY", "amount": "-10.00"},
		{"account_id": mustAccountIDByName(t, e, "Expenses:Food"), "commodity": "CNY", "amount": "10.00"},
	})

	dir := t.TempDir()
	status, _, raw = e.do(http.MethodGet, "/api/export", e.ownerToken, 1, nil)
	if status != http.StatusOK || len(raw) == 0 {
		t.Fatalf("单文件导出失败: %d %s", status, raw)
	}
	single := filepath.Join(dir, "family.beancount")
	if err := os.WriteFile(single, raw, 0o644); err != nil {
		t.Fatalf("写导出文件: %v", err)
	}
	runBeanCheck(t, dir, single)

	status, _, raw = e.do(http.MethodGet, "/api/export/archive", e.ownerToken, 1, nil)
	if status != http.StatusOK || len(raw) == 0 {
		t.Fatalf("目录导出失败: %d %s", status, raw)
	}
	zipPath := filepath.Join(dir, "archive.zip")
	if err := os.WriteFile(zipPath, raw, 0o644); err != nil {
		t.Fatalf("写 zip: %v", err)
	}
	if err := unzip(zipPath, dir); err != nil {
		t.Fatalf("解压目录导出: %v", err)
	}
	mainBean := filepath.Join(dir, "main.bean")
	if _, err := os.Stat(mainBean); err != nil {
		t.Fatalf("目录导出缺少 main.bean: %v", err)
	}
	runBeanCheck(t, dir, mainBean)
}

func unzip(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		out := filepath.Join(dest, filepath.Clean(f.Name))
		if !strings.HasPrefix(out, filepath.Clean(dest)+string(os.PathSeparator)) {
			return &os.PathError{Op: "unzip", Path: f.Name, Err: os.ErrPermission}
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// runBeanCheck 校验导出文件：子进程工作目录设为导出根（解析 include），
// 只传文件名。候选探测、错误分类与严格模式统一在 internal/testutil，
// 避免与 internal/export 的实现漂移（Linux CI 上曾因复制版误判而失败）。
func runBeanCheck(t *testing.T, dir, file string) {
	t.Helper()
	testutil.RunBeanCheck(t, filepath.Base(file), dir)
}
