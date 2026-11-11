<script setup lang="ts">
/**
 * P-18 同步状态（批 5 升级：只读状态页 → 队列管理页）
 *
 * 批 3 版本刻意留白的能力由 SyncEngine 支撑后全部落地：
 * - 待推送 / 死信计数与上次同步时间（引擎订阅实时推送）；
 * - 手动同步（syncNow）；
 * - 死信列表：可见、可重试、可丢弃、可导出文本（§6.6.1 #1「同步页可见」）；
 * - 库世代：服务端 health 返回的 instance_id 展示；与本地不一致时
 *   引擎会在下次同步自动清空本账本本地数据并全量重拉。
 */
import { computed, onMounted, ref } from 'vue'

import { syncStatusApi } from '@/api/client'
import { useSessionStore } from '@/stores/session'
import { useSyncStore } from '@/stores/sync'

const session = useSessionStore()
const sync = useSyncStore()

const checking = ref(false)
const online = ref<boolean | null>(null)
const serverInstanceId = ref('')
const syncBusy = ref(false)
const syncNotice = ref('')
const checkedAt = ref('')

const deadSelected = ref<number[]>([])

const lastSyncText = computed(() => {
  if (sync.lastSyncAt === 0) return '尚未同步'
  const d = new Date(sync.lastSyncAt)
  const pad = (v: number): string => (v < 10 ? `0${v}` : String(v))
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
})

const statusText = computed(() => {
  switch (sync.status) {
    case 'syncing':
      return '同步中…'
    case 'ok':
      return '已同步'
    case 'offline':
      return '离线（写入已保存在本机）'
    case 'error':
      return '同步出错（数据未丢失）'
    default:
      return '待机'
  }
})

