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

// ListAccounts 列出账本内账户。includeClosed=false（默认）仅返回未关闭账户
// （close_date IS NULL），按 sort_order、open_date、name 排序；true 则含已关闭账户。
// 0.4 起：账户「删除」= close，不再用 deleted_at 过滤（兼容旧库软删数据）。
// 0.4-C 起：排序加入 sort_order（用户拖拽自定义），同类型内升序。
func (s *Store) ListAccounts(ledgerID int64, includeClosed bool) ([]domain.Account, error) {
	where := `deleted_at IS NULL AND close_date IS NULL`
	if includeClosed {
		where = `deleted_at IS NULL`
	}
	rows, err := s.db.Query(
		`SELECT id, uuid, name, display_name, type, open_date, close_date, commodity_restriction,
		        icon, color, parent_uuid, sub_type, sort_order
		 FROM accounts WHERE ledger_id=? AND `+where+` ORDER BY type, sort_order, open_date, name`,
		ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := []domain.Account{}
	for rows.Next() {
		var a domain.Account
		var closeDate, restr, disp, icon, color, parent, sub sql.NullString
		if err := rows.Scan(&a.ID, &a.UUID, &a.Name, &disp, &a.Type, &a.OpenDate, &closeDate, &restr,
			&icon, &color, &parent, &sub, &a.SortOrder); err != nil {
			return nil, err
		}
		if closeDate.Valid {
			a.CloseDate = closeDate.String
		}
		if disp.Valid {
			a.DisplayName = disp.String
		}
		if restr.Valid {
			a.Restriction = restr.String
		}
		if icon.Valid {
			a.Icon = icon.String
		}
		if color.Valid {
			a.Color = color.String
		}
		if parent.Valid {
			a.ParentUUID = parent.String
		}
		if sub.Valid {
			a.SubType = sub.String
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// AccountByUUID 按 uuid 查询未删除账户。
func (s *Store) AccountByUUID(ledgerID int64, uuid string) (domain.Account, error) {
	var a domain.Account
	var closeDate, restr, disp, icon, color, parent, sub sql.NullString
	err := s.db.QueryRow(
		`SELECT id, uuid, name, display_name, type, open_date, close_date, commodity_restriction,
		        icon, color, parent_uuid, sub_type, sort_order
		 FROM accounts WHERE ledger_id=? AND uuid=? AND deleted_at IS NULL`,
		ledgerID, uuid,
	).Scan(&a.ID, &a.UUID, &a.Name, &disp, &a.Type, &a.OpenDate, &closeDate, &restr,
		&icon, &color, &parent, &sub, &a.SortOrder)
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
	if icon.Valid {
		a.Icon = icon.String
	}
	if color.Valid {
		a.Color = color.String
	}
	if parent.Valid {
		a.ParentUUID = parent.String
	}
	if sub.Valid {
		a.SubType = sub.String
	}
	return a, nil
}

// UpdateAccountSortOrder 更新账户自定义排序序号（0.4-C 分类拖拽排序）。
// op=update 写入 sync_log，使其他端经 pull 收到更新事件并重排。
// 账户不存在或无变动时不写 sync_log（幂等）。
func (s *Store) UpdateAccountSortOrder(ledgerID int64, uuid string, order int64) error {
	res, err := s.db.Exec(
		`UPDATE accounts SET sort_order=?, updated_at=datetime('now')
		 WHERE ledger_id=? AND uuid=? AND deleted_at IS NULL`,
		order, ledgerID, uuid,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		if _, err := appendSyncLogOn(s.db, ledgerID, domain.EntityAccount, uuid, domain.OpUpdate); err != nil {
			return err
		}
	}
	return nil
}

// AccountIDByUUID 按 uuid 查询未关闭账户的 id（用于记账引用，已关闭账户不可记账）。
func (s *Store) AccountIDByUUID(ledgerID int64, uuid string) (int64, error) {
	var id int64
	err := s.db.QueryRow(
		`SELECT id FROM accounts WHERE ledger_id=? AND uuid=? AND close_date IS NULL AND deleted_at IS NULL`,
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

// AccountNameExists 返回账本内同名账户（含已关闭）的 uuid；不存在返回空串。
// 用于建账户冲突检查：close 后同名仍占用，重建返回 existing → ErrConflict
// （Beancount 不允许 close 后再 open 同名）。
// 自然键合并需另行判断 existing 是否已关闭，避免合并到关闭账户。
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
		`INSERT INTO accounts(ledger_id, uuid, name, display_name, type, open_date, commodity_restriction,
		        icon, color, parent_uuid, sub_type)
		 VALUES(?, ?, ?, NULLIF(?,'') , ?, ?, NULLIF(?,'') , NULLIF(?,'') , NULLIF(?,'') , NULLIF(?,'') , NULLIF(?,'') )`,
		ledgerID, a.UUID, a.Name, a.DisplayName, a.Type, a.OpenDate, a.Restriction,
		a.Icon, a.Color, a.ParentUUID, a.SubType,
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
			Icon:        d.Icon,
			Color:       d.Color,
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
