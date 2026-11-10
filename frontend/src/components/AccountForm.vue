<script setup lang="ts">
/**
 * C-02 账户表单（批 2，新建 / 编辑期初统一组件）
 *
 * - create：类型 + 显示名（name 自动 = `类型:名称`，名称可含冒号形成三层分类）+
 *   开户日（FR-03：默认 min(账本期初日期, 今日)，且不得晚于账本期初日期）+
 *   可选期初（Assets / Liabilities，金额绝对值 + 方向，FR-04/05）；
 * - opening：只编辑期初（GET 预填 → POST 设置；AC-27：修改已有期初由后端
 *   opening_key 幂等生成「冲正 + 新建」成对交易，前端容忍重试）；
 * - 409 / 校验错误集中展示，带 `balances` 差额明细表（AC-28 关户守门同款）。
 */
import { computed, onMounted, reactive, ref } from 'vue'

import { accountApi, openingApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView, OpeningDirection, OpeningView } from '@/api/types'
import { isClosed } from '@/core/accounts'
import { validateOpeningForm } from '@/core/opening'
import { todayString } from '@/core/record'
import { useSessionStore } from '@/stores/session'
import MoneyText from '@/components/MoneyText.vue'

const props = withDefaults(
  defineProps<{
    mode: 'create' | 'opening'
    account?: AccountView | null
  }>(),
  { account: null },
)

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'saved'): void
}>()

const session = useSessionStore()

const loading = ref(false)
const submitting = ref(false)
const loadError = ref('')
const submitError = ref('')
const errorBalances = ref<Array<{ commodity: string; amount: string }>>([])
const notice = ref('')

const accountType = ref<'Assets' | 'Liabilities' | 'Expenses' | 'Income' | 'Equity'>('Assets')
const displayName = ref('')
const openDate = ref(todayString())

// 批 2.5 修 2c：父账户选择（create 模式；可选，选中后 name 自动拼前缀）。
const parentAccountId = ref<number | null>(null)
const knownAccounts = ref<AccountView[]>([])

const openingAmount = ref('')
const openingDirection = ref<OpeningDirection>('hold')
const openingHasExisting = ref(false)
const openingLoaded = ref(false)

const ledgerOpeningDate = computed(() => {
  for (const ledger of session.ledgers) {
    if (ledger.id === session.ledgerId && ledger.opening_date !== '') {
      return ledger.opening_date
    }
  }
  return todayString()
})

const isFundType = computed(
  () => accountType.value === 'Assets' || accountType.value === 'Liabilities',
)

/** 同类型、未关闭的可选父账户（批 2.5 修 2c）。 */
const parentOptions = computed(() =>
  knownAccounts.value.filter(
    (account) => account.type === accountType.value && !isClosed(account),
  ),
)

function resolveParent(): AccountView | null {
  if (parentAccountId.value === null) {
    return null
  }
  for (const account of knownAccounts.value) {
    if (account.id === parentAccountId.value) {
      return account
    }
  }
  return null
}

function onParentChange(event: Event): void {
  const raw = (event.target as HTMLSelectElement).value
  if (raw === '' || !/^\d+$/.test(raw)) {
    parentAccountId.value = null
    return
  }
  const id = Number(raw)
  parentAccountId.value = Number.isSafeInteger(id) && id > 0 ? id : null
}

async function loadKnownAccounts(): Promise<void> {
  try {
    const res = await accountApi.list()
    knownAccounts.value = Array.isArray(res.accounts) ? res.accounts : []
  } catch {
    // 父下拉为增强项，加载失败不阻断新建。
    knownAccounts.value = []
  }
}

/** FR-03：新建账户开户日默认 min(账本期初日期, 今日)，且不得晚于账本期初日期。 */
function defaultOpenDate(): string {
  return ledgerOpeningDate.value < todayString() ? ledgerOpeningDate.value : todayString()
}

function switchType(next: typeof accountType.value): void {
  accountType.value = next
  openingDirection.value = next === 'Liabilities' ? 'owe' : 'hold'
  parentAccountId.value = null
}

function onOpenDateChange(event: Event): void {
  openDate.value = (event.target as HTMLInputElement).value
}

function onOpeningAmountChange(event: Event): void {
  openingAmount.value = (event.target as HTMLInputElement).value
}

function onDirectionChange(event: Event): void {
  const value = (event.target as HTMLSelectElement).value
  openingDirection.value = value === 'owe' ? 'owe' : 'hold'
}

function clearErrors(): void {
  submitError.value = ''
  errorBalances.value = []
  notice.value = ''
}

async function loadOpening(): Promise<void> {
  if (props.account === null) {
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const view: OpeningView = await openingApi.get(props.account.uuid)
    openingHasExisting.value = view.has_opening === true
    if (view.has_opening && typeof view.amount === 'string') {
      openingAmount.value = view.amount
    }
    if (view.direction === 'hold' || view.direction === 'owe') {
      openingDirection.value = view.direction
    }
    openingLoaded.value = true
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '期初信息加载失败'
  } finally {
    loading.value = false
  }
}

