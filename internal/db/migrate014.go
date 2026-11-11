package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// migration014Name 是迁移 014 在 schema_migrations 中的记录名。
const migration014Name = "014_db_instance_id"

// migrate014：库世代（db_instance_id）。
//
// 背景：客户端同步依赖 {ledger_id:{instance_id,last_seq}} 比对水位。若服务端整库被
// 替换（restore）而 instance_id 不变，客户端不会回退水位、也不会触发全量重拉，
// 静默丢失 restore 带来的数据变更。因此每个数据库需要一个稳定且唯一的世代标识，
// restore 成功后由业务层（M2 恢复核心）调用 RotateInstanceID 重写该值。
//
// 设计要点：
//   - db_instance 为单例行（id=1），保证全局唯一、避免自增漂移；
//   - 迁移仅负责建表 + 首次播种 UUID，不在迁移内随机旋转——否则每次启动迁移都会
//     改变世代，失去「稳定标识」语义；旋转是显式业务动作（restore 成功）；
//   - 全部步骤在同一显式事务内完成，任一步失败整体回滚，schema_migrations 不记录本
//     迁移，修复后可安全重跑。
func migrate014(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("开启 014 事务: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`CREATE TABLE IF NOT EXISTS db_instance (
			id          INTEGER PRIMARY KEY CHECK (id = 1),
			instance_id TEXT NOT NULL,
			rotated_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	); err != nil {
		return fmt.Errorf("创建 db_instance: %w", err)
	}

	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM db_instance WHERE id = 1`).Scan(&n); err != nil {
		return fmt.Errorf("查询 db_instance: %w", err)
	}
	if n == 0 {
		if _, err := tx.Exec(
			`INSERT INTO db_instance(id, instance_id) VALUES (1, ?)`,
			uuid.NewString(),
		); err != nil {
			return fmt.Errorf("写入初始 instance_id: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交 014 事务: %w", err)
	}
	return nil
}
