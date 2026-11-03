package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"familyledger/internal/auth"
	"familyledger/internal/backup"
	"familyledger/internal/db"
	"familyledger/internal/repository"
	"familyledger/internal/service"
)

// backupTestEnv 为备份端点测试准备完整依赖。
type backupTestEnv struct {
	h        *Handler
	engine   *backup.Engine
	svc      *service.Service
	adminID  int64
	normalID int64
	dbDir    string
}

func newBackupTestEnv(t *testing.T) *backupTestEnv {
	t.Helper()

	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	svc := service.New(repository.New(conn))

	backupDir := filepath.Join(dir, "backups")
	engine, err := backup.NewEngine(conn, backupDir)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	admin, err := svc.Register("admin", "secret123", "Admin")
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	normal, err := svc.Register("normal", "secret123", "Normal")
	if err != nil {
		t.Fatalf("register normal: %v", err)
	}
	if !admin.User.IsAdmin {
		t.Fatal("首个用户应为管理员，测试前提不成立")
	}
	if normal.User.IsAdmin {
		t.Fatal("第二个用户不应为管理员，测试前提不成立")
	}

	return &backupTestEnv{
		h:        New(svc, engine),
		engine:   engine,
		svc:      svc,
		adminID:  admin.User.ID,
		normalID: normal.User.ID,
		dbDir:    backupDir,
	}
}

// call 以指定用户身份发起一次请求。
func (e *backupTestEnv) call(t *testing.T, uid int64, fn http.HandlerFunc, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req = req.WithContext(auth.WithUser(req.Context(), uid))
	rec := httptest.NewRecorder()
	fn(rec, req)
	return rec
}

// TestBackupListRequiresAdmin 非管理员访问备份端点必须 403。
func TestBackupListRequiresAdmin(t *testing.T) {
	env := newBackupTestEnv(t)

	rec := env.call(t, env.normalID, env.h.HandleBackupList, http.MethodGet, "/api/backup/list")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("普通用户应被拒绝（403），实际 %d：%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "管理员权限") {
		t.Fatalf("应提示需要管理员权限，实际：%s", rec.Body.String())
	}

	rec = env.call(t, env.normalID, env.h.HandleBackupRun, http.MethodPost, "/api/backup/run")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("普通用户手动备份应 403，实际 %d", rec.Code)
	}
}

// TestBackupRunAndDownload 管理员可创建并下载备份，内容长度一致。
func TestBackupRunAndDownload(t *testing.T) {
	env := newBackupTestEnv(t)

	rec := env.call(t, env.adminID, env.h.HandleBackupRun, http.MethodPost, "/api/backup/run")
	if rec.Code != http.StatusCreated {
		t.Fatalf("手动备份应返回 201，实际 %d：%s", rec.Code, rec.Body.String())
	}

	items, err := env.engine.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("应有 1 条备份记录，实际 %d（err=%v）", len(items), err)
	}
	name := items[0].Name

	dl := env.call(t, env.adminID, env.h.HandleBackupDownload, http.MethodGet,
		"/api/backup/download?name="+url.QueryEscape(name))
	if dl.Code != http.StatusOK {
		t.Fatalf("下载应返回 200，实际 %d：%s", dl.Code, dl.Body.String())
	}
	if int64(dl.Body.Len()) != items[0].Size {
		t.Fatalf("下载内容 %d 字节，与清单记录 %d 字节不一致", dl.Body.Len(), items[0].Size)
	}
	if ct := dl.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("Content-Type 应为 octet-stream，实际 %q", ct)
	}
	if cd := dl.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("应以下载方式返回，实际 %q", cd)
	}
}

// TestBackupDownloadPathTraversal 路径穿越的各种写法都必须被拒绝。
//
// 这是安全测试：任何一条漏网都可能导致任意文件读取。
func TestBackupDownloadPathTraversal(t *testing.T) {
	env := newBackupTestEnv(t)

	// 在备份目录的父目录放一个诱饵文件，若防护失效可能被读到
	outside := filepath.Join(filepath.Dir(env.dbDir), "secret.txt")
	if err := os.WriteFile(outside, []byte("TOP_SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 先造一份真实备份，确保白名单本身能通过
	if _, err := env.engine.Snapshot("manual"); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	cases := []string{
		"",                 // 空
		"../secret.txt",    // 相对穿越
		"../../etc/passwd", // 多级穿越
		"ledger-20261103-030000.db/../../secret.txt",
		`..\..\secret.txt`,           // Windows 分隔符
		"manifest.json",              // 非白名单文件
		"secret.txt",                 // 存在但非白名单
		"/etc/passwd",                // 绝对路径
		"ledger-XXXXXXXX-XXXXXX.db",  // 形式不符
		"ledger-20261103-030000.tar", // 后缀不符
	}
	for _, name := range cases {
		target := "/api/backup/download?name=" + url.QueryEscape(name)
		rec := env.call(t, env.adminID, env.h.HandleBackupDownload, http.MethodGet, target)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("name=%q 应返回 400，实际 %d", name, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "TOP_SECRET") {
			t.Fatalf("name=%q 泄露了备份目录外的文件内容", name)
		}
	}
}

// TestBackupDownloadMissingFile 目录内不存在的合法文件名应 404。
func TestBackupDownloadMissingFile(t *testing.T) {
	env := newBackupTestEnv(t)

	rec := env.call(t, env.adminID, env.h.HandleBackupDownload, http.MethodGet,
		"/api/backup/download?name="+url.QueryEscape("ledger-20200101-000000.db"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在的备份应 404，实际 %d", rec.Code)
	}
}

// TestBackupDisabledReturns503 备份未启用时返回 503，而不是伪装成功。
func TestBackupDisabledReturns503(t *testing.T) {
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "t.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	svc := service.New(repository.New(conn))
	admin, err := svc.Register("admin2", "secret123", "Admin")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// engine 传 nil 模拟「备份不可用」
	h := New(svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/backup/list", nil)
	req = req.WithContext(auth.WithUser(req.Context(), admin.User.ID))
	rec := httptest.NewRecorder()
	h.HandleBackupList(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("备份未启用应返回 503，实际 %d：%s", rec.Code, rec.Body.String())
	}
}

// TestBackupRunBusy 页面上重复点击「立即备份」时，第二次应得到明确冲突提示。
func TestBackupRunBusy(t *testing.T) {
	env := newBackupTestEnv(t)

	// 占用备份锁模拟「备份进行中」
	env.engine.LockRunForTest()
	defer env.engine.UnlockRunForTest()

	rec := env.call(t, env.adminID, env.h.HandleBackupRun, http.MethodPost, "/api/backup/run")
	if rec.Code != http.StatusConflict {
		t.Fatalf("备份进行中再次触发应 409，实际 %d：%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "正在执行") {
		t.Fatalf("应提示已有备份在执行，实际：%s", rec.Body.String())
	}
}