async function submitOpening(accountUuid: string, commodity: string): Promise<void> {
  const validation = validateOpeningForm({
    amount: openingAmount.value,
    direction: openingDirection.value,
    accountType: isFundType.value ? accountType.value : '',
    accountOpenDate: props.account === null ? openDate.value : (props.account.open_date ?? ''),
    ledgerOpeningDate: ledgerOpeningDate.value,
  })
  if (!validation.ok || validation.absAmount === null) {
    submitError.value = validation.errors.join('；')
    return
  }
  if (validation.absAmount === '0.00') {
    notice.value = '期初为 0，跳过期初录入'
    return
  }
  await openingApi.set(accountUuid, {
    amount: validation.absAmount,
    direction: openingDirection.value,
    commodity,
    date: ledgerOpeningDate.value,
  })
}

async function submit(): Promise<void> {
  if (submitting.value) {
    return
  }
  clearErrors()
  if (session.isViewer) {
    submitError.value = '只读成员不能修改账户'
    return
  }
  submitting.value = true
  try {
    if (props.mode === 'opening') {
      if (props.account === null || props.account.uuid === undefined) {
        submitError.value = '缺少账户上下文'
        return
      }
      await submitOpening(props.account.uuid, resolveCommodity(props.account))
    } else {
      const name = displayName.value.trim()
      if (name === '') {
        submitError.value = '请输入账户 / 分类名称'
        return
      }
      if (openDate.value > ledgerOpeningDate.value) {
        submitError.value = `开户日（${openDate.value}）不得晚于账本期初日期（${ledgerOpeningDate.value}）`
        return
      }
      // 批 2.5 修 2c：选中父账户时 name 自动拼接其全名前缀（三层分类）。
      const parent = resolveParent()
      const fullName =
        parent !== null ? `${parent.name}:${name}` : `${accountType.value}:${name}`
      const created = await accountApi.create({
        name: fullName,
        display_name: name,
        type: accountType.value,
        open_date: openDate.value,
        parent_uuid: parent !== null ? parent.uuid : undefined,
      })
      if (isFundType.value && openingAmount.value.trim() !== '') {
        await submitOpening(created.uuid, resolveCommodity(created as unknown as AccountView))
      } else {
        notice.value = '账户已创建'
      }
    }
    emit('saved')
    emit('close')
  } catch (err) {
    submitError.value = err instanceof ApiError ? err.message : '保存失败，请稍后重试'
    if (err instanceof ApiError && err.balances !== undefined) {
      errorBalances.value = err.balances
    }
  } finally {
    submitting.value = false
  }
}

function resolveCommodity(account: AccountView | null): string {
  if (account === null) {
    return session.defaultCommodity
  }
  if (account.commodity_restriction !== undefined && account.commodity_restriction !== '') {
    return account.commodity_restriction
  }
  return session.defaultCommodity
}

function close(): void {
  emit('close')
}

onMounted(() => {
  if (props.mode === 'create') {
    openDate.value = defaultOpenDate()
    void loadKnownAccounts()
  } else {
    void loadOpening()
  }
})
</script>

