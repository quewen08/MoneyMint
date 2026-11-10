<script setup lang="ts">
/**
 * P-04 交易流水（批 2）
 *
 * - 日历 / 列表双视图（对照 mobile-redesign-v2.html 账单页）；
 * - 筛选：日期范围 / 类型 / 账户 / 分类 / 成员，状态全部 query 化（AC-16）；
 * - CSV 导出（§14.5 #1）：字段 / 表头 / 转义与旧版一致，金额用后端定点字符串；
 * - 金额与符号口径复用 core（转账取转出方绝对值），组件不做任何浮点运算。
 */
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { accountApi, transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView, TransactionView } from '@/api/types'
import { buildMonthGrid, dailyTotals, monthLabel } from '@/core/calendar'
import { buildCsvRowsFromTransactions, buildTransactionsCsv, downloadCsv } from '@/core/csv'
import {
  applyFilters,
  emptyFilters,
  filtersFromQuery,
  filtersToQuery,
  hasActiveFilters,
  type TransactionFilters,
} from '@/core/filters'
import { formatAmount } from '@/core/Money'
import {
  accountLabel,
  displayAmountOf,
  todayString,
  transactionKind,
  transferRoute,
  type TransactionKind,
} from '@/core/record'
import { useSessionStore } from '@/stores/session'
import MoneyText from '@/components/MoneyText.vue'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()

const accounts = ref<AccountView[]>([])
const transactions = ref<TransactionView[]>([])
const loading = ref(false)
const loadError = ref('')
const notice = ref('')
const view = ref<'list' | 'calendar'>(
  route.query.view === 'calendar' ? 'calendar' : 'list',
)

const today = todayString()
const cursor = ref({
  year: Number(today.slice(0, 4)),
  month: Number(today.slice(5, 7)),
})

const filters = reactive<TransactionFilters>(filtersFromQuery(route.query))

const filterableAccounts = computed(() =>
  accounts.value.filter((account) => !account.close_date),
)
const categoryOptions = computed(() =>
  accounts.value.filter(
    (account) =>
      !account.close_date &&
      (account.type === 'Expenses' || account.type === 'Income'),
  ),
)
const memberOptions = computed(() => {
  const names: string[] = []
  for (const txn of transactions.value) {
    if (txn.created_by_name !== '' && names.indexOf(txn.created_by_name) < 0) {
      names.push(txn.created_by_name)
    }
  }
  return names
})

const filtered = computed(() =>
  applyFilters(transactions.value, filters, accounts.value),
)

const dayTotalsMap = computed(() =>
  dailyTotals(filtered.value, accounts.value, session.defaultCommodity),
)

const grid = computed(() =>
  buildMonthGrid(cursor.value.year, cursor.value.month, dayTotalsMap.value),
)

/** 按日期分组（后端按时间倒序返回，保持分组稳定）。 */
const grouped = computed(() => {
  const out: Array<{ date: string; items: TransactionView[] }> = []
  for (const txn of filtered.value) {
    const last = out[out.length - 1]
    if (last !== undefined && last.date === txn.date) {
      last.items.push(txn)
    } else {
      out.push({ date: txn.date, items: [txn] })
    }
  }
  return out
})

function kindOf(txn: TransactionView): TransactionKind {
  return transactionKind(txn.postings, accounts.value)
}

function rowAmount(txn: TransactionView): string {
  return displayAmountOf(txn.postings, accounts.value)
}

function rowMeta(txn: TransactionView): string {
  if (kindOf(txn) === 'transfer') {
    return transferRoute(txn.postings, accounts.value)
  }
  return txn.created_by_name
}

function onDateChange(key: 'from' | 'to', event: Event): void {
  const value = (event.target as HTMLInputElement).value
  const patch: Partial<TransactionFilters> = {}
  if (key === 'from') {
    patch.from = value
  } else {
    patch.to = value
  }
  updateFilters(patch)
}

function onKindChange(event: Event): void {
  const value = (event.target as HTMLSelectElement).value
  const kinds = ['all', 'expense', 'income', 'transfer']
  updateFilters({
    kind: kinds.indexOf(value) >= 0 ? (value as TransactionFilters['kind']) : 'all',
  })
}

