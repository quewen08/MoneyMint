<script setup lang="ts">
/**
 * P-13 账本管理 / 切换（批 3）
 *
 * 与旧版 ledgers_screen.dart + ledger_switcher.dart 1:1：列表（当前徽章 + 角色）、
 * 新建（默认币种 CNY）、改名（owner）、切换账本、期初日期（owner 专属、
 * 二次确认、上界 = min(今日, 最早非 opening 交易)）。
 *
 * 后端 PATCH /api/ledgers/{id} 的 name / default_commodity 是**非指针字段**：
 * 改名或改期初日期时必须把两者都带上，否则 name 空串会被拒、币种会被清空。
 */
import { computed, onMounted, ref } from 'vue'

import { ledgerManageApi, transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import { roleLabel } from '@/core/batch3'
import { todayString } from '@/core/record'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const loading = ref(false)
const busy = ref(false)
const loadError = ref('')
const notice = ref('')
const actionError = ref('')

const createOpen = ref(false)
const newName = ref('')
const renameTargetId = ref<number | null>(null)
const renameValue = ref('')

const openingOpen = ref(false)
const openingValue = ref('')
const openingCount = ref(0)
const openingBound = ref(todayString())

const currentLedger = computed(() => {
  for (const ledger of session.ledgers) {
    if (ledger.id === session.ledgerId) {
      return ledger
    }
  }
  return null
})
const isOwner = computed(() => currentLedger.value !== null && currentLedger.value.role === 'owner')

function messageOf(err: unknown): string {
  return err instanceof ApiError ? err.message : '操作失败，请稍后重试'
}

async function reload(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    await session.refreshLedgers()
  } catch (err) {
    loadError.value = messageOf(err)
  } finally {
    loading.value = false
  }
}

async function run(action: () => Promise<unknown>, success: string): Promise<void> {
  if (busy.value) {
    return
  }
  busy.value = true
  actionError.value = ''
  notice.value = ''
  try {
    await action()
    notice.value = success
    await reload()
  } catch (err) {
    actionError.value = messageOf(err)
  } finally {
    busy.value = false
  }
}

function submitCreate(): void {
  const name = newName.value.trim()
  if (name === '') {
    actionError.value = '请输入账本名称'
    return
  }
  void run(async () => {
    const created = await ledgerManageApi.create({ name, default_commodity: 'CNY' })
    await session.refreshLedgers()
    session.selectLedger(created.id)
    createOpen.value = false
    newName.value = ''
  }, `已创建「${name}」并切换过去`)
}

function openRename(id: number, name: string): void {
  renameTargetId.value = id
  renameValue.value = name
  actionError.value = ''
}

function submitRename(): void {
  const id = renameTargetId.value
  if (id === null) {
    return
  }
  const name = renameValue.value.trim()
  if (name === '') {
    actionError.value = '账本名称不能为空'
    return
  }
  let targetName = name
  let commodity = 'CNY'
  for (const ledger of session.ledgers) {
    if (ledger.id === id) {
      commodity = ledger.default_commodity || 'CNY'
      targetName = name
    }
  }
  void run(async () => {
    await ledgerManageApi.update(id, { name: targetName, default_commodity: commodity })
    renameTargetId.value = null
  }, '账本已改名')
}

async function openOpening(): Promise<void> {
  actionError.value = ''
  notice.value = ''
  openingValue.value = currentLedger.value === null ? todayString() : currentLedger.value.opening_date
  // 上界 = min(今日, 最早一笔非 opening 交易日期)；同时统计将被重建的期初笔数。
  let bound = todayString()
  let count = 0
  try {
    const res = await transactionApi.list()
    const txns = Array.isArray(res.transactions) ? res.transactions : []
    let earliest = ''
    for (const txn of txns) {
      const tags = txn.tags === undefined ? [] : txn.tags
      if (tags.indexOf('opening') >= 0) {
        count += 1
        continue
      }
      if (earliest === '' || txn.date < earliest) {
        earliest = txn.date
      }
    }
    if (earliest !== '' && earliest < bound) {
      bound = earliest
    }
  } catch (err) {
    actionError.value = `期初日期仅在线可修改：${messageOf(err)}`
    return
  }
  openingBound.value = bound
  openingCount.value = count
  openingOpen.value = true
}

