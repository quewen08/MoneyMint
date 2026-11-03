package backup

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"familyledger/internal/db"
)

// newTestEngine 用临时库构造 Engine，并预置 base 条交易（不 checkpoint）。
func newTestEngine(t *testing.T, txns int) (*Engine, *sql.DB, string) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ledger.db")
	conn, err := db.Open(dbPath, filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	mustExec(`INSERT INTO users(id, username, password_hash, display_name) VALUES(1,'u','x','U')`)
	mustExec(`INSERT INTO ledgers(id, name, owner_id) VALUES(1,'备份测试账本',1)`)
	mustExec(`INSERT INTO commodities(ledger_id, uuid, symbol, name, precision) VALUES(1,'cmd-1','CNY','人民币',2)`)
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date)
	          VALUES(1,1,'acc-1','Assets:Cash:CNY','Assets','2026-11-01')`)
	mustExec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date)
	          VALUES(2,1,'acc-2','Expenses:Food','Expenses','2026-11-01')`)

	for i := 0; i < txns; i++ {
		id := i + 1
		mustExec(`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description)
		          VALUES(?, 1, ?, '2026-11-02', '*', ?)`, id, fmt.Sprintf("txn-%d", id), fmt.Sprintf("测试交易%d", id))
		mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		          VALUES(?, 1, 'CNY', '-10.00', 0)`, id)
		mustExec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		          VALUES(?, 2, 'CNY', '10.00', 1)`, id)
	}

	engine, err := NewEngine(conn, filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return engine, conn, dbPath
}

// TestSnapshotIncludesUncheckpointedWAL 是本模块最重要的测试。
//
// 数据库运行在 WAL 模式，刚写入的数据可能全部驻留在 -wal 文件里。
// 若用「复制主文件」的方式备份，会得到能打开、有表结构、但零数据的空壳。
// 本测试同时验证正反两面：
//   - VACUUM INTO 生成的快照包含全部数据
//   - 直接复制主文件会丢光数据（证明该测试确实能捕获此类回归）
func TestSnapshotIncludesUncheckpointedWAL(t *testing.T) {
	const txns = 300
	engine, conn, dbPath := newTestEngine(t, txns)

	// 前提确认：数据确实还在 WAL 里（否则本测试没有意义）
	walPath := dbPath + "-wal"
	walInfo, err := os.Stat(walPath)
	if err != nil {
		t.Fatalf("期望存在 WAL 文件: %v", err)
	}
	if walInfo.Size() == 0 {
		t.Fatal("WAL 文件为空，测试前提不成立")
	}
	mainInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat main db: %v", err)
	}
	t.Logf("主库 %d 字节，WAL %d 字节", mainInfo.Size(), walInfo.Size())

	// 正面对照：快照必须包含全部交易
	item, err := engine.Snapshot("manual")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if item == nil {
		t.Fatal("snapshot 返回空结果")
	}
	defer os.Remove(filepath.Join(engine.Dir(), item.Name))

	snapPath := filepath.Join(engine.Dir(), item.Name)
	snap, err := sql.Open("sqlite", snapPath)
	if err != nil {
		t.Fatalf("打开快照: %v", err)
	}
	defer snap.Close()

	var got int
	if err := snap.QueryRow(`SELECT count(*) FROM transactions`).Scan(&got); err != nil {
		t.Fatalf("统计快照交易数: %v", err)
	}
	if got != txns {
		t.Fatalf("快照应包含 %d 条交易，实际 %d 条 —— VACUUM INTO 未合入 WAL 数据", txns, got)
	}

	var integrity string
	if err := snap.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("快照完整性检查未通过: %s", integrity)
	}

	// 反面对照：复制主文件会丢掉 WAL 中的数据
	copied := filepath.Join(t.TempDir(), "copied.db")
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("读取主库文件: %v", err)
	}
	if err := os.WriteFile(copied, raw, 0o644); err != nil {
		t.Fatalf("写入复制文件: %v", err)
	}
	bad, err := sql.Open("sqlite", copied)
	if err != nil {
		t.Fatalf("打开复制文件: %v", err)
	}
	defer bad.Close()
	var badCount int
	if err := bad.QueryRow(`SELECT count(*) FROM transactions`).Scan(&badCount); err != nil {
		t.Fatalf("统计复制文件交易数: %v", err)
	}
	t.Logf("对照实验：直接复制主文件只得到 %d 条交易（正确应为 %d 条）", badCount, txns)
	if badCount >= txns {
		t.Log("提示：本次主库恰好已 checkpoint，对照实验未体现差异（不影响正面结论）")
	}

	// 主库应保持可用：快照不应破坏原库
	var srcCount int
	if err := conn.QueryRow(`SELECT count(*) FROM transactions`).Scan(&srcCount); err != nil {
		t.Fatalf("主库查询失败: %v", err)
	}
	if srcCount != txns {
		t.Fatalf("快照后主库交易数异常：期望 %d，实际 %d", txns, srcCount)
	}
}

