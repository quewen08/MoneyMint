package restore

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"familyledger/internal/backup"
	"familyledger/internal/db"
)

// openTestDB 打开一份临时库（含完整 schema + 全部迁移），供恢复测试使用。
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	schemaPath, err := filepath.Abs("../../schema.sql")
	if err != nil {
		t.Fatalf("解析 schema 路径: %v", err)
	}
	conn, err := db.Open(dbPath, schemaPath)
	if err != nil {
		t.Fatalf("打开测试库: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// insertAdmin 直接写入一个系统管理员（绕过注册流程，仅测试用）。
func insertAdmin(t *testing.T, conn *sql.DB, name string) {
	t.Helper()
	if _, err := conn.Exec(
		`INSERT INTO users(username, password_hash, display_name, is_admin)
		 VALUES(?, 'x', ?, 1)`, name, name); err != nil {
		t.Fatalf("插入管理员 %s: %v", name, err)
	}
}

func userName(conn *sql.DB, name string) (string, bool) {
	var n string
	err := conn.QueryRow(`SELECT display_name FROM users WHERE username=?`, name).Scan(&n)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		panic(err)
	}
	return n, true
}

func TestPreviewRejectsNonSQLite(t *testing.T) {
	conn := openTestDB(t)
	insertAdmin(t, conn, "alice")

	f, err := os.CreateTemp("", "bad-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("this is not a database")
	f.Close()

	prev, err := Preview(conn, f.Name())
	if err != nil {
		t.Fatalf("Preview 不应返回硬错误: %v", err)
	}
	if prev.Valid {
		t.Fatalf("非 SQLite 文件应被判为无效")
	}
	if prev.Reason != "not_sqlite" {
		t.Fatalf("拒绝原因应为 not_sqlite，实际 %q", prev.Reason)
	}
}

func TestPreviewAndExecuteRoundTrip(t *testing.T) {
	conn := openTestDB(t)
	insertAdmin(t, conn, "alice")

	// 当前库世代
	beforeID, err := db.GetInstanceID(conn)
	if err != nil {
		t.Fatalf("读取实例ID: %v", err)
	}

	// 生成一份 pre-restore 基准备份
	backupDir := t.TempDir()
	engine, err := backup.NewEngine(conn, backupDir)
	if err != nil {
		t.Fatalf("创建备份引擎: %v", err)
	}
	item, err := engine.Snapshot("manual")
	if err != nil {
		t.Fatalf("生成基准备份: %v", err)
	}
	backupPath := filepath.Join(backupDir, item.Name)

	// 预检应认为该备份有效，且备份内仅 1 个用户
	prev, err := Preview(conn, backupPath)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !prev.Valid {
		t.Fatalf("有效备份被误判为无效: %s", prev.Error)
	}
	if prev.Backup.Users != 1 {
		t.Fatalf("备份内用户数应为 1，实际 %d", prev.Backup.Users)
	}

	// 篡改当前库：新增 bob，并把 alice 改名
	if _, err := conn.Exec(`INSERT INTO users(username, password_hash, display_name, is_admin) VALUES('bob','x','bob',0)`); err != nil {
		t.Fatalf("插入 bob: %v", err)
	}
	if _, err := conn.Exec(`UPDATE users SET display_name='alice2' WHERE username='alice'`); err != nil {
		t.Fatalf("改名 alice: %v", err)
	}
	if _, ok := userName(conn, "bob"); !ok {
		t.Fatalf("篡改后 bob 应存在")
	}

	// 执行还原
	res, err := Execute(conn, backupPath, engine)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// 还原后 bob 应消失、alice 名为原始值
	if _, ok := userName(conn, "bob"); ok {
		t.Fatalf("还原后 bob 不应存在")
	}
	if n, ok := userName(conn, "alice"); !ok || n != "alice" {
		t.Fatalf("还原后 alice 名应为 alice，实际 %q", n)
	}
	if res.Snapshot == "" {
		t.Fatalf("执行结果应回带 pre-restore 安全网快照名")
	}
	// pre-restore 快照是执行时新生成的「还原前当前库」副本，可再还原回去，
	// 文件名必然不同于传入的备份，但必须真实存在于备份目录。
	if _, err := os.Stat(filepath.Join(backupDir, res.Snapshot)); err != nil {
		t.Fatalf("pre-restore 快照文件应存在: %v", err)
	}

	// 实例 ID 必须已重写（否则客户端会静默丢同步）
	afterID, err := db.GetInstanceID(conn)
	if err != nil {
		t.Fatalf("读取实例ID: %v", err)
	}
	if afterID == beforeID {
		t.Fatalf("还原后实例ID未变化，客户端将无法感知库被替换")
	}
	if res.NewInstanceID != afterID {
		t.Fatalf("返回的实例ID与库内不一致")
	}
}

// TestPreviewDoesNotMutate 验收（m2-plan §8 恢复 #1）：预检绝不改动任何数据。
func TestPreviewDoesNotMutate(t *testing.T) {
	conn := openTestDB(t)
	insertAdmin(t, conn, "alice")

	backupDir := t.TempDir()
	engine, err := backup.NewEngine(conn, backupDir)
	if err != nil {
		t.Fatalf("创建备份引擎: %v", err)
	}
	item, err := engine.Snapshot("manual")
	if err != nil {
		t.Fatalf("生成基准备份: %v", err)
	}
	backupPath := filepath.Join(backupDir, item.Name)

	before, err := statsOf(conn)
	if err != nil {
		t.Fatalf("统计当前库: %v", err)
	}
	var tablesBefore int
	if err := conn.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`,
	).Scan(&tablesBefore); err != nil {
		t.Fatalf("统计表数: %v", err)
	}

	if _, err := Preview(conn, backupPath); err != nil {
		t.Fatalf("Preview: %v", err)
	}

	after, err := statsOf(conn)
	if err != nil {
		t.Fatalf("统计当前库: %v", err)
	}
	if before != after {
		t.Fatalf("预检改变了数据: before=%+v after=%+v", before, after)
	}
	var tablesAfter int
	if err := conn.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`,
	).Scan(&tablesAfter); err != nil {
		t.Fatalf("统计表数: %v", err)
	}
	if tablesBefore != tablesAfter {
		t.Fatalf("预检改变了表结构: %d -> %d", tablesBefore, tablesAfter)
	}
}

// TestPreviewRejectsFutureVersion 验收（m2-plan §8 恢复 #2 / RK-M2-02）：
// 备份来自更新版本时拒绝，原因明确。
func TestPreviewRejectsFutureVersion(t *testing.T) {
	conn := openTestDB(t)
	insertAdmin(t, conn, "alice")

	backupDir := t.TempDir()
	engine, err := backup.NewEngine(conn, backupDir)
	if err != nil {
		t.Fatalf("创建备份引擎: %v", err)
	}
	item, err := engine.Snapshot("manual")
	if err != nil {
		t.Fatalf("生成基准备份: %v", err)
	}
	backupPath := filepath.Join(backupDir, item.Name)

	// 把备份伪装成「未来版本」。
	bk, err := sql.Open("sqlite", backupPath)
	if err != nil {
		t.Fatalf("打开备份: %v", err)
	}
	if _, err := bk.Exec(`INSERT INTO schema_migrations(name) VALUES('099_from_the_future')`); err != nil {
		t.Fatalf("篡改备份版本: %v", err)
	}
	bk.Close()

	prev, err := Preview(conn, backupPath)
	if err != nil {
		t.Fatalf("Preview 不应返回硬错误: %v", err)
	}
	if prev.Valid {
		t.Fatalf("未来版本备份应被拒绝")
	}
	if prev.Reason != "future_version" {
		t.Fatalf("拒绝原因应为 future_version，实际 %q", prev.Reason)
	}
}

// TestRestoreOldSchemaBackupMigratesBackfill 验收（m2-plan §8 恢复 #7 / RK-M2-03）：
// 旧 schema 备份（缺新列）能成功还原，且迁移补齐缺失列。
//
// 关键前提：schema_migrations 必须随备份一起被替换，否则主库仍记录「新迁移已应用」，
// ApplyMigrations 不会重跑、缺列永远缺。本测试同时是该前提的回归守卫。
func TestRestoreOldSchemaBackupMigratesBackfill(t *testing.T) {
	conn := openTestDB(t)
	insertAdmin(t, conn, "alice")

	backupDir := t.TempDir()
	engine, err := backup.NewEngine(conn, backupDir)
	if err != nil {
		t.Fatalf("创建备份引擎: %v", err)
	}
	item, err := engine.Snapshot("manual")
	if err != nil {
		t.Fatalf("生成基准备份: %v", err)
	}
	backupPath := filepath.Join(backupDir, item.Name)

	// 把备份降级成「旧版本」：删掉 013 的列 + 对应迁移记录。
	// 注意先删引用该列的索引，否则 DROP COLUMN 会报错。
	bk, err := sql.Open("sqlite", backupPath)
	if err != nil {
		t.Fatalf("打开备份: %v", err)
	}
	if _, err := bk.Exec(`DROP INDEX IF EXISTS idx_txn_client_request`); err != nil {
		t.Fatalf("模拟旧 schema（删索引）: %v", err)
	}
	if _, err := bk.Exec(`ALTER TABLE transactions DROP COLUMN client_request_id`); err != nil {
		t.Fatalf("模拟旧 schema（删列）: %v", err)
	}
	if _, err := bk.Exec(`DELETE FROM schema_migrations WHERE name='013_txn_idempotency'`); err != nil {
		t.Fatalf("模拟旧 schema（删迁移记录）: %v", err)
	}
	bk.Close()

	if _, err := Execute(conn, backupPath, engine); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// 迁移补齐：013 的列必须回来，且迁移记录恢复完整。
	var colN int
	if err := conn.QueryRow(
		`SELECT count(*) FROM pragma_table_info('transactions') WHERE name='client_request_id'`,
	).Scan(&colN); err != nil {
		t.Fatalf("检查列: %v", err)
	}
	if colN == 0 {
		t.Fatalf("还原旧 schema 备份后 client_request_id 列缺失，迁移未补齐")
	}
	var n int
	if err := conn.QueryRow(
		`SELECT count(*) FROM schema_migrations WHERE name='013_txn_idempotency'`,
	).Scan(&n); err != nil {
		t.Fatalf("查迁移记录: %v", err)
	}
	if n != 1 {
		t.Fatalf("还原后迁移记录应恢复，013 记录数 = %d", n)
	}
}

// TestRestoreSnapshotRoundTrip 验收（m2-plan §8 恢复 #5）：pre-restore 快照可再还原回去。
func TestRestoreSnapshotRoundTrip(t *testing.T) {
	conn := openTestDB(t)
	insertAdmin(t, conn, "alice")

	backupDir := t.TempDir()
	engine, err := backup.NewEngine(conn, backupDir)
	if err != nil {
		t.Fatalf("创建备份引擎: %v", err)
	}
	item, err := engine.Snapshot("manual")
	if err != nil {
		t.Fatalf("生成基准备份: %v", err)
	}
	backupPath := filepath.Join(backupDir, item.Name)

	// 篡改：新增 bob。
	if _, err := conn.Exec(`INSERT INTO users(username, password_hash, display_name, is_admin) VALUES('bob','x','bob',0)`); err != nil {
		t.Fatalf("插入 bob: %v", err)
	}

	// 第一次还原（此时 pre-restore 快照 = 含 bob 的当前库）。
	res, err := Execute(conn, backupPath, engine)
	if err != nil {
		t.Fatalf("第一次 Execute: %v", err)
	}
	if _, ok := userName(conn, "bob"); ok {
		t.Fatalf("第一次还原后 bob 不应存在")
	}

	// 用 pre-restore 快照再还原回去：bob 应回来。
	snapshotPath := filepath.Join(backupDir, res.Snapshot)
	if _, err := Execute(conn, snapshotPath, engine); err != nil {
		t.Fatalf("用快照还原回去: %v", err)
	}
	if _, ok := userName(conn, "bob"); !ok {
		t.Fatalf("用 pre-restore 快照还原后 bob 应存在（快照可回退）")
	}
}
