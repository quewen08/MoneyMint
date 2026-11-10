<script setup lang="ts">
/**
 * P-08 账户详情（批 2）：余额（按币种）、该账户流水、期初设置入口、关户。
 *
 * 关户守门（AC-28 / AC-29）：
 * - 已关户账户展示 close_date 且不出现写入口；
 * - 关户前二次确认列出「受影响交易笔数」（affectedTransactionCount）；
 * - 服务端 409（余额非 0）时渲染 `ApiError.balances` 差额明细表，不只弹文案。
 */
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { accountApi, transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView, TransactionView } from '@/api/types'
import { affectedTransactionCount, balanceOf, isClosed } from '@/core/accounts'
import { accountLabel, displayAmountOf, transactionKind } from '@/core/record'
import { useSessionStore } from '@/stores/session'
import MoneyText from '@/components/MoneyText.vue'
import AccountForm from '@/components/AccountForm.vue'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()

const accounts = ref<AccountView[]>([])
const transactions = ref<TransactionView[]>([])
const loading = ref(false)
const loadError = ref('')
const notice = ref('')
const closing = ref(false)
const showCloseConfirm = ref(false)
const showOpeningForm = ref(false)
const closeError = ref('')
const closeBalances = ref<Array<{ commodity: string; amount: string }>>([])

const accountId = computed<number | null>(() => {
  const raw = route.params.id
  const value = typeof raw === 'string' ? raw : ''
  if (value === '' || !/^\d+$/.test(value)) {
    return null
  }
  const id = Number(value)
  return Number.isSafeInteger(id) && id > 0 ? id : null
})

const account = computed<AccountView | null>(() => {
  if (accountId.value === null) {
    return null
  }
  for (const item of accounts.value) {
    if (item.id === accountId.value) {
      return item
    }
  }
  return null
})

const balanceCodes = computed(() => {
  if (account.value === null || account.value.balances === undefined) {
    return []
  }
  return Object.keys(account.value.balances)
})

const accountTxns = computed<TransactionView[]>(() => {
  if (accountId.value === null) {
    return []
  }
  return transactions.value.filter((txn) => {
    let hit = false
    for (const posting of txn.postings) {
      if (posting.account_id === accountId.value) {
        hit = true
      }
    }
    return hit
  })
})

const affectedCount = computed(() =>
  accountId.value === null ? 0 : affectedTransactionCount(accountId.value, transactions.value),
)

const closed = computed(() => account.value !== null && isClosed(account.value))

/** 期初 / 关户写入口守卫：角色 + 关户状态（uuid 由列表接口透出，不再置灰）。 */
const canManage = computed(
  () => account.value !== null && !closed.value && !session.isViewer,
)

function rowAmount(txn: TransactionView): string {
  return displayAmountOf(txn.postings, accounts.value)
}

function kindLabel(txn: TransactionView): string {
  const kind = transactionKind(txn.postings, accounts.value)
  if (kind === 'transfer') {
    return '转账'
  }
  return kind === 'income' ? '收入' : '支出'
}

function commodityBalance(code: string): string {
  return account.value === null ? '0.00' : balanceOf(account.value, code)
}

async function load(): Promise<void> {
  if (accountId.value === null) {
    loadError.value = '缺少账户 id，请从账户列表进入'
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const [accountRes, txnRes] = await Promise.all([accountApi.list(), transactionApi.list()])
    accounts.value = Array.isArray(accountRes.accounts) ? accountRes.accounts : []
    transactions.value = Array.isArray(txnRes.transactions) ? txnRes.transactions : []
    if (account.value === null) {
      loadError.value = '未找到该账户（可能已关闭或不在当前账本）'
    }
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '账户详情加载失败'
  } finally {
    loading.value = false
  }
}

function openOpening(): void {
  if (session.isViewer) {
    notice.value = '只读成员不能修改期初'
    return
  }
  if (closed.value) {
    notice.value = '已关户账户不能修改期初'
    return
  }
  notice.value = ''
  showOpeningForm.value = true
}

