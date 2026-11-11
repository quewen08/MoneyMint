package service

import "familyledger/internal/db"

// GetInstanceID 返回当前数据库世代 UUID（库被 restore 后会变化）。
// 详见 internal/db 包说明：客户端据此判断是否需清空本地实体并全量重新 pull。
func (s *Service) GetInstanceID() (string, error) {
	return db.GetInstanceID(s.repo.DB())
}
