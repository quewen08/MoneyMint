/**
 * SyncEngine —— 同步编排（批 5，§6.6）
 *
 * 旧版 `sync_service.dart`（305 行）的重写，并补齐 §6.6.1 五条一致性设计：
 * 1. 失败分类 / 死信：pending 带 attempts / last_error_code / next_retry_at /
 *    state(pending|dead)；`account_closed` / `account_balance_not_zero` /
 *    `opening_via_api_only` / 幂等冲突 → dead；`account_not_found` → 全量 pull +
 *    自然键重映射重试一次，二次失败才 dead；网络与 5xx → 指数退避封顶 1h。
 * 2~3. 原子操作与 uuid_map 崩溃回放在 LocalStore（同事务落库 + push 前回放）。
 * 4. 多标签页租约：IndexedDB CAS，非 leader 直接跳过本次同步。
 * 5. 库世代：pull/push 响应 `instance_id` 与本地不一致 → pending 转隔离区 +
 *    清实体 + 全量重拉；水位只单调推进，空结果（checkpoint=0）不回退。
 *
 * 同步节奏与旧版一致：pull → push（最多两轮，自然键合并重映射）→ pull。
 */
import { ApiError } from '@/api/errors'
import type { SyncPullResponse, PushChange, SyncPushResponse, PushResult } from '@/api/types'

/** push 请求里的一条变更（wire：entity_type / op / entity）。 */
export type { PushChange }
import {
  classifyResult,
  DEAD_CODES,
  type LocalAccount,
  type LocalTxn,
  type PendingChange,
  type PendingEntry,
  type ResultDisposition,
} from './models'
import { backoffMs, type SyncStore } from './localStore'

/** 同步 API 适配层（生产用 `@/api/client` 的 syncApi，测试注入 mock）。 */
export interface SyncApiClient {
  pull(since: number, clientId: string): Promise<SyncPullResponse>
  push(clientId: string, since: number, changes: PushChange[]): Promise<SyncPushResponse>
}

export type SyncOutcome = 'ok' | 'offline' | 'error' | 'skipped'

export type SyncStatus = 'idle' | 'syncing' | 'ok' | 'offline' | 'error'

export interface SyncState {
  status: SyncStatus
  lastSyncAt: number
  pendingCount: number
  deadCount: number
}

export interface SyncEngineOptions {
  /** 租约 TTL（ms），默认 90s（§6.6.1 #4：30s 心跳 / 90s 过期抢占的收敛值）。 */
  leaseTtlMs?: number
  /** push 最大轮数，默认 2（与旧版一致）。 */
  maxPushRounds?: number
  now?: () => number
}

export class SyncEngine {
  private syncing = false
  private listeners = new Set<(state: SyncState) => void>()
  private statusValue: SyncStatus = 'idle'
  private lastSyncAt = 0
  private readonly leaseTtlMs: number
  private readonly maxPushRounds: number
  private readonly nowFn: () => number

  constructor(
    private store: SyncStore,
    private api: SyncApiClient,
    private ledgerIdProvider: () => number | null,
    options: SyncEngineOptions = {},
  ) {
    this.leaseTtlMs = options.leaseTtlMs ?? 90_000
    this.maxPushRounds = options.maxPushRounds ?? 2
    this.nowFn = options.now ?? (() => Date.now())
  }

  /** 订阅状态变化（SyncBadge / 同步页用）。返回取消函数。 */
  subscribe(listener: (state: SyncState) => void): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  private async emit(status: SyncStatus): Promise<void> {
    this.statusValue = status
    const ledgerId = this.ledgerIdProvider()
    const state: SyncState = {
      status,
      lastSyncAt: this.lastSyncAt,
      pendingCount: ledgerId === null ? 0 : await this.store.countPending(ledgerId, 'pending'),
      deadCount: ledgerId === null ? 0 : await this.store.countPending(ledgerId, 'dead'),
    }
    for (const listener of this.listeners) {
      listener(state)
    }
  }

  getState(): SyncState {
    return {
      status: this.statusValue,
      lastSyncAt: this.lastSyncAt,
      pendingCount: 0,
      deadCount: 0,
    }
  }

