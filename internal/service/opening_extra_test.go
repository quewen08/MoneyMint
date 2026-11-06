package service

// 期初重构补充测试：并发幂等、权益账户兜底、编辑失败原子性、同步关闭与 REST 一致性。

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"familyledger/internal/domain"
)

// TestOpeningConcurrentSameValueOnlyOnce 并发提交相同期初：只允许一条有效期初，且不报错。
// SQLite 单写连接会把两笔事务串行化，后来者必须走 unchanged 分支。
func TestOpeningConcurrentSameValueOnlyOnce(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)
	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}

	const n = 6
	var wg sync.WaitGroup
	errs := make([]error, n)
	res := make([]OpeningResult, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res[i], errs[i] = svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "100", Direction: "hold"})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("第 %d 次并发请求失败: %v", i, err)
		}
	}
	var active int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM transactions
		 WHERE ledger_id=? AND opening_account_uuid=?
		   AND reversal_of IS NULL AND (reversed_by IS NULL OR reversed_by='') AND deleted_at IS NULL`,
		lid, cash.UUID,
	).Scan(&active); err != nil {
		t.Fatalf("统计有效期初: %v", err)
	}
	if active != 1 {
		t.Fatalf("并发相同期初应只保留 1 条有效记录，实际 %d", active)
	}
	var total int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM transactions WHERE ledger_id=? AND opening_account_uuid=?`, lid, cash.UUID,
	).Scan(&total); err != nil {
		t.Fatalf("统计期初交易: %v", err)
	}
	if total != 1 {
		t.Fatalf("并发相同期初不应产生冲正，实际交易数 %d", total)
	}

	chk, err := svc.CheckIdentity(lid)
	if err != nil || !chk.Balanced {
		t.Fatalf("并发期初后恒等式应平衡: %+v err=%v", chk, err)
	}
}

// TestOpeningEquityReseedWhenMissing 权益账户缺失（老账本从未种子）时，
// 设置期初应同事务补种且保持平衡（FR-08）。
func TestOpeningEquityReseedWhenMissing(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)
	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}

	// 模拟老账本缺权益账户：此时尚无任何 posting 引用，可硬删。
	if _, err := svc.repo.DB().Exec(
		`DELETE FROM accounts WHERE ledger_id=? AND name=?`, lid, domain.OpeningEquityName,
	); err != nil {
		t.Fatalf("删除权益账户: %v", err)
	}

	res, err := svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "100", Direction: "hold"})
	if err != nil {
		t.Fatalf("权益缺失时设置期初应补种成功: %v", err)
	}
	if res.SignedAmount != "100.00" {
		t.Fatalf("新期初金额不符: %+v", res)
	}
	var alive int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM accounts WHERE ledger_id=? AND name=? AND deleted_at IS NULL`,
		lid, domain.OpeningEquityName,
	).Scan(&alive); err != nil {
		t.Fatalf("统计权益账户: %v", err)
	}
	if alive != 1 {
		t.Fatalf("权益账户应补种 1 个，实际 %d", alive)
	}
	chk, err := svc.CheckIdentity(lid)
	if err != nil || !chk.Balanced {
		t.Fatalf("补种后恒等式应平衡: %+v err=%v", chk, err)
	}
}

// TestOpeningEditAtomicWhenEquityClosed 编辑期初但权益账户已关闭：整体失败，
// 旧期初保持有效且不得留下冲正/新期初等半成品。
func TestOpeningEditAtomicWhenEquityClosed(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)
	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	if _, err := svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "100", Direction: "hold"}); err != nil {
		t.Fatalf("首次期初: %v", err)
	}
	if _, err := svc.repo.DB().Exec(
		`UPDATE accounts SET close_date=date('now') WHERE ledger_id=? AND name=? AND deleted_at IS NULL`,
		lid, domain.OpeningEquityName,
	); err != nil {
		t.Fatalf("关闭权益账户: %v", err)
	}

	_, err = svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "30", Direction: "hold"})
	if code := codedCode(t, err); code != domain.CodeAccountClosed {
		t.Fatalf("权益已关闭应返回 account_closed，实际 %s", code)
	}

	// 旧期初仍是唯一有效记录，金额未变。
	view, err := svc.GetOpening(lid, cash.UUID)
	if err != nil {
		t.Fatalf("读取旧期初: %v", err)
	}
	if !view.HasOpening || view.SignedAmount != "100.00" {
		t.Fatalf("失败后旧期初应保持 100.00: %+v", view)
	}
	var reversals int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM transactions WHERE ledger_id=? AND reversal_of IS NOT NULL`, lid,
	).Scan(&reversals); err != nil {
		t.Fatalf("统计冲正: %v", err)
	}
	if reversals != 0 {
		t.Fatalf("失败事务不应留下冲正记录，实际 %d", reversals)
	}
}

