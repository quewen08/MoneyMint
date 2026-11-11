package beancount

import (
	"strings"
	"testing"
)

// TestParseOwnExportShape 用与 internal/export.ExportLedger 同构的样本，
// 验证解析器能精确还原标题 / 币种 / 账户(open+close) / 价格 / 交易(含 tags 与 payee)。
func TestParseOwnExportShape(t *testing.T) {
	src := `; -*- mode: beancount -*-
; Exported from ledger "测试账本"

option "title" "测试账本"

2000-01-01 commodity CNY
  name: "人民币"
  precision: "2"

2000-01-01 commodity USD
2000-01-01 open Assets:Cash:CNY CNY
2000-01-01 open Expenses:Food
2000-06-01 close Expenses:Food

2000-01-01 price CNY 7.20 USD

2026-01-05 * "早餐"
  tags: "餐饮 出差"
  Assets:Cash:CNY -12.50 CNY
  Expenses:Food 12.50 CNY

2026-01-06 * "老板" "午餐"
  Assets:Cash:CNY -30.00 CNY
  Expenses:Food 30.00 CNY
`
	res, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse 不应返回硬错误: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("自有格式不应有解析错误: %v", res.Errors)
	}

	if res.Title != "测试账本" {
		t.Errorf("Title = %q, want 测试账本", res.Title)
	}

	if len(res.Commodities) != 2 {
		t.Fatalf("Commodities = %d, want 2", len(res.Commodities))
	}
	if res.Commodities[0].Symbol != "CNY" || res.Commodities[0].Name != "人民币" || res.Commodities[0].Prec != "2" {
		t.Errorf("commodity[0] 元数据丢失: %+v", res.Commodities[0])
	}
	if res.Commodities[1].Symbol != "USD" {
		t.Errorf("commodity[1].Symbol = %q, want USD", res.Commodities[1].Symbol)
	}

	if len(res.Accounts) != 2 {
		t.Fatalf("Accounts = %d, want 2 (Cash 与 Food)", len(res.Accounts))
	}
	if res.Accounts[0].Name != "Assets:Cash:CNY" || res.Accounts[0].Commodity != "CNY" {
		t.Errorf("Accounts[0] 错误: %+v", res.Accounts[0])
	}
	if res.Accounts[1].Name != "Expenses:Food" || res.Accounts[1].OpenDate != "2000-01-01" || res.Accounts[1].CloseDate != "2000-06-01" {
		t.Errorf("Accounts[1] open/close 绑定错误: %+v", res.Accounts[1])
	}

	if len(res.Prices) != 1 {
		t.Fatalf("Prices = %d, want 1", len(res.Prices))
	}
	if res.Prices[0].Commodity != "CNY" || res.Prices[0].Amount != "7.20" || res.Prices[0].Currency != "USD" {
		t.Errorf("Price 错误: %+v", res.Prices[0])
	}

	if len(res.Transactions) != 2 {
		t.Fatalf("Transactions = %d, want 2", len(res.Transactions))
	}
	t0 := res.Transactions[0]
	if t0.Date != "2026-01-05" || t0.Flag != "*" || t0.Narration != "早餐" {
		t.Errorf("T0 头错误: %+v", t0)
	}
	if strings.Join(t0.Tags, " ") != "餐饮 出差" {
		t.Errorf("T0 tags = %v, want [餐饮 出差]", t0.Tags)
	}
	if len(t0.Postings) != 2 || t0.Postings[0].Account != "Assets:Cash:CNY" || t0.Postings[0].Amount != "-12.50" || t0.Postings[1].Commodity != "CNY" {
		t.Errorf("T0 postings 错误: %+v", t0.Postings)
	}
	t1 := res.Transactions[1]
	if t1.Payee != "老板" || t1.Narration != "午餐" || len(t1.Postings) != 2 {
		t.Errorf("T1 payee/结构错误: %+v", t1)
	}
}

