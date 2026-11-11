/**
 * 同步内核本地模型（批 5，§6.6）
 *
 * 与旧版 `sync_models.dart` / `local_store.dart` 行为逐条对齐：
 * - 本地实体以 uuid 为主键，账户/交易/待推队列/水位/uuid 映射五张表；
 * - 本地生成 uuid 用 `loc-` 前缀（旧版口径），push 后按 server_uuid 重映射；
 * - pending 携带死信字段（§6.6.1 #1）：attempts / last_error_code / next_retry_at /
 *   state(pending|dead)——旧版没有这些字段导致 4xx 无限重试，本版补齐。
 *
 * wire 结构（SyncChange / PullResult / PushResult / PushRequest）复用
 * `@/api/types`，与 31 端点契约一致，不重复定义。
 */
import type { SyncChange } from '@/api/types'

/** 本地分录：引用账户 uuid（本地 `loc-*` 或服务端 uuid，push 前重映射）。 */
export interface LocalPosting {
  account_uuid: string
  commodity: string
  amount: string
}

/** 本地账户（服务端 pull 实体 / 本地创建共用形态，字段名与 wire 一致）。 */
export interface LocalAccount {
  uuid: string
  name: string
  display_name?: string
  type: string
  open_date: string
  close_date?: string
  commodity_restriction?: string
  icon?: string
  color?: string
  parent_uuid?: string
  sub_type?: string
  sort_order?: number
}

/** 本地交易。 */
export interface LocalTxn {
  uuid: string
  date: string
  flag: string
  description: string
  tags: string[]
  reversed_of?: string
  created_by_name?: string
  postings: LocalPosting[]
}

/**
 * 待推送变更。op 省略 = create（旧版口径）。
 * state：pending（待推 / 退避中）| dead（永久失败，同步页可见可丢弃）。
 */
export interface PendingChange {
  entity_type: 'account' | 'transaction' | 'commodity'
  op?: 'create' | 'delete' | 'close' | 'update'
  key: string
  entity: Record<string, unknown>
}

/** 待推送条目（含死信元数据；id 由存储层分配，单调递增保序）。 */
export interface PendingEntry {
  id: number
  state: 'pending' | 'dead'
  attempts: number
  last_error_code?: string
  last_error?: string
  /** 下次重试时间戳（ms）；0 表示立即可推。 */
  next_retry_at: number
  change: PendingChange
}

/** push 逐条结果（对齐 wire `results[]`；status 为字符串以兼容宽松解析）。 */
export interface SyncPushResultItem {
  index: number
  entity_type: string
  uuid?: string
  status: string
  accepted: boolean
  server_uuid?: string
  error?: string
  code?: string
  balances?: Array<{ commodity: string; amount: string }>
}

/**
 * 逐条结果分类（§6.6.1 #1 死信规则）：
 * - merged：自然键合并（server_uuid ≠ 本地 uuid）→ 重映射后重推；
 * - dead：永久失败码 → 转 dead，同步页可见；
 * - removed：成功 → 移除 pending；
 * - retry：临时失败 → 退避重试。
 * 码表与后端 `internal/domain/errors.go` 同源（契约测试断言）。
 */
export type ResultDisposition = 'removed' | 'merged' | 'dead' | 'retry'

/** 永久失败码（→ dead）。与后端稳定码白名单一致。 */
export const DEAD_CODES: readonly string[] = [
  'account_closed',
  'account_balance_not_zero',
  'opening_via_api_only',
  'idempotency_key_deleted',
  'idempotency_payload_mismatch',
  'validation',
]

/** 可重映射后重试的码（→ 全量 pull + 自然键重映射 + 重试一次）。 */
export const REMAP_RETRY_CODES: readonly string[] = ['account_not_found']

/** 按逐条结果分类（纯函数，便于契约测试）。 */
export function classifyResult(item: SyncPushResultItem): ResultDisposition {
  if (item.status !== 'ok') {
    if (item.code !== undefined && DEAD_CODES.includes(item.code)) return 'dead'
    if (item.code !== undefined && REMAP_RETRY_CODES.includes(item.code)) return 'retry'
    return 'retry' // 未知码按临时失败退避，不静默丢数据（RK-05）
  }
  if (
    item.server_uuid !== undefined &&
    item.server_uuid !== '' &&
    item.uuid !== '' &&
    item.server_uuid !== item.uuid
  ) {
    return 'merged'
  }
  return 'removed'
}

/** pull 变更的应用语义（对齐 local_store.dart `applyChange`；wire 省略 op = create）。 */
export type PullOp = 'create' | 'delete' | 'close' | 'update'

export function applyOpOf(change: SyncChange): PullOp {
  const raw = change.op as string | undefined
  const op = raw === undefined || raw === '' ? 'create' : raw
  return op as PullOp
}