  /**
   * 离线记账：本地写入 + 入待推队列（原子），随后尽力同步一次。
   * 网络失败不抛错——本地已落库，等下次 sync / online 事件补推（AC-04）。
   */
  async recordOffline(input: { account?: LocalAccount; txn?: LocalTxn; pending: PendingChange }): Promise<void> {
    const ledgerId = this.requireLedgerId()
    await this.store.recordEntry(ledgerId, input)
    await this.emit(this.statusValue)
    // 与旧版一致：写完立即尝试同步（联网即推；断网由 sync 内部兜底为 offline）。
    await this.sync().catch(() => undefined)
  }

  /** 手动 / 自动触发全量同步。并发调用收敛为一次。 */
  async sync(): Promise<SyncOutcome> {
    if (this.syncing) {
      return 'skipped'
    }
    const ledgerId = this.ledgerIdProvider()
    if (ledgerId === null) {
      return 'skipped'
    }
    this.syncing = true
    const clientId = await this.store.getClientId()
    const holder = `sync-${clientId}`
    let leaseHeld = false
    try {
      await this.emit('syncing')
      // §6.6.1 #4：非 leader 跳过（另一个标签页在同步）。
      leaseHeld = await this.store.tryAcquireLease(holder, this.nowFn(), this.leaseTtlMs)
      if (!leaseHeld) {
        await this.emit(this.statusValue === 'syncing' ? 'idle' : this.statusValue)
        return 'skipped'
      }

      // ① pull（增量；库世代不匹配时内部会全量重拉）。
      await this.pullNow(ledgerId, clientId, false)

      // ② push 循环（自然键合并重映射，最多 maxPushRounds 轮）。
      await this.pushLoop(ledgerId, clientId)

      // ③ 再 pull：合并 push 后服务端事件（含他人删除等）。
      await this.pullNow(ledgerId, clientId, false)

      this.lastSyncAt = this.nowFn()
      await this.emit('ok')
      return 'ok'
    } catch (err) {
      // 网络 / 5xx：离线退避（不要求 ApiError 实例，容错任意抛出形态）。
      const status = (err as { status?: unknown }).status
      const code = (err as { code?: unknown }).code
      const isOffline =
        code === 'network_error' || (typeof status === 'number' && status >= 500)
      if (isOffline) {
        await this.backoffAll(ledgerId, err as ApiError)
        await this.emit('offline')
        return 'offline'
      }
      // 401 已由客户端全局登出处理；其余 API 错误标记 error，保留队列。
      await this.emit('error')
      return 'error'
    } finally {
      if (leaseHeld) {
        await this.store.releaseLease(holder).catch(() => undefined)
      }
      this.syncing = false
    }
  }

  /** 退避：网络/5xx 时给所有 pending 项推进 attempts 并设下次重试时间。 */
  private async backoffAll(ledgerId: number, err: ApiError): Promise<void> {
    const entries = await this.store.getPending(ledgerId, 'pending')
    const now = this.nowFn()
    for (const entry of entries) {
      if (entry.next_retry_at > now) continue
      await this.store.scheduleRetry(
        ledgerId,
        entry.id,
        entry.attempts + 1,
        now + backoffMs(entry.attempts + 1),
        err.code ?? 'network_error',
        err.message,
      )
    }
  }

  /**
   * 拉取并应用服务端变更。
   * @param forceFull true 时从 0 重拉（库世代不匹配 / account_not_found 重映射）。
   */
  private async pullNow(ledgerId: number, clientId: string, forceFull: boolean): Promise<void> {
    const lastSeq = forceFull ? 0 : await this.store.getLastSeq(ledgerId)
    const resp = await this.api.pull(lastSeq, clientId)

    // §6.6.1 #5：库世代不匹配 → pending 转隔离区 + 清实体 + 全量重拉。
    const storedInstance = await this.store.getInstanceId(ledgerId)
    if (resp.instance_id !== undefined && resp.instance_id !== '') {
      if (storedInstance === null) {
        await this.store.setInstanceId(ledgerId, resp.instance_id)
      } else if (resp.instance_id !== storedInstance) {
        await this.store.quarantineAndReset(ledgerId, resp.instance_id)
        await this.emit(this.statusValue)
        const full = await this.api.pull(0, clientId)
        await this.store.applyPullChanges(ledgerId, full.changes, full.checkpoint)
        return
      }
    }

    // 水位单调推进：空增量（checkpoint=0）不回退（store 内亦守卫）。
    await this.store.applyPullChanges(ledgerId, resp.changes, resp.checkpoint)
    if (forceFull && resp.checkpoint > 0) {
      await this.store.setInstanceId(ledgerId, resp.instance_id ?? storedInstance ?? '')
    }
  }

