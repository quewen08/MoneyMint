package service

import (
	"familyledger/internal/export"
)

// ExportLedger 将指定账本导出为合法 Beancount 单文件文本。
func (s *Service) ExportLedger(ledgerID int64) (string, error) {
	return export.ExportLedger(s.repo.DB(), ledgerID)
}

// ExportLedgerFiles 将指定账本导出为多文件目录（map[相对路径]内容），
// 主文件 main.bean 用 include 组织 accounts/ 与 date/ 下的子文件。
func (s *Service) ExportLedgerFiles(ledgerID int64) (map[string][]byte, error) {
	return export.ExportLedgerFiles(s.repo.DB(), ledgerID)
}