// TestOpeningEquityRestoreWhenSoftDeleted 回归 BUG-04：
// 012 已执行（权益账户已存在）后该账户被人为软删，SetOpening 必须就地恢复
// 原账户而不是插入同名新账户（006 全局唯一索引会冲突，旧实现递归导致栈溢出）。
// 断言：恢复成功、复用原 id/uuid 与历史分录、无重复权益账户、余额平衡，
// 且再次保存同一期初保持幂等。
func TestOpeningEquityRestoreWhenSoftDeleted(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)
	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}

	// 首次期初：012 语义下的权益账户由运行时补种。
	if _, err := svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "100", Direction: "hold"}); err != nil {
		t.Fatalf("首次期初: %v", err)
	}

	var equityID int64
	var equityUUID string
	if err := svc.repo.DB().QueryRow(
		`SELECT id, COALESCE(uuid, '') FROM accounts
		 WHERE ledger_id=? AND name=? AND deleted_at IS NULL`,
		lid, domain.OpeningEquityName,
	).Scan(&equityID, &equityUUID); err != nil {
		t.Fatalf("读取期初权益账户: %v", err)
	}
	var postingsBefore int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM postings WHERE account_id=?`, equityID,
	).Scan(&postingsBefore); err != nil {
		t.Fatalf("统计权益分录: %v", err)
	}

	// 人为软删：模拟历史 002 删除语义残留，同时置 close_date 以验证恢复会一并清空。
	if _, err := svc.repo.DB().Exec(
		`UPDATE accounts SET deleted_at=datetime('now'), close_date=date('now') WHERE id=?`,
		equityID,
	); err != nil {
		t.Fatalf("软删权益账户: %v", err)
	}

	// 修改期初金额：旧实现此处递归插入撞唯一索引直至栈溢出，现在必须成功。
	res, err := svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "250", Direction: "hold"})
	if err != nil {
		t.Fatalf("软删权益账户后期初设置应成功: %v", err)
	}
	if res.Status != "updated" || res.SignedAmount != "250.00" {
		t.Fatalf("期初修正结果不符: %+v", res)
	}

	// 原地恢复：id/uuid 不变，deleted_at/close_date 清空，同名账户仅 1 个。
	var (
		gotID     int64
		gotUUID   string
		deletedAt sql.NullString
		closeDate sql.NullString
	)
	if err := svc.repo.DB().QueryRow(
		`SELECT id, COALESCE(uuid, ''), deleted_at, close_date FROM accounts
		 WHERE ledger_id=? AND name=? ORDER BY id LIMIT 1`,
		lid, domain.OpeningEquityName,
	).Scan(&gotID, &gotUUID, &deletedAt, &closeDate); err != nil {
		t.Fatalf("读取恢复后的权益账户: %v", err)
	}
	if gotID != equityID || gotUUID != equityUUID {
		t.Fatalf("应复用原权益账户 (id=%d uuid=%s)，实际 id=%d uuid=%s",
			equityID, equityUUID, gotID, gotUUID)
	}
	if deletedAt.Valid || closeDate.Valid {
		t.Fatalf("恢复后应清空 deleted_at/close_date: deleted_at=%v close_date=%v",
			deletedAt, closeDate)
	}
	var equityCount int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM accounts WHERE ledger_id=? AND name=?`,
		lid, domain.OpeningEquityName,
	).Scan(&equityCount); err != nil {
		t.Fatalf("统计同名权益账户: %v", err)
	}
	if equityCount != 1 {
		t.Fatalf("不应产生重复权益账户，实际 %d 个", equityCount)
	}

	// 历史分录复用：原期初 + 冲正 + 新期初各 1 条权益分录。
	var postingsAfter int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM postings WHERE account_id=?`, equityID,
	).Scan(&postingsAfter); err != nil {
		t.Fatalf("统计恢复后权益分录: %v", err)
	}
	if postingsAfter != postingsBefore+2 {
		t.Fatalf("权益分录应复用原账户并新增 2 条，之前 %d 之后 %d", postingsBefore, postingsAfter)
	}

	// sync_log 记录恢复动作（account/update，uuid 不变）。
	var updates int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM sync_log
		 WHERE ledger_id=? AND entity_type=? AND entity_uuid=? AND op=?`,
		lid, domain.EntityAccount, equityUUID, domain.OpUpdate,
	).Scan(&updates); err != nil {
		t.Fatalf("统计权益恢复同步事件: %v", err)
	}
	if updates < 1 {
		t.Fatalf("恢复权益账户应写 sync_log(account/update)，实际 %d 条", updates)
	}

	// 余额正确：现金 250.00，权益 -250.00，恒等式平衡。
	view, err := svc.GetOpening(lid, cash.UUID)
	if err != nil {
		t.Fatalf("读取期初: %v", err)
	}
	if !view.HasOpening || view.SignedAmount != "250.00" {
		t.Fatalf("恢复后期初应为 250.00: %+v", view)
	}
	chk, err := svc.CheckIdentity(lid)
	if err != nil || !chk.Balanced {
		t.Fatalf("恢复后恒等式应平衡: %+v err=%v", chk, err)
	}

	// 幂等：再次保存同一期初不应新增交易。
	var beforeTxn int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM transactions WHERE ledger_id=?`, lid,
	).Scan(&beforeTxn); err != nil {
		t.Fatalf("统计交易数: %v", err)
	}
	res2, err := svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "250", Direction: "hold"})
	if err != nil {
		t.Fatalf("重复保存同一期初应幂等: %v", err)
	}
	if res2.Status != "unchanged" {
		t.Fatalf("重复保存应返回 unchanged，实际 %s", res2.Status)
	}
	var afterTxn int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM transactions WHERE ledger_id=?`, lid,
	).Scan(&afterTxn); err != nil {
		t.Fatalf("统计交易数: %v", err)
	}
	if afterTxn != beforeTxn {
		t.Fatalf("幂等保存不应新增交易，之前 %d 之后 %d", beforeTxn, afterTxn)
	}
}