function openCloseConfirm(): void {
  if (session.isViewer) {
    notice.value = '只读成员不能关户'
    return
  }
  if (closed.value) {
    notice.value = '该账户已关户'
    return
  }
  notice.value = ''
  closeError.value = ''
  closeBalances.value = []
  showCloseConfirm.value = true
}

/** AC-29：确认后走 close（DELETE = Beancount close）；409 时渲染差额明细。 */
async function confirmClose(): Promise<void> {
  if (account.value === null || account.value.uuid === undefined || closing.value) {
    return
  }
  closing.value = true
  closeError.value = ''
  closeBalances.value = []
  try {
    await accountApi.close(account.value.uuid)
    showCloseConfirm.value = false
    notice.value = `已关户「${accountLabel(account.value)}」`
    await load()
  } catch (err) {
    closeError.value = err instanceof ApiError ? err.message : '关户失败，请稍后重试'
    if (err instanceof ApiError && err.balances !== undefined) {
      closeBalances.value = err.balances
    }
  } finally {
    closing.value = false
  }
}

function onOpeningSaved(): void {
  notice.value = '期初已保存（修改期初会由后端生成冲正 + 新建成对交易）'
  void load()
}

function goBack(): void {
  void router.back()
}

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
  <section class="acctd">
    <header class="acctd__hero fb-hero">
      <button type="button" class="acctd__back" @click="goBack">‹ 返回</button>
      <div v-if="account !== null" class="acctd__hero-body">
        <div class="acctd__label">{{ account.type }} · 开户 {{ account.open_date }}<template v-if="closed"> · 已关户（{{ account.close_date }}）</template></div>
        <h1 class="acctd__name">{{ accountLabel(account) }}</h1>
        <ul class="acctd__balances">
          <li v-for="code in balanceCodes" :key="code" class="acctd__balance">
            <span>{{ code }}</span>
            <MoneyText :amount="commodityBalance(code)" :commodity="code" size="sm" />
          </li>
        </ul>
      </div>
    </header>

    <div v-if="loadError" class="fb-card acctd__state acctd__state--error">
      <p>{{ loadError }}</p>
      <button class="fb-btn-ghost" type="button" @click="load">重新加载</button>
    </div>
    <div v-else-if="loading" class="fb-card acctd__state">正在加载账户详情…</div>

    <template v-else-if="account !== null">
      <div class="fb-card acctd__card">
        <button type="button" class="acctd__action" :disabled="!canManage" @click="openOpening">期初余额设置</button>
        <button type="button" class="acctd__action acctd__action--danger" :disabled="!canManage" @click="openCloseConfirm">
          关闭账户（Beancount close）
        </button>
        <p v-if="notice" class="acctd__notice" role="status">{{ notice }}</p>
        <p v-if="closed" class="acctd__hint">已关户账户保留全部历史交易，仅只读。</p>
      </div>

      <div v-if="showCloseConfirm" class="fb-card acctd__card acctd__confirm">
        <h3>确认关闭账户</h3>
        <p class="acctd__warn">该账户出现在 <strong>{{ affectedCount }}</strong> 笔未删除交易中；关闭后会置 close_date 并保留全部交易引用，操作前请确认余额已清零。</p>
        <div v-if="closeError" class="acctd__close-error">
          <p class="acctd__warn acctd__warn--error">{{ closeError }}</p>
          <div v-if="closeBalances.length > 0" class="acctd__diff">
            <p class="acctd__diff-label">按币种差额明细：</p>
            <table>
              <thead><tr><th>币种</th><th>差额</th></tr></thead>
              <tbody>
                <tr v-for="diff in closeBalances" :key="diff.commodity">
                  <td>{{ diff.commodity }}</td>
                  <td><MoneyText :amount="diff.amount" :commodity="diff.commodity" size="sm" /></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div class="acctd__confirm-actions">
          <button type="button" class="fb-btn-ghost" @click="showCloseConfirm = false">取消</button>
          <button type="button" class="fb-btn-primary" :disabled="closing" @click="confirmClose">
            {{ closing ? '关闭中…' : '确认关户' }}
          </button>
        </div>
      </div>

      <div class="fb-card acctd__card">
        <h3>该账户流水（{{ accountTxns.length }} 笔）</h3>
        <p v-if="accountTxns.length === 0" class="acctd__empty">暂无流水。</p>
        <ul v-else class="acctd__list">
          <li v-for="txn in accountTxns" :key="txn.id" class="acctd__row">
            <span class="acctd__row-info">
              <span class="acctd__row-desc">{{ txn.date }} · {{ txn.description }}</span>
              <span class="acctd__row-meta">{{ kindLabel(txn) }}</span>
            </span>
            <MoneyText
              :amount="rowAmount(txn)"
              :commodity="session.defaultCommodity"
              size="sm"
              :signed="transactionKind(txn.postings, accounts) !== 'transfer'"
            />
          </li>
        </ul>
      </div>
    </template>

    <AccountForm
      v-if="showOpeningForm && account !== null"
      mode="opening"
      :account="account"
      @close="showOpeningForm = false"
      @saved="onOpeningSaved"
    />
  </section>
