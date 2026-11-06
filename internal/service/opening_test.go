package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"familyledger/internal/domain"
)

// findAccount 按名称查找账户视图。
func findAccount(t *testing.T, svc *Service, name string) AccountView {
	t.Helper()
	accounts, err := svc.ListAccounts(lid, true)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	for _, a := range accounts {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("account %s not found", name)
	return AccountView{}
}

// codedCode 提取 CodedError 的稳定错误码。
func codedCode(t *testing.T, err error) string {
	t.Helper()
	var ce *domain.CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("期望 CodedError，实际 %v", err)
	}
	return ce.Code
}

// TestOpeningDirectionAndLifecycle 验证方向映射、修正冲正、幂等与清空。
func TestOpeningDirectionAndLifecycle(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	card, err := svc.CreateAccount(lid, "Liabilities:TestCard", "", "Liabilities", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	// 资产持有：正号
	res, err := svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "100", Direction: "hold"})
	if err != nil {
		t.Fatalf("set opening cash: %v", err)
	}
	if res.Status != "created" || res.SignedAmount != "100.00" || res.Version != 1 {
		t.Fatalf("首次期初结果不符: %+v", res)
	}
	view, err := svc.GetOpening(lid, cash.UUID)
	if err != nil {
		t.Fatalf("get opening: %v", err)
	}
	if !view.HasOpening || view.Direction != "hold" || view.Amount != "100.00" || view.SignedAmount != "100.00" {
		t.Fatalf("资产持有视图不符: %+v", view)
	}

	// 幂等：同内容重复提交不新增交易
	res, err = svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "100.00", Direction: "hold"})
	if err != nil {
		t.Fatalf("idempotent set: %v", err)
	}
	if res.Status != "unchanged" || res.TransactionUUID != view.TransactionUUID {
		t.Fatalf("同内容应 unchanged: %+v", res)
	}

	// 修正为欠款：冲正旧 + 新建，同事务
	res, err = svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "50", Direction: "owe"})
	if err != nil {
		t.Fatalf("update opening: %v", err)
	}
	if res.Status != "updated" || res.SignedAmount != "-50.00" || res.Version != 2 || res.ReversalUUID == "" {
		t.Fatalf("修正结果不符: %+v", res)
	}
	var reversedBy string
	if err := svc.repo.DB().QueryRow(
		`SELECT COALESCE(reversed_by,'') FROM transactions WHERE uuid=?`, view.TransactionUUID,
	).Scan(&reversedBy); err != nil {
		t.Fatalf("read reversed_by: %v", err)
	}
	if reversedBy != res.ReversalUUID {
		t.Fatalf("旧期初 reversed_by 应为冲正 uuid %s，实际 %q", res.ReversalUUID, reversedBy)
	}

	// 负债欠款：负号（前端只传绝对值）
	res, err = svc.SetOpening(lid, card.UUID, OpeningInput{Amount: "200", Direction: "owe"})
	if err != nil {
		t.Fatalf("set liability opening: %v", err)
	}
	if res.SignedAmount != "-200.00" {
		t.Fatalf("负债欠款应为负号，实际 %s", res.SignedAmount)
	}

	// 清空期初
	res, err = svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "0", Direction: "hold"})
	if err != nil {
		t.Fatalf("clear opening: %v", err)
	}
	if res.Status != "cleared" {
		t.Fatalf("清零应返回 cleared，实际 %+v", res)
	}
	view, err = svc.GetOpening(lid, cash.UUID)
	if err != nil {
		t.Fatalf("get cleared: %v", err)
	}
	if view.HasOpening {
		t.Fatalf("清零后 has_opening 应为 false: %+v", view)
	}

	// 恒等式保持平衡
	chk, err := svc.CheckIdentity(lid)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	if !chk.Balanced {
		t.Fatalf("期初操作后恒等式应平衡: %+v", chk.Diffs)
	}
}

