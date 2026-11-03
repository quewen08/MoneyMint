package backup

import (
	"math/rand"
	"sync"
	"time"
)

// catchUpWindow 定义「多久没备份就需要补跑」。
// NAS 常关机，错过定时点是常态，因此启动时必须检查而不是被动等待下一个整点。
const catchUpWindow = 24 * time.Hour

// catchUpJitterMax 是启动补跑的随机延迟上限。
// 多实例/多进程同时启动时错开，避免同一秒齐齐打库。
const catchUpJitterMax = 60 * time.Second

// Scheduler 负责「每日定时备份 + 启动补跑 + 清理」。
type Scheduler struct {
	engine      *Engine
	hour        int
	keepDaily   int
	keepMonthly int
	logf        func(format string, args ...any)

	stop chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

// NewScheduler 构造调度器。hour 取值 0-23（按进程本地时区）。
func NewScheduler(engine *Engine, hour, keepDaily, keepMonthly int) *Scheduler {
	if hour < 0 || hour > 23 {
		hour = 3
	}
	if keepDaily < 1 {
		keepDaily = 14
	}
	if keepMonthly < 0 {
		keepMonthly = 12
	}
	return &Scheduler{
		engine:      engine,
		hour:        hour,
		keepDaily:   keepDaily,
		keepMonthly: keepMonthly,
		logf:        engine.logf,
		stop:        make(chan struct{}),
	}
}

// Start 启动后台协程。非阻塞。
func (s *Scheduler) Start() {
	s.wg.Add(1)
	go s.loop()
	s.logf("backup: 调度器已启动 hour=%d keepDaily=%d keepMonthly=%d dir=%s",
		s.hour, s.keepDaily, s.keepMonthly, s.engine.Dir())
}

// Stop 停止调度并等待当前任务收尾（用于优雅停机）。
func (s *Scheduler) Stop() {
	s.once.Do(func() { close(s.stop) })
	s.wg.Wait()
	s.logf("backup: 调度器已停止")
}

// loop 是调度主循环：先检查补跑，再进入每日定时。
func (s *Scheduler) loop() {
	defer s.wg.Done()

	if s.needCatchUp(time.Now()) {
		last := s.engine.LastSuccess()
		if last.IsZero() {
			s.logf("backup: 尚无任何备份记录，启动补跑")
		} else {
			s.logf("backup: 距上次成功备份已 %s，启动补跑", time.Since(last).Round(time.Minute))
		}
		// 随机延迟，错开多实例同时启动
		jitter := time.Duration(rand.Int63n(int64(catchUpJitterMax)))
		select {
		case <-time.After(jitter):
			s.runOnce()
		case <-s.stop:
			return
		}
	}

	for {
		next := s.nextTrigger(time.Now())
		wait := time.Until(next)
		s.logf("backup: 下次自动备份 %s（%s 后）", next.Format("2006-01-02 15:04"), wait.Round(time.Minute))

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
			s.runOnce()
		case <-s.stop:
			timer.Stop()
			return
		}
	}
}

// needCatchUp 判断是否需要启动补跑。
func (s *Scheduler) needCatchUp(now time.Time) bool {
	last := s.engine.LastSuccess()
	if last.IsZero() {
		// 从未备份过：若库里还没数据也不急，但补一次成本极低，直接补
		return true
	}
	return now.Sub(last) >= catchUpWindow
}

// nextTrigger 计算下一个触发时刻（当天 hour 点已过则顺延到明天）。
func (s *Scheduler) nextTrigger(now time.Time) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), s.hour, 0, 0, 0, now.Location())
	if !t.After(now) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

// runOnce 执行一次「备份 + 清理」。任何错误只记日志，绝不 panic 或退出进程。
func (s *Scheduler) runOnce() {
	if _, err := s.engine.Snapshot("auto"); err != nil {
		s.logf("backup: 自动备份失败: %v", err)
		return
	}
	if _, err := s.engine.ApplyRetention(s.keepDaily, s.keepMonthly); err != nil {
		if err == ErrBusy {
			// 极端时序下与备份撞车，下次再清理即可
			return
		}
		s.logf("backup: 清理旧备份失败: %v", err)
	}
}
