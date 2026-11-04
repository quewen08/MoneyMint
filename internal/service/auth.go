package service

import (
	"familyledger/internal/auth"
	"familyledger/internal/domain"
	"familyledger/internal/repository"
)

// UserInfo 是返回给客户端的用户信息（含在默认账本中的角色）。
type UserInfo struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role"`
	// IsAdmin 是系统管理员标记（全局角色）。前端据此决定是否展示备份入口。
	IsAdmin bool `json:"is_admin"`
	// MustChangePassword 为 true 时前端必须先引导改密，否则业务接口一律 428。
	MustChangePassword bool `json:"must_change_password"`
}

// AuthResult 是注册/登录成功的返回。
type AuthResult struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// Register 注册新用户。
// 首个注册用户创建默认账本并成为 owner；后续用户仅建账号（等待 owner 邀请）。
// 注册开关（B-4）关闭时拒绝；全新库 + 预设 owner 场景下开关不影响（owner 走启动预设）。
func (s *Service) Register(username, password, displayName string) (AuthResult, error) {
	allowed, err := s.RegistrationAllowed()
	if err != nil {
		return AuthResult{}, err
	}
	if !allowed {
		return AuthResult{}, domain.ErrForbidden
	}
	if username == "" || len(password) < 6 {
		return AuthResult{}, domain.Invalidf("用户名必填，密码至少 6 位")
	}
	if displayName == "" {
		displayName = username
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}
	uid, isAdmin, err := s.repo.CreateUser(username, hash, displayName)
	if err != nil {
		return AuthResult{}, err
	}

	role := ""
	ledgerCnt, err := s.repo.LedgerCount()
	if err != nil {
		return AuthResult{}, err
	}
	if ledgerCnt == 0 {
		ledgerID, err := s.repo.CreateLedgerForOwner("家庭账本", uid)
		if err != nil {
			return AuthResult{}, err
		}
		// 首个账本自动写入默认账户。
		if err := s.repo.SeedDefaultAccounts(ledgerID); err != nil {
			return AuthResult{}, err
		}
		role = domain.RoleOwner
	}

	token, err := s.repo.CreateSession(uid)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		Token: token,
		User:  UserInfo{ID: uid, Username: username, DisplayName: displayName, Role: role, IsAdmin: isAdmin},
	}, nil
}

// Login 校验用户名密码，成功签发会话 token。
// 多账本后不再返回「默认账本角色」：角色由 GET /api/ledgers 按账本返回。
func (s *Service) Login(username, password string) (AuthResult, error) {
	u, err := s.repo.UserByUsername(username)
	if err != nil {
		return AuthResult{}, domain.ErrUnauthorized
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		return AuthResult{}, domain.ErrUnauthorized
	}
	token, err := s.repo.CreateSession(u.ID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		Token: token,
		User: UserInfo{
			ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: "",
			IsAdmin: u.IsAdmin, MustChangePassword: u.MustChangePassword,
		},
	}, nil
}

// Logout 注销当前会话。
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(token)
}

// Me 返回当前登录用户信息。
// 多账本后 role 不再绑定默认账本，恒为空串；账本角色由 GET /api/ledgers 返回。
func (s *Service) Me(userID int64) (UserInfo, error) {
	u, err := s.repo.UserByID(userID)
	if err != nil {
		return UserInfo{}, err
	}
	return UserInfo{
		ID: u.ID, Username: u.Username, DisplayName: u.DisplayName,
		Role: "", IsAdmin: u.IsAdmin, MustChangePassword: u.MustChangePassword,
	}, nil
}

// IsAdmin 判断用户是否为系统管理员（全库级操作的前置校验）。
func (s *Service) IsAdmin(userID int64) (bool, error) {
	u, err := s.repo.UserByID(userID)
	if err != nil {
		return false, err
	}
	return u.IsAdmin, nil
}