// TestOpeningRejectsInvalidAccountAndDirection 验证类型禁用与方向/金额校验。
func TestOpeningRejectsInvalidAccountAndDirection(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	food, err := svc.CreateAccount(lid, "Expenses:TestFood", "", "Expenses", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create food: %v", err)
	}
	_, err = svc.SetOpening(lid, food.UUID, OpeningInput{Amount: "10", Direction: "hold"})
	if code := codedCode(t, err); code != domain.CodeOpeningAccountNotOpenable {
		t.Fatalf("支出账户应被拒绝，code=%s", code)
	}

	cash, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	_, err = svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "10", Direction: "up"})
	if code := codedCode(t, err); code != domain.CodeOpeningDirectionInvalid {
		t.Fatalf("非法方向 code=%s", code)
	}
	_, err = svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "-10", Direction: "hold"})
	if code := codedCode(t, err); code != domain.CodeOpeningAmountInvalid {
		t.Fatalf("负数应被拒绝，code=%s", code)
	}
	if _, err := svc.GetOpening(lid, "not-exist"); codedCode(t, err) != domain.CodeAccountNotFound {
		t.Fatalf("不存在账户应为 account_not_found")
	}
}

// TestOpeningCommodityPriority 验证币种优先级：账户限制 → 账本默认 → CNY。
func TestOpeningCommodityPriority(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	usd, err := svc.CreateAccount(lid, "Assets:Cash:USD", "", "Assets", "", "USD", "", "", "", "")
	if err != nil {
		t.Fatalf("create usd: %v", err)
	}
	_, err = svc.SetOpening(lid, usd.UUID, OpeningInput{Amount: "10", Direction: "hold", Commodity: "CNY"})
	if code := codedCode(t, err); code != domain.CodeOpeningCommodityMismatch {
		t.Fatalf("币种与账户限制不符应拒绝，code=%s", code)
	}
	res, err := svc.SetOpening(lid, usd.UUID, OpeningInput{Amount: "10", Direction: "hold"})
	if err != nil {
		t.Fatalf("美元期初: %v", err)
	}
	if res.Commodity != "USD" {
		t.Fatalf("币种应取账户限制 USD，实际 %s", res.Commodity)
	}

	// 账本默认 USD（无账户限制）
	usdLedger, err := svc.CreateLedger(1, "美元账本", "USD")
	if err != nil {
		t.Fatalf("create usd ledger: %v", err)
	}
	plain, err := svc.CreateAccount(usdLedger.ID, "Assets:Bank", "", "Assets", "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("create plain account: %v", err)
	}
	res, err = svc.SetOpening(usdLedger.ID, plain.UUID, OpeningInput{Amount: "5", Direction: "hold"})
	if err != nil {
		t.Fatalf("账本默认币种期初: %v", err)
	}
	if res.Commodity != "USD" {
		t.Fatalf("币种应取账本默认 USD，实际 %s", res.Commodity)
	}

	// 账本默认与账户限制均空 → CNY
	cnyLedger, err := svc.CreateLedger(1, "兜底账本", "")
	if err != nil {
		t.Fatalf("create fallback ledger: %v", err)
	}
	fb, _ := svc.CreateAccount(cnyLedger.ID, "Assets:Fallback", "", "Assets", "", "", "", "", "", "")
	res, err = svc.SetOpening(cnyLedger.ID, fb.UUID, OpeningInput{Amount: "1", Direction: "hold"})
	if err != nil {
		t.Fatalf("兜底币种期初: %v", err)
	}
	if res.Commodity != "CNY" {
		t.Fatalf("兜底币种应为 CNY，实际 %s", res.Commodity)
	}
}

