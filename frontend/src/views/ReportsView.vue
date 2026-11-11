<script setup lang="ts">
/**
 * P-11 报表分析（批 3）
 *
 * 与旧版 reports_screen.dart 的功能 1:1：周/月/年 + 前后翻页、#tag 下钻、
 * 核心指标、收支趋势、支出分类占比与排行、成员贡献、试算平衡。
 * 口径差异（按批 2.5 铁律修正，勿回退）：
 * - 收支统计**分录级**且按币种过滤，转账（纯 Assets/Liabilities）天然剔除；
 * - 聚合全在 `core/reports.ts` 用 decimal.js 定点完成，禁止跨币种求和；
 * - 净资产走 `computeBalanceSummary` 的 ΣAssets + ΣLiabilities，负债保留负号。
 * 数据源为在线账户 / 交易列表（新版尚无批 5 的 IndexedDB 本地真相源），
 * 因此报表需要联网；离线内核落地后由它接管数据源。
 */
import { computed, onMounted, ref, watch } from 'vue'

import { accountApi, transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView, TransactionView } from '@/api/types'
import MoneyText from '@/components/MoneyText.vue'
import ReportCharts from '@/components/ReportCharts.vue'
import { computeBalanceSummary, type SummaryAccount } from '@/core/record'
import {
  categoryShares,
  collectCommodities,
  collectTags,
  filterReportTransactions,
  incomeExpenseTotals,
  memberContributions,
  periodBuckets,
  periodLabel,
  pickCommodity,
  shiftPeriod,
  trialBalanceRows,
  trendSeries,
  type ReportPeriod,
  type ReportTxn,
} from '@/core/reports'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

// 模板不做 TS 类型断言（Vue 模板表达式不支持 `as`），周期列表在 script 里定好类型。
const PERIODS: ReportPeriod[] = ['week', 'month', 'year']
const PERIOD_LABELS: Record<ReportPeriod, string> = {
  week: '周',
  month: '月',
  year: '年',
}

const accounts = ref<AccountView[]>([])
const txns = ref<ReportTxn[]>([])
const loading = ref(false)
const loadError = ref('')
const period = ref<ReportPeriod>('month')
const cursor = ref<Date>(new Date())
const tag = ref('')
const selectedCommodity = ref('')

const summaryAccounts = computed<SummaryAccount[]>(() => accounts.value as SummaryAccount[])
const summary = computed(() =>
  computeBalanceSummary(summaryAccounts.value, session.defaultCommodity),
)
const autoCommodity = computed(() =>
  pickCommodity(txns.value, summaryAccounts.value, session.defaultCommodity),
)
const commodities = computed(() => collectCommodities(txns.value, session.defaultCommodity))
const commodity = computed(() =>
  selectedCommodity.value === '' ? autoCommodity.value : selectedCommodity.value,
)
const buckets = computed(() => periodBuckets(period.value, cursor.value))
const tags = computed(() => collectTags(txns.value))
const filtered = computed(() =>
  filterReportTransactions(txns.value, buckets.value, period.value, tag.value),
)
const totals = computed(() =>
  incomeExpenseTotals(filtered.value, summaryAccounts.value, commodity.value),
)
const trend = computed(() =>
  trendSeries(filtered.value, summaryAccounts.value, commodity.value, buckets.value, period.value),
)
const shares = computed(() =>
  categoryShares(filtered.value, summaryAccounts.value, commodity.value),
)
const members = computed(() =>
  memberContributions(filtered.value, summaryAccounts.value, commodity.value),
)
const trial = computed(() => trialBalanceRows(summaryAccounts.value, commodity.value))
const rangeLabel = computed(() => periodLabel(period.value, cursor.value))

