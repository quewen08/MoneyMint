// Package domain 定义跨层共享的领域模型与哨兵错误。
//
// repository（数据访问）、service（业务逻辑）、handler（HTTP 适配）
// 均依赖本包中的类型，避免各层各自定义重复结构体。
package domain

// DefaultLedgerID 是首个注册用户创建的首个账本 id（AUTOINCREMENT 自 1 起），
// 也作为请求未携带 X-Ledger-Id 时的回退账本，保证旧脚本/旧客户端向前兼容。
// 多账本隔离（P1-C）落地后，业务账本一律从请求上下文（X-Ledger-Id）解析。
const DefaultLedgerID int64 = 1

// 账户类型（Beancount 五大类）。
const (
	TypeAssets      = "Assets"
	TypeLiabilities = "Liabilities"
	TypeEquity      = "Equity"
	TypeIncome      = "Income"
	TypeExpenses    = "Expenses"
)

// 成员角色。
const (
	RoleOwner  = "owner"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

// 同步实体类型与操作。
const (
	EntityAccount     = "account"
	EntityTransaction = "transaction"
	EntityCommodity   = "commodity"
	EntityPrice       = "price"

	OpCreate = "create"
	OpDelete = "delete"
)

// Account 是账本内的一个账户（Beancount open 指令实体）。
type Account struct {
	ID          int64
	UUID        string
	Name        string // Beancount 账户名（ASCII），如 Assets:Cash:CNY
	DisplayName string // 中文显示名（可选），如 现金；导出只用 Name
	Type        string
	OpenDate    string
	CloseDate   string // 空表示未关闭
	Restriction string // commodity_restriction，空表示无
}

// Posting 是交易中的一条分录。金额为有符号定点十进制字符串。
type Posting struct {
	AccountID   int64  // 在线记账时引用服务端 id
	AccountUUID string // 离线同步时引用账户 uuid
	AccountName string // 展示用（列表接口内联账户名）
	Commodity   string
	Amount      string
	Position    int
}

// Transaction 是一笔不可变交易。
type Transaction struct {
	ID          int64
	UUID        string
	Date        string
	Flag        string // * | !
	Description string
	CreatedBy   int64 // 创建者用户 id；0 表示无（同步 push 的离线交易无创建者）
	Postings    []Posting
}

// Commodity 是账本内的币种/商品（Beancount commodity 指令）。
type Commodity struct {
	UUID      string
	Symbol    string
	Name      string
	Precision int
}

// Price 是一条价格记录（Beancount price 指令）。
type Price struct {
	Commodity string
	Currency  string
	Date      string
	Rate      string
	Source    string
}

// Member 是账本成员及其角色。
type Member struct {
	UserID      int64
	Username    string
	DisplayName string
	Role        string
}

// User 是账号信息。
type User struct {
	ID           int64
	Username     string
	DisplayName  string
	PasswordHash string
}

// Ledger 是账本及其当前用户在其中的角色（多账本 P1-C）。
type Ledger struct {
	ID               int64
	Name             string
	DefaultCommodity string
	Role             string // 当前用户在该账本中的角色：owner | editor | viewer
}

// SyncLogEntry 是 sync_log 中的一条变更事件。
type SyncLogEntry struct {
	Seq        int64
	EntityType string
	EntityUUID string
	Op         string
}