// TestParseUnsupportedRejected 确认各类不支持语法均被收集为错误。
func TestParseUnsupportedRejected(t *testing.T) {
	src := `option "title" "x"

2020-01-01 open Assets:X

plugin "beancount.plugins.foo"
include "other.bean"
2020-01-02 pad Assets:X Cash:Y
2020-01-03 balance Assets:X 10 CNY
2020-01-04 note Assets:X "hi"
2020-01-05 document Assets:X "/tmp/a.pdf"
2020-01-06 event "trip" "go"
2020-01-07 custom "a" "b"
2020-01-08 * "txn"
  Assets:X 10 CNY {7.20 USD}
2020-01-09 price CNY 7.20 USD @ 7.30
`
	res, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse 不应返回硬错误: %v", err)
	}
	if len(res.Errors) != 10 {
		t.Fatalf("应收集到 10 个错误，实际 %d: %v", len(res.Errors), res.Errors)
	}
	msgs := map[string]bool{}
	for _, e := range res.Errors {
		msgs[e.Msg] = true
	}
	for _, want := range []string{
		"不支持的指令 \"plugin\"",
		"不支持的指令 \"include\"",
		"不支持的指令 \"pad\"",
		"不支持的指令 \"balance\"",
		"不支持的指令 \"note\"",
		"不支持的指令 \"document\"",
		"不支持的指令 \"event\"",
		"不支持的指令 \"custom\"",
		"不支持的成本注解 {}",
		"不支持的价格注解 @",
	} {
		if !msgs[want] {
			t.Errorf("缺少预期错误: %q (实际 %v)", want, res.Errors)
		}
	}
}

// TestParseErrorLineNumbers 校验错误携带的行号准确（行号是用户反馈的核心信息）。
func TestParseErrorLineNumbers(t *testing.T) {
	src := "option \"title\" \"t\"\n" + // 1
		"\n" + // 2
		"plugin \"x\"\n" + // 3
		"2020-01-01 open Assets:X\n" + // 4
		"include \"y.bean\"\n" // 5

	res, _ := Parse(src)
	lineOf := func(sub string) int {
		for _, e := range res.Errors {
			if strings.Contains(e.Msg, sub) {
				return e.Line
			}
		}
		return -1
	}
	if ln := lineOf("plugin"); ln != 3 {
		t.Errorf("plugin 错误行号 = %d, want 3", ln)
	}
	if ln := lineOf("include"); ln != 5 {
		t.Errorf("include 错误行号 = %d, want 5", ln)
	}
}

// TestParseAllErrorsCollected 确认扫描不遇首错即停，而是收集全部问题。
func TestParseAllErrorsCollected(t *testing.T) {
	src := "plugin \"a\"\n" +
		"include \"b\"\n" +
		"2020-01-01 * \"t\"\n" +
		"  Assets:X 1 CNY {2 USD}\n"
	res, _ := Parse(src)
	if len(res.Errors) != 3 {
		t.Fatalf("应收集 3 个错误（不遇首错即停），实际 %d: %v", len(res.Errors), res.Errors)
	}
}

// TestParseCommentsOnly 全是注释/空行的文件不应报错、不应产生实体。
func TestParseCommentsOnly(t *testing.T) {
	res, err := Parse("; just a comment\n\n; another\n")
	if err != nil {
		t.Fatalf("Parse 不应返回硬错误: %v", err)
	}
	if len(res.Errors) != 0 || len(res.Transactions) != 0 || len(res.Accounts) != 0 {
		t.Fatalf("空文件不应有实体或错误: %+v", res)
	}
}

// TestParseMissingAmount 分录缺金额应报错，但不影响其他行继续解析。
func TestParseMissingAmount(t *testing.T) {
	src := "2020-01-01 open Assets:X\n" +
		"2020-01-02 * \"t\"\n" +
		"  Assets:X\n" // 缺金额
	res, _ := Parse(src)
	if len(res.Errors) != 1 {
		t.Fatalf("应恰好 1 个错误，实际 %d: %v", len(res.Errors), res.Errors)
	}
	if !strings.Contains(res.Errors[0].Msg, "至少需要账户与金额") {
		t.Errorf("错误内容不符: %v", res.Errors[0])
	}
}
