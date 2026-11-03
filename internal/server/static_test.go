package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newWebRoot 构造一个最小前端产物目录（含 index.html 与一个 assets 文件）。
func newWebRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"),
		[]byte(`<html><script src="flutter_bootstrap.js"></script></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "flutter_bootstrap.js"), []byte("//boot"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("//app"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// doGet 发起一次请求并返回响应记录器。
func doGet(h http.HandlerFunc, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// TestSpaHandlerServesIndex 根路径返回前端入口。
func TestSpaHandlerServesIndex(t *testing.T) {
	h := spaHandler(newWebRoot(t))
	rec := doGet(h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / 期望 200，实际 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "flutter_bootstrap.js") {
		t.Fatalf("GET / 应返回 index.html，实际：%s", rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("index.html 期望 Cache-Control=no-cache，实际 %q", cc)
	}
}

// TestSpaFallbackForClientRoute 前端路由回退到 index.html（Flutter Web 必需）。
func TestSpaFallbackForClientRoute(t *testing.T) {
	root := newWebRoot(t)
	h := spaHandler(root)

	for _, route := range []string{"/transactions", "/accounts/detail", "/settings/ledgers"} {
		rec := doGet(h, route)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s 期望 200（SPA 回退），实际 %d", route, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "flutter_bootstrap.js") {
			t.Fatalf("GET %s 应回退到 index.html，实际：%s", route, rec.Body.String())
		}
	}
}

// TestStaticAssetCacheHeaders 带哈希的静态资源应长缓存。
func TestStaticAssetCacheHeaders(t *testing.T) {
	h := spaHandler(newWebRoot(t))
	rec := doGet(h, "/assets/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js 期望 200，实际 %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("/assets/ 资源期望 immutable 缓存，实际 %q", cc)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("静态资源缺少 X-Content-Type-Options: nosniff")
	}
}

// TestHashlessEntryNotCached 不带内容 hash 的入口资源必须 no-cache。
//
// main.dart.js 是全部业务代码且文件名固定：一旦被缓存，新部署后浏览器
// 仍会执行旧代码（实测后果：API 地址修复上线后用户浏览器仍请求旧地址）。
func TestHashlessEntryNotCached(t *testing.T) {
	h := spaHandler(newWebRoot(t))

	// newWebRoot 里没有 main.dart.js，补一个
	root := newWebRoot(t)
	if err := os.WriteFile(filepath.Join(root, "main.dart.js"), []byte("//app entry"), 0o644); err != nil {
		t.Fatal(err)
	}
	h = spaHandler(root)

	rec := doGet(h, "/main.dart.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /main.dart.js 期望 200，实际 %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("main.dart.js 不带内容 hash，必须 no-cache，实际 %q", cc)
	}

	rec = doGet(h, "/flutter.js")
	if rec.Code == http.StatusOK {
		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Fatalf("flutter.js 期望 no-cache，实际 %q", cc)
		}
	}
}

// TestMissingAssetReturns404 缺失的静态资源必须 404，不能回退成 index.html。
//
// 若回退，浏览器会把 HTML 当 JS 解析，产生难以定位的错误。
func TestMissingAssetReturns404(t *testing.T) {
	h := spaHandler(newWebRoot(t))
	for _, p := range []string{"/assets/missing.js", "/canvaskit/missing.wasm", "/favicon.ico"} {
		if rec := doGet(h, p); rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s 期望 404，实际 %d", p, rec.Code)
		}
	}
}

// TestAPIPathNotFallbackToIndex /api/ 前缀绝不回退到前端页面。
func TestAPIPathNotFallbackToIndex(t *testing.T) {
	h := spaHandler(newWebRoot(t))
	for _, p := range []string{"/api/unknown", "/api/", "/api"} {
		rec := doGet(h, p)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s 期望 404，实际 %d", p, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "flutter_bootstrap.js") {
			t.Fatalf("GET %s 不应返回前端 HTML", p)
		}
		if !strings.Contains(rec.Body.String(), "接口不存在") {
			t.Fatalf("GET %s 应返回 JSON 错误，实际：%s", p, rec.Body.String())
		}
	}
}

// TestPathTraversalBlocked 路径穿越不得读取 web 根之外的文件。
func TestPathTraversalBlocked(t *testing.T) {
	root := newWebRoot(t)

	// 在 web 根之外放一个敏感文件，尝试各种穿越写法都不应读到它。
	outside := filepath.Join(filepath.Dir(root), "secret_outside.txt")
	if err := os.WriteFile(outside, []byte("TOP_SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	h := spaHandler(root)
	targets := []string{
		"/../secret_outside.txt",
		"/assets/../../secret_outside.txt",
		"/%2e%2e/secret_outside.txt",
		"/..%2fsecret_outside.txt",
		"/assets/%2e%2e%2f%2e%2e%2fsecret_outside.txt",
		"/....//secret_outside.txt",
	}
	for _, p := range targets {
		rec := doGet(h, p)
		if strings.Contains(rec.Body.String(), "TOP_SECRET") {
			t.Fatalf("GET %s 泄露了 web 根之外的文件内容（状态码 %d）", p, rec.Code)
		}
	}
}

// TestDirectoryListingDisabled 目录请求不得列出文件清单。
func TestDirectoryListingDisabled(t *testing.T) {
	root := newWebRoot(t)
	h := spaHandler(root)

	rec := doGet(h, "/assets/")
	if strings.Contains(rec.Body.String(), "app.js") {
		t.Fatalf("GET /assets/ 不应列出目录内容，实际：%s", rec.Body.String())
	}
}

// TestNonGetMethodRejected 静态托管只接受 GET/HEAD。
func TestNonGetMethodRejected(t *testing.T) {
	h := spaHandler(newWebRoot(t))
	for _, m := range []string{http.MethodPost, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(m, "/", nil)
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s / 期望 405，实际 %d", m, rec.Code)
		}
	}
}

// TestMissingWebRootDoesNotPanic 产物目录不存在时返回提示，不 panic。
func TestMissingWebRootDoesNotPanic(t *testing.T) {
	h := spaHandler(filepath.Join(t.TempDir(), "not-exist"))

	rec := doGet(h, "/")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("后端未托管前端时期望 404，实际 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "前端未部署") {
		t.Fatalf("应提示前端未部署，实际：%s", rec.Body.String())
	}

	// /api/ 仍应返回 JSON 错误，而不是前端提示
	rec = doGet(h, "/api/unknown")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "接口不存在") {
		t.Fatalf("GET /api/unknown 期望 404 JSON，实际 %d %s", rec.Code, rec.Body.String())
	}
}

// TestWithinRoot 校验路径归属判断（穿越防护的基础函数）。
func TestWithinRoot(t *testing.T) {
	root := filepath.Join("a", "b")
	cases := []struct {
		full string
		want bool
	}{
		{filepath.Join("a", "b"), true},
		{filepath.Join("a", "b", "c.js"), true},
		{filepath.Join("a", "c"), false},
		{filepath.Join("a", "b", "..", "c"), false},
	}
	for _, c := range cases {
		if got := withinRoot(root, c.full); got != c.want {
			t.Fatalf("withinRoot(%q, %q) = %v，期望 %v", root, c.full, got, c.want)
		}
	}
}
