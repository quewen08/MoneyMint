/**
 * 同步状态 store（批 5）
 *
 * 包装 SyncEngine 给组件用：状态徽标（SyncBadge）、同步页（P-18）、
 * 记一笔的离线兜底都走这里。引擎是单例；账本 id 复用 client 的
 * getLedgerId()（与 session 同源），切换账本后首次 sync 自动按新账本拉取。
 */
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { getLedgerId, syncApi } from '@/api/client'
import { SyncEngine, type SyncState, type SyncStatus } from '@/core/sync/engine'
import { MemorySyncStore, type SyncStore } from '@/core/sync/localStore'
import { IdbSyncStore } from '@/core/sync/idbStore'
import type { LocalTxn, PendingChange, PendingEntry } from '@/core/sync/models'

let engineInstance: SyncEngine | null = null
let storeInstance: SyncStore | null = null
let stopAutoSync: (() => void) | null = null

/** 取（或创建）底层存储：优先 IndexedDB，无则内存降级。 */
async function ensureStore(): Promise<SyncStore> {
  if (storeInstance !== null) return storeInstance
  if (typeof indexedDB !== 'undefined') {
    try {
      storeInstance = await IdbSyncStore.open()
      return storeInstance
    } catch {
      // 打开失败（隐私模式 / 配额）：降级内存，不阻塞记账。
    }
  }
  storeInstance = new MemorySyncStore()
  return storeInstance
}

export const useSyncStore = defineStore('sync', () => {
  const status = ref<SyncStatus>('idle')
  const lastSyncAt = ref(0)
  const pendingCount = ref(0)
  const deadCount = ref(0)
  const deadItems = ref<PendingEntry[]>([])
  const initialized = ref(false)

  const hasBacklog = computed(() => pendingCount.value + deadCount.value > 0)

  async function ensureEngine(): Promise<SyncEngine> {
    if (engineInstance !== null) return engineInstance
    const store = await ensureStore()
    const engine = new SyncEngine(store, syncApi, () => getLedgerId())
    engine.subscribe((state: SyncState) => {
      status.value = state.status
      lastSyncAt.value = state.lastSyncAt
      pendingCount.value = state.pendingCount
      deadCount.value = state.deadCount
    })
    engineInstance = engine
    return engine
  }

  /** 应用启动时调用：建引擎、订阅状态、挂自动同步。 */
  async function bootstrap(): Promise<void> {
    if (initialized.value) return
    initialized.value = true
    const engine = await ensureEngine()
    stopAutoSync = engine.startAutoSync()
    void engine.sync().catch(() => undefined)
  }

  /** 手动触发同步（同步页按钮）。 */
  async function syncNow(): Promise<'ok' | 'offline' | 'error' | 'skipped'> {
    const engine = await ensureEngine()
    const outcome = await engine.sync()
    await loadQueues()
    return outcome
  }

  /** 刷新队列计数与死信列表（同步页 / 徽标）。 */
  async function loadQueues(): Promise<void> {
    await ensureEngine()
    if (storeInstance === null) return
    const ledgerId = getLedgerId()
    if (ledgerId === null) {
      deadItems.value = []
      pendingCount.value = 0
      deadCount.value = 0
      return
    }
    deadItems.value = await storeInstance.getPending(ledgerId, 'dead')
    pendingCount.value = await storeInstance.countPending(ledgerId, 'pending')
    deadCount.value = deadItems.value.length
  }

  /** 离线兜底：本地落一笔交易并入队（AC-04）。uuid 为本地 `loc-*` 生成。 */
  async function recordTxnOffline(txn: LocalTxn, pending: PendingChange): Promise<void> {
    const engine = await ensureEngine()
    await engine.recordOffline({ txn, pending })
    await loadQueues()
  }

  /** 死信：重试（state 回 pending、清退避）。 */
  async function reviveDead(ids: number[]): Promise<void> {
    if (storeInstance === null) return
    const ledgerId = getLedgerId()
    if (ledgerId === null) return
    await storeInstance.reviveDead(ledgerId, ids)
    await loadQueues()
    void (await ensureEngine()).sync().catch(() => undefined)
  }

  /** 死信：丢弃（用户在同步页明确确认后）。 */
  async function discardDead(ids: number[]): Promise<void> {
    if (storeInstance === null) return
    const ledgerId = getLedgerId()
    if (ledgerId === null) return
    await storeInstance.discardPending(ledgerId, ids)
    await loadQueues()
  }

  /** 卸载自动同步（测试 / 登出清理用）。 */
  function stopEngine(): void {
    if (stopAutoSync !== null) {
      stopAutoSync()
      stopAutoSync = null
    }
  }

  return {
    status,
    lastSyncAt,
    pendingCount,
    deadCount,
    deadItems,
    hasBacklog,
    initialized,
    bootstrap,
    syncNow,
    loadQueues,
    recordTxnOffline,
    reviveDead,
    discardDead,
    stopEngine,
  }
})