function padTime(d: Date): string {
  const pad = (v: number): string => (v < 10 ? `0${v}` : String(v))
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function check(): Promise<void> {
  if (checking.value) return
  checking.value = true
  try {
    const health = await syncStatusApi.health()
    online.value = health.status === 'ok'
    serverInstanceId.value = health.instance_id ?? ''
    checkedAt.value = padTime(new Date())
  } catch {
    online.value = false
    serverInstanceId.value = ''
  } finally {
    checking.value = false
  }
}

async function runSync(): Promise<void> {
  if (syncBusy.value) return
  syncBusy.value = true
  syncNotice.value = ''
  try {
    const outcome = await sync.syncNow()
    syncNotice.value =
      outcome === 'ok'
        ? '同步完成'
        : outcome === 'offline'
          ? '当前离线，本地数据已保留'
          : outcome === 'skipped'
            ? '另一个标签页正在同步'
            : '同步失败，请稍后重试'
    void check()
  } finally {
    syncBusy.value = false
  }
}

function toggleDead(id: number): void {
  const idx = deadSelected.value.indexOf(id)
  if (idx >= 0) {
    deadSelected.value = deadSelected.value.filter((v) => v !== id)
  } else {
    deadSelected.value = [...deadSelected.value, id]
  }
}

async function discardSelected(): Promise<void> {
  if (deadSelected.value.length === 0) return
  await sync.discardDead([...deadSelected.value])
  deadSelected.value = []
}

async function retrySelected(): Promise<void> {
  if (deadSelected.value.length === 0) return
  await sync.reviveDead([...deadSelected.value])
  deadSelected.value = []
  syncNotice.value = '已重新入队，正在重试'
}

/** 死信导出为文本（排障 / 手工补救用，§6.6.1 #1「可导出」）。 */
function exportDead(): void {
  const lines = sync.deadItems.map((entry) =>
    [
      `# id=${entry.id} state=${entry.state} attempts=${entry.attempts} code=${entry.last_error_code ?? ''}`,
      JSON.stringify(entry.change),
    ].join('\n'),
  )
  const blob = new Blob([lines.join('\n\n')], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'fambean-dead-queue.txt'
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  void check()
  void sync.loadQueues()
})
</script>

<template>
  <section class="syn">
    <header class="syn__hero fb-hero">
      <span class="syn__title">同步状态</span>
      <span class="syn__sub">离线队列 · 死信 · 库世代</span>
    </header>

    <div class="fb-card syn__card syn__status">
      <span class="syn__dot" :class="{ on: online === true, off: online === false }" />
      <div class="syn__status-text">
        <strong>{{ checking ? '正在检查…' : (online === true ? '服务正常' : '离线 / 服务不可达') }}</strong>
        <small>引擎状态：{{ statusText }} · 上次同步 {{ lastSyncText }}</small>
      </div>
      <button class="fb-btn-primary syn__btn" type="button" :disabled="syncBusy" @click="runSync">
        {{ syncBusy ? '同步中…' : '立即同步' }}
      </button>
    </div>

    <p v-if="syncNotice" class="syn__notice" role="status">{{ syncNotice }}</p>

    <div class="fb-card syn__card">
      <h2>队列</h2>
      <dl class="syn__grid">
        <dt>待推送</dt><dd>{{ sync.pendingCount }}</dd>
        <dt>死信（需处理）</dt><dd>{{ sync.deadCount }}</dd>
        <dt>当前账本</dt><dd>{{ session.ledgerName }}</dd>
        <dt>本次检查</dt><dd>{{ checkedAt || '—' }}</dd>
      </dl>
      <p v-if="serverInstanceId !== ''" class="syn__desc">
        服务端库世代：<code>{{ serverInstanceId }}</code>
      </p>
      <p class="syn__desc">
        服务端被「还原备份」后库世代会变化，下次同步引擎会自动清空本账本本地数据并全量重拉；
        离线期间的待推送变更会转入隔离区（下方死信列表），不会静默丢弃。
      </p>
    </div>

    <div class="fb-card syn__card">
      <div class="syn__card-head">
        <h2>死信队列</h2>
        <button
          v-if="sync.deadCount > 0"
          class="fb-btn-ghost syn__btn"
          type="button"
          @click="exportDead"
        >导出</button>
      </div>
      <p v-if="sync.deadItems.length === 0" class="syn__desc">
        暂无失败变更。离线写入会先进入待推送队列；永久失败（如记账到已关闭账户）才会进入这里。
      </p>
      <template v-else>
        <ul class="syn__dead">
          <li v-for="entry in sync.deadItems" :key="entry.id">
            <label class="syn__dead-row">
              <input
                type="checkbox"
                :checked="deadSelected.includes(entry.id)"
                @change="toggleDead(entry.id)"
              />
              <span class="syn__dead-body">
                <strong>{{ entry.change.entity_type }} · {{ entry.change.op ?? 'create' }}</strong>
                <small>{{ entry.last_error_code }}：{{ entry.last_error }}</small>
              </span>
            </label>
          </li>
        </ul>
        <div class="syn__buttons">
          <button class="fb-btn-ghost syn__btn" type="button" @click="retrySelected">重试选中</button>
          <button class="fb-btn-ghost syn__btn syn__btn--danger" type="button" @click="discardSelected">丢弃选中</button>
        </div>
      </template>
    </div>

    <div class="fb-card syn__card syn__note">
      <h2>协议与边界</h2>
      <p>账本采用 append-only 不可变交易 + 全局水位增量同步，天然几乎无冲突。</p>
      <p>
        水位只单调推进（空结果不回退）；多标签页通过存储租约选举，仅一个标签页执行同步；
        期初余额只能在线设置，不会进入离线队列。
      </p>
    </div>
  </section>
</template>

<style scoped>
.syn { padding-bottom: 20px; }
.syn__hero { padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px; border-radius: 0 0 24px 24px; }
.syn__title { display: block; font-size: 16px; font-weight: 700; }
.syn__sub { display: block; margin-top: 4px; font-size: 12px; color: var(--fb-on-hero-muted); }
.syn__card { margin-top: 14px; padding: 14px 16px; }
.syn__card h2 { margin: 0 0 10px; font-size: 14px; }
.syn__card-head { display: flex; align-items: center; justify-content: space-between; }
.syn__status { display: flex; align-items: center; gap: 12px; }
.syn__dot { width: 12px; height: 12px; border-radius: 50%; background: var(--fb-line); flex-shrink: 0; }
.syn__dot.on { background: var(--fb-green); }
.syn__dot.off { background: var(--fb-danger); }
.syn__status-text { flex: 1; min-width: 0; }
.syn__status-text strong { display: block; font-size: 14px; }
.syn__status-text small { font-size: 11px; color: var(--fb-sub); }
.syn__btn { padding: 6px 12px; font-size: 12px; white-space: nowrap; }
.syn__btn--danger { color: var(--fb-danger); border-color: var(--fb-danger); }
.syn__notice { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; background: var(--fb-green-soft); color: var(--fb-green); }
.syn__grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 12px; margin: 0 0 8px; font-size: 12px; }
.syn__grid dt { color: var(--fb-sub); }
.syn__grid dd { margin: 0; text-align: right; overflow-wrap: anywhere; }
.syn__desc { margin: 8px 0 0; font-size: 11px; color: var(--fb-sub); line-height: 1.8; overflow-wrap: anywhere; }
.syn__desc code { font-size: 10px; background: var(--fb-soft); padding: 1px 4px; border-radius: 4px; }
.syn__dead { list-style: none; margin: 0 0 10px; padding: 0; }
.syn__dead li { border-bottom: 1px solid var(--fb-line); }
.syn__dead li:last-child { border-bottom: none; }
.syn__dead-row { display: flex; align-items: flex-start; gap: 10px; padding: 8px 0; cursor: pointer; }
.syn__dead-body { min-width: 0; }
.syn__dead-body strong { display: block; font-size: 12px; }
.syn__dead-body small { display: block; font-size: 11px; color: var(--fb-danger); overflow-wrap: anywhere; }
.syn__buttons { display: flex; gap: 8px; flex-wrap: wrap; }
.syn__note p { margin: 0 0 8px; font-size: 11px; color: var(--fb-sub); line-height: 1.8; }
</style>
