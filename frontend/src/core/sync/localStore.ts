/**
 * LocalStore —— 离线真相源（批 5，§6.6）
 *
 * 旧版 `local_store.dart`（370 行，sembast_web）的 TypeScript 重写：
 * 五张表（accounts / txns / pending / meta / uuid_map），实体按账本命名空间
 * `key = `${ledgerId}:${uuid}`` 隔离（多账本不串库）。
 *
 * §6.6.1 #2 三原子操作（各自单个 readwrite 事务，跨多 store，崩溃只能落在
 * 事务外、重放幂等）：
 *   - recordEntry      本地写入 + 入队（记一笔不再分三次独立写）
 *   - applyPushResults push 结果落地：uuid_map 落库 + 删 pending + 改引用 + 推水位
 *     （uuid_map 与删 pending 同事务——§6.6.1 #3 崩溃回放的前提）
 *   - applyPullChanges pull 变更应用 + 推水位（同事务）
 *
 * 双实现：IndexedDB（生产，`typeof indexedDB !== 'undefined'`）与内存
 * （jsdom 单测 / 无 IndexedDB 环境降级）。两者行为由同一组测试约束。
 */
import type { LocalAccount, LocalTxn, PendingChange, PendingEntry } from './models'
import type { SyncChange } from '@/api/types'

export const PENDING_BACKOFF_BASE_MS = 30_000
export const PENDING_BACKOFF_MAX_MS = 60 * 60_000

/** 计算退避：指数 30s、60s、120s…封顶 1h（§6.6.1 #1 网络与 5xx）。 */
export function backoffMs(attempts: number): number {
  const ms = PENDING_BACKOFF_BASE_MS * 2 ** Math.max(0, attempts - 1)
  return Math.min(ms, PENDING_BACKOFF_MAX_MS)
}

export interface RecordEntryInput {
  account?: LocalAccount
  txn?: LocalTxn
  pending: PendingChange
}

export interface ApplyPushResultsInput {
  /** 自然键合并：本地 uuid → 服务端 uuid。 */
  uuidMap: Record<string, string>
  /** push 成功、可移除的 pending id。 */
  removedIds: number[]
  /** 需重写实体引用的 pending（合并后修正 postings）。 */
  updates: Array<{ id: number; change: PendingChange }>
  /** 服务端 checkpoint（水位单调推进，空值/回退由引擎判断后传入）。 */
  checkpoint: number
}

export interface SyncStore {
  /** 是否为持久实现（IndexedDB）；内存实现用于测试。 */
  readonly persistent: boolean

  getAllAccounts(ledgerId: number): Promise<LocalAccount[]>
  getAllTxns(ledgerId: number): Promise<LocalTxn[]>
  getAccount(ledgerId: number, uuid: string): Promise<LocalAccount | null>

  getClientId(): Promise<string>
  getLastSeq(ledgerId: number): Promise<number>
  getInstanceId(ledgerId: number): Promise<string | null>
  setInstanceId(ledgerId: number, id: string): Promise<void>

  recordEntry(ledgerId: number, input: RecordEntryInput): Promise<number>
  applyPushResults(ledgerId: number, input: ApplyPushResultsInput): Promise<void>
  applyPullChanges(ledgerId: number, changes: SyncChange[], checkpoint: number): Promise<void>

  getPending(ledgerId: number, state?: 'pending' | 'dead'): Promise<PendingEntry[]>
  countPending(ledgerId: number, state?: 'pending' | 'dead'): Promise<number>
  markDead(ledgerId: number, ids: number[], code: string, error: string): Promise<void>
  scheduleRetry(
    ledgerId: number,
    id: number,
    attempts: number,
    nextRetryAt: number,
    code: string,
    error: string,
  ): Promise<void>
  reviveDead(ledgerId: number, ids: number[]): Promise<void>
  discardPending(ledgerId: number, ids: number[]): Promise<void>

  /**
   * 库世代不匹配（restore 发生）：清该账本实体与水位 → 全量重拉前的落点；
   * pending **转隔离区（state=dead，code=instance_rotated）不静默删**（§6.6.1 #5）。
   */
  quarantineAndReset(ledgerId: number, newInstanceId: string): Promise<void>

  /** uuid_map 回放：返回待重映射表（§6.6.1 #3，push 前与启动时调用）。 */
  getUuidMap(ledgerId: number): Promise<Record<string, string>>

  /**
   * 多标签页租约（§6.6.1 #4）：单记录 CAS。Safari 14.1 无 navigator.locks /
   * BroadcastChannel，用 IndexedDB 单记录比较写入；非 leader 只读本地。
   */
  tryAcquireLease(holder: string, now: number, ttlMs: number): Promise<boolean>
  releaseLease(holder: string): Promise<void>
}

