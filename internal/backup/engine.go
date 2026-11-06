// Package backup 提供账本数据的一致性快照与保留管理。
//
// 核心约束（务必理解后再改动）：
//
// 数据库运行在 WAL 模式（见 internal/db/db.go），未 checkpoint 的数据驻留在
// `-wal` 文件中。实测：一份 4 KB 的主库文件对应 2 MB 的 WAL，直接复制主文件
// 得到的是「有表结构、零数据」的空壳（实测 500 行 → 0 行）。因此备份
// 必须用 SQLite 的 `VACUUM INTO` 生成一致性快照，禁止复制数据库文件。
//
// 全库级操作：备份涵盖所有账本与用户，调用方必须先校验系统管理员权限。
package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"familyledger/internal/export"
	"familyledger/internal/identity"

	_ "modernc.org/sqlite"
)

const (
	// snapshotTimeout 限制单次快照耗时。
	// 数据库连接池被限制为 1（SetMaxOpenConns(1)），VACUUM INTO 期间会独占该连接，
	// 所有 API 请求排队等待。超时保护用于避免异常情况下全站被长期阻塞。
	snapshotTimeout = 30 * time.Second

	snapshotPrefix  = "ledger-"
	snapshotTimeFmt = "20060102-150405"

	// manifestName 是备份清单文件名，记录每份备份的元信息。
	manifestName = "manifest.json"
)

// ErrBusy 表示已有备份任务在执行（同一时刻只允许一个快照）。
var ErrBusy = errors.New("已有备份任务正在执行，请稍后再试")

// snapshotNameRe 校验备份产物文件名合法性，防止清理与下载环节被路径穿越利用。
//
// 合法的三种形态：
//
//	ledger-20261103-030000.db               主产物（SQLite 快照）
//	ledger-20261103-030000.beancount        可选的整体导出
//	ledger-20261103-030000.l1.beancount     按账本导出的附加产物
//
// 同秒重复触发时会带 -001 之类的毫秒/序号后缀。
var snapshotNameRe = regexp.MustCompile(`^ledger-\d{8}-\d{6}(-\d{3})?(\.l\d+)?\.(db|beancount)$`)

// IdentityWarning 是备份快照的恒等式失衡告警（只记日志/清单，不阻断备份）。
type IdentityWarning struct {
	LedgerID int64           `json:"ledger_id"`
	Diffs    []identity.Diff `json:"diffs"`
}

// Item 是一份备份的元信息（写入 manifest.json，也是 API 返回结构）。
type Item struct {
	Name       string           `json:"name"`
	BeanFiles  []string         `json:"bean_files,omitempty"`
	Size       int64            `json:"size"`
	SHA256     string           `json:"sha256"`
	Trigger    string           `json:"trigger"` // auto | manual
	CreatedAt  time.Time        `json:"created_at"`
	DurationMS int64            `json:"duration_ms"`
	Stats      map[string]int64 `json:"stats,omitempty"`
	// IdentityWarnings 记录快照后检查发现的恒等式失衡账本（不阻断备份）。
	IdentityWarnings []IdentityWarning `json:"identity_warnings,omitempty"`
	// Error 非空表示该次备份失败（失败记录保留在清单里，便于排查，但不参与保留计数）
	Error string `json:"error,omitempty"`
}

// Succeeded 表示该记录是否为一次成功的备份。
func (i Item) Succeeded() bool { return i.Error == "" }

// manifest 是 manifest.json 的磁盘格式。
type manifest struct {
	Items []Item `json:"items"`
}

// Engine 负责生成快照与维护备份清单。
type Engine struct {
	db  *sql.DB
	dir string

	// runMu 串行化快照任务（TryLock 实现「忙则立即失败」而非排队）。
	runMu sync.Mutex
	// fileMu 保护下面两个字段与清单文件的一致性。
	fileMu sync.Mutex
	// lastErr 记录最近一次自动备份失败原因
	lastErr error
	// lastSuccess 记录最近一次成功备份时间
	lastSuccess time.Time
	// logf 可注入的日志函数（测试中可静默）
	logf func(format string, args ...any)
}