// TestCreateAccountOpeningDate 验证 FR-03：默认 min(账本日期, 今日)，不得晚于账本日期。
func TestCreateAccountOpeningDate(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	today := time.Now().Format("2006-01-02")
	a, err := svc.CreateAccount(lid, "Assets:Default", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create default: %v", err)
	}
	if a.OpenDate != today {
		t.Fatalf("默认 open_date 应为 %s，实际 %s", today, a.OpenDate)
	}

	// 先平移账本期初到昨天，再建账户：默认取账本日期
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if _, err := svc.UpdateLedger(1, lid, "家庭账本", "CNY", &yesterday); err != nil {
		t.Fatalf("update ledger opening date: %v", err)
	}
	b, err := svc.CreateAccount(lid, "Assets:Historical", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create historical: %v", err)
	}
	if b.OpenDate != yesterday {
		t.Fatalf("open_date 应取账本期初 %s，实际 %s", yesterday, b.OpenDate)
	}

	_, err = svc.CreateAccount(lid, "Assets:Future", "", "Assets", today, "CNY", "", "", "", "")
	if code := codedCode(t, err); code != domain.CodeAccountOpenDateAfter {
		t.Fatalf("晚于账本期初应拒绝，code=%s", code)
	}
}

// TestLedgerOpeningDateConstraintsAndRebuild 验证日期约束与重建期初。
func TestLedgerOpeningDateConstraintsAndRebuild(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	if _, err := svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "100", Direction: "hold"}); err != nil {
		t.Fatalf("set opening: %v", err)
	}

	// 未来日期非法
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	if _, err := svc.UpdateLedger(1, lid, "家庭账本", "CNY", &tomorrow); codedCode(t, err) != domain.CodeLedgerOpeningDateInvalid {
		t.Fatalf("未来日期应拒绝")
	}

	// 早于最早交易：先记一笔 2026-01-01，再把期初挪到 2026-02-01 应失败
	food, _ := svc.CreateAccount(lid, "Expenses:TestFood", "", "Expenses", "", "CNY", "", "", "", "")
	if _, err := svc.CreateTransaction(lid, "2026-01-01", "*", "旧交易", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "10.00"},
	}, 1); err != nil {
		t.Fatalf("create old txn: %v", err)
	}
	afterFirst := "2026-02-01"
	if _, err := svc.UpdateLedger(1, lid, "家庭账本", "CNY", &afterFirst); codedCode(t, err) != domain.CodeLedgerOpeningDateAfterTxn {
		t.Fatalf("晚于最早交易应拒绝")
	}

	// 合法平移：2025-12-31，期初交易重建到新日期，账户 open_date 下调
	newDate := "2025-12-31"
	view, err := svc.UpdateLedger(1, lid, "家庭账本", "CNY", &newDate)
	if err != nil {
		t.Fatalf("rebuild openings: %v", err)
	}
	if view.OpeningDate != newDate {
		t.Fatalf("账本期初应更新为 %s，实际 %s", newDate, view.OpeningDate)
	}
	op, err := svc.GetOpening(lid, cash.UUID)
	if err != nil {
		t.Fatalf("get rebuilt opening: %v", err)
	}
	if !op.HasOpening || op.Date != newDate {
		t.Fatalf("期初交易应重建到 %s: %+v", newDate, op)
	}
	var openDate string
	if err := svc.repo.DB().QueryRow(`SELECT open_date FROM accounts WHERE uuid=?`, cash.UUID).Scan(&openDate); err != nil {
		t.Fatalf("read open_date: %v", err)
	}
	if openDate != newDate {
		t.Fatalf("账户 open_date 应下调为 %s，实际 %s", newDate, openDate)
	}
	chk, err := svc.CheckIdentity(lid)
	if err != nil || !chk.Balanced {
		t.Fatalf("重建后恒等式应平衡: %+v err=%v", chk, err)
	}
}

