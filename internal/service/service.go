// Package service 承载业务逻辑：参数/业务规则校验、事务编排、跨实体的聚合操作。
//
// 它只依赖 repository（数据访问）与 auth（纯函数），不感知 HTTP。
// handler 层负责解析请求、调用本层、写出响应。
package service

import (
	"sync"
	"time"

	"familyledger/internal/ratelimit"
	"familyledger/internal/repository"
)

// Service 是业务逻辑的统一入口。
type Service struct {
	repo *repository.Store

	// loginLimiter 按「用户名」计数：5 次失败锁该账号。
	// loginIPLimiter 按「来源 IP」计数，阈值放宽（见 ConfigureLoginLimit）：
	// 家庭几乎总是共用同一个出口 IP（甚至就是 NAS 内网），用同样阈值会让
	// 一个人的手误把全家锁在门外。
	loginLimiter   *ratelimit.Limiter
	loginIPLimiter *ratelimit.Limiter

	// lastTouch 记录各会话上次写 last_seen_at 的时间，用于节流（10 分钟一次）。
	touchMu   sync.Mutex
	lastTouch map[string]time.Time
}

// New 构造 Service。
func New(repo *repository.Store) *Service {
	return &Service{
		repo:      repo,
		lastTouch: make(map[string]time.Time),
		// 默认关闭限流：只有组合根显式注入策略后才生效，
		// 这样单元测试与其他内部调用不会被限流干扰。
		loginLimiter:   ratelimit.New(0, 0, 0),
		loginIPLimiter: ratelimit.New(0, 0, 0),
	}
}
