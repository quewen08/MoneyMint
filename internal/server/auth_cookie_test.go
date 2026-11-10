package server

// D-7 认证改造的 HTTP 集成测试：Cookie 下发/属性、CSRF、CORS、回退开关、refresh/logout。
// 全部走真实装配（路由 + 中间件 + handler + service + SQLite），不 mock。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"familyledger/internal/db"
	"familyledger/internal/handler"
	"familyledger/internal/repository"
	"familyledger/internal/service"
)

// authEnv 是一个按指定认证/CORS 选项装配的测试服务器。
type authEnv struct {
	t    *testing.T
	srv  *httptest.Server
	conn *sql.DB
}

func newAuthEnv(t *testing.T, opts handler.AuthOptions, corsCfg corsOptions) *authEnv {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	svc := service.New(repository.New(conn))
	h := handler.New(svc, nil)
	h.ConfigureAuth(opts)
	srv := httptest.NewServer(newRouter(h, svc, "", corsCfg))
	t.Cleanup(srv.Close)
	return &authEnv{t: t, srv: srv, conn: conn}
}

// do 发起请求并读完响应体；resp.Body 已关闭。
func (e *authEnv) do(client *http.Client, method, path string, body any, headers map[string]string) (*http.Response, []byte) {
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
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatalf("read body: %v", err)
	}
	return resp, raw
}

