package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// seedBackups 造 count 份每日备份（写入真实空文件 + 清单记录）。
// start 为最早一份的时间，后续逐日递增。
func seedBackups(t *testing.T, e *Engine, count int, start time.Time, trigger string) []Item {
	t.Helper()
	items := make([]Item, 0, count)
	for i := 0; i < count; i++ {
		at := start.AddDate(0, 0, i)
		name := "ledger-" + at.Format(snapshotTimeFmt) + ".db"
		full := filepath.Join(e.Dir(), name)
		if err := os.WriteFile(full, []byte("fake-snapshot"), 0o644); err != nil {
			t.Fatalf("写入假备份 %s: %v", name, err)
		}
		it := Item{
			Name:      name,
			Size:      13,
			SHA256:    "deadbeef",
			Trigger:   trigger,
			CreatedAt: at,
		}
		if err := e.appendManifest(it); err != nil {
			t.Fatalf("写入清单: %v", err)
		}
		items = append(items, it)
	}
	return items
}

// TestRetentionKeepsDailyWindow 每日备份超量时只保留最近 N 份。
func TestRetentionKeepsDailyWindow(t *testing.T) {
	engine, _, _ := newTestEngine(t, 0)

	start := time.Now().AddDate(0, 0, -19).Truncate(time.Hour)
	seedBackups(t, engine, 20, start, "auto")

	res, err := engine.ApplyRetention(14, 0) // 关闭月度规则，只验每日窗口
	if err != nil {
		t.Fatalf("apply retention: %v", err)
	}
	if len(res.Removed) != 6 {
		t.Fatalf("20 份保留 14 份，应删除 6 份，实际删除 %d 份：%v", len(res.Removed), res.Removed)
	}
	if res.Kept != 14 {
		t.Fatalf("应保留 14 份，实际 %d 份", res.Kept)
	}

	items, err := engine.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 14 {
		t.Fatalf("清单应剩 14 条，实际 %d 条", len(items))
	}

	// 最旧的 6 份文件应已删除，最新的应还在
	for _, name := range res.Removed {
		if _, err := os.Stat(filepath.Join(engine.Dir(), name)); !os.IsNotExist(err) {
			t.Fatalf("被清理的备份文件 %s 仍然存在", name)
		}
	}
	newest := items[0].Name
	if _, err := os.Stat(filepath.Join(engine.Dir(), newest)); err != nil {
		t.Fatalf("最新备份 %s 不应被删除: %v", newest, err)
	}
}

// TestRetentionKeepsManualBackups 手动备份不参与自动清理。
func TestRetentionKeepsManualBackups(t *testing.T) {
	engine, _, _ := newTestEngine(t, 0)

	// 30 天前的一份手动备份
	manualAt := time.Now().AddDate(0, 0, -30).Truncate(time.Hour)
	seedBackups(t, engine, 1, manualAt, "manual")
	// 之后 20 份自动备份
	seedBackups(t, engine, 20, time.Now().AddDate(0, 0, -19).Truncate(time.Hour), "auto")

	res, err := engine.ApplyRetention(14, 0)
	if err != nil {
		t.Fatalf("apply retention: %v", err)
	}

	manualName := "ledger-" + manualAt.Format(snapshotTimeFmt) + ".db"
	for _, removed := range res.Removed {
		if removed == manualName {
			t.Fatal("手动备份被自动清理删除了")
		}
	}
	if _, err := os.Stat(filepath.Join(engine.Dir(), manualName)); err != nil {
		t.Fatalf("手动备份文件应保留: %v", err)
	}
}

// TestRetentionKeepsMonthlyHistory 即使每日份数很少，也应保住每月至少一份。
func TestRetentionKeepsMonthlyHistory(t *testing.T) {
	engine, _, _ := newTestEngine(t, 0)

	// 造 5 个月数据，每月 3 份，共 15 份
	base := time.Now().AddDate(0, -4, 0).Truncate(time.Hour)
	var all []Item
	for m := 0; m < 5; m++ {
		for d := 0; d < 3; d++ {
			at := base.AddDate(0, m, d).Truncate(time.Hour)
			name := "ledger-" + at.Format(snapshotTimeFmt) + ".db"
			if err := os.WriteFile(filepath.Join(engine.Dir(), name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			it := Item{Name: name, Trigger: "auto", CreatedAt: at, Size: 1}
			if err := engine.appendManifest(it); err != nil {
				t.Fatal(err)
			}
			all = append(all, it)
		}
	}

	// 只按每日保留 2 份，但月度保留 5 个月 → 应该每月都留下至少一份
	if _, err := engine.ApplyRetention(2, 5); err != nil {
		t.Fatalf("apply retention: %v", err)
	}

	items, err := engine.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	months := map[string]int{}
	for _, it := range items {
		months[it.CreatedAt.Format("2006-01")]++
	}
	if len(months) != 5 {
		t.Fatalf("应保留 5 个月的历史，实际只有 %d 个月：%v", len(months), months)
	}
}

// TestRetentionSkipsIllegalName 非法文件名不会被删除（白名单保护）。
func TestRetentionSkipsIllegalName(t *testing.T) {
	engine, _, _ := newTestEngine(t, 0)

	// 在备份目录外放一个文件，若清理逻辑存在穿越 bug 会删掉它
	outsideDir := filepath.Dir(engine.Dir())
	outside := filepath.Join(outsideDir, "must-not-delete.txt")
	if err := os.WriteFile(outside, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	// 清单里塞一条非法名记录（模拟清单被篡改）
	if err := engine.appendManifest(Item{
		Name:      "../must-not-delete.txt",
		Trigger:   "auto",
		CreatedAt: time.Now().AddDate(0, 0, -100),
		Size:      7,
	}); err != nil {
		t.Fatal(err)
	}
	seedBackups(t, engine, 3, time.Now().AddDate(0, 0, -3).Truncate(time.Hour), "auto")

	if _, err := engine.ApplyRetention(1, 0); err != nil {
		t.Fatalf("apply retention: %v", err)
	}

	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("备份目录之外的文件被误删：%v", err)
	}
}

// TestRetentionNoopWhenWithinLimits 未超限时不做任何删除。
func TestRetentionNoopWhenWithinLimits(t *testing.T) {
	engine, _, _ := newTestEngine(t, 0)
	seedBackups(t, engine, 3, time.Now().AddDate(0, 0, -2).Truncate(time.Hour), "auto")

	res, err := engine.ApplyRetention(14, 12)
	if err != nil {
		t.Fatalf("apply retention: %v", err)
	}
	if len(res.Removed) != 0 {
		t.Fatalf("未超限不应删除任何备份，实际删除 %v", res.Removed)
	}
	if res.Kept != 3 {
		t.Fatalf("应保留 3 份，实际 %d", res.Kept)
	}
}
