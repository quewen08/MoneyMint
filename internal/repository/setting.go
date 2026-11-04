// 全局键值设置（app_settings 表，迁移 009）。
// 存放系统级配置（注册开关等）；账本数据不在此表。
package repository

import (
	"database/sql"
	"errors"
	"fmt"
)

// SettingRegistration 注册开关的键名。值 "1"=允许，"0"=关闭。
const SettingRegistration = "registration_enabled"

// GetSetting 读设置；存在返回 (value, true, nil)。
func (s *Store) GetSetting(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting %s: %w", key, err)
	}
	return v, true, nil
}

// SetSetting 写设置（UPSERT）。
func (s *Store) SetSetting(key, value string) error {
	if _, err := s.db.Exec(
		`INSERT INTO app_settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=datetime('now')`,
		key, value); err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}

// UserCount 用户总数（判断是否全新库 / 预设 owner）。
func (s *Store) UserCount() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM users`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
