<script setup lang="ts">
/**
 * P-07 账户列表（批 2）：按 Assets / Liabilities 分组 + 余额小计
 * （口径与首页 balanceAccounts 一致：Σ 后端带符号余额，负债组为负）。
 */
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { accountApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView } from '@/api/types'
import { balanceOf, buildAccountTree, flattenAccountTree, groupAccountsByType, isClosed } from '@/core/accounts'
import { accountLabel } from '@/core/record'
import { usePrivacyStore } from '@/stores/privacy'
import { useSessionStore } from '@/stores/session'
import MoneyText from '@/components/MoneyText.vue'

const router = useRouter()
const session = useSessionStore()
const privacy = usePrivacyStore()

const accounts = ref<AccountView[]>([])
const loading = ref(false)
const loadError = ref('')
const notice = ref('')

/** 批 2.5 修 2b：层级树渲染（parent_uuid 优先 / 冒号兜底；父行小计含子聚合）。 */
const groups = computed(() =>
  groupAccountsByType(accounts.value, ['Assets', 'Liabilities']).map((group) => ({
    type: group.type,
    label: group.label,
    subtotal: group.subtotal,
    rows: flattenAccountTree(buildAccountTree(group.items)),
  })),
)

function subtotalOf(group: { subtotal: Record<string, string> }): string {
  const value = group.subtotal[session.defaultCommodity]
  return value === undefined ? '0.00' : value
}

/** 父行（有子）显示聚合小计；叶子行显示自身余额。 */
function rowBalanceText(row: {
  account: AccountView
  childCount: number
  subtotal: Record<string, string>
}): string {
  if (privacy.enabled) {
    return '••••'
  }
  if (row.childCount > 0) {
    const value = row.subtotal[session.defaultCommodity]
    return value === undefined ? '0.00' : value
  }
  return balanceOf(row.account, session.defaultCommodity)
}

function openDetail(account: AccountView): void {
  void router.push({ name: 'account-detail', params: { id: String(account.id) } })
}

function openCreate(): void {
  if (session.isViewer) {
    notice.value = '只读成员不能新建账户'
    return
  }
  notice.value = ''
  void router.push({ name: 'accounts', query: { create: '1' } })
}

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const res = await accountApi.list()
    accounts.value = Array.isArray(res.accounts) ? res.accounts : []
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '账户加载失败'
  } finally {
    loading.value = false
  }
}

watch(
  () => session.ledgerId,
  () => {
    accounts.value = []
    void load()
  },
)

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="accts">
    <header class="accts__hero fb-hero">
      <div class="accts__hero-top">
        <span class="accts__label">账户管理 · {{ session.ledgerName }}</span>
        <button type="button" class="accts__create" @click="openCreate">＋ 新建账户</button>
      </div>
    </header>

    <div v-if="loadError" class="fb-card accts__state accts__state--error">
      <p>{{ loadError }}</p>
      <button class="fb-btn-ghost" type="button" @click="load">重新加载</button>
    </div>
    <div v-else-if="loading" class="fb-card accts__state">正在加载账户…</div>

    <template v-else>
      <div v-for="group in groups" :key="group.type" class="fb-card accts__card">
        <div class="accts__head">
          <h3>{{ group.label }}</h3>
          <MoneyText :amount="subtotalOf(group)" :commodity="session.defaultCommodity" size="sm" />
        </div>
        <p v-if="group.rows.length === 0" class="accts__empty">暂无{{ group.label }}账户。</p>
        <ul v-else class="accts__list">
          <li
            v-for="row in group.rows"
            :key="row.account.id"
            class="accts__row"
            :style="{ paddingLeft: row.depth * 16 + 'px' }"
          >
            <button type="button" class="accts__row-main" @click="openDetail(row.account)">
              <span class="accts__name">
                {{ accountLabel(row.account) }}
                <span v-if="row.childCount > 0" class="accts__closed">含 {{ row.childCount }} 个子账户</span>
                <span v-if="isClosed(row.account)" class="accts__closed">已关户</span>
              </span>
              <span class="accts__meta">{{ row.account.type }} · {{ row.account.open_date }} · {{ rowBalanceText(row) }}</span>
            </button>
          </li>
        </ul>
      </div>

      <p class="accts__hint fb-muted">点击账户查看详情、设置期初余额或关户；新建账户支持同时录入期初。</p>
      <p v-if="notice" class="accts__notice" role="status">{{ notice }}</p>
    </template>
  </section>
</template>

<style scoped>
.accts {
  padding-bottom: 20px;
}

.accts__hero {
  padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px;
  border-radius: 0 0 24px 24px;
}

.accts__hero-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.accts__label {
  font-size: 15px;
  font-weight: 700;
}

.accts__create {
  border: none;
  border-radius: var(--fb-radius-pill);
  background: var(--fb-on-hero-pill);
  color: var(--fb-white);
  padding: 6px 12px;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
}

.accts__card {
  margin-top: 14px;
  padding: 14px 16px;
}

.accts__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.accts__head h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.accts__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--fb-line);
}

.accts__row:last-child {
  border-bottom: none;
}

.accts__row-main {
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

.accts__name {
  font-size: 13px;
  font-weight: 500;
  color: var(--fb-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.accts__closed {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 6px;
  border-radius: var(--fb-radius-pill);
  background: var(--fb-soft);
  color: var(--fb-sub);
  font-size: 10px;
}

.accts__meta {
  font-size: 11px;
  color: var(--fb-sub);
  font-variant-numeric: var(--fb-font-numeric);
}

.accts__empty {
  margin: 0;
  padding: 12px 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-sub);
}

.accts__hint {
  margin: 14px 0 0;
  font-size: 11px;
  text-align: center;
}

.accts__notice {
  margin: 10px 0 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-blue);
}

.accts__state {
  margin-top: 14px;
  padding: 20px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--fb-sub);
}

.accts__state--error {
  color: var(--fb-danger);
}

.accts__state p {
  margin: 0 0 12px;
}
</style>