  /** push 循环（最多两轮）：收集合并映射 → 重映射 → 重推。 */
  private async pushLoop(ledgerId: number, clientId: string): Promise<void> {
    for (let round = 1; round <= this.maxPushRounds; round += 1) {
      // §6.6.1 #3：push 前回放 uuid_map（崩溃恢复路径）。
      await this.replayUuidMap(ledgerId)

      const now = this.nowFn()
      const all = await this.store.getPending(ledgerId, 'pending')
      const entries = all.filter((e) => e.next_retry_at <= now)
      if (entries.length === 0) return

      const changes: PushChange[] = entries.map((e) => ({
        entity_type: e.change.entity_type,
        op: e.change.op ?? 'create',
        entity: e.change.entity,
      }))
      const lastSeq = await this.store.getLastSeq(ledgerId)
      const resp = await this.api.push(clientId, lastSeq, changes)

      let sawNotFound = false
      const uuidMap: Record<string, string> = {}
      const removedIds: number[] = []
      const deadIds: number[] = []
      const retryIds: number[] = []
      const retryCodes = new Map<number, string>()
      const retryErrors = new Map<number, string>()
      let deadCode = ''
      let deadError = ''

      for (let i = 0; i < resp.results.length; i += 1) {
        const result = resp.results[i]
        const entry = entries[result.index] ?? entries[i]
        if (entry === undefined) continue
        const disposition = this.dispositionOf(result)
        if (disposition === 'merged') {
          // 自然键合并：服务端已有同名账户 → 记映射并**移除本地 pending**（旧版
          // r.ok → removedKeys 口径）；本地账户实体由 applyPushResults 重映射。
          uuidMap[result.uuid ?? ''] = result.server_uuid ?? ''
          removedIds.push(entry.id)
        } else if (disposition === 'removed') {
          removedIds.push(entry.id)
        } else if (disposition === 'dead') {
          deadIds.push(entry.id)
          deadCode = result.code ?? 'dead'
          deadError = result.error ?? '永久失败'
        } else {
          // 临时失败（含 account_not_found）。
          retryIds.push(entry.id)
          retryCodes.set(entry.id, result.code ?? 'error')
          retryErrors.set(entry.id, result.error ?? '')
          if (result.code === 'account_not_found') sawNotFound = true
        }
      }

      // 合并重映射：修正剩余 pending 的 postings 引用（与旧版 _rewritePendingRefs 一致）。
      const removedSet = new Set<number>(removedIds)
      const updates = collectPendingUpdates(entries, removedSet, uuidMap)

      await this.store.applyPushResults(ledgerId, {
        uuidMap,
        removedIds,
        updates,
        checkpoint: resp.checkpoint,
      })
      if (deadIds.length > 0) {
        await this.store.markDead(ledgerId, deadIds, deadCode, deadError)
      }

      // 临时失败处置：account_not_found 全量 pull 后重映射重试一次，二次失败才 dead；
      // 其余退避（§6.6.1 #1）。
      if (retryIds.length > 0) {
        const retrySet = new Set(retryIds)
        const stillPending = await this.store.getPending(ledgerId, 'pending')
        for (const entry of stillPending) {
          if (!retrySet.has(entry.id)) continue
          const code = retryCodes.get(entry.id) ?? 'error'
          const message = retryErrors.get(entry.id) ?? ''
          const nextAttempt = entry.attempts + 1
          if (code === 'account_not_found' && (nextAttempt >= 2 || round >= this.maxPushRounds)) {
            await this.store.markDead(ledgerId, [entry.id], code, message !== '' ? message : '账户重映射重试仍失败')
            continue
          }
          // 还有轮次的重映射失败：记 attempt 但保持立即可推（下一轮重试）；
          // 其他临时失败：指数退避。
          const nextRetryAt = sawNotFound && round < this.maxPushRounds ? this.nowFn() : this.nowFn() + backoffMs(nextAttempt)
          await this.store.scheduleRetry(ledgerId, entry.id, nextAttempt, nextRetryAt, code, message)
        }
      }
      if (sawNotFound && round < this.maxPushRounds) {
        await this.pullNow(ledgerId, clientId, true)
      }

      if (resp.results.every((r) => this.dispositionOf(r) === 'removed' || this.dispositionOf(r) === 'merged')) {
        return
      }
    }
  }

