package service

import (
	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// UserInfo 是返回给客户端的用户信息（含在默认账本中的角色）。
type UserInfo struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role"`
}

// AuthResult 是注册/登录成功的返回。
type AuthResult struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// Register 注册新用户。
// 首个注册用户创建默认账本并成为 owner；后续用户仅建账号（等待 owner 邀请）。
func (s *Service) Register(username, password, displayName string) (AuthResult, error) {
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
	uid, err := s.repo.CreateUser(username, hash, displayName)
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
		User:  UserInfo{ID: uid, Username: username, DisplayName: displayName, Role: role},
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
		User:  UserInfo{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: ""},
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
	return UserInfo{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: ""}, nil
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
	if err := s.repo.DeleteSessionsByUser(userID); err != nil {
		return "", err
	}
	return s.repo.CreateSession(userID)
}

// Authenticate 解析 token 对应的用户 id（供鉴权中间件）。
func (s *Service) Authenticate(token string) (int64, error) {
	return s.repo.UserIDByToken(token)
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
