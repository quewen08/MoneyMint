package repository

import (
	"familyledger/internal/domain"
)

// SyncLogSince 返回 id>since 的全部同步事件，按 id 升序。
func (s *Store) SyncLogSince(ledgerID int64, since int64) ([]domain.SyncLogEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, entity_type, entity_uuid, op FROM sync_log
		 WHERE ledger_id=? AND id>? ORDER BY id`,
		ledgerID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []domain.SyncLogEntry{}
	for rows.Next() {
		var e domain.SyncLogEntry
		if err := rows.Scan(&e.Seq, &e.EntityType, &e.EntityUUID, &e.Op); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// MaxSyncSeq 返回账本当前最大 sync_log.id（即水位）。
func (s *Store) MaxSyncSeq(ledgerID int64) (int64, error) {
	var maxSeq int64
	if err := s.db.QueryRow(
		`SELECT coalesce(max(id),0) FROM sync_log WHERE ledger_id=?`, ledgerID,
	).Scan(&maxSeq); err != nil {
		return 0, err
	}
	return maxSeq, nil
}

// UpsertCheckpoint 记录客户端同步水位。
func (s *Store) UpsertCheckpoint(ledgerID int64, clientID string, seq int64) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_checkpoints(ledger_id, client_id, last_seq)
		 VALUES(?, ?, ?)
		 ON CONFLICT(ledger_id, client_id) DO UPDATE
		 SET last_seq=excluded.last_seq, last_synced_at=datetime('now')`,
		ledgerID, clientID, seq,
	)
	return err
}