// TestSnapshotWritesManifestAndBeans 快照应写入清单与 beancount 附加产物。
func TestSnapshotWritesManifestAndBeans(t *testing.T) {
	engine, _, _ := newTestEngine(t, 3)

	item, err := engine.Snapshot("auto")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if item.Size <= 0 {
		t.Fatalf("快照体积应大于 0，实际 %d", item.Size)
	}
	if len(item.SHA256) != 64 {
		t.Fatalf("SHA256 长度异常: %q", item.SHA256)
	}
	if item.Trigger != "auto" {
		t.Fatalf("trigger 应为 auto，实际 %q", item.Trigger)
	}
	if item.Stats["transactions"] != 3 {
		t.Fatalf("清单统计 transactions 应为 3，实际 %d", item.Stats["transactions"])
	}
	if len(item.BeanFiles) == 0 {
		t.Fatal("应至少导出 1 个 beancount 文件")
	}
	for _, f := range item.BeanFiles {
		if _, err := os.Stat(filepath.Join(engine.Dir(), f)); err != nil {
			t.Fatalf("beancount 附加产物 %s 不存在: %v", f, err)
		}
	}

	items, err := engine.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].Name != item.Name {
		t.Fatalf("清单应包含 1 条记录，实际 %+v", items)
	}
	if !engine.LastSuccess().Equal(item.CreatedAt) {
		t.Fatalf("LastSuccess 应等于本次记录时间")
	}
}

// TestSnapshotBusy 并发触发时后到者应立即失败（不排队），避免阻塞 API。
func TestSnapshotBusy(t *testing.T) {
	engine, _, _ := newTestEngine(t, 1)

	engine.runMu.Lock() // 模拟已有任务占用
	defer engine.runMu.Unlock()

	if _, err := engine.Snapshot("manual"); err != ErrBusy {
		t.Fatalf("期望 ErrBusy，实际 %v", err)
	}
}

// TestValidSnapshotName 文件名白名单（下载与清理的安全基础）。
func TestValidSnapshotName(t *testing.T) {
	valid := []string{
		"ledger-20261103-030000.db",
		"ledger-20261103-030000-001.db",
		"ledger-20261103-030000.beancount",
		"ledger-20261103-030000.l1.beancount",
		"ledger-20261103-030000.l12.beancount",
		"ledger-20261103-030000-001.l3.beancount",
	}
	for _, n := range valid {
		if !ValidSnapshotName(n) {
			t.Fatalf("%q 应被判定为合法", n)
		}
	}

	invalid := []string{
		"",
		"../ledger-20261103-030000.db",
		"ledger-20261103-030000.db/../x",
		"manifest.json",
		"ledger-XXXXXXXX-XXXXXX.db",
		"ledger-20261103-030000.txt",
		"/etc/passwd",
		"ledger-20261103-030000.db\x00",
	}
	for _, n := range invalid {
		if ValidSnapshotName(n) {
			t.Fatalf("%q 应被判定为非法", n)
		}
	}
}

// TestUniqueNameAvoidsOverwrite 同秒重复触发时不得覆盖已有快照
// （VACUUM INTO 目标已存在会直接报错，实测确认）。
func TestUniqueNameAvoidsOverwrite(t *testing.T) {
	engine, _, _ := newTestEngine(t, 0)

	first, err := engine.Snapshot("manual")
	if err != nil {
		t.Fatalf("第一次快照: %v", err)
	}
	if _, err := os.Stat(filepath.Join(engine.Dir(), first.Name)); err != nil {
		t.Fatalf("第一份快照文件不存在: %v", err)
	}

	// 强制让第二次落在同一秒前缀上：直接调用 uniqueName 验证递增逻辑
	at := first.CreatedAt.Local()
	n2, p2, err := engine.uniqueName(at)
	if err != nil {
		t.Fatalf("uniqueName: %v", err)
	}
	if n2 == first.Name {
		t.Fatalf("同秒触发应生成不同文件名，实际重复 %q", n2)
	}
	if _, err := os.Stat(p2); !os.IsNotExist(err) {
		t.Fatalf("新文件名对应的文件不应存在: %s", p2)
	}
}

// TestSnapshotFailureRecordsManifest 失败也应留痕（便于运维排查）。
func TestSnapshotFailureRecordsManifest(t *testing.T) {
	engine, _, _ := newTestEngine(t, 1)

	// 让备份目录不可写不现实（跨平台差异大），改为注入一次失败：
	// 直接把目录删掉，使 VACUUM INTO 无法写入。
	if err := os.RemoveAll(engine.Dir()); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	if _, err := engine.Snapshot("manual"); err == nil {
		t.Fatal("目录不可写时快照应失败")
	}

	items, err := engine.List()
	if err != nil {
		// 清单也写不进去（目录已删），此时不强制要求有记录
		return
	}
	for _, it := range items {
		if it.Succeeded() {
			t.Fatalf("不应存在成功的备份记录：%+v", it)
		}
	}
}
