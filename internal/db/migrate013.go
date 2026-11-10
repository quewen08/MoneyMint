package db

import (
	"database/sql"
	"fmt"
)

// migration013Name 是迁移 013 在 schema_migrations 中的记录名。
const migration013Name = "013_txn_idempotency"

// migrate013：在线记账幂等键（G-04 / AC-23 / R-14）。
//
// 背景：在线 POST /api/transactions 每次都会生成新 uuid，弱网重试即产生重复交易；
// append-only 模型下无法原地改，只能冲正补救，因此这是账本类底线。
//
// 落库设计（与设计文档 §7 一致）：
//   - transactions 增加 client_request_id（客户端幂等标识）与
//     client_payload_hash（规范化请求体哈希，用于识别「同键不同内容」）；
//   - 部分唯一索引 UNIQUE(ledger_id, client_request_id) WHERE client_request_id IS NOT NULL，
//     并发重复提交由数据库兜底，应用层「先查后写」挡不住并发；
//   - 旧数据两列为 NULL，不参与唯一索引，完全不受影响。
//
// 全部步骤在同一个显式事务内完成（连接池 MaxOpenConns=1，事务外不得再用 conn），
// 任一步失败整体回滚，schema_migrations 不记录本迁移，修复后可安全重跑。
func migrate013(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("开启 013 事务: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	cols := []struct{ name, ddl string }{
		{"client_request_id", "TEXT"},
		{"client_payload_hash", "TEXT"},
	}
	for _, c := range cols {
		ok, err := columnExistsTx(tx, "transactions", c.name)
		if err != nil {
			return fmt.Errorf("检查 transactions.%s: %w", c.name, err)
		}
		if ok {
			continue
		}
		if _, err := tx.Exec(`ALTER TABLE transactions ADD COLUMN ` + c.name + ` ` + c.ddl); err != nil {
			return fmt.Errorf("新增 transactions.%s: %w", c.name, err)
		}
	}

	// 部分唯一索引：仅约束带幂等键的行；NULL 行（旧数据与未传键的旧客户端）不受影响。
	if _, err := tx.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_txn_client_request
		 ON transactions(ledger_id, client_request_id)
		 WHERE client_request_id IS NOT NULL`,
	); err != nil {
		return fmt.Errorf("创建幂等键唯一索引: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交 013 事务: %w", err)
	}
	return nil
}