</template>

<style scoped>
.acctd {
  padding-bottom: 20px;
}

.acctd__hero {
  padding: calc(16px + env(safe-area-inset-top, 0px)) 20px 20px;
  border-radius: 0 0 24px 24px;
}

.acctd__back {
  border: none;
  background: var(--fb-on-hero-pill);
  color: var(--fb-white);
  border-radius: var(--fb-radius-pill);
  padding: 5px 12px;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
}

.acctd__hero-body {
  margin-top: 14px;
}

.acctd__label {
  font-size: 12px;
  color: var(--fb-on-hero-muted);
}

.acctd__name {
  margin: 6px 0 0;
  font-size: 20px;
  font-weight: 800;
}

.acctd__balances {
  list-style: none;
  margin: 12px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.acctd__balance {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--fb-on-hero-muted);
}

.acctd__balance :deep(.money-text) {
  color: var(--fb-white);
}

.acctd__balance :deep(.money-text--negative) {
  color: var(--fb-white);
}

.acctd__card {
  margin-top: 14px;
  padding: 14px 16px;
}

.acctd__card h3 {
  margin: 0 0 10px;
  font-size: 14px;
  font-weight: 600;
}

.acctd__action {
  width: 100%;
  height: 44px;
  margin-bottom: 10px;
  border: none;
  border-radius: var(--fb-radius-input);
  background: var(--fb-blue);
  color: var(--fb-white);
  font-size: 14px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
}

.acctd__action:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.acctd__action--danger {
  background: var(--fb-danger);
}

.acctd__notice {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--fb-blue);
}

.acctd__hint {
  margin: 8px 0 0;
  font-size: 11px;
  color: var(--fb-sub);
}

.acctd__confirm h3 {
  color: var(--fb-warn);
}

.acctd__warn {
  margin: 0 0 10px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--fb-ink);
}

.acctd__warn--error {
  color: var(--fb-danger);
}

.acctd__diff-label {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--fb-danger);
  font-weight: 600;
}

.acctd__diff table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.acctd__diff th,
.acctd__diff td {
  text-align: left;
  padding: 4px 8px;
  border-bottom: 1px solid var(--fb-line);
}

.acctd__confirm-actions {
  display: flex;
  gap: 10px;
}

.acctd__confirm-actions > button {
  flex: 1;
}

.acctd__list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.acctd__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--fb-line);
}

.acctd__row:last-child {
  border-bottom: none;
}

.acctd__row-info {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.acctd__row-desc {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.acctd__row-meta {
  font-size: 11px;
  color: var(--fb-sub);
}

.acctd__empty {
  margin: 0;
  padding: 14px 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-sub);
}

.acctd__state {
  margin-top: 14px;
  padding: 20px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--fb-sub);
}

.acctd__state--error {
  color: var(--fb-danger);
}

.acctd__state p {
  margin: 0 0 12px;
}
</style>
