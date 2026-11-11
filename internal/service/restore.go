package service

import (
	"familyledger/internal/backup"
	"familyledger/internal/domain"
	"familyledger/internal/restore"
)

// PreviewRestore 预检上传的备份文件（只读，不改动数据）。仅系统管理员可用。
//
// 全库级操作：备份含全部账本与用户，故校验系统管理员而非账本角色。
func (s *Service) PreviewRestore(userID int64, backupPath string) (*restore.PreviewResult, error) {
	admin, err := s.IsAdmin(userID)
	if err != nil {
		return nil, err
	}
	if !admin {
		return nil, domain.ErrForbidden
	}
	return restore.Preview(s.repo.DB(), backupPath)
}

// ExecuteRestore 执行还原：预检 → pre-restore 快照 → 逐表替换 → 重跑迁移 → 重写库世代。
//
// confirm 必须等于 restore.ConfirmWord（"RESTORE"），否则返回 ErrInvalid，
// 防止误操作把整库覆盖掉。engine 用于生成安全网快照，可为 nil（跳过快照）。
func (s *Service) ExecuteRestore(userID int64, backupPath, confirm string, engine *backup.Engine) (*restore.Result, error) {
	admin, err := s.IsAdmin(userID)
	if err != nil {
		return nil, err
	}
	if !admin {
		return nil, domain.ErrForbidden
	}
	if confirm != restore.ConfirmWord {
		return nil, domain.Invalidf("需要输入确认词 %s 才能执行还原", restore.ConfirmWord)
	}
	return restore.Execute(s.repo.DB(), backupPath, engine)
}
