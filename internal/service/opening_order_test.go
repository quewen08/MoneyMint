package service

// 回归测试：期初解析不应依赖 postings.position 的顺序。
//
// 迁移 012 的启发式识别对分录顺序不敏感（目标账户与权益账户任一在前都能识别），
// 但 activeOpeningTx 直接取 position 最小的第一条分录作为目标账户金额。
// 因此「权益在前、目标账户在后」的老期初会被读成相反符号：
//   - GET 期初把资产 +100 显示成欠款 -100；
//   - 以相同金额再次保存会被误判为变更并产生一次多余冲正。

import (
	"database/sql"
	"path/filepath"
	"testing"

	"familyledger/internal/db"
	"familyledger/internal/domain"
	"familyledger/internal/repository"
)

func newOpeningOrderFixture(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "order.db"), filepath.Join("..", "..", "schema.sql"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	svc := New(repository.New(conn))
	if _, err := svc.Register("alice", "secret123", "Alice"); err != nil {
		t.Fatalf("register owner: %v", err)
	}
	return svc, conn
}

func TestGetOpeningIsIndependentOfPostingOrder(t *testing.T) {
	svc, conn := newOpeningOrderFixture(t)

	acc, err := svc.CreateAccount(lid, "Assets:LegacyOrder", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := svc.SetOpening(lid, acc.UUID, OpeningInput{Amount: "100", Direction: domain.OpeningDirectionHold}); err != nil {
		t.Fatalf("set opening: %v", err)
	}

	// 模拟旧数据：把目标账户的分录挪到 position=1，权益分录挪到 position=0。
	var txnID int64
	if err := conn.QueryRow(
		`SELECT id FROM transactions
		 WHERE ledger_id=? AND opening_account_uuid=?
		   AND reversal_of IS NULL AND reversed_by IS NULL AND deleted_at IS NULL
		 ORDER BY id DESC LIMIT 1`, lid, acc.UUID,
	).Scan(&txnID); err != nil {
		t.Fatalf("定位有效期初交易: %v", err)
	}
	if _, err := conn.Exec(
		`UPDATE postings SET position = CASE WHEN account_id=? THEN 1 ELSE 0 END WHERE transaction_id=?`,
		acc.ID, txnID,
	); err != nil {
		t.Fatalf("调整分录顺序: %v", err)
	}

	view, err := svc.GetOpening(lid, acc.UUID)
	if err != nil {
		t.Fatalf("get opening: %v", err)
	}
	if !view.HasOpening || view.Direction != domain.OpeningDirectionHold || view.SignedAmount != "100.00" {
		t.Fatalf("BUG: 期初解析依赖分录顺序：期望 hold/+100.00，实际 direction=%q signed=%q amount=%q",
			view.Direction, view.SignedAmount, view.Amount)
	}
}

func TestSetOpeningIsIdempotentWhenTargetPostingNotFirst(t *testing.T) {
	svc, conn := newOpeningOrderFixture(t)

	acc, err := svc.CreateAccount(lid, "Assets:LegacyOrder2", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := svc.SetOpening(lid, acc.UUID, OpeningInput{Amount: "100", Direction: domain.OpeningDirectionHold}); err != nil {
		t.Fatalf("set opening: %v", err)
	}

	var txnID int64
	if err := conn.QueryRow(
		`SELECT id FROM transactions
		 WHERE ledger_id=? AND opening_account_uuid=?
		   AND reversal_of IS NULL AND reversed_by IS NULL AND deleted_at IS NULL
		 ORDER BY id DESC LIMIT 1`, lid, acc.UUID,
	).Scan(&txnID); err != nil {
		t.Fatalf("定位有效期初交易: %v", err)
	}
	if _, err := conn.Exec(
		`UPDATE postings SET position = CASE WHEN account_id=? THEN 1 ELSE 0 END WHERE transaction_id=?`,
		acc.ID, txnID,
	); err != nil {
		t.Fatalf("调整分录顺序: %v", err)
	}

	res, err := svc.SetOpening(lid, acc.UUID, OpeningInput{Amount: "100", Direction: domain.OpeningDirectionHold})
	if err != nil {
		t.Fatalf("再次保存期初: %v", err)
	}
	if res.Status != "unchanged" {
		t.Fatalf("BUG: 相同金额应幂等返回 unchanged，实际 %q（多出 reversal=%q）", res.Status, res.ReversalUUID)
	}
}
