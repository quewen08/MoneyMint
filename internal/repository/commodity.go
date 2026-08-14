package repository

import (
	"database/sql"
	"errors"

	"familyledger/internal/domain"

	"github.com/google/uuid"
)

// CommodityByUUID 按 uuid 查询币种。
func (s *Store) CommodityByUUID(ledgerID int64, uuid string) (domain.Commodity, error) {
	var c domain.Commodity
	var name sql.NullString
	err := s.db.QueryRow(
		`SELECT uuid, symbol, name, "precision" FROM commodities WHERE ledger_id=? AND uuid=?`,
		ledgerID, uuid,
	).Scan(&c.UUID, &c.Symbol, &name, &c.Precision)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Commodity{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Commodity{}, err
	}
	c.Name = name.String
	return c, nil
}

// CommodityUUIDExists 判断账本内是否已存在该 uuid 的币种。
func (s *Store) CommodityUUIDExists(ledgerID int64, uuid string) (bool, error) {
	var n int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM commodities WHERE ledger_id=? AND uuid=?`, ledgerID, uuid,
	).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// CommoditySymbolExists 返回同 symbol 币种的先到者 uuid；不存在返回空串。
func (s *Store) CommoditySymbolExists(ledgerID int64, symbol string) (string, error) {
	var uuid string
	err := s.db.QueryRow(
		`SELECT uuid FROM commodities WHERE ledger_id=? AND symbol=?`, ledgerID, symbol,
	).Scan(&uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return uuid, err
}

// EnsureCommodity 若账本内尚无该币种则补一条默认记录（precision=2），
// 保证导出 beancount 时有 commodity 指令可写；新建时写入 sync_log。
func (s *Store) EnsureCommodity(ledgerID int64, symbol string) error {
	if symbol == "" {
		return domain.Invalidf("commodity 不能为空")
	}
	if err := ensureCommodityOn(s.db, ledgerID, symbol); err != nil {
		return err
	}
	return nil
}

// InsertCommodity 创建币种并写入 sync_log。
func (s *Store) InsertCommodity(ledgerID int64, c domain.Commodity) error {
	if c.UUID == "" {
		c.UUID = uuid.NewString()
	}
	if _, err := s.db.Exec(
		`INSERT INTO commodities(ledger_id, uuid, symbol, name, precision)
		 VALUES(?, ?, ?, ?, ?)`,
		ledgerID, c.UUID, c.Symbol, c.Name, c.Precision,
	); err != nil {
		return err
	}
	if _, err := appendSyncLogOn(s.db, ledgerID, domain.EntityCommodity, c.UUID, domain.OpCreate); err != nil {
		return err
	}
	return nil
}

// ensureCommodityOn 在给定 executor（db 或 tx）上幂等补建币种。
func ensureCommodityOn(e execer, ledgerID int64, symbol string) error {
	var cnt int
	if err := e.QueryRow(
		`SELECT count(*) FROM commodities WHERE ledger_id=? AND symbol=?`,
		ledgerID, symbol,
	).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	cuuid := uuid.NewString()
	if _, err := e.Exec(
		`INSERT INTO commodities(ledger_id, uuid, symbol, name, precision)
		 VALUES(?, ?, ?, ?, 2)`,
		ledgerID, cuuid, symbol, symbol,
	); err != nil {
		return err
	}
	if _, err := appendSyncLogOn(e, ledgerID, domain.EntityCommodity, cuuid, domain.OpCreate); err != nil {
		return err
	}
	return nil
}
