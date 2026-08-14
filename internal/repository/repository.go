// Package repository 是数据访问层：所有 SQL 集中于此，向上层暴露
// 面向领域的查询与「聚合级」原子写入方法，service 层不再直接拼 SQL。
//
// 后期平滑切换 Postgres 时，只需替换本包内部的 SQL 方言，service/handler 无感。
package repository

import (
	"database/sql"
)

// Store 持有数据库连接，是数据访问的统一入口。
type Store struct {
	db *sql.DB
}

// New 构造 Store。
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB 返回底层数据库连接，仅供导出器等需要直接读全量数据的低层组件使用。
func (s *Store) DB() *sql.DB { return s.db }

// execer 是 *sql.DB 与 *sql.Tx 共有的方法接口，
// 便于在事务内外复用 ensureCommodityOn / appendSyncLogOn 等辅助函数。
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// appendSyncLogOn 向 sync_log 追加一条变更事件（id 即全局单调水位），
// 返回新写入的 sync_log id（新水位）。可运行在 *sql.DB 或 *sql.Tx 上。
func appendSyncLogOn(e execer, ledgerID int64, entityType, entityUUID, op string) (int64, error) {
	res, err := e.Exec(
		`INSERT INTO sync_log(ledger_id, entity_type, entity_uuid, op)
		 VALUES(?, ?, ?, ?)`,
		ledgerID, entityType, entityUUID, op,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
