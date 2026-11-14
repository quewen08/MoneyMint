// Package beancount 提供 Beancount 文本的纯函数解析器（零数据库、零文件依赖），
// 供 M2 导入功能把 .beancount 文本解析为结构化结果。
//
// 设计约束：
//   - 仅支持自家导出用到的「严格子集」：option / commodity / open / close / price /
//     交易块（含 metadata）；遇到不支持的语法（include / plugin / balance / pad /
//     note / document / event / custom / cost 注解 {} / price 注解 @）整体拒绝。
//   - 错误策略：扫描完整文件、收集全部错误（含行号）后一次性返回，用户一次看到所有问题。
//   - 不解析 include（纯函数无文件路径上下文）；导入以单文件 ExportLedger 产物为契约，
//     多文件归档（含 include）不在导入范围。
package beancount

import (
	"fmt"
	"strings"
)

// ParseError 是带行号的解析错误。
type ParseError struct {
	Line int    `json:"line"`
	Msg  string `json:"msg"`
}

func (e ParseError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

// Commodity 是 commodity 指令（币种声明）。
type Commodity struct {
	Date   string `json:"date"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Prec   string `json:"prec"`
}

// Account 是 open/close 指令组合出的账户。
type Account struct {
	Name      string `json:"name"`
	OpenDate  string `json:"open_date"`
	Commodity string `json:"commodity"`  // open 的币种限制，空表示无限制
	CloseDate string `json:"close_date"` // 空表示未关闭
}

// Price 是 price 指令。
type Price struct {
	Date      string `json:"date"`
	Commodity string `json:"commodity"` // 被定价的币种
	Amount    string `json:"amount"`    // 单价（定点十进制字符串）
	Currency  string `json:"currency"`  // 计价币种
}

// Posting 是交易下的一条分录。
type Posting struct {
	Account   string `json:"account"`
	Amount    string `json:"amount"` // 空表示由 Beancount 推断（本应用导出恒非空）
	Commodity string `json:"commodity"`
}

// Transaction 是一笔交易（含其下所有分录与 metadata）。
type Transaction struct {
	Date      string    `json:"date"`
	Flag      string    `json:"flag"`
	Payee     string    `json:"payee"`
	Narration string    `json:"narration"`
	Tags      []string  `json:"tags"`
	Note      string    `json:"note"` // 未被 tags 捕获的其他 metadata 拼接
	Postings  []Posting `json:"postings"`
}

// ParseResult 是一次解析的完整结果。Errors 非空表示文件含不支持语法，应整体拒绝。
type ParseResult struct {
	Title        string        `json:"title"`
	Currency     string        `json:"currency"` // option operating_currency
	Commodities  []Commodity   `json:"commodities"`
	Accounts     []Account     `json:"accounts"`
	Prices       []Price       `json:"prices"`
	Transactions []Transaction `json:"transactions"`
	Errors       []ParseError  `json:"errors"`
}

// unsupportedTop 是列在顶层、直接拒绝的指令（无日期）。
var unsupportedTop = map[string]bool{
	"include":  true,
	"plugin":   true,
	"note":     true,
	"document": true,
	"event":    true,
	"custom":   true,
	"pad":      true,
	"balance":  true,
	"pushtag":  true,
	"poptag":   true,
}

// Parse 解析 Beancount 文本，返回结构化结果与全部错误（含行号）。
// 解析错误优先返回在 result.Errors 中，调用方应检查其长度决定是否整体拒绝。
func Parse(text string) (*ParseResult, error) {
	res := &ParseResult{}
	var curTxn *Transaction
	var curCommodity *Commodity

	lines := strings.Split(text, "\n")
	for i, raw := range lines {
		lineNo := i + 1
		code := stripComment(raw)
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			curTxn, curCommodity = nil, nil
			continue
		}
		if strings.HasPrefix(trimmed, ";") {
			curTxn, curCommodity = nil, nil
			continue
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if indent > 0 {
			if curTxn != nil {
				parseTxnIndented(curTxn, trimmed, lineNo, res)
			} else if curCommodity != nil {
				parseCommodityIndented(curCommodity, trimmed, lineNo, res)
			} else {
				res.Errors = append(res.Errors, ParseError{lineNo, "缩进内容出现在任何块之外"})
			}
			continue
		}

		// 顶层指令：结束当前块
		curTxn, curCommodity = nil, nil

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "option":
			if len(fields) >= 3 {
				key, val := unquote(fields[1]), unquote(fields[2])
				switch key {
				case "title":
					res.Title = val
				case "operating_currency":
					res.Currency = val
				}
			}
		case "include", "plugin", "note", "document", "event", "custom", "pad", "balance", "pushtag", "poptag":
			res.Errors = append(res.Errors, ParseError{lineNo, fmt.Sprintf("不支持的指令 %q", fields[0])})
		default:
			date, rest, ok := splitDate(trimmed)
			if !ok {
				res.Errors = append(res.Errors, ParseError{lineNo, "无法识别的顶层行"})
				continue
			}
			restFields := strings.Fields(rest)
			if len(restFields) == 0 {
				res.Errors = append(res.Errors, ParseError{lineNo, "日期后缺少指令"})
				continue
			}
			keyword := restFields[0]
			switch keyword {
			case "open":
				name, restr := parseOpen(restFields)
				if name == "" {
					res.Errors = append(res.Errors, ParseError{lineNo, "open 缺少账户名"})
					break
				}
				res.Accounts = append(res.Accounts, Account{Name: name, OpenDate: date, Commodity: restr})
			case "close":
				if len(restFields) < 2 {
					res.Errors = append(res.Errors, ParseError{lineNo, "close 缺少账户名"})
					break
				}
				name := restFields[1]
				// 绑定到最近一个同名且尚未 close 的账户；找不到则补一条仅含 close 的记录。
				bound := false
				for j := len(res.Accounts) - 1; j >= 0; j-- {
					if res.Accounts[j].Name == name && res.Accounts[j].CloseDate == "" {
						res.Accounts[j].CloseDate = date
						bound = true
						break
					}
				}
				if !bound {
					res.Accounts = append(res.Accounts, Account{Name: name, CloseDate: date})
				}
			case "commodity":
				if len(restFields) < 2 {
					res.Errors = append(res.Errors, ParseError{lineNo, "commodity 缺少币种符号"})
					break
				}
				res.Commodities = append(res.Commodities, Commodity{Date: date, Symbol: restFields[1]})
				curCommodity = &res.Commodities[len(res.Commodities)-1]
			case "price":
				if strings.Contains(rest, "@") {
					res.Errors = append(res.Errors, ParseError{lineNo, "不支持的价格注解 @"})
					break
				}
				if len(restFields) < 4 {
					res.Errors = append(res.Errors, ParseError{lineNo, "price 需要 币种 金额 计价币种"})
					break
				}
				res.Prices = append(res.Prices, Price{
					Date:      date,
					Commodity: restFields[1],
					Amount:    restFields[2],
					Currency:  restFields[3],
				})
			case "*", "!", "txn":
				flag := keyword
				if flag == "txn" {
					flag = "*"
				}
				payee, narration := parseTxnNarration(restFields[1:])
				res.Transactions = append(res.Transactions, Transaction{
					Date:      date,
					Flag:      flag,
					Payee:     payee,
					Narration: narration,
				})
				curTxn = &res.Transactions[len(res.Transactions)-1]
			default:
				if unsupportedTop[keyword] {
					res.Errors = append(res.Errors, ParseError{lineNo, fmt.Sprintf("不支持的指令 %q", keyword)})
				} else {
					res.Errors = append(res.Errors, ParseError{lineNo, fmt.Sprintf("不支持的指令 %q", keyword)})
				}
			}
		}
	}

	return res, nil
}

// parseTxnIndented 处理交易块内的缩进行（分录或 metadata）。
func parseTxnIndented(txn *Transaction, trimmed string, lineNo int, res *ParseResult) {
	if strings.Contains(trimmed, "{") {
		res.Errors = append(res.Errors, ParseError{lineNo, "不支持的成本注解 {}"})
		return
	}
	if isMetadataLine(trimmed) {
		key, val := splitMeta(trimmed)
		switch key {
		case "tags":
			if v := strings.TrimSpace(val); v != "" {
				txn.Tags = append(txn.Tags, strings.Fields(v)...)
			}
		default:
			if txn.Note != "" {
				txn.Note += "; "
			}
			txn.Note += key + ": " + val
		}
		return
	}
	// 分录
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		res.Errors = append(res.Errors, ParseError{lineNo, "分录至少需要账户与金额"})
		return
	}
	posting := Posting{Account: fields[0], Amount: fields[1]}
	if len(fields) >= 3 {
		posting.Commodity = fields[2]
	}
	txn.Postings = append(txn.Postings, posting)
}

// parseCommodityIndented 处理 commodity 块内的 metadata（name / precision）。
func parseCommodityIndented(c *Commodity, trimmed string, lineNo int, res *ParseResult) {
	if !isMetadataLine(trimmed) {
		res.Errors = append(res.Errors, ParseError{lineNo, "commodity 块内应为 metadata"})
		return
	}
	key, val := splitMeta(trimmed)
	switch key {
	case "name":
		c.Name = val
	case "precision":
		c.Prec = val
	default:
		// 其他 commodity metadata 忽略
	}
}

// parseOpen 从 [open, NAME, RESTR?] 取出账户名与币种限制。
func parseOpen(fields []string) (string, string) {
	if len(fields) < 2 {
		return "", ""
	}
	restr := ""
	if len(fields) >= 3 {
		restr = fields[2]
	}
	return fields[1], restr
}

// parseTxnNarration 从交易头剩余字段解析 payee / narration。
// ["\"老板\"", "\"午餐\""] → (老板, 午餐)；["\"早餐\""] → ("", 早餐)。
func parseTxnNarration(rest []string) (payee, narration string) {
	if len(rest) == 0 {
		return "", ""
	}
	first := unquote(rest[0])
	if len(rest) >= 2 {
		return first, unquote(rest[1])
	}
	return "", first
}

// splitDate 把 `YYYY-MM-DD <rest>` 拆成日期与剩余部分。
func splitDate(s string) (string, string, bool) {
	sp := strings.SplitN(s, " ", 2)
	if len(sp) < 2 {
		return "", "", false
	}
	if !isValidDate(sp[0]) {
		return "", "", false
	}
	return sp[0], sp[1], true
}

func isValidDate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}

// isMetadataLine 判断缩进行是否为 metadata（首词以冒号结尾，如 `tags:`）。
func isMetadataLine(trimmed string) bool {
	fields := strings.SplitN(trimmed, " ", 2)
	if len(fields) == 0 {
		return false
	}
	return strings.HasSuffix(fields[0], ":")
}

// splitMeta 拆分 `key: value` 为键与（去引号）值。
func splitMeta(s string) (string, string) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return s, ""
	}
	return strings.TrimSpace(s[:idx]), unquote(strings.TrimSpace(s[idx+1:]))
}

// unquote 去除首尾双引号并处理转义引号。
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return strings.ReplaceAll(s[1:len(s)-1], `\"`, `"`)
	}
	return s
}

// stripComment 去除行内 ` ;` 之后的注释（Beancount 注释前需有空格）。
func stripComment(line string) string {
	if idx := strings.Index(line, " ;"); idx >= 0 {
		return line[:idx]
	}
	return line
}
