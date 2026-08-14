// 多文件目录导出：把账本序列化为 Beancount 分文件结构（main.bean + accounts/ + date/）。
//
// 目录结构（解压后可直接对 main.bean 跑 bean-check）：
//
//	main.bean                      # 主入口：commodity/price 声明 + include
//	accounts/assets.bean           # 资产账户 open/close
//	accounts/liabilities.bean      # 负债账户
//	accounts/income.bean           # 收入账户
//	accounts/expenses.bean         # 支出账户
//	accounts/equity.bean           # 权益账户
//	date/{year}/{year}-{MM}.bean   # 按月交易
//	date/{year}/{year}.bean        # 年度入口，include 该年 12 个月
//
// include 均为相对路径，main.bean 在根、date/{year}/{year}.bean 在各自年度目录，
// 路径层级与 Beancount 惯例一致，bean-check 可正确解析。
package export

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// accountTypeFile 把账户类型映射为账户定义文件名（小写）。
var accountTypeFile = map[string]string{
	"Assets":      "assets",
	"Liabilities": "liabilities",
	"Income":      "income",
	"Expenses":    "expenses",
	"Equity":      "equity",
}

var accountTypeOrder = []string{"Assets", "Liabilities", "Equity", "Income", "Expenses"}

// ExportLedgerFiles 将账本导出为多文件目录，返回 map[相对路径]文件内容。
func ExportLedgerFiles(db *sql.DB, ledgerID int64) (map[string][]byte, error) {
	data, err := loadLedgerData(db, ledgerID)
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{}

	// 1) 账户定义文件（五类，每类一个文件）。
	for _, typ := range accountTypeOrder {
		var b strings.Builder
		fmt.Fprintf(&b, "; %s 账户\n", typ)
		for _, a := range data.accountsByType[typ] {
			if a.hasRestr {
				fmt.Fprintf(&b, "%s open %s %s\n", a.openDate, a.name, a.restr)
			} else {
				fmt.Fprintf(&b, "%s open %s\n", a.openDate, a.name)
			}
			if a.hasClose {
				fmt.Fprintf(&b, "%s close %s\n", a.closeDate, a.name)
			}
		}
		if len(data.accountsByType[typ]) > 0 {
			b.WriteString("\n")
		}
		files["accounts/"+accountTypeFile[typ]+".bean"] = []byte(b.String())
	}

	// 2) 按年/月组织交易文件。
	years := make([]int, 0, len(data.txnsByMonth))
	for y := range data.txnsByMonth {
		years = append(years, y)
	}
	sort.Ints(years)

	var yearIncludes []string
	for _, y := range years {
		dir := fmt.Sprintf("date/%d", y)
		var monthIncludes []string
		for m := 1; m <= 12; m++ {
			txns := data.txnsByMonth[y][m]
			fileName := fmt.Sprintf("%d-%02d.bean", y, m)
			var b strings.Builder
			fmt.Fprintf(&b, "; %d 年 %d 月交易\n", y, m)
			for _, t := range txns {
				writeTxnBlock(&b, t, data.precOf)
			}
			files[dir+"/"+fileName] = []byte(b.String())
			monthIncludes = append(monthIncludes, fileName)
		}
		// 年度入口文件 include 该年 12 个月。
		var yb strings.Builder
		fmt.Fprintf(&yb, "; %d 年交易（按月份引入）\n", y)
		for _, mi := range monthIncludes {
			fmt.Fprintf(&yb, "include %q\n", mi)
		}
		files[dir+fmt.Sprintf("/%d.bean", y)] = []byte(yb.String())
		yearIncludes = append(yearIncludes, fmt.Sprintf("date/%d/%d.bean", y, y))
	}

	// 3) main.bean：title + commodity + price + include。
	var main strings.Builder
	main.WriteString("; -*- mode: beancount -*-\n")
	fmt.Fprintf(&main, "; Exported from ledger %q\n\n", data.title)
	fmt.Fprintf(&main, "option \"title\" %q\n\n", data.title)

	// commodity 指令（epoch 日期，保证在任何使用之前声明）。
	syms := make([]string, 0, len(data.used))
	for s := range data.used {
		syms = append(syms, s)
	}
	sort.Strings(syms)
	for _, sym := range syms {
		fmt.Fprintf(&main, "%s commodity %s\n", data.epoch, sym)
		if m, ok := data.comm[sym]; ok {
			fmt.Fprintf(&main, "  name: %q\n", m.Name)
			fmt.Fprintf(&main, "  precision: %q\n", fmt.Sprintf("%d", m.Prec))
		}
	}
	main.WriteString("\n")

	// price 指令。
	for _, p := range data.prices {
		fmt.Fprintf(&main, "%s price %s %s %s\n", p.date, p.commodity, formatAmount(p.rate, data.precOf(p.currency)), p.currency)
	}
	if len(data.prices) > 0 {
		main.WriteString("\n")
	}

	// include 账户定义。
	for _, typ := range accountTypeOrder {
		fmt.Fprintf(&main, "include \"accounts/%s.bean\"\n", accountTypeFile[typ])
	}
	main.WriteString("\n")

	// include 年度文件。
	for _, yi := range yearIncludes {
		fmt.Fprintf(&main, "include %q\n", yi)
	}

	files["main.bean"] = []byte(main.String())
	return files, nil
}

// writeTxnBlock 写一笔交易及其分录（空行分隔）。
func writeTxnBlock(b *strings.Builder, t txn, precOf func(string) int) {
	fmt.Fprintf(b, "%s %s %q\n", t.date, t.flag, t.desc)
	for _, p := range t.postings {
		fmt.Fprintf(b, "  %s %s %s\n", p.account, formatAmount(p.amount, precOf(p.commodity)), p.commodity)
	}
	b.WriteString("\n")
}