// NewEngine 创建快照引擎；dir 不存在时自动创建。
func NewEngine(db *sql.DB, dir string) (*Engine, error) {
	if db == nil {
		return nil, fmt.Errorf("backup: db 不能为 nil")
	}
	if dir == "" {
		return nil, fmt.Errorf("backup: 备份目录不能为空")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("解析备份目录: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("创建备份目录 %s: %w", abs, err)
	}
	e := &Engine{db: db, dir: abs, logf: log.Printf}
	// 载入历史状态，供启动补跑判断
	if items, err := e.List(); err == nil {
		for _, it := range items {
			if it.Succeeded() && it.CreatedAt.After(e.lastSuccess) {
				e.lastSuccess = it.CreatedAt
			}
		}
	}
	return e, nil
}

// Dir 返回备份目录的绝对路径。
func (e *Engine) Dir() string { return e.dir }

// LockRunForTest / UnlockRunForTest 占用备份锁，仅供跨包测试模拟「备份进行中」。
// 生产代码不得调用；必须配对使用。
func (e *Engine) LockRunForTest()   { e.runMu.Lock() }
func (e *Engine) UnlockRunForTest() { e.runMu.Unlock() }

// LastSuccess 返回最近一次成功备份的时间（零值表示从未备份）。
func (e *Engine) LastSuccess() time.Time {
	e.fileMu.Lock()
	defer e.fileMu.Unlock()
	return e.lastSuccess
}

// Snapshot 生成一份一致性快照。
//
// 步骤严格按序：拿锁 → 生成唯一文件名 → VACUUM INTO → 完整性校验 →
// 行数比对 → 计算哈希 → 导出 beancount → 写清单。
// 任一步失败都会清理残缺文件，绝不留下「看起来有备份、实际不可用」的文件。
func (e *Engine) Snapshot(trigger string) (*Item, error) {
	if !e.runMu.TryLock() {
		return nil, ErrBusy
	}
	defer e.runMu.Unlock()

	started := time.Now()
	name, target, err := e.uniqueName(started)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), snapshotTimeout)
	defer cancel()

	// 1) 一致性快照（WAL 中未落盘的数据会一并合入目标文件）
	if _, err := e.db.ExecContext(ctx, `VACUUM INTO ?`, target); err != nil {
		e.removeQuietly(target)
		return e.fail(trigger, name, started, fmt.Errorf("生成快照: %w", err))
	}

	// 2) 校验：完整性 + 行数
	stats, err := e.validate(target)
	if err != nil {
		e.removeQuietly(target)
		return e.fail(trigger, name, started, err)
	}

	// 2.1) 恒等式检查：仅告警、不阻断（设计 §3.4.4）。
	warnings := e.checkIdentityWarnings(target)

	// 3) 哈希与体积
	size, sum, err := fileDigest(target)
	if err != nil {
		e.removeQuietly(target)
		return e.fail(trigger, name, started, fmt.Errorf("计算校验和: %w", err))
	}

	// 4) 附加产物：每个账本导出一份 beancount（人可读、可手工抢救）。
	//    从快照库导出而非主库，确保与原数据严格一致；失败仅告警，不影响备份成立。
	beans := e.exportBeans(target, name)

	finished := time.Now()
	item := Item{
		Name:             name,
		BeanFiles:        beans,
		Size:             size,
		SHA256:           sum,
		Trigger:          trigger,
		CreatedAt:        started.UTC(),
		DurationMS:       finished.Sub(started).Milliseconds(),
		Stats:            stats,
		IdentityWarnings: warnings,
	}

	if err := e.appendManifest(item); err != nil {
		// 清单写不进去时快照文件已存在，仍应告知调用方，但不要删除数据
		return &item, fmt.Errorf("写入备份清单: %w", err)
	}

	e.fileMu.Lock()
	e.lastSuccess = item.CreatedAt
	e.lastErr = nil
	e.fileMu.Unlock()

	e.logf("backup: 快照完成 name=%s size=%dKB rows=%v 耗时=%dms trigger=%s",
		name, size/1024, stats, item.DurationMS, trigger)
	return &item, nil
}