function valueOf(bucket: Record<string, string>): string {
  const value = bucket[commodity.value]
  return value === undefined ? '0.00' : value
}

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const [accountRes, txnRes] = await Promise.all([accountApi.list(), transactionApi.list()])
    accounts.value = Array.isArray(accountRes.accounts) ? accountRes.accounts : []
    const rawTxns: TransactionView[] = Array.isArray(txnRes.transactions)
      ? txnRes.transactions
      : []
    txns.value = rawTxns.map((txn) => ({
      id: txn.id,
      date: txn.date,
      description: txn.description,
      created_by_name: txn.created_by_name,
      tags: txn.tags === undefined ? [] : txn.tags,
      postings: txn.postings.map((posting) => ({
        account_id: posting.account_id,
        commodity: posting.commodity,
        amount: posting.amount,
      })),
    }))
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '报表数据加载失败'
  } finally {
    loading.value = false
  }
}

function setPeriod(next: ReportPeriod): void {
  period.value = next
  cursor.value = new Date()
}

function move(delta: number): void {
  cursor.value = shiftPeriod(period.value, cursor.value, delta)
}

watch(
  () => session.ledgerId,
  () => {
    selectedCommodity.value = ''
    tag.value = ''
    void load()
  },
)

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="rpt">
    <header class="rpt__hero fb-hero">
      <span class="rpt__title">报表分析</span>
      <span class="rpt__sub">按分录口径统计 · 单币种展示，不跨币种折算</span>
    </header>

    <div class="fb-card rpt__filters">
      <div class="rpt__row">
        <div class="rpt__chips">
          <button
            v-for="item in PERIODS"
            :key="item"
            type="button"
            class="rpt__chip"
            :class="{ on: period === item }"
            @click="setPeriod(item)"
          >{{ PERIOD_LABELS[item] }}</button>
        </div>
        <div class="rpt__pager">
          <button type="button" class="rpt__chip" aria-label="上一期" @click="move(-1)">‹</button>
          <span class="rpt__range">{{ rangeLabel }}</span>
          <button type="button" class="rpt__chip" aria-label="下一期" @click="move(1)">›</button>
        </div>
      </div>
      <div class="rpt__row rpt__row--wrap">
        <label class="rpt__select">
          币种
          <select v-model="selectedCommodity" class="fb-input rpt__select-input">
            <option value="">自动（{{ autoCommodity }}）</option>
            <option v-for="code in commodities" :key="code" :value="code">{{ code }}</option>
          </select>
        </label>
        <div class="rpt__chips rpt__chips--tags">
          <button
            type="button"
            class="rpt__chip"
            :class="{ on: tag === '' }"
            @click="tag = ''"
          >全部</button>
          <button
            v-for="item in tags"
            :key="item"
            type="button"
            class="rpt__chip"
            :class="{ on: tag === item }"
            @click="tag = item"
          >#{{ item }}</button>
        </div>
      </div>
    </div>

    <div v-if="loadError" class="fb-card rpt__state rpt__state--error">
      <p>{{ loadError }}</p>
      <button class="fb-btn-ghost" type="button" @click="load">重新加载</button>
    </div>
    <div v-else-if="loading" class="fb-card rpt__state">正在加载报表数据…</div>
    <div v-else-if="filtered.length === 0" class="fb-card rpt__state">
      本期还没有交易，换个时间范围或标签看看。
    </div>

    <template v-else>
      <div class="rpt__metrics">
        <div class="fb-card rpt__metric">
          <span class="rpt__metric-label">净资产</span>
          <MoneyText :amount="valueOf(summary.netWorth)" :commodity="commodity" size="lg" />
        </div>
        <div class="fb-card rpt__metric">
          <span class="rpt__metric-label">总资产</span>
          <MoneyText :amount="valueOf(summary.assets)" :commodity="commodity" />
        </div>
        <div class="fb-card rpt__metric">
          <span class="rpt__metric-label">总负债</span>
          <MoneyText :amount="valueOf(summary.liabilities)" :commodity="commodity" />
        </div>
        <div class="fb-card rpt__metric">
          <span class="rpt__metric-label">交易笔数</span>
          <strong class="rpt__count">{{ filtered.length }}</strong>
        </div>
      </div>

      <div class="fb-card rpt__card">
        <div class="rpt__card-head">
          <h2>收支趋势</h2>
          <span class="fb-muted">
            收入 <MoneyText :amount="totals.income" :commodity="commodity" size="sm" />
            · 支出 <MoneyText :amount="totals.expense" :commodity="commodity" size="sm" />
          </span>
        </div>
        <div class="rpt__legend">
          <span><i class="rpt__dot rpt__dot--income" />收入</span>
          <span><i class="rpt__dot rpt__dot--expense" />支出</span>
        </div>
        <ReportCharts
          kind="trend"
          :labels="trend.labels"
          :income="trend.income"
          :expense="trend.expense"
        />
      </div>

      <div class="rpt__grid">
        <div class="fb-card rpt__card">
          <div class="rpt__card-head"><h2>支出分类占比</h2></div>
          <ReportCharts kind="share" :shares="shares" :height="200" />
          <ul v-if="shares.length > 0" class="rpt__legend-list">
            <li v-for="(item, index) in shares" :key="item.label">
              <i class="rpt__dot" :class="`rpt__dot--${index % 6}`" />
              <span class="rpt__legend-name">{{ item.label }}</span>
              <MoneyText :amount="item.amount" :commodity="commodity" size="sm" />
              <span class="fb-muted">{{ Math.round(item.ratio * 100) }}%</span>
            </li>
          </ul>
          <p v-else class="rpt__empty">本期没有支出分类数据。</p>
        </div>

        <div class="fb-card rpt__card">
          <div class="rpt__card-head"><h2>支出排行</h2></div>
          <ol v-if="shares.length > 0" class="rpt__rank">
            <li v-for="(item, index) in shares.slice(0, 10)" :key="item.label">
              <span class="rpt__rank-no">{{ index + 1 }}</span>
              <span class="rpt__rank-name">{{ item.label }}</span>
              <span class="rpt__bar"><i :style="{ width: `${Math.round(item.ratio * 100)}%` }" /></span>
              <MoneyText :amount="item.amount" :commodity="commodity" size="sm" />
            </li>
          </ol>
          <p v-else class="rpt__empty">本期没有支出记录。</p>
        </div>
      </div>

      <div class="rpt__grid">
        <div class="fb-card rpt__card">
          <div class="rpt__card-head"><h2>成员贡献</h2></div>
          <p v-if="members.length === 0" class="rpt__empty">
            还没有带记账人的交易（联网同步后显示成员署名）。
          </p>
          <ul v-else class="rpt__members">
            <li v-for="item in members" :key="item.name">
              <span class="rpt__avatar">{{ item.name.charAt(0) }}</span>
              <span class="rpt__member-name">{{ item.name }}<small>{{ item.count }} 笔</small></span>
              <span class="rpt__bar"><i :style="{ width: `${Math.round(item.ratio * 100)}%` }" /></span>
              <MoneyText :amount="item.amount" :commodity="commodity" size="sm" />
            </li>
          </ul>
        </div>

        <div class="fb-card rpt__card">
          <div class="rpt__card-head"><h2>试算平衡</h2><span class="fb-muted">{{ commodity }}</span></div>
          <p v-if="trial.length === 0" class="rpt__empty">暂无余额数据，先记几笔吧。</p>
          <table v-else class="rpt__table">
            <thead><tr><th>账户</th><th>类型</th><th>余额</th></tr></thead>
            <tbody>
              <tr v-for="row in trial" :key="row.label">
                <td>{{ row.label }}</td>
                <td>{{ row.type }}</td>
                <td><MoneyText :amount="row.balance" :commodity="commodity" size="sm" /></td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.rpt { padding-bottom: 20px; }
