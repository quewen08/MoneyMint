<script setup lang="ts">
/**
 * P-10 币种管理（批 2，只读聚合）
 *
 * ⚠️ 已知契约缺口：后端 31 条端点中无币种管理端点（币种来自账户余额与
 * 交易分录）。本期做只读聚合：每个币种的资产 / 负债 / 净资产小计、涉及
 * 账户数与交易笔数；增删币种待后端端点后开放（见交付说明的已知遗留）。
 */
import { computed, onMounted, ref, watch } from 'vue'

import { accountApi, transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView, TransactionView } from '@/api/types'
import { computeBalanceSummary, type SummaryAccount } from '@/core/record'
import { useSessionStore } from '@/stores/session'
import MoneyText from '@/components/MoneyText.vue'

const session = useSessionStore()

const accounts = ref<AccountView[]>([])
const transactions = ref<TransactionView[]>([])
const loading = ref(false)
const loadError = ref('')

const summary = computed(() =>
  computeBalanceSummary(accounts.value as SummaryAccount[], session.defaultCommodity),
)

interface CommodityRow {
  code: string
  assets: string
  liabilities: string
  netWorth: string
  accountCount: number
  txnCount: number
  isMain: boolean
}

const rows = computed<CommodityRow[]>(() => {
  const codes: string[] = []
  for (const account of accounts.value) {
    const balances = account.balances
    if (balances === undefined) {
      continue
    }
    for (const code of Object.keys(balances)) {
      if (codes.indexOf(code) < 0) {
        codes.push(code)
      }
    }
  }
  for (const txn of transactions.value) {
    for (const posting of txn.postings) {
      if (posting.commodity !== '' && codes.indexOf(posting.commodity) < 0) {
        codes.push(posting.commodity)
      }
    }
  }
  const main = summary.value.commodity
  const out: CommodityRow[] = []
  for (const code of codes) {
    const assets = summary.value.assets[code] === undefined ? '0.00' : summary.value.assets[code]
    const liabilities =
      summary.value.liabilities[code] === undefined ? '0.00' : summary.value.liabilities[code]
    const netWorth = summary.value.netWorth[code] === undefined ? '0.00' : summary.value.netWorth[code]
    let accountCount = 0
    for (const account of accounts.value) {
      const balances = account.balances
      if (balances !== undefined && balances[code] !== undefined) {
        accountCount += 1
      }
    }
    let txnCount = 0
    for (const txn of transactions.value) {
      let hit = false
      for (const posting of txn.postings) {
        if (posting.commodity === code) {
          hit = true
        }
      }
      if (hit) {
        txnCount += 1
      }
    }
    out.push({ code, assets, liabilities, netWorth, accountCount, txnCount, isMain: code === main })
  }
  // 主币种排最前，其余按字母序。
  return out.sort((a, b) => {
    if (a.isMain !== b.isMain) {
      return a.isMain ? -1 : 1
    }
    return a.code < b.code ? -1 : a.code > b.code ? 1 : 0
  })
})

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const [accountRes, txnRes] = await Promise.all([accountApi.list(), transactionApi.list()])
    accounts.value = Array.isArray(accountRes.accounts) ? accountRes.accounts : []
    transactions.value = Array.isArray(txnRes.transactions) ? txnRes.transactions : []
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '币种数据加载失败'
  } finally {
    loading.value = false
  }
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
  <section class="cmd">
    <header class="cmd__hero fb-hero">
      <span class="cmd__label">币种管理 · 账本默认 {{ session.defaultCommodity }}</span>
    </header>

    <div v-if="loadError" class="fb-card cmd__state cmd__state--error">
      <p>{{ loadError }}</p>
      <button class="fb-btn-ghost" type="button" @click="load">重新加载</button>
    </div>
    <div v-else-if="loading" class="fb-card cmd__state">正在加载币种数据…</div>

    <div v-else class="fb-card cmd__card">
      <p v-if="rows.length === 0" class="cmd__empty">账本里还没有任何币种。</p>
      <table v-else class="cmd__table">
        <thead>
          <tr>
            <th>币种</th>
            <th>总资产</th>
            <th>负债</th>
            <th>净资产</th>
            <th>账户</th>
            <th>交易</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.code">
            <td>{{ row.code }}<span v-if="row.isMain" class="cmd__main">默认</span></td>
            <td><MoneyText :amount="row.assets" :commodity="row.code" size="sm" /></td>
            <td><MoneyText :amount="row.liabilities" :commodity="row.code" size="sm" /></td>
            <td><MoneyText :amount="row.netWorth" :commodity="row.code" size="sm" /></td>
            <td>{{ row.accountCount }}</td>
            <td>{{ row.txnCount }}</td>
          </tr>
        </tbody>
      </table>
      <p class="cmd__hint">
        币种清单来自账户余额与交易分录的聚合；币种的增删与汇率管理待后端端点后开放。
      </p>
    </div>
  </section>
</template>

<style scoped>
.cmd {
  padding-bottom: 20px;
}

.cmd__hero {
  padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px;
  border-radius: 0 0 24px 24px;
}

.cmd__label {
  font-size: 15px;
  font-weight: 700;
}

.cmd__card {
  margin-top: 14px;
  padding: 14px 16px;
}

.cmd__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.cmd__table th,
.cmd__table td {
  text-align: left;
  padding: 8px 6px;
  border-bottom: 1px solid var(--fb-line);
  white-space: nowrap;
}

.cmd__table th {
  font-size: 11px;
  color: var(--fb-sub);
  font-weight: 600;
}

.cmd__main {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 6px;
  border-radius: var(--fb-radius-pill);
  background: var(--fb-blue-soft);
  color: var(--fb-blue);
  font-size: 10px;
}

.cmd__empty {
  margin: 0;
  padding: 16px 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-sub);
}

.cmd__hint {
  margin: 12px 0 0;
  font-size: 11px;
  color: var(--fb-sub);
  line-height: 1.7;
}

.cmd__state {
  margin-top: 14px;
  padding: 20px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--fb-sub);
}

.cmd__state--error {
  color: var(--fb-danger);
}

.cmd__state p {
  margin: 0 0 12px;
}
</style>