// TestSyncCloseParityWithREST 同步 op=close 与 REST 关闭共用同一余额校验：
// 余额非 0 均拒绝并回传差额；结清后均能关闭。
func TestSyncCloseParityWithREST(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)
	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	food, err := svc.CreateAccount(lid, "Expenses:QA:Food", "", "Expenses", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create food: %v", err)
	}
	if _, err := svc.CreateTransaction(lid, "2026-01-02", "*", "早餐", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "10.00"},
	}, 1); err != nil {
		t.Fatalf("create txn: %v", err)
	}

	// REST：409 语义由服务层 BalanceNotZeroError 表达。
	var bz *domain.BalanceNotZeroError
	if err := svc.DeleteAccount(lid, cash.UUID); err == nil {
		t.Fatal("余额非 0 时 REST 关闭应失败")
	} else if !errors.As(err, &bz) {
		t.Fatalf("REST 关闭应返回 BalanceNotZeroError，实际 %v", err)
	}
	if len(bz.Balances) != 1 || bz.Balances[0].Commodity != "CNY" || bz.Balances[0].Amount != "-10.00" {
		t.Fatalf("REST 差额明细不符: %+v", bz.Balances)
	}

	// 同步：同余额应返回 accepted=false + account_balance_not_zero + 差额。
	entity, _ := json.Marshal(map[string]any{"uuid": cash.UUID})
	out, err := svc.SyncPush(lid, 1, PushRequest{
		ClientID: "c1",
		Changes:  []PushChange{{EntityType: domain.EntityAccount, Op: domain.OpClose, Entity: entity}},
	})
	if err != nil {
		t.Fatalf("sync push: %v", err)
	}
	if len(out.Results) != 1 {
		t.Fatalf("应返回 1 条结果，实际 %d", len(out.Results))
	}
	r := out.Results[0]
	if r.Accepted || r.Code != domain.CodeAccountBalanceNotZero || r.Status != "error" {
		t.Fatalf("同步关闭应被拒绝并带错误码: %+v", r)
	}
	if len(r.Balances) != 1 || r.Balances[0].Amount != "-10.00" {
		t.Fatalf("同步差额明细不符: %+v", r.Balances)
	}
	// 账户仍打开。
	if a := findAccount(t, svc, "Assets:Cash:CNY"); a.CloseDate != "" {
		t.Fatalf("同步关闭失败后账户不应关闭: %+v", a)
	}

	// 结清后同步关闭成功。
	if _, err := svc.CreateTransaction(lid, "2026-01-03", "*", "报销", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "-10.00"},
	}, 1); err != nil {
		t.Fatalf("offset txn: %v", err)
	}
	out, err = svc.SyncPush(lid, 1, PushRequest{
		ClientID: "c1",
		Changes:  []PushChange{{EntityType: domain.EntityAccount, Op: domain.OpClose, Entity: entity}},
	})
	if err != nil {
		t.Fatalf("sync push close: %v", err)
	}
	if !out.Results[0].Accepted {
		t.Fatalf("余额为 0 时同步关闭应成功: %+v", out.Results[0])
	}
	var closeDate string
	if err := svc.repo.DB().QueryRow(`SELECT COALESCE(close_date,'') FROM accounts WHERE uuid=?`, cash.UUID).Scan(&closeDate); err != nil {
		t.Fatalf("读取 close_date: %v", err)
	}
	if closeDate == "" {
		t.Fatal("同步关闭后 close_date 应非空")
	}
	// 已关闭账户再次 REST 关闭：幂等视为不存在（handler 映射 404）。
	if err := svc.DeleteAccount(lid, cash.UUID); err != domain.ErrNotFound {
		t.Fatalf("已关闭账户再次关闭应返回 ErrNotFound，实际 %v", err)
	}
}