function submitOpening(): void {
  const id = session.ledgerId
  const ledger = currentLedger.value
  if (id === null || ledger === null) {
    return
  }
  const value = openingValue.value
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    actionError.value = '请选择合法日期（YYYY-MM-DD）'
    return
  }
  if (value > openingBound.value) {
    actionError.value = `期初日期不得晚于 ${openingBound.value}`
    return
  }
  void run(async () => {
    await ledgerManageApi.update(id, {
      name: ledger.name,
      default_commodity: ledger.default_commodity || 'CNY',
      opening_date: value,
    })
    openingOpen.value = false
  }, `期初日期已调整为 ${value}，期初交易已重建`)
}

function switchTo(id: number, name: string): void {
  if (id === session.ledgerId) {
    return
  }
  session.selectLedger(id)
  notice.value = `已切换到「${name}」，各页面数据将按新账本重新加载`
  actionError.value = ''
}

onMounted(() => { void reload() })
</script>

<template>
  <section class="ldg">
    <header class="ldg__hero fb-hero">
      <span class="ldg__title">账本管理</span>
      <span class="ldg__sub">切换账本会重新拉取该账本数据</span>
    </header>

    <div class="fb-card ldg__toolbar">
      <span class="fb-muted">共 {{ session.ledgers.length }} 个账本</span>
      <button class="fb-btn-primary ldg__btn" type="button" :disabled="busy" @click="createOpen = true">新建账本</button>
    </div>

    <p v-if="notice" class="ldg__notice ldg__notice--ok" role="status">{{ notice }}</p>
    <p v-if="actionError" class="ldg__notice ldg__notice--err" role="alert">{{ actionError }}</p>

    <div v-if="createOpen" class="fb-card ldg__panel">
      <h2>新建账本</h2>
      <label class="ldg__field">
        <span>账本名称（默认币种 CNY）</span>
        <input v-model="newName" class="fb-input" type="text" placeholder="如 家庭日常" />
      </label>
      <div class="ldg__actions">
        <button class="fb-btn-ghost" type="button" @click="createOpen = false">取消</button>
        <button class="fb-btn-primary" type="button" :disabled="busy" @click="submitCreate">创建并切换</button>
      </div>
    </div>

    <div v-if="renameTargetId !== null" class="fb-card ldg__panel">
      <h2>重命名账本</h2>
      <label class="ldg__field">
        <span>新名称</span>
        <input v-model="renameValue" class="fb-input" type="text" />
      </label>
      <div class="ldg__actions">
        <button class="fb-btn-ghost" type="button" @click="renameTargetId = null">取消</button>
        <button class="fb-btn-primary" type="button" :disabled="busy" @click="submitRename">保存</button>
      </div>
    </div>

    <div v-if="loadError" class="fb-card ldg__state ldg__state--err">
      <p>加载失败：{{ loadError }}</p>
      <button class="fb-btn-ghost" type="button" @click="reload">重新加载</button>
    </div>
    <div v-else-if="loading" class="fb-card ldg__state">正在加载账本…</div>
    <div v-else-if="session.ledgers.length === 0" class="fb-card ldg__state">还没有账本，创建一个吧。</div>
    <div v-else class="fb-card ldg__list">
      <div v-for="ledger in session.ledgers" :key="ledger.id" class="ldg__row">
        <span class="ldg__icon">▤</span>
        <span class="ldg__name">
          {{ ledger.name }}
          <small>{{ roleLabel(ledger.role) }} · 期初 {{ ledger.opening_date }}</small>
          <em v-if="ledger.id === session.ledgerId" class="ldg__current">当前</em>
        </span>
        <button
          v-if="ledger.id !== session.ledgerId"
          class="fb-btn-ghost ldg__btn"
          type="button"
          @click="switchTo(ledger.id, ledger.name)"
        >切换</button>
        <button
          v-if="ledger.role === 'owner'"
          class="fb-btn-ghost ldg__btn"
          type="button"
          :disabled="busy"
          @click="openRename(ledger.id, ledger.name)"
        >改名</button>
      </div>
    </div>

    <div v-if="isOwner && currentLedger" class="fb-card ldg__panel ldg__opening">
      <h2>期初日期</h2>
      <p class="fb-muted">当前：{{ currentLedger.opening_date }}。修改会冲正并重建全部期初交易，仅在线可操作。</p>
      <button class="fb-btn-ghost" type="button" :disabled="busy" @click="openOpening">调整期初日期</button>
    </div>

    <div v-if="openingOpen" class="fb-card ldg__panel ldg__panel--danger">
      <h2>修改期初日期？</h2>
      <p>
        期初日期将从 {{ currentLedger ? currentLedger.opening_date : '—' }} 调整为
        {{ openingValue }}。将重建 {{ openingCount }} 笔期初交易（旧期初冲正后重新生成），确认继续吗？
      </p>
      <label class="ldg__field">
        <span>新期初日期（不得晚于 {{ openingBound }}）</span>
        <input v-model="openingValue" class="fb-input" type="date" :max="openingBound" />
      </label>
      <div class="ldg__actions">
        <button class="fb-btn-ghost" type="button" @click="openingOpen = false">取消</button>
        <button class="fb-btn-primary" type="button" :disabled="busy" @click="submitOpening">确认并重建</button>
      </div>
    </div>

    <p class="ldg__hint">提示：切换账本会重新加载该账本数据；旧账本尚未同步的离线变更将在批 5 的同步内核中处理。</p>
  </section>