// ledgerData 是多文件导出所需的全部数据。
type ledgerData struct {
	title          string
	comm           map[string]*CommodityMeta
	used           map[string]bool
	epoch          string
	accountsByType map[string][]account
	prices         []price
	txnsByMonth    map[int]map[int][]txn // year -> month -> txns
	precOf         func(string) int
}

// loadLedgerData 加载账本数据，与 ExportLedger 共享查询逻辑。
func loadLedgerData(db *sql.DB, ledgerID int64) (*ledgerData, error) {
	d := &ledgerData{
		comm:           map[string]*CommodityMeta{},
		used:           map[string]bool{},
		accountsByType: map[string][]account{},
		prices:         []price{},
		txnsByMonth:    map[int]map[int][]txn{},
	}

	if err := db.QueryRow(`SELECT name FROM ledgers WHERE id=?`, ledgerID).Scan(&d.title); err != nil {
		return nil, err
	}

	// 币种
	rows, err := db.Query(`SELECT symbol, name, "precision" FROM commodities WHERE ledger_id=?`, ledgerID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sym, name string
		var prec int
		if err := rows.Scan(&sym, &name, &prec); err != nil {
			rows.Close()
			return nil, err
		}
		d.comm[sym] = &CommodityMeta{Name: name, Prec: prec, HasMeta: true}
		d.used[sym] = true
	}
	rows.Close()

	// 账户（排除软删）
	arows, err := db.Query(`SELECT name, type, open_date, close_date, commodity_restriction
		FROM accounts WHERE ledger_id=? AND deleted_at IS NULL ORDER BY open_date, name`, ledgerID)
	if err != nil {
		return nil, err
	}
	for arows.Next() {
		var a account
		var closeDate, restr sql.NullString
		if err := arows.Scan(&a.name, &a.typ, &a.openDate, &closeDate, &restr); err != nil {
			arows.Close()
			return nil, err
		}
		if closeDate.Valid {
			a.closeDate, a.hasClose = closeDate.String, true
		}
		if restr.Valid && restr.String != "" {
			a.restr, a.hasRestr = restr.String, true
			d.used[a.restr] = true
		}
		d.accountsByType[a.typ] = append(d.accountsByType[a.typ], a)
	}
	arows.Close()

	// 价格
	prows, err := db.Query(`SELECT commodity, currency, date, CAST(rate AS TEXT)
		FROM prices WHERE ledger_id=? ORDER BY date`, ledgerID)
	if err != nil {
		return nil, err
	}
	for prows.Next() {
		var p price
		if err := prows.Scan(&p.commodity, &p.currency, &p.date, &p.rate); err != nil {
			prows.Close()
			return nil, err
		}
		d.prices = append(d.prices, p)
		d.used[p.commodity], d.used[p.currency] = true, true
	}
	prows.Close()

	// 交易 + 分录
	trows, err := db.Query(`SELECT id, date, flag, description
		FROM transactions WHERE ledger_id=? AND deleted_at IS NULL ORDER BY date, id`, ledgerID)
	if err != nil {
		return nil, err
	}
	var txns []txn
	for trows.Next() {
		var t txn
		if err := trows.Scan(&t.id, &t.date, &t.flag, &t.desc); err != nil {
			trows.Close()
			return nil, err
		}
		txns = append(txns, t)
	}
	trows.Close()

	for i := range txns {
		posts, err := db.Query(`SELECT a.name, p.commodity, CAST(p.amount AS TEXT), p.position
			FROM postings p JOIN accounts a ON a.id=p.account_id
			WHERE p.transaction_id=? ORDER BY p.position`, txns[i].id)
		if err != nil {
			return nil, err
		}
		for posts.Next() {
			var p posting
			var pos int
			if err := posts.Scan(&p.account, &p.commodity, &p.amount, &pos); err != nil {
				posts.Close()
				return nil, err
			}
			txns[i].postings = append(txns[i].postings, p)
			d.used[p.commodity] = true
		}
		posts.Close()

		year, month := parseYearMonth(txns[i].date)
		if d.txnsByMonth[year] == nil {
			d.txnsByMonth[year] = map[int][]txn{}
		}
		d.txnsByMonth[year][month] = append(d.txnsByMonth[year][month], txns[i])
	}

	// epoch = 所有日期的最小值
	epoch := ""
	for y := range d.txnsByMonth {
		for _, ms := range d.txnsByMonth[y] {
			for _, t := range ms {
				if epoch == "" || t.date < epoch {
					epoch = t.date
				}
			}
		}
	}
	for _, as := range d.accountsByType {
		for _, a := range as {
			if epoch == "" || a.openDate < epoch {
				epoch = a.openDate
			}
		}
	}
	for _, p := range d.prices {
		if epoch == "" || p.date < epoch {
			epoch = p.date
		}
	}
	if epoch == "" {
		epoch = "2000-01-01"
	}
	d.epoch = epoch

	d.precOf = func(sym string) int {
		if m, ok := d.comm[sym]; ok {
			return m.Prec
		}
		return 2
	}
	return d, nil
}

// parseYearMonth 解析 "YYYY-MM-DD" 为年、月（解析失败返回 0,0）。
func parseYearMonth(date string) (int, int) {
	if len(date) < 7 {
		return 0, 0
	}
	var y, m int
	if _, err := fmt.Sscanf(date, "%d-%d", &y, &m); err != nil {
		return 0, 0
	}
	return y, m
}
