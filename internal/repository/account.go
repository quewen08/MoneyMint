package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"familyledger/internal/domain"
	"familyledger/internal/seed"

	"github.com/google/uuid"
)

// ListAccounts 列出账本内所有未删除账户，按 open_date、name 排序。
func (s *Store) ListAccounts(ledgerID int64) ([]domain.Account, error) {
	rows, err := s.db.Query(
		`SELECT id, uuid, name, display_name, type, open_date, commodity_restriction
		 FROM accounts WHERE ledger_id=? AND deleted_at IS NULL ORDER BY open_date, name`,
		ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := []domain.Account{}
	for rows.Next() {
		var a domain.Account
		var restr, disp sql.NullString
		if err := rows.Scan(&a.ID, &a.UUID, &a.Name, &disp, &a.Type, &a.OpenDate, &restr); err != nil {
			return nil, err
		}
		if disp.Valid {
			a.DisplayName = disp.String
		}
		if restr.Valid {
			a.Restriction = restr.String
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// AccountByUUID 按 uuid 查询未删除账户。
func (s *Store) AccountByUUID(ledgerID int64, uuid string) (domain.Account, error) {
	var a domain.Account
	var closeDate, restr, disp sql.NullString
	err := s.db.QueryRow(
		`SELECT id, uuid, name, display_name, type, open_date, close_date, commodity_restriction
		 FROM accounts WHERE ledger_id=? AND uuid=? AND deleted_at IS NULL`,
		ledgerID, uuid,
	).Scan(&a.ID, &a.UUID, &a.Name, &disp, &a.Type, &a.OpenDate, &closeDate, &restr)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Account{}, err
	}
	if disp.Valid {
		a.DisplayName = disp.String
	}
	if closeDate.Valid {
		a.CloseDate = closeDate.String
	}
	if restr.Valid {
		a.Restriction = restr.String
	}
	return a, nil
}

// AccountIDByUUID 按 uuid 查询未删除账户的 id。
func (s *Store) AccountIDByUUID(ledgerID int64, uuid string) (int64, error) {
	var id int64
	err := s.db.QueryRow(
		`SELECT id FROM accounts WHERE ledger_id=? AND uuid=? AND deleted_at IS NULL`,
		ledgerID, uuid,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	return id, err
}

// AccountUUIDExists 判断账本内是否已存在该 uuid 的账户（含软删）。
func (s *Store) AccountUUIDExists(ledgerID int64, uuid string) (bool, error) {
	var n int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM accounts WHERE ledger_id=? AND uuid=?`, ledgerID, uuid,
	).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// AccountNameExists 返回账本内「未删除」同名账户的先到者 uuid；不存在返回空串。
func (s *Store) AccountNameExists(ledgerID int64, name string) (string, error) {
	var uuid string
	err := s.db.QueryRow(
		`SELECT uuid FROM accounts WHERE ledger_id=? AND name=? AND deleted_at IS NULL`,
		ledgerID, name,
	).Scan(&uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return uuid, err
}

// InsertAccount 创建账户并写入 sync_log（create）。
// 若带币种限制，会先 EnsureCommodity 补建该币种。
// 同名未删除账户已存在时返回 domain.ErrConflict。
func (s *Store) InsertAccount(ledgerID int64, a domain.Account) (int64, error) {
	if existing, err := s.AccountNameExists(ledgerID, a.Name); err != nil {
		return 0, err
	} else if existing != "" {
		return 0, domain.ErrConflict
	}
	if a.UUID == "" {
		a.UUID = uuid.NewString()
	}
	if a.Restriction != "" {
		if err := s.EnsureCommodity(ledgerID, a.Restriction); err != nil {
			return 0, err
		}
	}
	res, err := s.db.Exec(
		`INSERT INTO accounts(ledger_id, uuid, name, display_name, type, open_date, commodity_restriction)
		 VALUES(?, ?, ?, NULLIF(?,'') , ?, ?, NULLIF(?,'') )`,
		ledgerID, a.UUID, a.Name, a.DisplayName, a.Type, a.OpenDate, a.Restriction,
	)
	if err != nil {
		return 0, fmt.Errorf("创建账户失败: %w", err)
	}
	id, _ := res.LastInsertId()
	if _, err := appendSyncLogOn(s.db, ledgerID, domain.EntityAccount, a.UUID, domain.OpCreate); err != nil {
		return 0, err
	}
	return id, nil
}

// SeedDefaultAccounts 建账时批量写入默认账户（支出/收入/资产/负债/权益）。
// 复用 InsertAccount（含 uuid 生成与 sync_log 写入），同名已存在则跳过，
// 因此可对每个账本重复调用而不会产生重复账户。open_date 取建账当日。
func (s *Store) SeedDefaultAccounts(ledgerID int64) error {
	openDate := time.Now().Format("2006-01-02")
	for _, d := range seed.DefaultAccounts {
		_, err := s.InsertAccount(ledgerID, domain.Account{
			Name:        d.Name,
			DisplayName: d.DisplayName,
			Type:        d.Type,
			OpenDate:    openDate,
		})
		if err != nil {
			if errors.Is(err, domain.ErrConflict) {
				continue // 已存在，幂等跳过
			}
			return err
		}
	}
	return nil
}