</template>

<style scoped>
.ldg { padding-bottom: 20px; }
.ldg__hero { padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px; border-radius: 0 0 24px 24px; }
.ldg__title { display: block; font-size: 16px; font-weight: 700; }
.ldg__sub { display: block; margin-top: 4px; font-size: 12px; color: var(--fb-on-hero-muted); }
.ldg__toolbar { margin-top: 14px; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; font-size: 12px; }
.ldg__btn { padding: 6px 12px; font-size: 12px; }
.ldg__notice { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; }
.ldg__notice--ok { background: var(--fb-green-soft); color: var(--fb-green); }
.ldg__notice--err { background: var(--fb-danger-soft); color: var(--fb-danger); }
.ldg__panel { margin-top: 12px; padding: 14px 16px; }
.ldg__panel h2 { margin: 0 0 10px; font-size: 14px; }
.ldg__panel p { margin: 0 0 10px; font-size: 12px; }
.ldg__panel--danger { border: 1px solid var(--fb-warn); }
.ldg__field { display: block; margin-bottom: 10px; font-size: 12px; color: var(--fb-sub); }
.ldg__field span { display: block; margin-bottom: 4px; }
.ldg__field .fb-input { width: 100%; max-width: 260px; }
.ldg__actions { display: flex; justify-content: flex-end; gap: 8px; }
.ldg__list { margin-top: 12px; padding: 6px 14px; }
.ldg__row { display: flex; align-items: center; gap: 10px; padding: 10px 0; border-bottom: 1px solid var(--fb-line); flex-wrap: wrap; }
.ldg__row:last-child { border-bottom: none; }
.ldg__icon { font-size: 18px; color: var(--fb-blue); }
.ldg__name { flex: 1; min-width: 130px; font-size: 13px; }
.ldg__name small { display: block; font-size: 11px; color: var(--fb-sub); }
.ldg__current { display: inline-block; margin-left: 6px; padding: 1px 7px; border-radius: var(--fb-radius-pill); background: var(--fb-green-soft); color: var(--fb-green); font-size: 10px; font-style: normal; }
.ldg__opening { margin-top: 14px; }
.ldg__state { margin-top: 14px; padding: 28px 16px; text-align: center; font-size: 13px; color: var(--fb-sub); }
.ldg__state--err { color: var(--fb-danger); }
.ldg__state p { margin: 0 0 12px; }
.ldg__hint { margin: 14px 4px 0; font-size: 11px; color: var(--fb-sub); line-height: 1.7; }
</style>
