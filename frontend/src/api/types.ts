/**
 * 后端 REST 契约类型（批 0）
 *
 * 依据 `internal/server/router.go` 的 31 条路由与 `internal/handler/*`、
 * `internal/service/*`、`internal/backup` 的请求/响应字段手写，不使用生成器。
 * 约定：
 * - 金额一律为定点十进制字符串（MoneyString），禁止 number；
 * - 日期为 `YYYY-MM-DD`，时间戳为后端 JSON 序列化结果（RFC3339 / datetime 字符串）；
 * - 服务端自增 id 为 number，跨端引用一律用 uuid。
 */

/** 账本内账户类型（Beancount 五大类）。 */
export type AccountType = 'Assets' | 'Liabilities' | 'Equity' | 'Income' | 'Expenses'

/** 成员角色。 */
export type LedgerRole = 'owner' | 'editor' | 'viewer'

/** 交易标记：* 正常 / ! 待核对。 */
export type TxnFlag = '*' | '!'

/** 同步实体类型。 */
export type SyncEntityType = 'account' | 'transaction' | 'commodity' | 'price'

/** 同步操作：create / delete / close / update。 */
export type SyncOp = 'create' | 'delete' | 'close' | 'update'

/** 期初方向：持有 / 欠款。 */
export type OpeningDirection = 'hold' | 'owe'

/** 期初写入结果状态。 */
export type OpeningStatus = 'created' | 'updated' | 'cleared' | 'unchanged'

/** 定点十进制金额字符串。 */
export type MoneyString = string

/** `YYYY-MM-DD` 日期字符串。 */
export type ISODate = string

/** 后端时间字符串（datetime('now') / RFC3339）。 */
export type ISODateTime = string

/** 账本内账户 uuid。 */
export type AccountUUID = string

/** 交易 uuid。 */
export type TransactionUUID = string

/** 通用 JSON 对象。 */
export type JsonRecord = Record<string, unknown>

// ---------- 健康检查 ----------

/** GET /api/health */
export interface HealthResponse {
  status: string
}

// ---------- 认证 ----------

/** POST /api/auth/register 请求体。 */
export interface RegisterRequest {
  username: string
  password: string
  display_name?: string
}

/** POST /api/auth/login 请求体。 */
export interface LoginRequest {
  username: string
  password: string
}

/** 当前用户信息（role 为默认账本角色，可能为空串）。 */
export interface UserInfo {
  id: number
  username: string
  display_name?: string
  role: LedgerRole | ''
  is_admin: boolean
  must_change_password: boolean
}

/** 注册 / 登录成功响应。 */
export interface AuthResult {
  token: string
  user: UserInfo
}

/** GET /api/auth/me */
export interface MeResponse {
  user: UserInfo
}

/** POST /api/auth/refresh */
export interface RefreshResponse {
  token: string
}

/** POST /api/auth/change-password 请求体。 */
export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

/** POST /api/auth/change-password 响应。 */
export interface ChangePasswordResponse {
  ok: boolean
  token: string
}

/** POST /api/auth/logout 响应（注意 ok 是字符串 "true"，保持后端原样）。 */
export interface LogoutResponse {
  ok: string
}

/** 登录设备列表项（token 绝不下发，id 是可吊销的 SID 前缀）。 */
export interface SessionItem {
  id: string
  created_at: ISODateTime
  last_seen_at: ISODateTime
  expires_at: ISODateTime
  ip: string
  device: string
  current: boolean
}

/** GET /api/auth/sessions */
export interface SessionsResponse {
  sessions: SessionItem[]
}

/** DELETE /api/auth/sessions/{sid} */
export interface RevokeSessionResponse {
  ok: boolean
}

/** POST /api/auth/sessions/revoke-others */
export interface RevokeOtherSessionsResponse {
  ok: boolean
  revoked: number
}

// ---------- 账本 ----------

/** 账本视图（含当前用户角色与期初日期）。 */
export interface LedgerView {
  id: number
  name: string
  default_commodity?: string
  opening_date: ISODate
  role: LedgerRole
}

/** GET /api/ledgers */
export interface LedgerListResponse {
  ledgers: LedgerView[]
}

/** POST /api/ledgers 请求体。 */
export interface CreateLedgerRequest {
  name: string
  default_commodity?: string
}