// TestRebuildOpeningsSkipsClosedAccount 回归 BUG-01：账本期初日期前移时，
// 已关闭账户不参与期初重建（其 open_date 保持不变），且重建后任何有效期初
// 的日期都不得早于对应账户的 open_date（否则导出 bean-check 报 inactive account）。
func TestRebuildOpeningsSkipsClosedAccount(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, err := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create cash: %v", err)
	}
	if _, err := svc.SetOpening(lid, cash.UUID, OpeningInput{Amount: "100", Direction: "hold"}); err != nil {
		t.Fatalf("set opening: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	food, err := svc.CreateAccount(lid, "Expenses:TestFood", "", "Expenses", "", "CNY", "", "", "", "")
	if err != nil {
		t.Fatalf("create food: %v", err)
	}
	// 当日全部转出后关闭账户。
	if _, err := svc.CreateTransaction(lid, today, "*", "余额转出", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-100.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "100.00"},
	}, 1); err != nil {
		t.Fatalf("create txn: %v", err)
	}
	if err := svc.DeleteAccount(lid, cash.UUID); err != nil {
		t.Fatalf("close account: %v", err)
	}

	// 账本期初日前移到昨日，触发重建。
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if _, err := svc.UpdateLedger(1, lid, "家庭账本", "CNY", &yesterday); err != nil {
		t.Fatalf("rebuild openings: %v", err)
	}

	// 已关闭账户的期初交易保持原日期，open_date 不被下调。
	op, err := svc.GetOpening(lid, cash.UUID)
	if err != nil {
		t.Fatalf("get closed opening: %v", err)
	}
	if !op.HasOpening || op.Date != today {
		t.Fatalf("已关闭账户期初不应被重建：期望日期 %s，实际 %+v", today, op)
	}
	var openDate, closeDate string
	if err := svc.repo.DB().QueryRow(
		`SELECT COALESCE(open_date,''), COALESCE(close_date,'') FROM accounts WHERE uuid=?`,
		cash.UUID,
	).Scan(&openDate, &closeDate); err != nil {
		t.Fatalf("读取账户日期: %v", err)
	}
	if openDate != today || closeDate == "" {
		t.Fatalf("已关闭账户日期不应变化：open_date=%q close_date=%q", openDate, closeDate)
	}

	// 全账本不变量：有效期初日期 >= 对应账户 open_date。
	var bad int
	if err := svc.repo.DB().QueryRow(
		`SELECT count(*) FROM transactions t
		 JOIN accounts a ON a.ledger_id=t.ledger_id AND a.uuid=t.opening_account_uuid
		 WHERE t.ledger_id=? AND t.opening_account_uuid IS NOT NULL AND t.opening_account_uuid<>''
		   AND t.reversal_of IS NULL AND (t.reversed_by IS NULL OR t.reversed_by='')
		   AND t.deleted_at IS NULL
		   AND a.open_date IS NOT NULL AND a.open_date<>'' AND t.date < a.open_date`,
		lid,
	).Scan(&bad); err != nil {
		t.Fatalf("检查期初日期不变量: %v", err)
	}
	if bad != 0 {
		t.Fatalf("存在 %d 条期初早于账户 open_date", bad)
	}
}

// TestCloseAccountBalanceNotZero 验证关闭账户余额非 0 返回结构化 409 信息。
func TestCloseAccountBalanceNotZero(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	food, _ := svc.CreateAccount(lid, "Expenses:TestFood", "", "Expenses", "", "CNY", "", "", "", "")
	if _, err := svc.CreateTransaction(lid, "2026-01-02", "*", "早餐", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "-10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "10.00"},
	}, 1); err != nil {
		t.Fatalf("create txn: %v", err)
	}

	err := svc.DeleteAccount(lid, cash.UUID)
	var bz *domain.BalanceNotZeroError
	if !errors.As(err, &bz) {
		t.Fatalf("余额非 0 应返回 BalanceNotZeroError，实际 %v", err)
	}
	if len(bz.Balances) != 1 || bz.Balances[0].Commodity != "CNY" || bz.Balances[0].Amount != "-10.00" {
		t.Fatalf("差额明细不符: %+v", bz.Balances)
	}
	// 账户保持打开
	if a := findAccount(t, svc, "Assets:Cash:CNY"); a.CloseDate != "" {
		t.Fatalf("校验失败时账户不应被关闭")
	}

	// 对冲后余额为 0，关闭成功
	if _, err := svc.CreateTransaction(lid, "2026-01-03", "*", "报销", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "-10.00"},
	}, 1); err != nil {
		t.Fatalf("offset txn: %v", err)
	}
	if err := svc.DeleteAccount(lid, cash.UUID); err != nil {
		t.Fatalf("余额为 0 应关闭成功: %v", err)
	}
}