  /** 与 models.classifyResult 同一判定（引擎内复用，避免双口径）。 */
  private dispositionOf(result: PushResult): ResultDisposition {
    return classifyResult(result)
  }

  /** uuid_map 崩溃回放：把映射再应用到 pending 引用（幂等）。 */
  private async replayUuidMap(ledgerId: number): Promise<void> {
    const map = await this.store.getUuidMap(ledgerId)
    const keys = Object.keys(map)
    if (keys.length === 0) return
    const entries = await this.store.getPending(ledgerId, 'pending')
    const updates = collectPendingUpdates(entries, new Set<number>(), map)
    if (updates.length > 0) {
      await this.store.applyPushResults(ledgerId, {
        uuidMap: {},
        removedIds: [],
        updates,
        checkpoint: await this.store.getLastSeq(ledgerId),
      })
    }
  }

  /** 自动同步挂载：online / visibilitychange / 周期定时。返回卸载函数。 */
  startAutoSync(intervalMs = 60_000): () => void {
    const onOnline = (): void => {
      void this.sync().catch(() => undefined)
    }
    const onVisible = (): void => {
      if (typeof document === 'undefined' || document.visibilityState === 'visible') {
        void this.sync().catch(() => undefined)
      }
    }
    if (typeof window !== 'undefined') {
      window.addEventListener('online', onOnline)
      document.addEventListener('visibilitychange', onVisible)
    }
    const timer = setInterval(() => {
      void this.sync().catch(() => undefined)
    }, intervalMs)
    return () => {
      if (typeof window !== 'undefined') {
        window.removeEventListener('online', onOnline)
        document.removeEventListener('visibilitychange', onVisible)
      }
      clearInterval(timer)
    }
  }

  private requireLedgerId(): number {
    const ledgerId = this.ledgerIdProvider()
    if (ledgerId === null || ledgerId <= 0) {
      throw new Error('尚未选择账本，离线写入缺少账本上下文')
    }
    return ledgerId
  }
}

/**
 * 收集需要重写实体引用的 pending（合并重映射 / uuid_map 回放共用）。
 * 与旧版 `_rewritePendingRefs` 一致：只改 transaction 的 postings.account_uuid。
 */
function collectPendingUpdates(
  entries: PendingEntry[],
  removedSet: Set<number>,
  uuidMap: Record<string, string>,
): Array<{ id: number; change: PendingChange }> {
  const updates: Array<{ id: number; change: PendingChange }> = []
  for (const entry of entries) {
    if (removedSet.has(entry.id)) continue
    if (entry.change.entity_type !== 'transaction') continue
    const postings = entry.change.entity['postings']
    if (!Array.isArray(postings)) continue
    let changed = false
    const newPostings = postings.map((p) => {
      const pm = { ...(p as Record<string, unknown>) }
      const accountUuid = pm['account_uuid']
      if (typeof accountUuid === 'string' && uuidMap[accountUuid] !== undefined) {
        pm['account_uuid'] = uuidMap[accountUuid]
        changed = true
      }
      return pm
    })
    if (!changed) continue
    updates.push({
      id: entry.id,
      change: { ...entry.change, entity: { ...entry.change.entity, postings: newPostings } },
    })
  }
  return updates
}

/** 死信码表导出（同步页展示与测试用）。 */
export { DEAD_CODES }