.rpt__hero { padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px; border-radius: 0 0 24px 24px; }
.rpt__title { display: block; font-size: 16px; font-weight: 700; }
.rpt__sub { display: block; margin-top: 4px; font-size: 12px; color: var(--fb-on-hero-muted); }
.rpt__filters { margin-top: 14px; padding: 12px 14px; }
.rpt__row { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.rpt__row--wrap { margin-top: 10px; }
.rpt__chips { display: flex; gap: 6px; flex-wrap: wrap; }
.rpt__chips--tags { flex: 1; }
.rpt__pager { display: flex; align-items: center; gap: 8px; }
.rpt__range { font-size: 12px; min-width: 140px; text-align: center; }
.rpt__chip { border: 1px solid var(--fb-line); background: var(--fb-card); color: var(--fb-sub); border-radius: var(--fb-radius-pill); padding: 4px 12px; font-size: 12px; font-family: inherit; cursor: pointer; }
.rpt__chip.on { background: var(--fb-blue); border-color: var(--fb-blue); color: var(--fb-white); }
.rpt__select { display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--fb-sub); }
.rpt__select-input { width: 140px; padding: 5px 8px; font-size: 12px; }
.rpt__metrics { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px; margin-top: 14px; }
.rpt__metric { padding: 14px; display: flex; flex-direction: column; gap: 6px; }
.rpt__metric-label { font-size: 11px; color: var(--fb-sub); }
.rpt__count { font-size: 26px; font-weight: 800; }
.rpt__card { margin-top: 14px; padding: 14px 16px; }
.rpt__card-head { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
.rpt__card-head h2 { margin: 0; font-size: 14px; }
.rpt__card-head span { font-size: 11px; }
.rpt__legend { display: flex; gap: 14px; margin-top: 8px; font-size: 11px; color: var(--fb-sub); }
.rpt__dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 5px; background: var(--fb-blue); }
.rpt__dot--income { background: #2ba471; }
.rpt__dot--expense { background: #3b6ef6; }
.rpt__dot--0 { background: #3b6ef6; } .rpt__dot--1 { background: #2ba471; }
.rpt__dot--2 { background: #e8590c; } .rpt__dot--3 { background: #8b5cf6; }
.rpt__dot--4 { background: #d93025; } .rpt__dot--5 { background: #0ea5e9; }
.rpt__legend-list, .rpt__rank, .rpt__members { list-style: none; margin: 10px 0 0; padding: 0; }
.rpt__legend-list li, .rpt__rank li, .rpt__members li { display: flex; align-items: center; gap: 8px; padding: 5px 0; font-size: 12px; }
.rpt__legend-name, .rpt__rank-name, .rpt__member-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rpt__rank-no { width: 18px; color: var(--fb-sub); }
.rpt__bar { width: 70px; height: 6px; border-radius: 3px; background: var(--fb-soft); overflow: hidden; flex-shrink: 0; }
.rpt__bar i { display: block; height: 100%; background: var(--fb-blue); }
.rpt__avatar { width: 26px; height: 26px; border-radius: 50%; background: var(--fb-blue-soft); color: var(--fb-blue); display: flex; align-items: center; justify-content: center; font-weight: 700; }
.rpt__member-name small { display: block; font-size: 10px; color: var(--fb-sub); }
.rpt__grid { display: grid; grid-template-columns: 1fr; gap: 0; }
.rpt__grid > .rpt__card { margin-top: 14px; }
.rpt__empty { margin: 12px 0 0; font-size: 12px; color: var(--fb-sub); text-align: center; }
.rpt__state { margin-top: 14px; padding: 28px 16px; text-align: center; font-size: 13px; color: var(--fb-sub); }
.rpt__state--error { color: var(--fb-danger); }
.rpt__state p { margin: 0 0 12px; }
.rpt__table { width: 100%; border-collapse: collapse; font-size: 12px; margin-top: 10px; }
.rpt__table th, .rpt__table td { text-align: left; padding: 6px 4px; border-bottom: 1px solid var(--fb-line); }
.rpt__table th { color: var(--fb-sub); font-size: 11px; }
@media (min-width: 900px) {
  .rpt__metrics { grid-template-columns: repeat(4, 1fr); }
  .rpt__grid { grid-template-columns: 1fr 1fr; gap: 14px; }
  .rpt__grid > .rpt__card { margin-top: 14px; }
}
</style>
