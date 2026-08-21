// Package export 将数据库中的账本序列化为合法 Beancount 文本。
//
// 设计约束（已用 bean-check 验证通过）：
//   - 账户必须有 open 指令，且 open_date 早于该账户首笔交易；
//   - 所有出现过的币种都要有 commodity 指令（epoch 日期，保证在一切使用之前声明）；
//   - 同一交易下，按币种分组的金额之和必须为 0（借贷平衡）；
//   - 金额一律以定点十进制字符串写出，避免浮点误差（DB 中请用最小单位整数或 decimal 字符串存储）。
//
// 与 export_demo/seed_and_export.py 为同一套序列化逻辑。
package export

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// CommodityMeta 描述一个币种/商品在导出时的元数据。
type CommodityMeta struct {
	Name    string
	Prec    int
	HasMeta bool
}

type account struct {
	name, typ, openDate, closeDate, restr string
	hasClose, hasRestr                    bool
}

type price struct {
	commodity, currency, date, rate string
}

type posting struct {
	account, commodity, amount string
}

type txn struct {
	id               int64
	date, flag, desc string
	tags             []string
	postings         []posting
}

// ExportLedger 将指定账本导出为合法 Beancount 文本。
func ExportLedger(db *sql.DB, ledgerID int64) (string, error) {
	// 1. 币种元数据
	comm := map[string]*CommodityMeta{}
	rows, err := db.Query(`SELECT symbol, name, "precision" FROM commodities WHERE ledger_id=?`, ledgerID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var sym, name string
		var prec int
		if err := rows.Scan(&sym, &name, &prec); err != nil {
			rows.Close()
			return "", err
		}
		comm[sym] = &CommodityMeta{Name: name, Prec: prec, HasMeta: true}
	}
	rows.Close()

	// 2. 账户（排除软删账户）
	var accounts []account
	arows, err := db.Query(`SELECT name, type, open_date, close_date, commodity_restriction
		FROM accounts WHERE ledger_id=? AND deleted_at IS NULL ORDER BY open_date, name`, ledgerID)
	if err != nil {
		return "", err
	}
	for arows.Next() {
		var a account
		var closeDate, restr sql.NullString
		if err := arows.Scan(&a.name, &a.typ, &a.openDate, &closeDate, &restr); err != nil {
			arows.Close()
			return "", err
		}
		if closeDate.Valid {
			a.closeDate, a.hasClose = closeDate.String, true
		}
		if restr.Valid && restr.String != "" {
			a.restr, a.hasRestr = restr.String, true
		}
		accounts = append(accounts, a)
	}
	arows.Close()

	// 3. 价格
	var prices []price
	prows, err := db.Query(`SELECT commodity, currency, date, CAST(rate AS TEXT)
		FROM prices WHERE ledger_id=? ORDER BY date`, ledgerID)
	if err != nil {
		return "", err
	}
	for prows.Next() {
		var p price
		if err := prows.Scan(&p.commodity, &p.currency, &p.date, &p.rate); err != nil {
			prows.Close()
			return "", err
		}
		prices = append(prices, p)
	}
	prows.Close()

	// 4. 交易 + 分录（排除软删交易）
	var txns []txn
	trows, err := db.Query(`SELECT id, date, flag, description, tags
		FROM transactions WHERE ledger_id=? AND deleted_at IS NULL ORDER BY date, id`, ledgerID)
	if err != nil {
		return "", err
	}
	for trows.Next() {
		var t txn
		var tags sql.NullString
		if err := trows.Scan(&t.id, &t.date, &t.flag, &t.desc, &tags); err != nil {
			trows.Close()
			return "", err
		}
		if tags.Valid && tags.String != "" {
			var parsed []string
			if jsonErr := json.Unmarshal([]byte(tags.String), &parsed); jsonErr == nil {
				t.tags = parsed
			}
		}
		// 先收集交易，postings 在 trows 关闭后再查，
		// 避免在单连接(MaxOpenConns=1)下同时持有两结果集导致死锁。
		txns = append(txns, t)
	}
	trows.Close()

	// 逐笔交易查询其 posting（此时 trows 已关闭，连接可用）。
	for i := range txns {
		prows2, err := db.Query(`SELECT a.name, p.commodity, CAST(p.amount AS TEXT), p.position
			FROM postings p JOIN accounts a ON a.id=p.account_id
			WHERE p.transaction_id=? ORDER BY p.position`, txns[i].id)
		if err != nil {
			return "", err
		}
		for prows2.Next() {
			var p posting
			var pos int
			if err := prows2.Scan(&p.account, &p.commodity, &p.amount, &pos); err != nil {
				prows2.Close()
				return "", err
			}
			txns[i].postings = append(txns[i].postings, p)
		}
		prows2.Close()
	}

	// 收集所有出现过的币种
	used := map[string]bool{}
	for s := range comm {
		used[s] = true
	}
	for _, a := range accounts {
		if a.hasRestr {
			used[a.restr] = true
		}
	}
	for _, p := range prices {
		used[p.commodity], used[p.currency] = true, true
	}
	for _, t := range txns {
		for _, p := range t.postings {
			used[p.commodity] = true
		}
	}

	// epoch = 所有日期的最小值（保证 commodity 在任何使用之前声明）
	epoch := ""
	for _, d := range append(append(datesOfAccounts(accounts), datesOfPrices(prices)...), datesOfTxns(txns)...) {
		if epoch == "" || d < epoch {
			epoch = d
		}
	}
	if epoch == "" {
		epoch = "2000-01-01"
	}

	precOf := func(sym string) int {
		if m, ok := comm[sym]; ok {
			return m.Prec
		}
		return 2
	}

	var b strings.Builder

	var title string
	if err := db.QueryRow(`SELECT name FROM ledgers WHERE id=?`, ledgerID).Scan(&title); err != nil {
		return "", err
	}
	b.WriteString("; -*- mode: beancount -*-\n")
	fmt.Fprintf(&b, "; Exported from ledger %q\n\n", title)
	fmt.Fprintf(&b, "option \"title\" %q\n\n", title)

	// 1) commodity 指令（按符号排序）
	syms := make([]string, 0, len(used))
	for s := range used {
		syms = append(syms, s)
	}
	sort.Strings(syms)
	for _, sym := range syms {
		fmt.Fprintf(&b, "%s commodity %s\n", epoch, sym)
		if m, ok := comm[sym]; ok {
			fmt.Fprintf(&b, "  name: %q\n", m.Name)
			fmt.Fprintf(&b, "  precision: %q\n", fmt.Sprintf("%d", m.Prec))
		}
	}
	b.WriteString("\n")

	// 2) open / close（按账户遍历：open 紧跟其 close）
	//    支持 Beancount close 后再 open 同名账户：每个账户的 close 紧随其 open 之后输出，
	//    避免多个 open 堆叠导致 duplicate open 指令（0.4-A close 语义）。
	sort.SliceStable(accounts, func(i, j int) bool {
		if accounts[i].openDate != accounts[j].openDate {
			return accounts[i].openDate < accounts[j].openDate
		}
		return accounts[i].name < accounts[j].name
	})
	for _, a := range accounts {
		if a.hasRestr {
			fmt.Fprintf(&b, "%s open %s %s\n", a.openDate, a.name, a.restr)
		} else {
			fmt.Fprintf(&b, "%s open %s\n", a.openDate, a.name)
		}
		if a.hasClose {
			fmt.Fprintf(&b, "%s close %s\n", a.closeDate, a.name)
		}
	}
	b.WriteString("\n")

	// 3) price
	for _, p := range prices {
		fmt.Fprintf(&b, "%s price %s %s %s\n", p.date, p.commodity, formatAmount(p.rate, precOf(p.currency)), p.currency)
	}
	b.WriteString("\n")

	// 4) 交易
	for _, t := range txns {
		fmt.Fprintf(&b, "%s %s %q\n", t.date, t.flag, t.desc)
		// 标签以元数据形式写出：Beancount 的 #tag 语法仅允许 ASCII，
		// 而本应用需要支持中文标签，故统一用 `tags: "..."` 元数据行，
		// 既可保留中文又能通过 bean-check。
		if joined := joinTags(t.tags); joined != "" {
			fmt.Fprintf(&b, "  tags: %q\n", joined)
		}
		for _, p := range t.postings {
			fmt.Fprintf(&b, "  %s %s %s\n", p.account, formatAmount(p.amount, precOf(p.commodity)), p.commodity)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func datesOfAccounts(accounts []account) []string {
	out := make([]string, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, a.openDate)
	}
	return out
}

func datesOfPrices(prices []price) []string {
	out := make([]string, 0, len(prices))
	for _, p := range prices {
		out = append(out, p.date)
	}
	return out
}

func datesOfTxns(txns []txn) []string {
	out := make([]string, 0, len(txns))
	for _, t := range txns {
		out = append(out, t.date)
	}
	return out
}

// formatAmount 将数据库中的数值字符串按指定精度格式化为定点十进制。
func formatAmount(s string, prec int) string {
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return s
	}
	return r.FloatString(prec)
}

// joinTags 把标签切片清洗后用空格拼接成一个字符串（用于导出为 Beancount 的
// `tags: "..."` 元数据值）。空/纯空白标签被忽略；首尾空白被裁掉。无有效标签返回空串。
func joinTags(tags []string) string {
	clean := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		clean = append(clean, t)
	}
	return strings.Join(clean, " ")
}
