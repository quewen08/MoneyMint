package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// GetInstanceID 返回当前数据库的「库世代」UUID。
//
// 每个数据库（不论全新安装还是从备份 restore）都持有唯一且稳定的 instance_id，
// 客户端据此判断「我本地这份数据是否还对应同一个库」。一旦 instance_id 变化，
// 说明服务端整库已被替换（典型场景：restore 成功），客户端应清空本地实体并
// 全量重新 pull，否则会沿用旧水位、静默丢失 restore 带来的数据变更。
func GetInstanceID(conn *sql.DB) (string, error) {
	var id string
	if err := conn.QueryRow(`SELECT instance_id FROM db_instance WHERE id = 1`).Scan(&id); err != nil {
		return "", fmt.Errorf("读取 db_instance_id: %w", err)
	}
	return id, nil
}

// RotateInstanceID 重写库世代 UUID（restore 成功后必须调用），返回新值。
//
// 单例行约束 id=1 保证全局唯一；写入走显式主键值，不依赖自增。
// 并发安全由连接池 MaxOpenConns=1 保证（与现有迁移一致）。
func RotateInstanceID(conn *sql.DB) (string, error) {
	newID := uuid.NewString()
	if _, err := conn.Exec(
		`UPDATE db_instance SET instance_id = ?, rotated_at = datetime('now') WHERE id = 1`,
		newID,
	); err != nil {
		return "", fmt.Errorf("重写 db_instance_id: %w", err)
	}
	return newID, nil
}