// ---------------------------------------------------------------------------
// 内存实现（测试 / 无 IndexedDB 环境）
// ---------------------------------------------------------------------------

export class MemorySyncStore implements SyncStore {
  readonly persistent = false
  private accounts = new Map<string, LocalAccount>()
  private txns = new Map<string, LocalTxn>()
  private pendings = new Map<number, PendingEntry>()
  private meta = new Map<string, unknown>()
  private uuidMaps = new Map<string, Record<string, string>>()
  private nextId = 1
  private clientId: string | null = null
  private lease: { holder: string; expiresAt: number } | null = null

  private k(ledgerId: number, uuid: string): string {
    return `${ledgerId}:${uuid}`
  }

  async getAllAccounts(ledgerId: number): Promise<LocalAccount[]> {
    const prefix = `${ledgerId}:`
    const out: LocalAccount[] = []
    for (const [key, value] of this.accounts) {
      if (key.startsWith(prefix)) out.push(value)
    }
    return out
  }

  async getAllTxns(ledgerId: number): Promise<LocalTxn[]> {
    const prefix = `${ledgerId}:`
    const out: LocalTxn[] = []
    for (const [key, value] of this.txns) {
      if (key.startsWith(prefix)) out.push(value)
    }
    return out.sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0))
  }

  async getAccount(ledgerId: number, uuid: string): Promise<LocalAccount | null> {
    return this.accounts.get(this.k(ledgerId, uuid)) ?? null
  }

  async getClientId(): Promise<string> {
    if (this.clientId === null) {
      this.clientId = `web-${Date.now()}`
    }
    return this.clientId
  }

  async getLastSeq(ledgerId: number): Promise<number> {
    const v = this.meta.get(`seq.${ledgerId}`)
    return typeof v === 'number' ? v : 0
  }

  async getInstanceId(ledgerId: number): Promise<string | null> {
    const v = this.meta.get(`instance.${ledgerId}`)
    return typeof v === 'string' ? v : null
  }

  async setInstanceId(ledgerId: number, id: string): Promise<void> {
    this.meta.set(`instance.${ledgerId}`, id)
  }

  async recordEntry(ledgerId: number, input: RecordEntryInput): Promise<number> {
    if (input.account !== undefined) {
      this.accounts.set(this.k(ledgerId, input.account.uuid), input.account)
    }
    if (input.txn !== undefined) {
      this.txns.set(this.k(ledgerId, input.txn.uuid), input.txn)
    }
    const id = this.nextId
    this.nextId += 1
    this.pendings.set(id, {
      id,
      state: 'pending',
      attempts: 0,
      next_retry_at: 0,
      change: input.pending,
    })
    return id
  }

  async applyPushResults(ledgerId: number, input: ApplyPushResultsInput): Promise<void> {
    const map = this.uuidMaps.get(String(ledgerId)) ?? {}
    for (const [localUuid, serverUuid] of Object.entries(input.uuidMap)) {
      map[localUuid] = serverUuid
      // 实体重映射：账户 key 换成 server uuid，交易 postings 引用改写。
      const acc = this.accounts.get(this.k(ledgerId, localUuid))
      if (acc !== undefined) {
        this.accounts.delete(this.k(ledgerId, localUuid))
        this.accounts.set(this.k(ledgerId, serverUuid), { ...acc, uuid: serverUuid })
      }
      for (const [key, txn] of this.txns) {
        if (!key.startsWith(`${ledgerId}:`)) continue
        if (!txn.postings.some((p) => p.account_uuid === localUuid)) continue
        this.txns.set(key, {
          ...txn,
          postings: txn.postings.map((p) =>
            p.account_uuid === localUuid ? { ...p, account_uuid: serverUuid } : p,
          ),
        })
      }
    }
    if (Object.keys(map).length > 0) {
      this.uuidMaps.set(String(ledgerId), map)
    }
    for (const id of input.removedIds) {
      this.pendings.delete(id)
    }
    for (const u of input.updates) {
      const cur = this.pendings.get(u.id)
      if (cur !== undefined) {
        this.pendings.set(u.id, { ...cur, change: u.change })
      }
    }
    const cur = await this.getLastSeq(ledgerId)
    if (input.checkpoint > cur) {
      this.meta.set(`seq.${ledgerId}`, input.checkpoint)
    }
  }

  async applyPullChanges(
    ledgerId: number,
    changes: Array<{ seq: number; entity_type: string; op: string; entity: Record<string, unknown> }>,
    checkpoint: number,
  ): Promise<void> {
    for (const c of changes) {
      const uuid = c.entity['uuid']
      if (typeof uuid !== 'string' || uuid === '') continue
      const rawOp = c.op as string | undefined
      const op: string = rawOp === undefined || rawOp === '' ? 'create' : rawOp
      if (op === 'update' || op === 'create') {
        if (c.entity_type === 'account') {
          const prev = this.accounts.get(this.k(ledgerId, uuid))
          this.accounts.set(this.k(ledgerId, uuid), {
            ...(prev ?? {}),
            ...(c.entity as unknown as LocalAccount),
          })
        } else if (c.entity_type === 'transaction') {
          this.txns.set(this.k(ledgerId, uuid), c.entity as unknown as LocalTxn)
        }
      } else if (op === 'close') {
        const prev = this.accounts.get(this.k(ledgerId, uuid))
        if (prev !== undefined) {
          this.accounts.set(this.k(ledgerId, uuid), {
            ...prev,
            close_date: (c.entity['close_date'] as string | undefined) ?? '',
          })
        }
      } else if (op === 'delete') {
        if (c.entity_type === 'transaction') {
          this.txns.delete(this.k(ledgerId, uuid))
        } else if (c.entity_type === 'account') {
          const prev = this.accounts.get(this.k(ledgerId, uuid))
          if (prev !== undefined) {
            // 账户不再 delete（0.4-A 起按 close 兼容）；本地不存在则忽略。
            this.accounts.set(this.k(ledgerId, uuid), {
              ...prev,
              close_date: (c.entity['close_date'] as string | undefined) ?? '',
            })
          }
        }
      }
    }
    const cur = await this.getLastSeq(ledgerId)
    if (checkpoint > cur) {
      this.meta.set(`seq.${ledgerId}`, checkpoint)
    }
  }

  async getPending(ledgerId: number, state?: 'pending' | 'dead'): Promise<PendingEntry[]> {
    const out: PendingEntry[] = []
    for (const entry of this.pendings.values()) {
      if (state === undefined || entry.state === state) out.push(entry)
    }
    return out.sort((a, b) => a.id - b.id)
  }

  async countPending(ledgerId: number, state?: 'pending' | 'dead'): Promise<number> {
    return (await this.getPending(ledgerId, state)).length
  }

  async markDead(ledgerId: number, ids: number[], code: string, error: string): Promise<void> {
    for (const id of ids) {
      const cur = this.pendings.get(id)
      if (cur !== undefined && cur.state === 'pending') {
        this.pendings.set(id, {
          ...cur,
          state: 'dead',
          last_error_code: code,
          last_error: error,
        })
      }
    }
  }

  async scheduleRetry(
    ledgerId: number,
    id: number,
    attempts: number,
    nextRetryAt: number,
    code: string,
    error: string,
  ): Promise<void> {
    const cur = this.pendings.get(id)
    if (cur !== undefined && cur.state === 'pending') {
      this.pendings.set(id, {
        ...cur,
        attempts,
        next_retry_at: nextRetryAt,
        last_error_code: code,
        last_error: error,
      })
    }
  }

  async reviveDead(ledgerId: number, ids: number[]): Promise<void> {
    for (const id of ids) {
      const cur = this.pendings.get(id)
      if (cur !== undefined && cur.state === 'dead') {
        this.pendings.set(id, { ...cur, state: 'pending', attempts: 0, next_retry_at: 0 })
      }
    }
  }

  async discardPending(ledgerId: number, ids: number[]): Promise<void> {
    for (const id of ids) {
      this.pendings.delete(id)
    }
  }

  async quarantineAndReset(ledgerId: number, newInstanceId: string): Promise<void> {
    const prefix = `${ledgerId}:`
    for (const key of [...this.accounts.keys()]) {
      if (key.startsWith(prefix)) this.accounts.delete(key)
    }
    for (const key of [...this.txns.keys()]) {
      if (key.startsWith(prefix)) this.txns.delete(key)
    }
    for (const entry of this.pendings.values()) {
      if (entry.state === 'pending') {
        this.pendings.set(entry.id, {
          ...entry,
          state: 'dead',
          last_error_code: 'instance_rotated',
          last_error: '服务端账本已被还原，本条待同步变更已隔离（同步页可导出后丢弃或重试）',
        })
      }
    }
    this.meta.set(`seq.${ledgerId}`, 0)
    this.meta.set(`instance.${ledgerId}`, newInstanceId)
  }

  async getUuidMap(ledgerId: number): Promise<Record<string, string>> {
    return this.uuidMaps.get(String(ledgerId)) ?? {}
  }

  async tryAcquireLease(holder: string, now: number, ttlMs: number): Promise<boolean> {
    if (this.lease !== null && this.lease.holder !== holder && this.lease.expiresAt > now) {
      return false
    }
    this.lease = { holder, expiresAt: now + ttlMs }
    return true
  }

  async releaseLease(holder: string): Promise<void> {
    if (this.lease !== null && this.lease.holder === holder) {
      this.lease = null
    }
  }
}
