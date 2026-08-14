package service

import (
	"familyledger/internal/auth"
	"familyledger/internal/domain"
)

// MemberView 是成员列表项的返回视图。
type MemberView struct {
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// requireOwner 校验用户是否为指定账本 owner。
func (s *Service) requireOwner(ledgerID, userID int64) error {
	role, err := s.repo.RoleOf(ledgerID, userID)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner {
		return domain.ErrForbidden
	}
	return nil
}

// ListMembers 列出账本成员（仅 owner）。
func (s *Service) ListMembers(ledgerID, userID int64) ([]MemberView, error) {
	if err := s.requireOwner(ledgerID, userID); err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(ledgerID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberView, 0, len(members))
	for _, m := range members {
		out = append(out, MemberView{
			UserID:      m.UserID,
			Username:    m.Username,
			DisplayName: m.DisplayName,
			Role:        m.Role,
		})
	}
	return out, nil
}

// AddMember owner 按用户名把已注册用户加入账本（角色 editor/viewer）。
// 返回成员 id 与最终生效的角色（role 为空时默认 viewer）。
func (s *Service) AddMember(ledgerID, userID int64, username, role string) (int64, string, error) {
	if err := s.requireOwner(ledgerID, userID); err != nil {
		return 0, "", err
	}
	if username == "" {
		return 0, "", domain.Invalidf("username 必填")
	}
	if role == "" {
		role = domain.RoleViewer
	}
	if role != domain.RoleEditor && role != domain.RoleViewer {
		return 0, "", domain.Invalidf("role 仅支持 editor/viewer（owner 由账本创建者承担）")
	}
	member, err := s.repo.UserByUsername(username)
	if err != nil {
		return 0, "", domain.ErrNotFound
	}
	if err := s.repo.AddMember(ledgerID, member.ID, role, userID); err != nil {
		return 0, "", err
	}
	return member.ID, role, nil
}

// UpdateMemberRole owner 调整成员角色。
func (s *Service) UpdateMemberRole(ledgerID, userID, memberID int64, role string) error {
	if err := s.requireOwner(ledgerID, userID); err != nil {
		return err
	}
	if memberID == userID {
		return domain.Invalidf("不能修改自己的角色（保持至少一个 owner）")
	}
	if role != domain.RoleEditor && role != domain.RoleViewer {
		return domain.Invalidf("role 仅支持 editor/viewer")
	}
	ok, err := s.repo.UpdateMemberRole(ledgerID, memberID, role)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	return nil
}

// RemoveMember owner 移除成员（禁止移除自己或 owner）。
func (s *Service) RemoveMember(ledgerID, userID, memberID int64) error {
	if err := s.requireOwner(ledgerID, userID); err != nil {
		return err
	}
	if memberID == userID {
		return domain.Invalidf("不能移除自己")
	}
	role, err := s.repo.RoleOf(ledgerID, memberID)
	if err != nil {
		return err
	}
	if role == "" {
		return domain.ErrNotFound
	}
	if role == domain.RoleOwner {
		return domain.Invalidf("不能移除账本 owner")
	}
	if _, err := s.repo.RemoveMember(ledgerID, memberID); err != nil {
		return err
	}
	return nil
}

// ResetMemberPassword owner 重置成员密码，并使其所有会话失效。
func (s *Service) ResetMemberPassword(ledgerID, userID, memberID int64, newPassword string) error {
	if err := s.requireOwner(ledgerID, userID); err != nil {
		return err
	}
	if len(newPassword) < 6 {
		return domain.Invalidf("新密码至少 6 位")
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	ok, err := s.repo.UpdateUserPassword(memberID, hash)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	return s.repo.DeleteSessionsByUser(memberID)
}