function onNumberChange(key: 'account' | 'category', event: Event): void {
  const raw = (event.target as HTMLSelectElement).value
  if (raw === '' || !/^\d+$/.test(raw)) {
    const patch: Partial<TransactionFilters> = {}
    if (key === 'account') {
      patch.account = null
    } else {
      patch.category = null
    }
    updateFilters(patch)
    return
  }
  const id = Number(raw)
  const resolved = Number.isSafeInteger(id) && id > 0 ? id : null
  const patch: Partial<TransactionFilters> = {}
  if (key === 'account') {
    patch.account = resolved
  } else {
    patch.category = resolved
  }
  updateFilters(patch)
}

function onMemberChange(event: Event): void {
  updateFilters({ member: (event.target as HTMLSelectElement).value })
}

/** 筛选状态写回 URL（AC-16 写路径），并保留日历视图标记。 */
function updateFilters(patch: Partial<TransactionFilters>): void {
  Object.assign(filters, patch)
  syncQuery()
}

function setView(next: 'list' | 'calendar'): void {
  view.value = next
  syncQuery()
}

function syncQuery(): void {
  const query = filtersToQuery({ ...filters })
  if (view.value === 'calendar') {
    query.view = 'calendar'
  }
  void router.replace({ query })
}

function clearFilters(): void {
  Object.assign(filters, emptyFilters())
  syncQuery()
}

/** 日历点日：把筛选收敛到该日并切回列表。 */
function pickDay(key: string): void {
  updateFilters({ from: key, to: key })
  setView('list')
}

function shiftMonth(delta: number): void {
  let year = cursor.value.year
  let month = cursor.value.month + delta
  if (month < 1) {
    month = 12
    year -= 1
  } else if (month > 12) {
    month = 1
    year += 1
  }
  cursor.value = { year, month }
}

/** §14.5 #1：导出当前筛选结果；account 列为 display_name 优先（旧版语义）。 */
function exportCsv(): void {
  const rows = buildCsvRowsFromTransactions(filtered.value, accounts.value)
  let postingCount = 0
  for (const row of rows) {
    postingCount = postingCount + row.postings.length
  }
  downloadCsv('family-ledger.csv', buildTransactionsCsv(rows))
  notice.value = `已导出 CSV（${rows.length} 笔交易，${postingCount} 条分录）`
}

function openDetail(txn: TransactionView): void {
  void router.push({ name: 'transaction-detail', query: { id: String(txn.id) } })
}

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const [accountRes, txnRes] = await Promise.all([accountApi.list(), transactionApi.list()])
    accounts.value = Array.isArray(accountRes.accounts) ? accountRes.accounts : []
    transactions.value = Array.isArray(txnRes.transactions) ? txnRes.transactions : []
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '流水加载失败'
  } finally {
    loading.value = false
  }
}

// AC-16：浏览器前进 / 后退或外部链接变化时，从 query 还原筛选与视图。
watch(
  () => route.query,
  () => {
    Object.assign(filters, filtersFromQuery(route.query))
    view.value = route.query.view === 'calendar' ? 'calendar' : 'list'
  },
)

