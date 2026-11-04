package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"familyledger/internal/domain"
	"familyledger/internal/ratelimit"
)

// SessionView 是「登录设备」列表项。Token 绝不下发——前端拿到的是 SID
// （token 的 sha256 前 16 位十六进制），只能用于吊销，不能用来冒充登录。
type SessionView struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	ExpiresAt  string `json:"expires_at"`
	IP         string `json:"ip"`
	Device     string `json:"device"`
	Current    bool   `json:"current"`
}

// sessionID 由 token 派生对外的稳定短标识。
func sessionID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}

// ListSessions 列出当前用户的登录设备（按最近活跃倒序）。
func (s *Service) ListSessions(userID int64, currentToken string) ([]SessionView, error) {
	sessions, err := s.repo.ListSessionsByUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]SessionView, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, SessionView{
			ID:         sessionID(sess.Token),
			CreatedAt:  sess.CreatedAt,
			LastSeenAt: sess.LastSeenAt,
			ExpiresAt:  sess.ExpiresAt,
			IP:         describeIP(sess.IP),
			Device:     describeUA(sess.UserAgent),
			Current:    currentToken != "" && sess.Token == currentToken,
		})
	}
	return out, nil
}

// RevokeSession 吊销某个设备（按 SID）。吊销当前设备等价于退出登录。
func (s *Service) RevokeSession(userID int64, sid, currentToken string) error {
	if sid == "" {
		return domain.Invalidf("缺少会话标识")
	}
	sessions, err := s.repo.ListSessionsByUser(userID)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if sessionID(sess.Token) == sid {
			return s.repo.DeleteSession(sess.Token)
		}
	}
	return domain.ErrNotFound
}

// RevokeOtherSessions 退出其他设备，返回吊销条数（保留当前会话）。
func (s *Service) RevokeOtherSessions(userID int64, currentToken string) (int, error) {
	if currentToken == "" {
		return 0, domain.ErrUnauthorized
	}
	return s.repo.DeleteOtherSessions(userID, currentToken)
}

// AttachSessionMeta 补写会话的设备信息（登录/注册/refresh 后由 handler 调用）。
// 尽力而为：失败不影响登录本身。
func (s *Service) AttachSessionMeta(token, ip, userAgent string) {
	if token == "" {
		return
	}
	_ = s.repo.AttachSessionMeta(token, ip, truncate(userAgent, 256))
}

// TouchSession 刷新最近活跃时间；同一 token 10 分钟内只写一次库，
// 避免每个 API 请求都产生一次 UPDATE（SQLite 单写者，写多了会互相排队）。
func (s *Service) TouchSession(token string) {
	if token == "" {
		return
	}
	s.touchMu.Lock()
	if last, ok := s.lastTouch[token]; ok && time.Since(last) < 10*time.Minute {
		s.touchMu.Unlock()
		return
	}
	s.lastTouch[token] = time.Now()
	if len(s.lastTouch) > 4096 { // 惰性清理，家庭规模不会触发
		cut := time.Now().Add(-time.Hour)
		for k, v := range s.lastTouch {
			if v.Before(cut) {
				delete(s.lastTouch, k)
			}
		}
	}
	s.touchMu.Unlock()
	_ = s.repo.TouchSession(token)
}

// LoginFrom 是带失败限流的登录入口（handler 调用）。
//
// 与 Login 的分工：Login 只做凭证校验（测试与其他内部路径复用），
// LoginFrom 负责「按用户名 + IP 计数、超限拒绝、成功清零」的策略。
func (s *Service) LoginFrom(username, password, ip string) (AuthResult, error) {
	userKey := "u:" + strings.ToLower(strings.TrimSpace(username))
	ipKey := "ip:" + strings.TrimSpace(ip)

	// 用户名维度：直接锁该账号
	if s.loginLimiter.Enabled() {
		if ok, wait := s.loginLimiter.Allow(userKey); !ok {
			return AuthResult{}, &domain.RateLimitError{RetryAfter: wait}
		}
	}
	// IP 维度：阈值放宽，只挡「一个来源疯狂撞库」
	if s.loginIPLimiter.Enabled() && ipKey != "ip:" {
		if ok, wait := s.loginIPLimiter.Allow(ipKey); !ok {
			return AuthResult{}, &domain.RateLimitError{RetryAfter: wait}
		}
	}

	res, err := s.Login(username, password)
	if err != nil {
		// 只统计凭证失败；数据库错误不占用用户的重试额度。
		if err == domain.ErrUnauthorized {
			s.loginLimiter.Fail(userKey)
			if ipKey != "ip:" {
				s.loginIPLimiter.Fail(ipKey)
			}
		}
		return AuthResult{}, err
	}
	s.loginLimiter.Reset(userKey)
	if ipKey != "ip:" {
		s.loginIPLimiter.Reset(ipKey)
	}
	return res, nil
}

// ConfigureLoginLimit 配置登录失败限流（组合根在装配时调用）。
//
// max 为单账号窗口内允许的失败次数（<=0 表示关闭限流）；
// IP 维度自动放宽到 4 倍：家里多台设备通常共用一个出口 IP，
// 用同一阈值会让一个人的手误把全家锁在门外。
func (s *Service) ConfigureLoginLimit(max int, window, lockout time.Duration) {
	s.loginLimiter = ratelimit.New(max, window, lockout)
	s.loginIPLimiter = ratelimit.New(max*4, window, lockout)
}

// describeIP 把回环地址换成人话；其余保留原样（家里看到的通常是内网 IP）。
func describeIP(ip string) string {
	switch ip {
	case "", "::1", "127.0.0.1":
		return "本机"
	}
	return ip
}

// ---- UA → 可读设备名 ----

// describeUA 把 User-Agent 压成「Chrome · Windows」这类可读标签。
// 不引第三方库：只需要给自家人看懂，不要精确到版本号。
func describeUA(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return "未知设备"
	}
	browser := "浏览器"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/"), strings.Contains(ua, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	osName := "未知系统"
	switch {
	case strings.Contains(ua, "Windows"):
		osName = "Windows"
	case strings.Contains(ua, "Android"):
		osName = "Android"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iOS"):
		osName = "iOS"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		osName = "macOS"
	case strings.Contains(ua, "Linux"):
		osName = "Linux"
	}
	return browser + " · " + osName
}

// truncate 截断超长 UA，避免被恶意超长头撑爆列内容。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
