// Package domain 定义跨层共享的领域模型与哨兵错误。
//
// repository（数据访问）、service（业务逻辑）、handler（HTTP 适配）
// 均依赖本包中的类型，避免各层各自定义重复结构体。
package domain

import "time"

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
	OpClose  = "close" // 0.4 起：账户「删除」= Beancount close（置 close_date，保留历史交易）
	OpUpdate = "update" // 0.4-C 起：账户排序等字段更新（不涉及 open/close 语义）
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
	Icon        string // 图标标识（展示用，不影响导出）
	Color       string // 颜色 hex（展示用）
	ParentUUID  string // 父账户 uuid（三级分类结构；空为该层级根）
	SubType     string // 账户子类型（借记账户/信用卡/虚拟账户等，展示用）
	SortOrder   int64  // 用户自定义排序（0.4-C）；同类型内升序排列
}

// Posting 是交易中的一条分录。金额为有符号定点十进制字符串。
type Posting struct {
	AccountID   int64  // 在线记账时引用服务端 id
	AccountUUID string // 离线同步时引用账户 uuid
	AccountName string // 展示用（列表接口内联账户名）
	// AccountDisplay 是账户展示名（「现金」），无展示名时回落为 Beancount 全名。
	// 与 AccountName 分开：导出/校验依赖全名，界面展示用展示名。
	AccountDisplay string
	Commodity      string
	Amount         string
	Position       int
}

// Transaction 是一笔不可变交易。
type Transaction struct {
	ID          int64
	UUID        string
	Date        string
	Flag        string // * | !
	Description string
	CreatedBy   int64      // 创建者用户 id；0 表示无（同步 push 的离线交易无创建者）
	CreatedByName string  // 创建者显示名；由 repository JOIN users 取得，created_by=0 时为空串
	Tags        []string   // 标签（展示/检索/导出 #tag 用）
	Postings    []Posting
	DeletedAt   string // 软删时间（回收站用）；空串表示未删除

	// 期初/冲正元数据（迁移 012）。身份由这些列承载，不使用 tags 判定。
	ReversalOf         string // 被冲正的原交易 uuid（冲正交易上非空）
	OpeningAccountUUID string // 期初交易/其冲正所服务的账户 uuid
	OpeningVersion     int64  // 账户维度期初版本，从 1 递增
	OpeningKey         string // 仅期初交易：opening:{account_uuid}:{version}
	ReversedBy         string // 原期初交易：冲正交易 uuid；非空即失效
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
	// IsAdmin 表示系统管理员（全局角色，非账本级）。
	// 首个注册用户自动成为管理员；全库级操作（备份/下载/恢复）仅管理员可执行。
	IsAdmin bool
	// MustChangePassword 表示当前密码是「别人替我设的」（预设 owner / 管理员重置 /
	// CLI 重置），必须先改密才能使用业务功能。用户改密后清零。
	MustChangePassword bool
}

// Session 是一条登录会话（M3 会话列表与吊销）。
// Token 只在服务端内部流转，绝不下发给前端——对外用 SID（token 的哈希前缀）。
type Session struct {
	Token      string
	UserID     int64
	CreatedAt  string
	ExpiresAt  string
	LastSeenAt string
	UserAgent  string
	IP         string
}

// RateLimitError 表示因失败次数过多被限流，RetryAfter 为建议等待时长。
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return "尝试次数过多，请稍后再试"
}

// Ledger 是账本及其当前用户在其中的角色（多账本 P1-C）。
type Ledger struct {
	ID               int64
	Name             string
	DefaultCommodity string
	Role             string // 当前用户在该账本中的角色：owner | editor | viewer
	OpeningDate      string // 'YYYY-MM-DD'；迁移 012 落库
	CreatedAt        string // 建账时间（UTC，datetime('now')）
}

// 期初方向（FR-05）：持有 / 欠款。
const (
	OpeningDirectionHold = "hold"
	OpeningDirectionOwe  = "owe"
)

// OpeningEquityName 是期初余额对应的权益账户名（固定，不带账本前缀）。
const OpeningEquityName = "Equity:Opening-Balances"

// SyncLogEntry 是 sync_log 中的一条变更事件。
type SyncLogEntry struct {
	Seq        int64
	EntityType string
	EntityUUID string
	Op         string
}