/** PATCH /api/ledgers/{id} 请求体；字段缺省表示不修改。 */
export interface UpdateLedgerRequest {
  name?: string
  default_commodity?: string
  opening_date?: ISODate
}

// ---------- 账户 ----------

/** 账户列表项（含按币种余额）。 */
export interface AccountView {
  id: number
  name: string
  display_name?: string
  /** 账户 uuid（批 2 起列表响应已透出；期初编辑 / 关户 / 删除依赖）。 */
  uuid: string
  type: AccountType
  open_date: ISODate
  close_date?: ISODate
  commodity_restriction?: string
  icon?: string
  color?: string
  parent_uuid?: AccountUUID
  sub_type?: string
  sort_order: number
  balances?: Record<string, MoneyString>
}

/** 创建账户响应。 */
export interface AccountCreated {
  id: number
  uuid: AccountUUID
  name: string
  display_name?: string
  type: AccountType
  open_date: ISODate
  icon?: string
  color?: string
  parent_uuid?: AccountUUID
  sub_type?: string
}

/** GET /api/accounts */
export interface AccountListResponse {
  accounts: AccountView[]
}

/** POST /api/accounts 请求体。 */
export interface CreateAccountRequest {
  name: string
  display_name?: string
  type: AccountType
  open_date: ISODate
  commodity?: string
  icon?: string
  color?: string
  parent_uuid?: AccountUUID
  sub_type?: string
}

/** DELETE /api/accounts/{uuid}（Beancount close 语义）。 */
export interface OkResponse {
  ok: boolean
}

// ---------- 期初 ----------

/** GET /api/accounts/{uuid}/opening */
export interface OpeningView {
  has_opening: boolean
  account_uuid: AccountUUID
  amount?: MoneyString
  direction?: OpeningDirection
  signed_amount?: MoneyString
  commodity?: string
  version?: number
  date?: ISODate
  transaction_uuid?: TransactionUUID
}

/** POST /api/accounts/{uuid}/opening 请求体（amount 为非负绝对值）。 */
export interface SetOpeningRequest {
  amount: MoneyString
  direction: OpeningDirection
  commodity?: string
  date?: ISODate
}

/** POST /api/accounts/{uuid}/opening 响应。 */
export interface OpeningResult {
  status: OpeningStatus
  account_uuid: AccountUUID
  transaction_uuid?: TransactionUUID
  reversal_uuid?: TransactionUUID
  version: number
  signed_amount: MoneyString
  commodity: string
  date: ISODate
}

// ---------- 恒等式检查 ----------

/** 某币种的恒等式差额。 */
export interface IdentityDiff {
  commodity: string
  sum: MoneyString
}

/** GET /api/identity/check */
export interface IdentityCheckResult {
  balanced: boolean
  diffs: IdentityDiff[]
  posting_count: number
  checked_at: ISODateTime
  duration_ms: number
}

// ---------- 交易 ----------

/** 交易列表中的分录。 */
export interface PostingView {
  account_id: number
  account: string
  commodity: string
  amount: MoneyString
}

/** 交易列表项。 */
export interface TransactionView {
  id: number
  /** 交易 uuid（批 2 起列表响应已透出；软删依赖）。 */
  uuid: string
  date: ISODate
  flag: TxnFlag
  description: string
  created_by_name: string
  /** 标签（迁移 005 起后端已持久化；报表 #tag 下钻用，旧响应可能缺失）。 */
  tags?: string[]
  postings: PostingView[]
}

/** GET /api/transactions */
export interface TransactionListResponse {
  transactions: TransactionView[]
}

/** POST /api/transactions 请求体（在线记账必须用 account_id）。 */
export interface CreateTransactionRequest {
  date: ISODate
  flag?: TxnFlag
  description: string
  tags?: string[]
  postings: Array<{
    account_id: number
    commodity: string
    amount: MoneyString
  }>
}

/** POST /api/transactions 响应。 */
export interface TransactionCreated {
  id: number
  uuid: TransactionUUID
  date: ISODate
  postings: number
}

/** 回收站分录视图（account 为 Beancount 全名，account_display 为展示名）。 */
export interface TrashPostingView {
  account: string
  account_display: string
  commodity: string
  amount: MoneyString
}

