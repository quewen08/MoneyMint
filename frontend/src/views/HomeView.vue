<script setup lang="ts">
/**
 * P-03 首页 / 余额（替换批 0 占位页）
 *
 * - 净资产 / 总资产 / 负债汇总，口径统一 `ΣAssets + ΣLiabilities`（core/record）；
 * - 最近流水入口 + 记一笔入口；净资产为负由 MoneyText 保留负号（AC-12）；
 * - 覆盖空 / 加载 / 错误 / 正常四态；金额一律 Money 定点，组件不自行 abs()。
 */
import { computed, onMounted, ref, watch } from 'vue'

import { accountApi, transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView, TransactionView } from '@/api/types'
import {
  accountLabel,
  computeBalanceSummary,
  displayAmountOf,
  transactionKind,
  transferRoute,
  type TransactionKind,
} from '@/core/record'
import { useRecordSheetStore } from '@/stores/recordSheet'
import { useSessionStore } from '@/stores/session'
import MoneyText from '@/components/MoneyText.vue'

const session = useSessionStore()
const sheet = useRecordSheetStore()

const accounts = ref<AccountView[]>([])
const transactions = ref<TransactionView[]>([])
const loading = ref(false)
const loadError = ref('')
const notice = ref('')

const summary = computed(() =>
  computeBalanceSummary(accounts.value, session.defaultCommodity),
)
const recent = computed(() => transactions.value.slice(0, 5))
const balanceAccounts = computed(() =>
  accounts.value.filter(
    (account) =>
      !account.close_date &&
      (account.type === 'Assets' || account.type === 'Liabilities'),
  ),
)

/** 展示类型：全资金分录 → transfer；含 Expenses / Income → 支出 / 收入；其余回退支出。 */
function txnKind(txn: TransactionView): TransactionKind {
  return transactionKind(txn.postings, accounts.value)
}

/** 用户视角金额：统一走 core 口径（转账取转出方绝对值，支出/收入带符号原值）。 */
function displayAmount(txn: TransactionView): string {
  return displayAmountOf(txn.postings, accounts.value)
}

/** 转账行的元信息：`转出账户 → 转入账户`。 */
function routeLabel(txn: TransactionView): string {
  return transferRoute(txn.postings, accounts.value)
}

function accountBalance(account: AccountView): string {
  const balances = account.balances
  if (balances === undefined) {
    return '0.00'
  }
  const value = balances[summary.value.commodity]
  return value === undefined ? '0.00' : value
}

async function load(): Promise<void> {
  if (session.ledgerId === null) {
    loadError.value = '尚未选择账本'
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const [accountRes, transactionRes] = await Promise.all([
      accountApi.list(),
      transactionApi.list(),
    ])
    accounts.value = Array.isArray(accountRes.accounts) ? accountRes.accounts : []
    transactions.value = Array.isArray(transactionRes.transactions)
      ? transactionRes.transactions
      : []
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '账本数据加载失败'
  } finally {
    loading.value = false
  }
}

function openRecord(): void {
  if (session.isViewer) {
    notice.value = '只读成员不能记账'
    return
  }
  if (session.needsInvite) {
    notice.value = '请先等待 owner 邀请加入账本'
    return
  }
  notice.value = ''
  sheet.open()
}

function showAllNotice(): void {
  notice.value = '流水列表 / 筛选页将在批 2 开放'
}

onMounted(() => {
  void load()
})

// E-08：切账本时清空视图缓存并重新加载；保存成功后刷新余额与流水。
watch(
  () => session.ledgerId,
  () => {
    accounts.value = []
    transactions.value = []
    void load()
  },
)
watch(
  () => sheet.savedEpoch,
  () => {
    void load()
  },
)
</script>