// fail 记录一次失败备份（写入清单便于运维排查），并返回错误。
func (e *Engine) fail(trigger, name string, started time.Time, cause error) (*Item, error) {
	e.logf("backup: 快照失败 name=%s trigger=%s err=%v", name, trigger, cause)

	item := Item{
		Name:       name,
		Trigger:    trigger,
		CreatedAt:  started.UTC(),
		DurationMS: time.Since(started).Milliseconds(),
		Error:      cause.Error(),
	}
	if err := e.appendManifest(item); err != nil {
		e.logf("backup: 记录失败备份时出错: %v", err)
	}

	e.fileMu.Lock()
	e.lastErr = cause
	e.fileMu.Unlock()

	return nil, cause
}

// uniqueName 生成不冲突的备份文件名。
//
// VACUUM INTO 要求目标文件不存在（实测会报 "output file already exists"），
// 因此同一秒内重复触发时追加毫秒后缀。
func (e *Engine) uniqueName(at time.Time) (string, string, error) {
	base := snapshotPrefix + at.Format(snapshotTimeFmt)
	for attempt := 0; attempt < 1000; attempt++ {
		name := base + ".db"
		if attempt > 0 {
			name = fmt.Sprintf("%s-%03d.db", base, attempt)
		}
		target := filepath.Join(e.dir, name)
		if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
			return name, target, nil
		} else if err != nil {
			return "", "", fmt.Errorf("检查目标文件: %w", err)
		}
	}
	return "", "", fmt.Errorf("无法生成唯一备份文件名")
}

// validate 校验快照可用性：完整性检查 + 与主库行数比对。
//
// 行数比对的语义：快照生成期间主库仍可能有写入，因此「快照行数 <= 主库行数」正常；
// 若出现「快照多于主库」说明数据来源异常（例如误取了他库），必须拒绝。
func (e *Engine) validate(snapshotPath string) (map[string]int64, error) {
	snap, err := sql.Open("sqlite", snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("打开快照: %w", err)
	}
	defer snap.Close()

	var integrity string
	if err := snap.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return nil, fmt.Errorf("完整性检查执行失败: %w", err)
	}
	if integrity != "ok" {
		return nil, fmt.Errorf("快照完整性检查未通过: %s", integrity)
	}

	tables := []string{"users", "ledgers", "accounts", "transactions", "postings"}
	stats := make(map[string]int64, len(tables))
	for _, t := range tables {
		var snapCount int64
		if err := snap.QueryRow(`SELECT count(*) FROM ` + t).Scan(&snapCount); err != nil {
			return nil, fmt.Errorf("统计快照表 %s: %w", t, err)
		}
		var srcCount int64
		if err := e.db.QueryRow(`SELECT count(*) FROM ` + t).Scan(&srcCount); err != nil {
			return nil, fmt.Errorf("统计主库表 %s: %w", t, err)
		}
		if snapCount > srcCount {
			return nil, fmt.Errorf("快照表 %s 行数(%d)大于主库(%d)，数据来源异常",
				t, snapCount, srcCount)
		}
		stats[t] = snapCount
	}
	return stats, nil
}

// checkIdentityWarnings 对快照库执行全账本恒等式检查，返回失衡账本（仅告警）。
func (e *Engine) checkIdentityWarnings(snapshotPath string) []IdentityWarning {
	snap, err := sql.Open("sqlite", snapshotPath)
	if err != nil {
		e.logf("backup: 打开快照做恒等式检查失败: %v", err)
		return nil
	}
	defer snap.Close()
	results, err := identity.CheckAll(snap)
	if err != nil {
		e.logf("backup: 恒等式检查失败: %v", err)
		return nil
	}
	var warnings []IdentityWarning
	for _, r := range results {
		if r.Balanced {
			continue
		}
		w := IdentityWarning{LedgerID: r.LedgerID, Diffs: r.Diffs}
		warnings = append(warnings, w)
		e.logf("backup: 账本 %d 恒等式失衡（备份已保留）: %v", r.LedgerID, r.Diffs)
	}
	return warnings
}

