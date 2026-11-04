// Package ratelimit 提供进程内的固定窗口失败计数限流（登录暴力破解防护）。
//
// 为什么放内存而不是落库：
//   - 家庭 NAS 场景是内网低对抗环境，限流只需挡住「脚本连续试密码」，
//     重启清零完全可接受；
//   - 登录是认证前置路径，此时还没有会话/账本上下文，为它加一张表 +
//     每次失败写盘（WAL 争用）不值得；
//   - 需要持久化时再换实现，接口不变。
//
// 窗口语义：`Window` 内最多失败 `Max` 次；超过后锁 `Lockout`，锁到期自动
// 重置计数。成功登录调用 Reset 清零（避免正常用户偶尔输错被累计）。
package ratelimit

import (
	"sync"
	"time"
)

// Limiter 是一个并发安全的固定窗口失败计数器。
type Limiter struct {
	max     int
	window  time.Duration
	lockout time.Duration
	now     func() time.Time // 便于测试注入时钟

	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	fails       int
	windowStart time.Time
	lockedUntil time.Time
}

// New 构造限流器。max<=0 或 window<=0 时退化为「不限流」（返回 nil 安全对象）。
func New(max int, window, lockout time.Duration) *Limiter {
	if max <= 0 || window <= 0 {
		max = 0
	}
	return &Limiter{
		max:     max,
		window:  window,
		lockout: lockout,
		now:     time.Now,
		entries: make(map[string]*entry),
	}
}

// Enabled 表示是否真正生效。
func (l *Limiter) Enabled() bool { return l != nil && l.max > 0 }

// Allow 判断该 key 当前是否允许尝试。被锁时返回 false 与剩余等待时长。
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if !l.Enabled() {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	e := l.entries[key]
	if e == nil {
		return true, 0
	}
	now := l.now()
	if now.Before(e.lockedUntil) {
		return false, e.lockedUntil.Sub(now)
	}
	// 窗口过期则视为重新开始
	if now.Sub(e.windowStart) >= l.window && e.lockedUntil.IsZero() {
		delete(l.entries, key)
	}
	return true, 0
}

// Fail 记录一次失败；达到阈值则进入锁定。
func (l *Limiter) Fail(key string) {
	if !l.Enabled() {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	e := l.entries[key]
	if e == nil {
		e = &entry{windowStart: now}
		l.entries[key] = e
	} else if now.Sub(e.windowStart) >= l.window {
		// 固定窗口：过了窗口重新计数
		e.fails = 0
		e.windowStart = now
		e.lockedUntil = time.Time{}
	}
	e.fails++
	if e.fails >= l.max {
		e.lockedUntil = now.Add(l.lockout)
		e.fails = 0
	}
	l.sweep(now)
}

// Reset 清除某 key 的失败记录（登录成功时调用）。
func (l *Limiter) Reset(key string) {
	if !l.Enabled() {
		return
	}
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

// RetryAfter 返回被锁时的剩余时长（未锁返回 0），用于响应头/文案。
func (l *Limiter) RetryAfter(key string) time.Duration {
	allowed, d := l.Allow(key)
	if allowed {
		return 0
	}
	return d
}

// sweep 惰性清理过期条目，避免长时间运行后 map 无限增长。
// 只在条目数超过阈值时才跑，正常家庭规模不会触发。
func (l *Limiter) sweep(now time.Time) {
	if len(l.entries) < 1024 {
		return
	}
	for k, e := range l.entries {
		if now.Sub(e.windowStart) >= l.window && now.After(e.lockedUntil) {
			delete(l.entries, k)
		}
	}
}
