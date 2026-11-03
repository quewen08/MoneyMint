package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RetentionResult 描述一次清理的结果。
type RetentionResult struct {
	// Removed 是被删除的备份名（主产物 .db）
	Removed []string
	// FreedBytes 是释放的空间（主产物 + 附加产物）
	FreedBytes int64
	// Kept 是清理后保留的备份数
	Kept int
}

// ApplyRetention 按保留策略清理旧备份。
//
// 保留规则（三者取并集，互不排斥）：
//  1. 最近 keepDaily 份成功备份（不论日期）
//  2. 最近 keepMonthly 个「每月至少一份」（维持长期历史，避免只有最近两周）
//  3. 全部 manual 触发且成功的备份（用户主动创建的，不自动清理）
//
// 约束：
//   - 只处理成功的备份；失败记录保留在清单里供排查（其文件在失败时已被清理）
//   - 删除前逐个文件过 ValidSnapshotName 白名单，绝不使用通配符删除
//   - 与快照互斥（复用 runMu），避免「边备份边删」导致计数错乱
func (e *Engine) ApplyRetention(keepDaily, keepMonthly int) (*RetentionResult, error) {
	if keepDaily < 1 {
		keepDaily = 1
	}
	if keepMonthly < 0 {
		keepMonthly = 0
	}

	// 与 Snapshot 互斥：拿不到锁说明正在备份，本次跳过（下次备份成功后会再清理）
	if !e.runMu.TryLock() {
		return nil, ErrBusy
	}
	defer e.runMu.Unlock()

	e.fileMu.Lock()
	defer e.fileMu.Unlock()

	items, err := e.readManifestLocked() // 已按时间倒序
	if err != nil {
		return nil, err
	}

	keep := make(map[string]bool, len(items))

	// 规则 1：最近 keepDaily 份成功备份
	successCount := 0
	for _, it := range items {
		if !it.Succeeded() {
			continue
		}
		if successCount < keepDaily {
			keep[it.Name] = true
			successCount++
		}
	}

	// 规则 2：最近 keepMonthly 个自然月，每月保留一份
	if keepMonthly > 0 {
		seenMonth := make(map[string]bool, keepMonthly)
		for _, it := range items {
			if !it.Succeeded() {
				continue
			}
			month := it.CreatedAt.Format("2006-01")
			if seenMonth[month] {
				continue
			}
			if len(seenMonth) >= keepMonthly {
				break
			}
			seenMonth[month] = true
			keep[it.Name] = true
		}
	}

	// 规则 3：手动备份永不自动清理
	for _, it := range items {
		if it.Succeeded() && it.Trigger == "manual" {
			keep[it.Name] = true
		}
	}

	var (
		remaining []Item
		result    = &RetentionResult{}
	)
	for _, it := range items {
		if !it.Succeeded() {
			// 失败记录：保留（无文件可删），但超过 90 天的清理掉避免清单无限增长
			if time.Since(it.CreatedAt) > 90*24*time.Hour {
				continue
			}
			remaining = append(remaining, it)
			continue
		}
		if keep[it.Name] {
			remaining = append(remaining, it)
			continue
		}

		// 待删除：先删附加产物，再删主产物
		for _, bean := range it.BeanFiles {
			result.FreedBytes += e.removeSnapshotFile(bean)
		}
		result.FreedBytes += e.removeSnapshotFile(it.Name)
		result.Removed = append(result.Removed, it.Name)
	}
	result.Kept = len(remaining)

	if len(result.Removed) > 0 {
		if err := e.writeManifestLocked(remaining); err != nil {
			return result, fmt.Errorf("清理后写入清单: %w", err)
		}
		e.logf("backup: 清理完成 删除=%d 份 保留=%d 份 释放=%dKB",
			len(result.Removed), result.Kept, result.FreedBytes/1024)
	}

	return result, nil
}

// removeSnapshotFile 删除一个备份文件并返回释放的字节数。
// 文件名必须先过白名单：这条防线保证清理逻辑永远不会删到备份目录之外。
func (e *Engine) removeSnapshotFile(name string) int64 {
	if !ValidSnapshotName(name) {
		e.logf("backup: 跳过非法文件名 %q（清理阶段）", name)
		return 0
	}
	full := filepath.Join(e.dir, filepath.Base(name))
	info, err := os.Stat(full)
	if err != nil {
		return 0
	}
	if err := os.Remove(full); err != nil {
		e.logf("backup: 删除 %s 失败: %v", name, err)
		return 0
	}
	return info.Size()
}