// E-08：切账本时清空视图缓存并重新加载。
watch(
  () => session.ledgerId,
  () => {
    accounts.value = []
    transactions.value = []
    void load()
  },
)

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="txn">
    <header class="txn__hero fb-hero">
      <div class="txn__hero-top">
        <span class="txn__hero-label">交易流水 · {{ session.ledgerName }}</span>
        <button type="button" class="txn__export" @click="exportCsv">⇩ 导出 CSV</button>
      </div>
      <div class="txn__seg">
        <button type="button" :class="{ on: view === 'calendar' }" @click="setView('calendar')">日历</button>
        <button type="button" :class="{ on: view === 'list' }" @click="setView('list')">列表</button>
      </div>
    </header>

    <div class="fb-card txn__filters">
      <label class="txn__filter"><span>从</span><input class="fb-input" type="date" :value="filters.from" @change="onDateChange('from', $event)" /></label>
      <label class="txn__filter"><span>到</span><input class="fb-input" type="date" :value="filters.to" @change="onDateChange('to', $event)" /></label>
      <label class="txn__filter"><span>类型</span>
        <select class="fb-input" :value="filters.kind" @change="onKindChange">
          <option value="all">全部</option>
          <option value="expense">支出</option>
          <option value="income">收入</option>
          <option value="transfer">转账</option>
        </select>
      </label>
      <label class="txn__filter"><span>账户</span>
        <select class="fb-input" :value="filters.account === null ? '' : String(filters.account)" @change="onNumberChange('account', $event)">
          <option value="">全部账户</option>
          <option v-for="acc in filterableAccounts" :key="acc.id" :value="String(acc.id)">{{ accountLabel(acc) }}</option>
        </select>
      </label>
      <label class="txn__filter"><span>分类</span>
        <select class="fb-input" :value="filters.category === null ? '' : String(filters.category)" @change="onNumberChange('category', $event)">
          <option value="">全部分类</option>
          <option v-for="cat in categoryOptions" :key="cat.id" :value="String(cat.id)">{{ accountLabel(cat) }}</option>
        </select>
      </label>
      <label class="txn__filter"><span>成员</span>
        <select class="fb-input" :value="filters.member" @change="onMemberChange">
          <option value="">全部成员</option>
          <option v-for="name in memberOptions" :key="name" :value="name">{{ name }}</option>
        </select>
      </label>
      <button v-if="hasActiveFilters(filters)" type="button" class="txn__clear" @click="clearFilters">清除筛选</button>
    </div>

    <div v-if="loadError" class="fb-card txn__state txn__state--error">
      <p>{{ loadError }}</p>
      <button class="fb-btn-ghost" type="button" @click="load">重新加载</button>
    </div>
    <div v-else-if="loading" class="fb-card txn__state">正在加载流水…</div>

    <template v-else>
      <!-- 日历视图 -->
      <div v-if="view === 'calendar'" class="fb-card txn__card">
        <div class="txn__month">
          <button type="button" class="txn__month-btn" @click="shiftMonth(-1)">‹</button>
          <strong>{{ monthLabel(cursor.year, cursor.month) }}</strong>
          <button type="button" class="txn__month-btn" @click="shiftMonth(1)">›</button>
        </div>
        <div class="txn__cal">
          <span v-for="week in ['一', '二', '三', '四', '五', '六', '日']" :key="week" class="txn__cal-w">{{ week }}</span>
          <button
            v-for="cell in grid"
            :key="cell.key"
            type="button"
            class="txn__cal-d"
            :class="{ dim: !cell.inMonth }"
            @click="pickDay(cell.key)"
          >
            <span class="txn__cal-n">{{ cell.day }}</span>
            <span v-if="cell.expense !== '0.00'" class="txn__cal-e">{{ formatAmount(cell.expense) }}</span>
            <span v-if="cell.income !== '0.00'" class="txn__cal-i">+{{ formatAmount(cell.income) }}</span>
          </button>
        </div>
      </div>

      <!-- 列表视图 -->
      <div v-if="view === 'list'" class="fb-card txn__card">
        <p v-if="grouped.length === 0" class="txn__empty">没有符合筛选条件的流水。</p>
        <template v-for="group in grouped" :key="group.date">
          <div class="txn__day">
            <span>{{ group.date }}</span>
            <span class="txn__day-sum" v-if="dayTotalsMap[group.date] !== undefined">
              <MoneyText :amount="dayTotalsMap[group.date].expense" :commodity="session.defaultCommodity" size="sm" />
              <MoneyText :amount="dayTotalsMap[group.date].income" :commodity="session.defaultCommodity" size="sm" signed />
            </span>
          </div>
          <ul class="txn__list">
            <li v-for="txn in group.items" :key="txn.id" class="txn__row">
              <button type="button" class="txn__row-main" @click="openDetail(txn)">
                <span class="txn__row-desc">
                  <span v-if="kindOf(txn) === 'transfer'" class="txn__badge">转账</span>
                  {{ txn.description }}
                </span>
                <span class="txn__row-meta">{{ rowMeta(txn) }}</span>
              </button>
              <MoneyText
                :amount="rowAmount(txn)"
                :commodity="session.defaultCommodity"
                size="sm"
                :signed="kindOf(txn) !== 'transfer'"
              />
            </li>
          </ul>
        </template>
      </div>

      <p v-if="notice" class="txn__notice" role="status">{{ notice }}</p>
    </template>
  </section>
