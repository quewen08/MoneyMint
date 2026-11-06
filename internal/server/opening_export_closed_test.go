package server

// 回归测试：账本期初日期前移后，已关闭账户的期初被重建但 open_date 不随之下调，
// 会导致导出文本出现「交易日期早于账户开立日」，bean-check 失败。
//
// 复现链（全部通过公开 API）：
//   1. 建账户并录入期初（账户 open_date = 账本期初日 = 今日）；
//   2. 当日转出全部余额后关闭账户（close_date = 今日）；
//   3. owner 把账本期初日期前移到昨日（≤ 今日且 ≤ 最早非期初交易，合法）；
//   4. 导出 → 单文件 bean-check 应退出 0，实际会报 inactive account。

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHTTPClosedAccountRebuildKeepsExportValid(t *testing.T) {
	e := newOpeningHTTPEnv(t)

	cashID, cashUUID := e.createAccount(1, "Assets:Cash:CNY", "Assets", "CNY")
	bankID, _ := e.createAccount(1, "Assets:Bank", "Assets", "CNY")

	status, _, raw := e.do(http.MethodPost, "/api/accounts/"+cashUUID+"/opening", e.ownerToken, 1,
		map[string]any{"amount": "100", "direction": "hold"})
	e.decodeMap(http.StatusOK, status, raw)

	// 当日把 100 全部转出，随后关闭账户（余额为 0，允许关闭）。
	today := time.Now().Format("2006-01-02")
	e.createTxn(1, today, "余额转出", []map[string]any{
		{"account_id": cashID, "commodity": "CNY", "amount": "-100.00"},
		{"account_id": bankID, "commodity": "CNY", "amount": "100.00"},
	})
	status, _, raw = e.do(http.MethodDelete, "/api/accounts/"+cashUUID, e.ownerToken, 1, nil)
	if status != http.StatusOK {
		t.Fatalf("余额为 0 应可关闭，实际 %d: %s", status, raw)
	}

	// 账本期初日前移到昨日，触发全部有效期初重建。
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	status, _, raw = e.do(http.MethodPatch, "/api/ledgers/1", e.ownerToken, 1, map[string]any{
		"name": "家庭账本", "default_commodity": "CNY", "opening_date": yesterday,
	})
	if status != http.StatusOK {
		t.Fatalf("owner 前移账本日期应成功，实际 %d: %s", status, raw)
	}

	// 导出必须仍然可通过 bean-check。
	dir := t.TempDir()
	status, _, raw = e.do(http.MethodGet, "/api/export", e.ownerToken, 1, nil)
	if status != http.StatusOK || len(raw) == 0 {
		t.Fatalf("导出失败: %d %s", status, raw)
	}
	single := filepath.Join(dir, "closed_rebuild.beancount")
	if err := os.WriteFile(single, raw, 0o644); err != nil {
		t.Fatalf("写导出文件: %v", err)
	}
	runBeanCheck(t, dir, single)
}