// RegistrationAllowed 读注册开关（B-4）。
// app_settings 有关键时以持久化值为准；缺失（全新库未初始化）默认允许。
func (s *Service) RegistrationAllowed() (bool, error) {
	v, ok, err := s.repo.GetSetting(repository.SettingRegistration)
	if err != nil {
		return false, err
	}
	if ok {
		return v == "1", nil
	}
	return true, nil
}

// SetRegistrationAllowed 设置注册开关，仅系统管理员可操作（B-4）。
func (s *Service) SetRegistrationAllowed(userID int64, enabled bool) error {
	admin, err := s.IsAdmin(userID)
	if err != nil {
		return err
	}
	if !admin {
		return domain.ErrForbidden
	}
	v := "0"
	if enabled {
		v = "1"
	}
	return s.repo.SetSetting(repository.SettingRegistration, v)
}

// Refresh 用当前有效 token 换取新 token（再续 30 天），旧 token 立即失效。
func (s *Service) Refresh(token string) (string, error) {
	uid, err := s.repo.UserIDByToken(token)
	if err != nil {
		return "", domain.ErrUnauthorized
	}
	if err := s.repo.DeleteSession(token); err != nil {
		return "", err
	}
	return s.repo.CreateSession(uid)
}

// ChangePassword 凭旧密码修改密码；成功后该用户全部会话失效并签发新 token。
func (s *Service) ChangePassword(userID int64, oldPassword, newPassword string) (string, error) {
	if len(newPassword) < 6 {
		return "", domain.Invalidf("新密码至少 6 位")
	}
	u, err := s.repo.UserByID(userID)
	if err != nil {
		return "", err
	}
	if !auth.CheckPassword(u.PasswordHash, oldPassword) {
		return "", domain.Invalidf("旧密码错误")
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return "", err
	}
	if _, err := s.repo.UpdateUserPassword(userID, hash); err != nil {
		return "", err
	}
	// 用户自己改过密码，强制改密标记随之解除（M3）。
	if _, err := s.repo.SetMustChangePassword(userID, false); err != nil {
		return "", err
	}
	if err := s.repo.DeleteSessionsByUser(userID); err != nil {
		return "", err
	}
	return s.repo.CreateSession(userID)
}

// SessionUser 是 token 解析出的会话主体（供鉴权中间件）。
type SessionUser struct {
	UserID int64
	// MustChangePassword 为 true 时，中间件只放行改密/登出/me/refresh。
	MustChangePassword bool
}

// Authenticate 解析 token 对应的用户与会话状态（供鉴权中间件）。
//
// 比只返回 uid 多一次 users 查询：强制改密是「每个请求都要判断」的策略，
// 只能从库里取；会话表本身不缓存该标记（管理员重置密码后要立刻生效）。
func (s *Service) Authenticate(token string) (SessionUser, error) {
	uid, err := s.repo.UserIDByToken(token)
	if err != nil {
		return SessionUser{}, err
	}
	u, err := s.repo.UserByID(uid)
	if err != nil {
		return SessionUser{}, err
	}
	return SessionUser{UserID: uid, MustChangePassword: u.MustChangePassword}, nil
}

// MarkPasswordChangeRequired 标记某用户必须先改密（预设 owner / CLI 重置等
// 「密码由系统代设」的入口调用）。用户不存在返回 ErrNotFound。
func (s *Service) MarkPasswordChangeRequired(username string) error {
	u, err := s.repo.UserByUsername(username)
	if err != nil {
		return err
	}
	if _, err := s.repo.SetMustChangePassword(u.ID, true); err != nil {
		return err
	}
	return nil
}

// AuthorizeLedger 校验用户是否为账本成员（供鉴权中间件）。
func (s *Service) AuthorizeLedger(uid, ledgerID int64) error {
	ok, err := s.repo.IsMember(ledgerID, uid)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// LedgerRole 返回用户在账本中的角色；非成员返回空串（供鉴权中间件）。
func (s *Service) LedgerRole(uid, ledgerID int64) (string, error) {
	return s.repo.RoleOf(ledgerID, uid)
}
