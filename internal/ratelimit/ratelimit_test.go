package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterLocksAfterMaxFails(t *testing.T) {
	clock := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	l := New(3, 15*time.Minute, 10*time.Minute)
	l.now = func() time.Time { return clock }

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("u:alice"); !ok {
			t.Fatalf("第 %d 次尝试不应被锁", i+1)
		}
		l.Fail("u:alice")
	}
	ok, wait := l.Allow("u:alice")
	if ok {
		t.Fatalf("连续失败 3 次后应被锁定")
	}
	if wait <= 0 || wait > 10*time.Minute {
		t.Fatalf("等待时长异常: %v", wait)
	}

	// 其他 key 不受影响（按用户名独立计数）
	if ok, _ := l.Allow("u:bob"); !ok {
		t.Fatalf("不同用户不应互相牵连")
	}

	// 锁定到期后恢复
	clock = clock.Add(10*time.Minute + time.Second)
	if ok, _ := l.Allow("u:alice"); !ok {
		t.Fatalf("锁定到期后应恢复")
	}
}

func TestLimiterWindowResetsCount(t *testing.T) {
	clock := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	l := New(3, 10*time.Minute, 10*time.Minute)
	l.now = func() time.Time { return clock }

	l.Fail("ip:1.2.3.4")
	l.Fail("ip:1.2.3.4")
	// 窗口过期：计数清零，不因历史失败被锁
	clock = clock.Add(11 * time.Minute)
	l.Fail("ip:1.2.3.4")
	if ok, _ := l.Allow("ip:1.2.3.4"); !ok {
		t.Fatalf("跨窗口后计数应重置")
	}
}

func TestLimiterResetOnSuccess(t *testing.T) {
	clock := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	l := New(2, 10*time.Minute, 10*time.Minute)
	l.now = func() time.Time { return clock }

	l.Fail("u:carol")
	l.Reset("u:carol")
	l.Fail("u:carol") // 只有一次失败，不该锁
	if ok, _ := l.Allow("u:carol"); !ok {
		t.Fatalf("成功登录应清零计数")
	}
}

func TestLimiterDisabled(t *testing.T) {
	l := New(0, 0, 0)
	if l.Enabled() {
		t.Fatalf("max<=0 时应关闭限流")
	}
	for i := 0; i < 100; i++ {
		l.Fail("u:x")
	}
	if ok, _ := l.Allow("u:x"); !ok {
		t.Fatalf("未启用时永远放行")
	}
}
