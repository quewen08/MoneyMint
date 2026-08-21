// Package seed 定义建账时自动创建的默认账户清单。
//
// 命名约定（核心约束：导出 .beancount 必须过 bean-check）：
//   - Name 必须是 Beancount 合法账户名（ASCII，首字母大写，用 ':' 分层），
//     即 Assets / Liabilities / Equity / Income / Expenses 五大根之一。
//   - DisplayName 是给前端展示的中文名，不参与导出。
// 建账（service.CreateLedger / 注册首个账本）成功后调用 repository.SeedDefaultAccounts
// 逐条插入；同名已存在则跳过，幂等可重复执行。
package seed

import "familyledger/internal/domain"

// DefaultAccount 是建账时自动创建的默认账户模板。
type DefaultAccount struct {
	Name        string // Beancount ASCII 账户名
	DisplayName string // 中文显示名
	Type        string // domain.Type* 之一
	Icon        string // 图标（emoji，展示用；不影响导出）
	Color       string // 颜色 hex（展示用）
}

// DefaultAccounts 是默认账户清单，按 Beancount 五大类分组。
// 来源：用户提供的初始收支/资产负债账户 + 权益「期初余额」账户。
// Icon/Color 为展示默认值，UI 也可被用户后续覆盖；导出 .beancount 忽略这两个字段。
var DefaultAccounts = []DefaultAccount{
	// ---- 支出 (Expenses) ----
	{Name: "Expenses:Food", DisplayName: "餐饮", Type: domain.TypeExpenses, Icon: "🍜", Color: "#E8590C"},
	{Name: "Expenses:Groceries", DisplayName: "食品", Type: domain.TypeExpenses, Icon: "🛒", Color: "#2F9E44"},
	{Name: "Expenses:Transport", DisplayName: "交通", Type: domain.TypeExpenses, Icon: "🚌", Color: "#1971C2"},
	{Name: "Expenses:Shopping", DisplayName: "购物", Type: domain.TypeExpenses, Icon: "🛍️", Color: "#9C36B5"},
	{Name: "Expenses:Entertainment", DisplayName: "娱乐", Type: domain.TypeExpenses, Icon: "🎮", Color: "#E64980"},
	{Name: "Expenses:Medical", DisplayName: "医疗", Type: domain.TypeExpenses, Icon: "💊", Color: "#0CA678"},
	{Name: "Expenses:Housing", DisplayName: "住房", Type: domain.TypeExpenses, Icon: "🏠", Color: "#5C7CFA"},
	{Name: "Expenses:Child", DisplayName: "孩子", Type: domain.TypeExpenses, Icon: "🍼", Color: "#F59F00"},
	{Name: "Expenses:Communication", DisplayName: "通讯", Type: domain.TypeExpenses, Icon: "📱", Color: "#15AABF"},
	{Name: "Expenses:Clothing", DisplayName: "服饰", Type: domain.TypeExpenses, Icon: "👕", Color: "#BE4BDB"},
	{Name: "Expenses:Beauty", DisplayName: "美容", Type: domain.TypeExpenses, Icon: "💄", Color: "#F06595"},
	{Name: "Expenses:Education", DisplayName: "教育", Type: domain.TypeExpenses, Icon: "📚", Color: "#4263EB"},
	{Name: "Expenses:Sports", DisplayName: "运动", Type: domain.TypeExpenses, Icon: "⚽", Color: "#2B8A3E"},
	{Name: "Expenses:Travel", DisplayName: "旅行", Type: domain.TypeExpenses, Icon: "✈️", Color: "#1098AD"},
	{Name: "Expenses:Digital", DisplayName: "数码", Type: domain.TypeExpenses, Icon: "💻", Color: "#495057"},
	{Name: "Expenses:Gift", DisplayName: "礼物", Type: domain.TypeExpenses, Icon: "🎁", Color: "#D6336C"},
	{Name: "Expenses:Office", DisplayName: "办公", Type: domain.TypeExpenses, Icon: "📎", Color: "#868E96"},
	{Name: "Expenses:Other", DisplayName: "其他", Type: domain.TypeExpenses, Icon: "💸", Color: "#868E96"},

	// ---- 收入 (Income) ----
	{Name: "Income:Salary", DisplayName: "薪资", Type: domain.TypeIncome, Icon: "💰", Color: "#2F9E44"},
	{Name: "Income:Bonus", DisplayName: "奖金", Type: domain.TypeIncome, Icon: "🧧", Color: "#E03131"},
	{Name: "Income:Investment", DisplayName: "投资收益", Type: domain.TypeIncome, Icon: "📈", Color: "#2B8A3E"},
	{Name: "Income:PartTime", DisplayName: "兼职", Type: domain.TypeIncome, Icon: "🛠️", Color: "#1971C2"},
	{Name: "Income:Rent", DisplayName: "租金", Type: domain.TypeIncome, Icon: "🏘️", Color: "#5C7CFA"},
	{Name: "Income:Interest", DisplayName: "利息", Type: domain.TypeIncome, Icon: "🏦", Color: "#0CA678"},
	{Name: "Income:Refund", DisplayName: "退款", Type: domain.TypeIncome, Icon: "↩️", Color: "#15AABF"},
	{Name: "Income:Gift", DisplayName: "礼金", Type: domain.TypeIncome, Icon: "🎉", Color: "#D6336C"},
	{Name: "Income:Other", DisplayName: "其他", Type: domain.TypeIncome, Icon: "➕", Color: "#868E96"},

	// ---- 资产 (Assets) ----
	{Name: "Assets:Cash", DisplayName: "现金", Type: domain.TypeAssets, Icon: "💵", Color: "#F59F00"},
	{Name: "Assets:RealEstate", DisplayName: "房产", Type: domain.TypeAssets, Icon: "🏢", Color: "#5C7CFA"},
	{Name: "Assets:Alipay", DisplayName: "支付宝", Type: domain.TypeAssets, Icon: "🔵", Color: "#1971C2"},
	{Name: "Assets:WeChat", DisplayName: "微信", Type: domain.TypeAssets, Icon: "🟢", Color: "#2F9E44"},

	// ---- 负债 (Liabilities) ----
	{Name: "Liabilities:Mortgage", DisplayName: "房贷", Type: domain.TypeLiabilities, Icon: "🏦", Color: "#E8590C"},
	{Name: "Liabilities:CreditCard", DisplayName: "信用卡", Type: domain.TypeLiabilities, Icon: "💳", Color: "#D6336C"},
	{Name: "Liabilities:Other", DisplayName: "其他", Type: domain.TypeLiabilities, Icon: "📄", Color: "#868E96"},

	// ---- 权益 (Equity) ----
	// 期初余额账户：录入初始资产/负债时与本科目对冲，使借贷平衡。
	{Name: "Equity:Opening-Balances", DisplayName: "期初余额", Type: domain.TypeEquity, Icon: "⚖️", Color: "#868E96"},
}