/** 回收站条目。 */
export interface TrashItemView {
  uuid: TransactionUUID
  date: ISODate
  flag: TxnFlag
  description: string
  tags: string[]
  deleted_at: ISODateTime
  created_by_name: string
  postings: TrashPostingView[]
}

/** GET /api/transactions/trash */
export interface TrashListResponse {
  transactions: TrashItemView[]
}

/** POST /api/transactions/{uuid}/restore */
export interface RestoreTransactionResponse {
  ok: boolean
}

// ---------- 成员 ----------

/** 成员列表项。 */
export interface MemberView {
  user_id: number
  username: string
  display_name: string
  role: LedgerRole
}

/** GET /api/ledger/members */
export interface MemberListResponse {
  members: MemberView[]
}

/** POST /api/ledger/members 请求体。 */
export interface AddMemberRequest {
  username: string
  role: LedgerRole
}

/** POST /api/ledger/members 响应。 */
export interface AddMemberResponse {
  ok: boolean
  user_id: number
  role: LedgerRole
}

/** POST /api/ledger/members/{userID} 请求体。 */
export interface UpdateMemberRequest {
  role: LedgerRole
}

/** POST /api/ledger/members/{userID} 响应。 */
export interface UpdateMemberResponse {
  ok: boolean
  user_id: number
  role: LedgerRole
}

/** POST /api/ledger/members/{userID}/reset-password 请求体。 */
export interface ResetMemberPasswordRequest {
  new_password: string
}

// ---------- 导出 ----------

/**
 * GET /api/export：`text/plain` 附件（family.beancount）。
 * 前端以 Blob/文本处理，不在此定义结构化响应。
 */
export type ExportResult = Blob

/** GET /api/export/archive：`application/zip` 附件。 */
export type ExportArchiveResult = Blob

// ---------- 同步 ----------

/** 某币种余额差额（关闭账户校验 / push 失败明细）。 */
export interface BalanceDiff {
  commodity: string
  amount: MoneyString
}

/** pull 返回的一条变更。 */
export interface SyncChange {
  seq: number
  entity_type: SyncEntityType
  op: SyncOp
  entity: JsonRecord
}

/** GET /api/sync/pull 查询参数。 */
export interface SyncPullQuery {
  since: number
  client_id: string
}

/** GET /api/sync/pull 响应。 */
export interface SyncPullResponse {
  checkpoint: number
  changes: SyncChange[]
}

/** push 请求中的一条变更。 */
export interface PushChange {
  entity_type: SyncEntityType
  op: SyncOp
  entity: JsonRecord
}

/** POST /api/sync/push 请求体。 */
export interface SyncPushRequest {
  client_id: string
  since: number
  changes: PushChange[]
}

/** push 中单条变更的处理结果。 */
export interface PushResult {
  index: number
  entity_type: SyncEntityType
  uuid?: string
  status: string
  accepted: boolean
  server_uuid?: string
  error?: string
  code?: string
  balances?: BalanceDiff[]
}

/** POST /api/sync/push 响应。 */
export interface SyncPushResponse {
  checkpoint: number
  accepted: number
  results: PushResult[]
}

// ---------- 备份（全库级，仅系统管理员） ----------

/** 备份记录中的恒等式告警。 */
export interface BackupIdentityWarning {
  ledger_id: number
  diffs: IdentityDiff[]
}

/** 备份清单项（manifest.json 的磁盘格式，也是 API 返回结构）。 */
export interface BackupItem {
  name: string
  bean_files?: string[]
  size: number
  sha256: string
  trigger: 'auto' | 'manual'
  created_at: ISODateTime
  duration_ms: number
  stats?: Record<string, number>
  identity_warnings?: BackupIdentityWarning[]
  /** 非空表示该次备份失败（失败记录保留在清单里）。 */
  error?: string
}

/** GET /api/backup/list */
export interface BackupListResponse {
  items: BackupItem[]
}

/** GET /api/backup/download 查询参数。 */
export interface BackupDownloadQuery {
  name: string
}

// ---------- 设置 ----------

/** 注册开关（GET 公开 / POST 仅系统管理员）。 */
export interface RegistrationSettings {
  enabled: boolean
}

/** POST /api/settings/registration 请求体。 */
export interface SetRegistrationRequest {
  enabled: boolean
}