<template>
  <section class="home">
    <div class="home__hero fb-hero">
      <div class="home__hero-top">
        <span class="home__ledger">📚 {{ session.ledgerName }}</span>
        <span class="home__hint">{{ summary.commodity }}</span>
      </div>
      <div class="home__label">净资产</div>
      <MoneyText
        class="home__net"
        :amount="summary.netWorthTotal"
        :commodity="summary.commodity"
        size="lg"
      />
      <div class="home__hero-row">
        <div class="home__metric">
          <span>总资产</span>
          <MoneyText :amount="summary.assetTotal" :commodity="summary.commodity" size="sm" />
        </div>
        <div class="home__vline"></div>
        <div class="home__metric">
          <span>负债</span>
          <MoneyText :amount="summary.liabilityTotal" :commodity="summary.commodity" size="sm" />
        </div>
      </div>
    </div>

    <div v-if="loadError" class="fb-card home__state home__state--error">
      <p>{{ loadError }}</p>
      <button class="fb-btn-ghost" type="button" @click="load">重新加载</button>
    </div>
    <div v-else-if="loading" class="fb-card home__state">正在加载账本数据…</div>

    <template v-else>
      <div class="fb-card home__actions">
        <button
          class="home__record"
          type="button"
          :disabled="session.isViewer || session.needsInvite"
          @click="openRecord"
        >＋ 记一笔</button>
        <p class="fb-muted">
          {{
            session.isViewer
              ? '只读成员不能记账'
              : '支出 / 收入 / 转账，自动生成配平分录'
          }}
        </p>
      </div>

      <div class="fb-card home__card">
        <div class="home__card-head">
          <h3>最近流水</h3>
          <button type="button" class="home__more" @click="showAllNotice">查看全部 ›</button>
        </div>
        <p v-if="recent.length === 0" class="home__empty">
          还没有流水，点「记一笔」开始记录吧。
        </p>
        <ul v-else class="home__list">
          <li v-for="txn in recent" :key="txn.id" class="home__row">
            <span class="home__row-info">
              <span class="home__row-desc">
                <span v-if="txnKind(txn) === 'transfer'" class="home__badge">转账</span>
                {{ txn.description }}
              </span>
              <span class="home__row-meta">
                <template v-if="txnKind(txn) === 'transfer'">
                  {{ txn.date }} · {{ routeLabel(txn) }}
                </template>
                <template v-else>
                  {{ txn.date }} · {{ txn.created_by_name }}
                </template>
              </span>
            </span>
            <MoneyText
              :amount="displayAmount(txn)"
              :commodity="summary.commodity"
              size="sm"
              :signed="txnKind(txn) !== 'transfer'"
            />
          </li>
        </ul>
      </div>

      <div class="fb-card home__card">
        <div class="home__card-head">
          <h3>账户余额</h3>
          <span class="fb-muted">{{ balanceAccounts.length }} 个账户</span>
        </div>
        <p v-if="balanceAccounts.length === 0" class="home__empty">
          还没有账户，请由 owner 在账户管理中创建。
        </p>
        <ul v-else class="home__list">
          <li v-for="account in balanceAccounts" :key="account.id" class="home__row">
            <span class="home__row-info">
              <span class="home__row-desc">{{ accountLabel(account) }}</span>
              <span class="home__row-meta">{{ account.type }} · {{ summary.commodity }}</span>
            </span>
            <MoneyText
              :amount="accountBalance(account)"
              :commodity="summary.commodity"
              size="sm"
            />
          </li>
        </ul>
      </div>

      <p v-if="notice" class="home__notice" role="status">{{ notice }}</p>
    </template>
  </section>
</template>

<style scoped>
.home {
  padding-bottom: 20px;
}

.home__hero {
  padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px;
  border-radius: 0 0 30px 30px;
}

.home__hero-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}

.home__ledger {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  border-radius: var(--fb-radius-pill);
  background: var(--fb-on-hero-pill);
  font-size: 11px;
}

.home__hint {
  font-size: 11px;
  color: var(--fb-on-hero-muted);
}

.home__label {
  font-size: 12px;
  color: var(--fb-on-hero-muted);
}

.home__net {
  margin-top: 4px;
  color: var(--fb-white);
}

.home__net :deep(.money-text--negative) {
  color: var(--fb-white);
}

.home__hero-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 16px;
}

.home__metric {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.home__metric > span {
  font-size: 11px;
  color: var(--fb-on-hero-muted);
}

.home__vline {
  width: 1px;
  height: 34px;
  background: var(--fb-on-hero-pill);
}

.home__actions {
  margin-top: 14px;
  padding: 16px;
  text-align: center;
}

.home__record {
  width: 100%;
  height: 48px;
  border: none;
  border-radius: 14px;
  background: var(--fb-blue);
  color: var(--fb-white);
  font-size: 16px;
  font-weight: 700;
  font-family: inherit;
  cursor: pointer;
}

.home__record:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.home__actions p {
  margin: 10px 0 0;
  font-size: 12px;
}

.home__card {
  margin-top: 14px;
  padding: 16px;
}

.home__card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.home__card-head h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.home__more {
  border: none;
  background: transparent;
  color: var(--fb-blue);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
}

.home__empty {
  margin: 0;
  padding: 14px 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-sub);
}

.home__list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.home__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--fb-line);
}

.home__row:last-child {
  border-bottom: none;
}

.home__row-info {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.home__row-desc {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.home__row-meta {
  font-size: 11px;
  color: var(--fb-sub);
}

/* 转账 badge：中性灰，避免与支出红 / 收入绿混淆 */
.home__badge {
  display: inline-block;
  margin-right: 6px;
  padding: 1px 7px;
  border-radius: var(--fb-radius-pill);
  background: var(--fb-soft);
  color: var(--fb-sub);
  font-size: 10px;
  font-weight: 600;
  line-height: 1.6;
  vertical-align: middle;
}

.home__state {
  margin-top: 14px;
  padding: 20px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--fb-sub);
}

.home__state--error {
  color: var(--fb-danger);
}

.home__state p {
  margin: 0 0 12px;
}

.home__notice {
  margin: 14px 0 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-blue);
}
</style>