<template>
  <div class="af-mask" @click.self="close">
    <section class="af-panel" role="dialog" aria-modal="true" :aria-label="mode === 'create' ? '新建账户' : '期初余额'">
      <header class="af-head">
        <h3>{{ mode === 'create' ? '新建账户' : `期初余额 · ${account !== null ? account.name : ''}` }}</h3>
        <button class="af-close" type="button" aria-label="关闭" @click="close">✕</button>
      </header>

      <div v-if="loadError" class="af-alert af-alert--error">{{ loadError }}</div>
      <div v-if="submitError" class="af-alert af-alert--error" role="alert">{{ submitError }}</div>
      <div v-if="notice" class="af-alert af-alert--ok">{{ notice }}</div>

      <div v-if="errorBalances.length > 0" class="af-balances">
        <p class="af-balances__label">按币种差额明细（关户 / 冲正守门）：</p>
        <table>
          <thead><tr><th>币种</th><th>差额</th></tr></thead>
          <tbody>
            <tr v-for="diff in errorBalances" :key="diff.commodity">
              <td>{{ diff.commodity }}</td>
              <td><MoneyText :amount="diff.amount" :commodity="diff.commodity" size="sm" /></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="loading" class="af-body af-muted">正在加载期初信息…</div>

      <div v-else class="af-body">
        <template v-if="mode === 'create'">
          <label class="af-field">
            <span>类型</span>
            <select class="fb-input" :value="accountType" @change="switchType(($event.target as HTMLSelectElement).value as typeof accountType)">
              <option value="Assets">资产（Assets）</option>
              <option value="Liabilities">负债（Liabilities）</option>
              <option value="Expenses">支出分类（Expenses）</option>
              <option value="Income">收入分类（Income）</option>
              <option value="Equity">权益（Equity）</option>
            </select>
          </label>
          <label class="af-field">
            <span>名称（可含冒号形成三级分类，如 `餐饮:早餐`）</span>
            <input class="fb-input" v-model="displayName" placeholder="如：招商储蓄 / 餐饮" />
          </label>
          <label class="af-field">
            <span>父账户（可选；选中后名称自动拼接其前缀）</span>
            <select class="fb-input" :value="parentAccountId === null ? '' : String(parentAccountId)" @change="onParentChange">
              <option value="">无（直接挂在类型根下）</option>
              <option v-for="acc in parentOptions" :key="acc.id" :value="String(acc.id)">
                {{ acc.name }}
              </option>
            </select>
          </label>
          <label class="af-field">
            <span>开户日（不得晚于账本期初日期 {{ ledgerOpeningDate }}）</span>
            <input class="fb-input" type="date" :value="openDate" @change="onOpenDateChange" />
          </label>

          <template v-if="isFundType">
            <div class="af-section">期初余额（可选）</div>
            <div class="af-row">
              <label class="af-field">
                <span>金额（绝对值）</span>
                <input class="fb-input" :value="openingAmount" @change="onOpeningAmountChange" placeholder="如：3200" />
              </label>
              <label class="af-field">
                <span>方向</span>
                <select class="fb-input" :value="openingDirection" @change="onDirectionChange">
                  <option value="hold">持有</option>
                  <option value="owe">欠款</option>
                </select>
              </label>
            </div>
            <p class="af-hint">持有 = 正、欠款 = 负；期初交易日期固定为账本期初日期 {{ ledgerOpeningDate }}。</p>
          </template>
          <p v-else class="af-hint">{{ accountType }} 类型账户不设期初余额。</p>
        </template>

        <template v-else>
          <p class="af-hint" v-if="openingHasExisting">当前账户已有期初，修改保存后由后端生成「冲正 + 新建」成对交易（重复提交由幂等键保证只生效一次）。</p>
          <p class="af-hint" v-else>该账户尚未设置期初。</p>
          <div class="af-row">
            <label class="af-field">
              <span>金额（绝对值）</span>
              <input class="fb-input" :value="openingAmount" @change="onOpeningAmountChange" placeholder="如：12500" />
            </label>
            <label class="af-field">
              <span>方向</span>
              <select class="fb-input" :value="openingDirection" @change="onDirectionChange">
                <option value="hold">持有</option>
                <option value="owe">欠款</option>
              </select>
            </label>
          </div>
          <p class="af-hint">期初交易日期 = 账本期初日期 {{ ledgerOpeningDate }}；币种 {{ resolveCommodity(account) }}。</p>
        </template>
      </div>

      <div class="af-foot">
        <button type="button" class="fb-btn-ghost" @click="close">取消</button>
        <button type="button" class="fb-btn-primary" :disabled="submitting || loading" @click="submit">
          {{ submitting ? '保存中…' : '保存' }}
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.af-mask {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background: rgba(20, 28, 48, 0.45);
  z-index: 60;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.af-panel {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-height: 92vh;
  background: var(--fb-card);
  border-radius: 20px 20px 0 0;
  overflow: hidden;
}

@media (min-width: 1024px) {
  .af-mask {
    align-items: center;
  }

  .af-panel {
    width: 440px;
    max-width: 92vw;
    max-height: 88vh;
    border-radius: 18px;
  }
}

.af-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--fb-line);
}

.af-head h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.af-close {
  border: none;
  background: transparent;
  color: var(--fb-sub);
  font-size: 16px;
  cursor: pointer;
}

.af-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 18px 16px;
}

.af-field {
  display: block;
  margin-top: 10px;
}

.af-field > span {
  display: block;
  font-size: 12px;
  color: var(--fb-sub);
  margin-bottom: 6px;
}

.af-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.af-section {
  margin-top: 14px;
  font-size: 12px;
  font-weight: 600;
  color: var(--fb-ink);
}

.af-hint {
  margin: 10px 0 0;
  font-size: 11px;
  line-height: 1.7;
  color: var(--fb-sub);
}

.af-alert {
  margin: 8px 18px 0;
  padding: 8px 12px;
  border-radius: var(--fb-radius-input);
  font-size: 12px;
}

.af-alert--error {
  background: var(--fb-danger-soft);
  color: var(--fb-danger);
}

.af-alert--ok {
  background: var(--fb-green-soft);
  color: var(--fb-green);
}

.af-balances {
  margin: 8px 18px 0;
  padding: 10px 12px;
  border-radius: var(--fb-radius-input);
  background: var(--fb-danger-soft);
}

.af-balances__label {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--fb-danger);
  font-weight: 600;
}

.af-balances table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.af-balances th,
.af-balances td {
  text-align: left;
  padding: 4px 8px;
  border-bottom: 1px solid var(--fb-line);
}

.af-foot {
  display: flex;
  gap: 10px;
  padding: 10px 18px calc(10px + env(safe-area-inset-bottom, 0px));
}

.af-foot > button {
  flex: 1;
}
</style>
