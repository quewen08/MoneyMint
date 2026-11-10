package server

// 在线记账幂等键（迁移 013 / AC-23）的 HTTP 集成测试。

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"familyledger/internal/handler"
)

// idemTxnResponse 是创建交易的响应子集。
type idemTxnResponse struct {
	ID         int64  `json:"id"`
	UUID       string `json:"uuid"`
	Idempotent bool   `json:"idempotent"`
}

// errmap 解析错误响应中的 code。
func errmap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解析错误响应失败: %v，响应体: %s", err, raw)
	}
	return m
}

// TestTransactionIdempotency 覆盖：重复提交只落一条、200 + 同 uuid、
// 软删后重提 409、同键改内容 409、不同键正常两条、头优先于 body。
func TestTransactionIdempotency(t *testing.T) {
	e := newAuthEnv(t, handler.DefaultAuthOptions(), corsOptions{})
	_, _, token := e.register()
	auth := map[string]string{"Authorization": "Bearer " + token}

	createAccount := func(name, typ string) int64 {
		t.Helper()
		resp, raw := e.do(http.DefaultClient, http.MethodPost, "/api/accounts",
			map[string]any{"name": name, "type": typ, "commodity": "CNY"}, auth)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("建账户 %s 期望 201，实际 %d: %s", name, resp.StatusCode, raw)
		}
		var out struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || out.ID == 0 {
			t.Fatalf("解析账户响应失败: %v %s", err, raw)
		}
		return out.ID
	}
	expenses := createAccount("Expenses:TestIdem", "Expenses")
	assets := createAccount("Assets:TestIdem", "Assets")

	body := func(desc, amount string) map[string]any {
		return map[string]any{
			"date": "2026-08-03", "flag": "*", "description": desc,
			"postings": []map[string]any{
				{"account_id": expenses, "commodity": "CNY", "amount": amount},
				{"account_id": assets, "commodity": "CNY", "amount": negateForTest(amount)},
			},
		}
	}
	post := func(key, desc, amount string, extra map[string]string) (*http.Response, []byte) {
		t.Helper()
		b := body(desc, amount)
		b["client_request_id"] = key
		h := map[string]string{"Authorization": "Bearer " + token}
		for k, v := range extra {
			h[k] = v
		}
		return e.do(http.DefaultClient, http.MethodPost, "/api/transactions", b, h)
	}

	// 1) 首次 → 201。
	resp, raw := post("req-1", "早餐", "10.00", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("首次提交期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
	var first idemTxnResponse
	if err := json.Unmarshal(raw, &first); err != nil || first.UUID == "" {
		t.Fatalf("解析首次响应失败: %v %s", err, raw)
	}
	if first.Idempotent {
		t.Fatalf("首次提交不应标记 idempotent")
	}

	// 2) 重放 → 200 + 同 uuid + idempotent。
	resp, raw = post("req-1", "早餐", "10.00", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("重放期望 200，实际 %d: %s", resp.StatusCode, raw)
	}
	var second idemTxnResponse
	if err := json.Unmarshal(raw, &second); err != nil {
		t.Fatalf("解析重放响应失败: %v %s", err, raw)
	}
	if second.UUID != first.UUID || second.ID != first.ID {
		t.Fatalf("重放返回的 uuid/id 应与首次一致，首次 %+v，重放 %+v", first, second)
	}
	if !second.Idempotent {
		t.Fatalf("重放响应应标记 idempotent=true")
	}
	if n := countTxnByKey(t, e, "req-1"); n != 1 {
		t.Fatalf("同一幂等键应只落一条交易，实际 %d", n)
	}

	// 3) 同键改金额 → 409 idempotency_payload_mismatch。
	resp, raw = post("req-1", "早餐", "11.00", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("同键不同内容期望 409，实际 %d: %s", resp.StatusCode, raw)
	}
	if code := errmap(t, raw)["code"]; code != "idempotency_payload_mismatch" {
		t.Fatalf("期望 code=idempotency_payload_mismatch，实际 %v", code)
	}
	if n := countTxnByKey(t, e, "req-1"); n != 1 {
		t.Fatalf("内容不匹配时不得新增交易，实际 %d 条", n)
	}

	// 4) 软删后重提同键 → 409 idempotency_key_deleted。
	resp, raw = e.do(http.DefaultClient, http.MethodDelete, "/api/transactions/"+first.UUID, nil, auth)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("删除交易期望 2xx，实际 %d: %s", resp.StatusCode, raw)
	}
	resp, raw = post("req-1", "早餐", "10.00", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("软删后重提期望 409，实际 %d: %s", resp.StatusCode, raw)
	}
	if code := errmap(t, raw)["code"]; code != "idempotency_key_deleted" {
		t.Fatalf("期望 code=idempotency_key_deleted，实际 %v", code)
	}

	// 5) 不同键 → 正常新增第二条。
	resp, raw = post("req-2", "早餐", "10.00", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("新键期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
	var third idemTxnResponse
	if err := json.Unmarshal(raw, &third); err != nil {
		t.Fatalf("解析新键响应失败: %v %s", err, raw)
	}
	if third.UUID == first.UUID {
		t.Fatalf("不同幂等键不应复用同一交易 uuid")
	}
	if n := countTxnRows(t, e); n < 2 {
		t.Fatalf("不同键应有至少两条交易，实际 %d", n)
	}

	// 6) Idempotency-Key 头优先于 body。
	resp, raw = post("body-key", "午餐", "12.00", map[string]string{"Idempotency-Key": "header-key"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("头优先提交期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
	var fourth idemTxnResponse
	if err := json.Unmarshal(raw, &fourth); err != nil {
		t.Fatalf("解析头优先响应失败: %v %s", err, raw)
	}
	if got := txnKeyOf(t, e, fourth.UUID); got != "header-key" {
		t.Fatalf("应使用 Idempotency-Key 头作为幂等键，实际落库 %q", got)
	}
	if countTxnByKey(t, e, "body-key") != 0 {
		t.Fatalf("header 存在时不应使用 body 的 client_request_id")
	}
}

// TestTransactionIdempotencyConcurrent 5 个并发同键请求只落一条（AC-23）。
func TestTransactionIdempotencyConcurrent(t *testing.T) {
	e := newAuthEnv(t, handler.DefaultAuthOptions(), corsOptions{})
	_, _, token := e.register()
	auth := map[string]string{"Authorization": "Bearer " + token}

	createAccount := func(name, typ string) int64 {
		t.Helper()
		_, raw := e.do(http.DefaultClient, http.MethodPost, "/api/accounts",
			map[string]any{"name": name, "type": typ, "commodity": "CNY"}, auth)
		var out struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || out.ID == 0 {
			t.Fatalf("解析账户响应失败: %v %s", err, raw)
		}
		return out.ID
	}
	expenses := createAccount("Expenses:TestRace", "Expenses")
	assets := createAccount("Assets:TestRace", "Assets")

	payload := map[string]any{
		"date": "2026-08-04", "description": "并发", "client_request_id": "race-key",
		"postings": []map[string]any{
			{"account_id": expenses, "commodity": "CNY", "amount": "7.00"},
			{"account_id": assets, "commodity": "CNY", "amount": "-7.00"},
		},
	}

	const workers = 5
	var wg sync.WaitGroup
	statuses := make(chan int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, _ := e.do(http.DefaultClient, http.MethodPost, "/api/transactions", payload, auth)
			statuses <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(statuses)

	created := 0
	for st := range statuses {
		switch st {
		case http.StatusCreated:
			created++
		case http.StatusOK:
		default:
			t.Fatalf("并发重复提交出现非预期状态码 %d", st)
		}
	}
	if created != 1 {
		t.Fatalf("并发同键应恰好一次 201，实际 %d 次", created)
	}
	if n := countTxnByKey(t, e, "race-key"); n != 1 {
		t.Fatalf("并发同键应只落一条交易，实际 %d", n)
	}
}

// negateForTest 把定点十进制金额取反（仅测试构造平衡分录用，不参与生产金额运算）。
func negateForTest(s string) string {
	if len(s) > 0 && s[0] == '-' {
		return s[1:]
	}
	return "-" + s
}

func countTxnByKey(t *testing.T, e *authEnv, key string) int {
	t.Helper()
	var n int
	if err := e.conn.QueryRow(
		`SELECT count(*) FROM transactions WHERE client_request_id=? AND deleted_at IS NULL`, key,
	).Scan(&n); err != nil {
		t.Fatalf("统计幂等键 %s 失败: %v", key, err)
	}
	return n
}

func countTxnRows(t *testing.T, e *authEnv) int {
	t.Helper()
	var n int
	if err := e.conn.QueryRow(`SELECT count(*) FROM transactions`).Scan(&n); err != nil {
		t.Fatalf("统计交易总数失败: %v", err)
	}
	return n
}

func txnKeyOf(t *testing.T, e *authEnv, uuid string) string {
	t.Helper()
	var key string
	if err := e.conn.QueryRow(
		`SELECT COALESCE(client_request_id, '') FROM transactions WHERE uuid=?`, uuid,
	).Scan(&key); err != nil {
		t.Fatalf("读取交易 %s 幂等键失败: %v", uuid, err)
	}
	return key
}
