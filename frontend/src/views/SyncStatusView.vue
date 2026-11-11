<script setup lang="ts">
/**
 * P-18 同步状态（批 3，只读状态页）
 *
 * 与旧版 sync_screen.dart 的差异是**刻意且如实标注**的：
 * 离线内核（IndexedDB 真相源 / pending 队列 / 租约 / 死信）属于批 5，
 * 本期不伪造「待同步 N 条」「立即同步」这类需要内核支撑的能力。
 * 本页只做只读可视：后端连通性、当前账本、账户数 / 交易数、协议说明。
 *
 * 特别说明：服务端 `SyncPull` 会按 client_id 写回 sync_checkpoints，
 * 因此**不用 pull 来探测水位**（会静默篡改设备水位），水位展示留到批 4/5。
 */
import { onMounted, ref } from 'vue'

import { accountApi, syncStatusApi, transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const checking = ref(false)
const online = ref<boolean | null>(null)
const healthText = ref('')
const accountCount = ref<number | null>(null)
const txnCount = ref<number | null>(null)
const loadError = ref('')
const checkedAt = ref('')

function messageOf(err: unknown): string {
  return err instanceof ApiError ? err.message : '无法连接服务器'
}

async function check(): Promise<void> {
  if (checking.value) {
    return
  }
  checking.value = true
  loadError.value = ''
  try {
    const health = await syncStatusApi.health()
    online.value = health.status === 'ok'
    healthText.value = health.status === 'ok' ? '服务正常' : `状态：${health.status}`
    const [accountRes, txnRes] = await Promise.all([accountApi.list(), transactionApi.list()])
    accountCount.value = Array.isArray(accountRes.accounts) ? accountRes.accounts.length : 0
    txnCount.value = Array.isArray(txnRes.transactions) ? txnRes.transactions.length : 0
    const now = new Date()
    const pad = (value: number): string => (value < 10 ? `0${value}` : String(value))
    checkedAt.value = `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`
  } catch (err) {
    online.value = false
    healthText.value = '离线 / 服务不可达'
    loadError.value = messageOf(err)
  } finally {
    checking.value = false
  }
}

onMounted(() => { void check() })
</script>

<template>
  <section class="syn">
    <header class="syn__hero fb-hero">
      <span class="syn__title">同步状态</span>
      <span class="syn__sub">只读状态检查</span>
    </header>

    <div class="fb-card syn__card syn__status">
      <span class="syn__dot" :class="{ on: online === true, off: online === false }" />
      <div class="syn__status-text">
        <strong>{{ checking ? '正在检查…' : healthText || '尚未检查' }}</strong>
        <small>{{ online === true ? '后端可达，当前账本数据已加载' : '请检查网络或后端服务' }}</small>
      </div>
      <button class="fb-btn-ghost syn__btn" type="button" :disabled="checking" @click="check">立即检查</button>
    </div>

    <p v-if="loadError" class="syn__notice syn__notice--err" role="alert">{{ loadError }}</p>

    <div class="fb-card syn__card">
      <h2>同步信息</h2>
      <dl class="syn__grid">
        <dt>当前账本</dt><dd>{{ session.ledgerName }}</dd>
        <dt>账本 id</dt><dd>{{ session.ledgerId === null ? '—' : session.ledgerId }}</dd>
        <dt>账户数</dt><dd>{{ accountCount === null ? '—' : accountCount }}</dd>
        <dt>交易数</dt><dd>{{ txnCount === null ? '—' : txnCount }}</dd>
        <dt>本次检查</dt><dd>{{ checkedAt || '—' }}</dd>
        <dt>待推送队列</dt><dd>批 5 离线内核接管</dd>
        <dt>服务端水位</dt><dd>批 4 迁移 014 后可读</dd>
      </dl>
    </div>

    <div class="fb-card syn__card syn__note">
      <h2>协议与边界</h2>
      <p>账本采用 append-only 不可变交易 + 全局水位增量同步，天然几乎无冲突。</p>
      <p>
        本页刻意不提供「立即同步 / 待同步条数」：服务端 <code>GET /api/sync/pull</code>
        会按 client_id 写回 <code>sync_checkpoints</code>，用来探测水位会静默篡改设备水位。
        离线记账、pending 队列、失败重试与隔离区在批 5 的内核里实现；
        库世代 <code>instance_id</code> 由批 4 的迁移 014 引入。
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
.syn__status { display: flex; align-items: center; gap: 12px; }
.syn__dot { width: 12px; height: 12px; border-radius: 50%; background: var(--fb-line); flex-shrink: 0; }
.syn__dot.on { background: var(--fb-green); }
.syn__dot.off { background: var(--fb-danger); }
.syn__status-text { flex: 1; min-width: 0; }
.syn__status-text strong { display: block; font-size: 14px; }
.syn__status-text small { font-size: 11px; color: var(--fb-sub); }
.syn__btn { padding: 6px 12px; font-size: 12px; }
.syn__notice { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; }
.syn__notice--err { background: var(--fb-danger-soft); color: var(--fb-danger); }
.syn__grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 12px; margin: 0; font-size: 12px; }
.syn__grid dt { color: var(--fb-sub); }
.syn__grid dd { margin: 0; text-align: right; }
.syn__note p { margin: 0 0 8px; font-size: 11px; color: var(--fb-sub); line-height: 1.8; }
.syn__note code { background: var(--fb-soft); padding: 1px 4px; border-radius: 4px; }
</style>