// exportBeans 从快照库为每个账本导出一份 beancount 文本。
//
// 这是附加产物：失败不影响备份成立（db 快照才是权威数据），
// 仅记录告警，避免因导出器的边角问题导致整个备份失败。
func (e *Engine) exportBeans(snapshotPath, snapshotName string) []string {
	snap, err := sql.Open("sqlite", snapshotPath)
	if err != nil {
		e.logf("backup: 打开快照导出 bean 失败: %v", err)
		return nil
	}
	defer snap.Close()

	rows, err := snap.Query(`SELECT id FROM ledgers ORDER BY id`)
	if err != nil {
		e.logf("backup: 查询账本失败: %v", err)
		return nil
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	base := snapshotName[:len(snapshotName)-len(".db")]
	var files []string
	for _, id := range ids {
		text, err := export.ExportLedger(snap, id)
		if err != nil {
			e.logf("backup: 导出账本 %d 的 bean 失败: %v", id, err)
			continue
		}
		fname := fmt.Sprintf("%s.l%d.beancount", base, id)
		full := filepath.Join(e.dir, fname)
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			e.logf("backup: 写入 %s 失败: %v", fname, err)
			continue
		}
		files = append(files, fname)
	}
	return files
}

// ---- 清单读写 ----

// List 返回全部备份记录，按时间倒序（最新在前）。
func (e *Engine) List() ([]Item, error) {
	e.fileMu.Lock()
	defer e.fileMu.Unlock()
	return e.readManifestLocked()
}

// readManifestLocked 读取清单（调用方需持有 fileMu）。
func (e *Engine) readManifestLocked() ([]Item, error) {
	data, err := os.ReadFile(filepath.Join(e.dir, manifestName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取备份清单: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析备份清单: %w", err)
	}
	sort.Slice(m.Items, func(i, j int) bool {
		return m.Items[i].CreatedAt.After(m.Items[j].CreatedAt)
	})
	return m.Items, nil
}

// appendManifest 追加一条记录（原子写：先写临时文件再重命名）。
func (e *Engine) appendManifest(item Item) error {
	e.fileMu.Lock()
	defer e.fileMu.Unlock()

	items, err := e.readManifestLocked()
	if err != nil {
		// 清单损坏时以本次记录重建，避免整个备份功能被坏文件卡死
		e.logf("backup: 清单损坏，按本次记录重建: %v", err)
		items = nil
	}
	return e.writeManifestLocked(append(items, item))
}

// writeManifestLocked 原子写入整份清单（调用方需持有 fileMu）。
func (e *Engine) writeManifestLocked(items []Item) error {
	data, err := json.MarshalIndent(manifest{Items: items}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化备份清单: %w", err)
	}

	tmp := filepath.Join(e.dir, manifestName+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("写入临时清单: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(e.dir, manifestName)); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("替换清单文件: %w", err)
	}
	return nil
}

// ---- 工具函数 ----

// ValidSnapshotName 判断文件名是否为合法的备份产物名。
// 下载与清理都必须先过这一关，防止路径穿越与误删。
func ValidSnapshotName(name string) bool {
	return snapshotNameRe.MatchString(name)
}

// removeQuietly 删除文件，忽略错误（清理路径不应掩盖原始错误）。
func (e *Engine) removeQuietly(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		e.logf("backup: 清理残留文件 %s 失败: %v", path, err)
	}
}

// fileDigest 计算文件大小与 SHA256。
func fileDigest(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return 0, "", err
	}

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return 0, "", err
	}
	return st.Size(), hex.EncodeToString(h.Sum(nil)), nil
}