</template>

<style scoped>
.txn {
  padding-bottom: 20px;
}

.txn__hero {
  padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 18px;
  border-radius: 0 0 24px 24px;
}

.txn__hero-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.txn__hero-label {
  font-size: 15px;
  font-weight: 700;
}

.txn__export {
  border: none;
  border-radius: var(--fb-radius-pill);
  background: var(--fb-on-hero-pill);
  color: var(--fb-white);
  padding: 6px 12px;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
}

.txn__seg {
  display: flex;
  gap: 4px;
  margin-top: 14px;
  background: var(--fb-on-hero-pill);
  border-radius: var(--fb-radius-input);
  padding: 3px;
}

.txn__seg button {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--fb-on-hero-muted);
  padding: 7px 0;
  border-radius: 8px;
  font-size: 13px;
  font-family: inherit;
  cursor: pointer;
}

.txn__seg button.on {
  background: var(--fb-card);
  color: var(--fb-ink);
  font-weight: 700;
}

.txn__filters {
  margin-top: 14px;
  padding: 12px 14px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: flex-end;
}

.txn__filter {
  flex: 1 1 120px;
  min-width: 110px;
}

.txn__filter > span {
  display: block;
  font-size: 11px;
  color: var(--fb-sub);
  margin-bottom: 4px;
}

.txn__clear {
  border: none;
  background: transparent;
  color: var(--fb-blue);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
}

.txn__card {
  margin-top: 14px;
  padding: 14px 16px;
}

.txn__month {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  margin-bottom: 10px;
  font-size: 14px;
}

.txn__month-btn {
  border: none;
  background: var(--fb-soft);
  border-radius: 8px;
  width: 28px;
  height: 28px;
  font-size: 14px;
  color: var(--fb-ink);
  cursor: pointer;
}

.txn__cal {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 4px 2px;
  text-align: center;
}

.txn__cal-w {
  font-size: 11px;
  color: var(--fb-sub);
  padding: 4px 0;
}

.txn__cal-d {
  border: none;
  background: transparent;
  border-radius: 10px;
  padding: 5px 0 7px;
  font-family: inherit;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.txn__cal-d.dim {
  opacity: 0.35;
}

.txn__cal-n {
  font-size: 13px;
  font-weight: 500;
  color: var(--fb-ink);
}

.txn__cal-e {
  font-size: 9px;
  color: var(--fb-danger);
  font-variant-numeric: var(--fb-font-numeric);
}

.txn__cal-i {
  font-size: 9px;
  color: var(--fb-green);
  font-variant-numeric: var(--fb-font-numeric);
}

.txn__day {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--fb-sub);
  padding: 12px 0 4px;
}

.txn__day-sum {
  display: inline-flex;
  gap: 10px;
}

.txn__list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.txn__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--fb-line);
}

.txn__row:last-child {
  border-bottom: none;
}

.txn__row-main {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  padding: 0;
  text-align: left;
  font-family: inherit;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.txn__row-desc {
  font-size: 13px;
  font-weight: 500;
  color: var(--fb-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.txn__row-meta {
  font-size: 11px;
  color: var(--fb-sub);
}

.txn__badge {
  display: inline-block;
  margin-right: 6px;
  padding: 1px 7px;
  border-radius: var(--fb-radius-pill);
  background: var(--fb-soft);
  color: var(--fb-sub);
  font-size: 10px;
  font-weight: 600;
  vertical-align: middle;
}

.txn__empty {
  margin: 0;
  padding: 16px 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-sub);
}

.txn__state {
  margin-top: 14px;
  padding: 20px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--fb-sub);
}

.txn__state--error {
  color: var(--fb-danger);
}

.txn__state p {
  margin: 0 0 12px;
}

.txn__notice {
  margin: 14px 0 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-blue);
}
</style>