// TestOpeningTagRejectedEverywhere 验证通用记账与同步 push 均拒绝 opening 标签。
func TestOpeningTagRejectedEverywhere(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	cash, _ := svc.CreateAccount(lid, "Assets:Cash:CNY", "", "Assets", "", "CNY", "", "", "", "")
	food, _ := svc.CreateAccount(lid, "Expenses:TestFood", "", "Expenses", "", "CNY", "", "", "", "")

	_, err := svc.CreateTransaction(lid, "2026-01-02", "*", "伪造期初", []PostingInput{
		{AccountID: cash.ID, Commodity: "CNY", Amount: "10.00"},
		{AccountID: food.ID, Commodity: "CNY", Amount: "-10.00"},
	}, 1, "opening")
	if code := codedCode(t, err); code != domain.CodeOpeningViaAPIOnly {
		t.Fatalf("在线记账应拒绝 opening 标签，code=%s", code)
	}

	entity, _ := json.Marshal(map[string]any{
		"uuid":        "loc-1",
		"date":        "2026-01-02",
		"flag":        "*",
		"description": "离线伪造期初",
		"tags":        []string{"opening"},
		"postings": []map[string]any{
			{"account_uuid": cash.UUID, "commodity": "CNY", "amount": "10.00"},
			{"account_uuid": food.UUID, "commodity": "CNY", "amount": "-10.00"},
		},
	})
	out, err := svc.SyncPush(lid, 1, PushRequest{
		ClientID: "c1",
		Changes:  []PushChange{{EntityType: domain.EntityTransaction, Op: domain.OpCreate, Entity: entity}},
	})
	if err != nil {
		t.Fatalf("sync push: %v", err)
	}
	if len(out.Results) != 1 || out.Results[0].Code != domain.CodeOpeningViaAPIOnly || out.Results[0].Accepted {
		t.Fatalf("同步 push 应拒绝 opening 标签: %+v", out.Results)
	}
}

// TestIdentityDetectsUnbalanced 验证恒等式检查能发现直接写入的单边分录。
func TestIdentityDetectsUnbalanced(t *testing.T) {
	svc := newTestService(t)
	registerOwner(t, svc)

	chk, err := svc.CheckIdentity(lid)
	if err != nil || !chk.Balanced {
		t.Fatalf("空账本应平衡: %+v err=%v", chk, err)
	}

	// 绕过业务校验，直接插入一条单边分录
	if _, err := svc.repo.DB().Exec(`INSERT INTO accounts(id, ledger_id, uuid, name, type, open_date)
		VALUES(999, 1, 'bad-acc', 'Assets:Bad', 'Assets', '2026-01-01')`); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := svc.repo.DB().Exec(`INSERT INTO transactions(id, ledger_id, uuid, date, flag, description)
		VALUES(999, 1, 'bad-txn', '2026-01-01', '*', '单边')`); err != nil {
		t.Fatalf("insert txn: %v", err)
	}
	if _, err := svc.repo.DB().Exec(`INSERT INTO postings(transaction_id, account_id, commodity, amount, position)
		VALUES(999, 999, 'CNY', '7.00', 0)`); err != nil {
		t.Fatalf("insert posting: %v", err)
	}

	chk, err = svc.CheckIdentity(lid)
	if err != nil || chk.Balanced {
		t.Fatalf("单边分录应被识别为失衡: %+v err=%v", chk, err)
	}
	if len(chk.Diffs) != 1 || chk.Diffs[0].Commodity != "CNY" || chk.Diffs[0].Sum != "7.00" {
		t.Fatalf("差额明细不符: %+v", chk.Diffs)
	}
}