// register 用新 cookie jar 注册首个用户（owner），返回客户端、响应体与 token。
func (e *authEnv) register() (*http.Client, []byte, string) {
	e.t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		e.t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{Jar: jar}
	resp, raw := e.do(client, http.MethodPost, "/api/auth/register", map[string]any{
		"username": "alice", "password": "secret123", "display_name": "Alice",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		e.t.Fatalf("注册期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		e.t.Fatalf("解析注册响应: %v", err)
	}
	if out.Token == "" {
		e.t.Fatalf("兼容期注册响应应包含 token: %s", raw)
	}
	return client, raw, out.Token
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestAuthCookieAttributesAndCSRF Cookie 属性 + 业务端点 + CSRF 规则。
func TestAuthCookieAttributesAndCSRF(t *testing.T) {
	e := newAuthEnv(t, handler.DefaultAuthOptions(), corsOptions{})
	client, _, token := e.register()

	// 重新登录一次以直接拿到 Set-Cookie（注册响应的 Cookie 已被 jar 消费）。
	jarClient, _ := cookiejar.New(nil)
	c2 := &http.Client{Jar: jarClient}
	resp, _ := e.do(c2, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "alice", "password": "secret123",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("登录期望 200，实际 %d", resp.StatusCode)
	}
	ck := findCookie(resp.Cookies(), handler.SessionCookieName)
	if ck == nil {
		t.Fatalf("登录响应未下发 %s", handler.SessionCookieName)
	}
	if ck.Value == "" || ck.Value == token {
		t.Fatalf("登录 Cookie 值应为新的会话 token，实际 %q", ck.Value)
	}
	if ck.Path != "/" || !ck.HttpOnly {
		t.Fatalf("Cookie 必须 Path=/ 且 HttpOnly，实际 path=%q httponly=%v", ck.Path, ck.HttpOnly)
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Fatalf("Cookie SameSite 期望 Lax，实际 %v", ck.SameSite)
	}
	if ck.MaxAge != 2592000 {
		t.Fatalf("Cookie Max-Age 期望 2592000（30 天），实际 %d", ck.MaxAge)
	}
	if ck.Secure {
		t.Fatalf("LEDGER_COOKIE_SECURE 默认 false，Cookie 不应带 Secure")
	}
	if ck.Domain != "" {
		t.Fatalf("Cookie 不应设置 Domain，实际 %q", ck.Domain)
	}

	// Cookie 可直接访问业务端点（GET 不需要 CSRF）。
	resp, raw := e.do(c2, http.MethodGet, "/api/accounts", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Cookie 会话 GET /api/accounts 期望 200，实际 %d: %s", resp.StatusCode, raw)
	}

	// 无 X-CSRF 的 Cookie 写请求 → 403 csrf_required。
	accBody := map[string]any{"name": "Expenses:TestFood", "type": "Expenses", "commodity": "CNY"}
	resp, raw = e.do(c2, http.MethodPost, "/api/accounts", accBody, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("缺少 X-CSRF 期望 403，实际 %d: %s", resp.StatusCode, raw)
	}
	var errBody struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw, &errBody); err != nil || errBody.Code != "csrf_required" {
		t.Fatalf("缺少 X-CSRF 应返回 code=csrf_required，实际 %s", raw)
	}

	// 带 X-CSRF 通过。
	resp, raw = e.do(c2, http.MethodPost, "/api/accounts", accBody, map[string]string{"X-CSRF": "1"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("带 X-CSRF 期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
	var acc struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &acc); err != nil || acc.ID == 0 {
		t.Fatalf("解析账户响应失败: %v %s", err, raw)
	}
	resp, raw = e.do(c2, http.MethodPost, "/api/accounts",
		map[string]any{"name": "Assets:TestCash", "type": "Assets", "commodity": "CNY"},
		map[string]string{"X-CSRF": "1"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("建第二个账户期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
	var acc2 struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &acc2); err != nil || acc2.ID == 0 {
		t.Fatalf("解析第二个账户响应失败: %v %s", err, raw)
	}

	// Cookie + X-CSRF 记一笔 → 201。
	resp, raw = e.do(c2, http.MethodPost, "/api/transactions", map[string]any{
		"date": "2026-08-02", "description": "早餐", "postings": []map[string]any{
			{"account_id": acc.ID, "commodity": "CNY", "amount": "10.00"},
			{"account_id": acc2.ID, "commodity": "CNY", "amount": "-10.00"},
		},
	}, map[string]string{"X-CSRF": "1"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Cookie 记账期望 201，实际 %d: %s", resp.StatusCode, raw)
	}

	// Bearer 请求豁免 CSRF（旧版客户端）。
	bearer := &http.Client{}
	resp, raw = e.do(bearer, http.MethodPost, "/api/accounts",
		map[string]any{"name": "Expenses:TestTransport", "type": "Expenses", "commodity": "CNY"},
		map[string]string{"Authorization": "Bearer " + token})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Bearer 无 X-CSRF 应豁免，期望 201，实际 %d: %s", resp.StatusCode, raw)
	}

	// Cookie 与 Bearer 同时存在（旧版浏览器场景）：仍按 Bearer 豁免，不能打断旧版。
	resp, raw = e.do(client, http.MethodPost, "/api/accounts",
		map[string]any{"name": "Expenses:TestFun", "type": "Expenses", "commodity": "CNY"},
		map[string]string{"Authorization": "Bearer " + token})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Cookie+Bearer 并存时应豁免 CSRF，期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
}

// TestAuthCookieDisabled LEDGER_AUTH_COOKIE=false：不下发 Cookie，Bearer 正常且无 CSRF。
func TestAuthCookieDisabled(t *testing.T) {
	e := newAuthEnv(t, handler.AuthOptions{CookieEnabled: false, ExposeToken: true}, corsOptions{})
	client, _, token := e.register()

	resp, _ := e.do(client, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "alice", "password": "secret123",
	}, nil)
	if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("关闭 Cookie 时不应下发 Set-Cookie，实际 %v", got)
	}

	bearer := &http.Client{}
	resp, raw := e.do(bearer, http.MethodPost, "/api/accounts",
		map[string]any{"name": "Expenses:TestBearer", "type": "Expenses", "commodity": "CNY"},
		map[string]string{"Authorization": "Bearer " + token})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Bearer 建账户期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
}

// TestAuthExposeTokenDisabled LEDGER_AUTH_EXPOSE_TOKEN=false：响应体无 token，Cookie 可用。
func TestAuthExposeTokenDisabled(t *testing.T) {
	e := newAuthEnv(t, handler.AuthOptions{CookieEnabled: true, ExposeToken: false}, corsOptions{})
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, raw := e.do(client, http.MethodPost, "/api/auth/register", map[string]any{
		"username": "bob", "password": "secret123", "display_name": "Bob",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("注册期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("解析注册响应: %v", err)
	}
	if _, ok := body["token"]; ok {
		t.Fatalf("关闭 token 暴露后响应不应含 token 字段: %s", raw)
	}
	if findCookie(resp.Cookies(), handler.SessionCookieName) == nil {
		t.Fatalf("关闭 token 暴露后必须下发 Cookie，否则无法登录")
	}
	resp, raw = e.do(client, http.MethodGet, "/api/auth/me", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Cookie 访问 /api/auth/me 期望 200，实际 %d: %s", resp.StatusCode, raw)
	}
}

// TestCORSWhitelist 白名单命中回显 + 凭证 + Vary；未命中不回显。
func TestCORSWhitelist(t *testing.T) {
	e := newAuthEnv(t, handler.DefaultAuthOptions(),
		corsOptions{origins: []string{"http://app.local"}})
	client := &http.Client{}
	allowed := map[string]string{"Origin": "http://app.local"}

	resp, _ := e.do(client, http.MethodGet, "/api/health", nil, allowed)
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://app.local" {
		t.Fatalf("命中来源应回显 Origin，实际 %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("白名单模式应允许凭证，实际 %q", got)
	}
	if !strings.Contains(strings.Join(resp.Header.Values("Vary"), ","), "Origin") {
		t.Fatalf("应输出 Vary: Origin，实际 %v", resp.Header.Values("Vary"))
	}
	if h := resp.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(h, "X-CSRF") {
		t.Fatalf("Allow-Headers 应包含 X-CSRF，实际 %q", h)
	}

	resp, _ = e.do(client, http.MethodGet, "/api/health", nil, map[string]string{"Origin": "http://evil.local"})
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("未命中来源不应回显 Origin，实际 %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("未命中来源不应允许凭证，实际 %q", got)
	}

	// OPTIONS 预检：204 且不进入认证（无 token）。
	resp, _ = e.do(client, http.MethodOptions, "/api/accounts", nil, map[string]string{
		"Origin":                         "http://app.local",
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "x-csrf,content-type",
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS 预检期望 204，实际 %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://app.local" {
		t.Fatalf("预检应回显来源，实际 %q", got)
	}
}

// TestCORSWildcardAndSameOrigin 通配模式不带凭证；未配置时仅同源（不输出 CORS 头）。
func TestCORSWildcardAndSameOrigin(t *testing.T) {
	client := &http.Client{}
	headers := map[string]string{"Origin": "http://any.local"}

	wild := newAuthEnv(t, handler.DefaultAuthOptions(), corsOptions{wildcard: true})
	resp, _ := wild.do(client, http.MethodGet, "/api/health", nil, headers)
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("通配模式应输出 *，实际 %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("通配模式不得允许凭证，实际 %q", got)
	}

	same := newAuthEnv(t, handler.DefaultAuthOptions(), corsOptions{})
	resp, _ = same.do(client, http.MethodGet, "/api/health", nil, headers)
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("未配置 CORS 时不应输出 Allow-Origin，实际 %q", got)
	}
}

// TestRefreshAndLogoutCookie refresh 后旧 token 立即失效、新 Cookie 可用；logout 清空 Cookie。
func TestRefreshAndLogoutCookie(t *testing.T) {
	e := newAuthEnv(t, handler.DefaultAuthOptions(), corsOptions{})
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	resp, raw := e.do(client, http.MethodPost, "/api/auth/register", map[string]any{
		"username": "carol", "password": "secret123", "display_name": "Carol",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("注册期望 201，实际 %d: %s", resp.StatusCode, raw)
	}
	old := findCookie(resp.Cookies(), handler.SessionCookieName)
	if old == nil {
		t.Fatalf("注册未下发 Cookie")
	}

	resp, raw = e.do(client, http.MethodPost, "/api/auth/refresh", nil, map[string]string{"X-CSRF": "1"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh 期望 200，实际 %d: %s", resp.StatusCode, raw)
	}
	fresh := findCookie(resp.Cookies(), handler.SessionCookieName)
	if fresh == nil || fresh.Value == "" || fresh.Value == old.Value {
		t.Fatalf("refresh 应写回新的会话 Cookie")
	}

	// 旧 token 走 Bearer 立即 401。
	bearer := &http.Client{}
	resp, raw = e.do(bearer, http.MethodGet, "/api/auth/me", nil,
		map[string]string{"Authorization": "Bearer " + old.Value})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refresh 后旧 token 期望 401，实际 %d: %s", resp.StatusCode, raw)
	}
	// 新 Cookie 可用。
	resp, raw = e.do(client, http.MethodGet, "/api/auth/me", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("新 Cookie 访问 me 期望 200，实际 %d: %s", resp.StatusCode, raw)
	}

	// logout：200，Set-Cookie 含 Max-Age=0。
	resp, raw = e.do(client, http.MethodPost, "/api/auth/logout", nil, map[string]string{"X-CSRF": "1"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout 期望 200，实际 %d: %s", resp.StatusCode, raw)
	}
	rawCookie := strings.Join(resp.Header.Values("Set-Cookie"), "; ")
	if !strings.Contains(rawCookie, handler.SessionCookieName+"=") || !strings.Contains(rawCookie, "Max-Age=0") {
		t.Fatalf("logout 应清空 Cookie（Max-Age=0），实际 %q", rawCookie)
	}

	// Cookie 已被浏览器删除 → 401。
	resp, raw = e.do(client, http.MethodGet, "/api/auth/me", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("logout 后期望 401，实际 %d: %s", resp.StatusCode, raw)
	}
}
