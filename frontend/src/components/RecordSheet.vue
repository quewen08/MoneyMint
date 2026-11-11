<script setup lang="ts">
/**
 * C-01 记一笔（4 个 Dart 文件 → 1 个组件，B-16）
 *
 * - 支出 / 收入 / 转账三 Tab + 金额大屏 + 数字键盘 + 分类九宫格 +
 *   资金账户必选 + 分录预览（沿用 v2 原型 F-3 视觉与功能）；
 * - PC 弹窗 / 移动 Sheet / 全屏三种容器收敛为同一组件，靠 CSS 断点 + variant 切换；
 * - 在线 postings 用 `account_id` 整数，金额一律定点十进制字符串；
 * - G-04 幂等：每次提交生成新 UUID 作 `Idempotency-Key`，同内容重试复用，
 *   内容变化或命中软删 / payload 冲突时自动换新键；
 * - 连续记账（再记一笔 / 复制上一笔）+ 按账本命名空间暂存的草稿（AC-14 / AC-32）。
 */
import { computed, onMounted, reactive, ref, watch } from 'vue'

import {
  accountApi,
  newIdempotencyKey,
  transactionApi,
  type TransactionCreateResult,
} from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView, CreateTransactionRequest, TxnFlag } from '@/api/types'
import { currencySymbol, formatMoney, Money } from '@/core/Money'
import {
  accountLabel,
  buildPostings,
  evaluateAmountExpression,
  todayString,
  type PostingDraft,
  type RecordForm,
  type RecordKind,
} from '@/core/record'
import { recordFundAccounts } from '@/core/accounts'
import { balanceDiffByCommodity } from '@/core/record'
import {
  clearDraft,
  readDefaultAccounts,
  readDraft,
  setDefaultAccount,
  writeDraft,
  type DefaultAccountKind,
} from '@/core/storage'
import { usePrivacyStore } from '@/stores/privacy'
import { useRecordSheetStore } from '@/stores/recordSheet'
import { useSessionStore } from '@/stores/session'

const props = withDefaults(
  defineProps<{
    /** auto：按 1024 断点自动切换；其余用于强制指定容器形态。 */
    variant?: 'auto' | 'dialog' | 'sheet' | 'fullscreen'
  }>(),
  { variant: 'auto' },
)

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'saved', result: TransactionCreateResult): void
}>()

const session = useSessionStore()
const sheet = useRecordSheetStore()
const privacy = usePrivacyStore()

const accounts = ref<AccountView[]>([])
const loading = ref(false)
const loadError = ref('')
const submitting = ref(false)
const submitError = ref('')
const notice = ref('')
const showSuccess = ref(false)
const lastResult = ref<TransactionCreateResult | null>(null)

const form = reactive<RecordForm>({
  kind: 'expense',
  amountExpression: '',
  date: todayString(),
  description: '',
  tags: [],
  categoryAccountId: null,
  fundAccountId: null,
  transferFromAccountId: null,
  transferToAccountId: null,
  advancedPostings: [],
})

// 批 2.5 修 5：高级双录模式（默认仍简化三 Tab）。
const advancedMode = ref(false)
const advancedError = ref('')

// 幂等键生命周期：提交成功 / 表单语义变化后换新键，纯网络重试复用。
let idempotencyKey: string | null = null
let lastPayloadJson = ''

const tabs: Array<{ key: RecordKind; label: string }> = [
  { key: 'expense', label: '支出' },
  { key: 'income', label: '收入' },
  { key: 'transfer', label: '转账' },
]
const keyboardKeys: string[] = [
  '7', '8', '9', '⌫',
  '4', '5', '6', '＋',
  '1', '2', '3', '－',
  '.', '0', '再记', '保存',
]

const containerClass = computed(() =>
  props.variant === 'auto' ? 'rs-mask--auto' : `rs-mask--${props.variant}`,
)

const openAccounts = computed(() =>
  accounts.value.filter((account) => !account.close_date),
)
const expenseAccounts = computed(() =>
  openAccounts.value.filter((account) => account.type === 'Expenses'),
)
const incomeAccounts = computed(() =>
  openAccounts.value.filter((account) => account.type === 'Income'),
)
// AC-28：记一笔可选资金账户 = 未关闭的 Assets / Liabilities（统一 core 口径）。
const fundAccounts = computed(() => recordFundAccounts(accounts.value))
const categoryOptions = computed(() =>
  form.kind === 'income' ? incomeAccounts.value : expenseAccounts.value,
)
const categoryGrid = computed(() => categoryOptions.value.slice(0, 9))

const amountMoney = computed<Money>(() => {
  if (form.amountExpression === '') {
    return Money.zero()
  }
  try {
    return evaluateAmountExpression(form.amountExpression)
  } catch {
    return Money.zero()
  }
})
const amountDisplay = computed(() => amountMoney.value.formatAmount())
const currency = computed(() => {
  const symbol = currencySymbol(session.defaultCommodity)
  return symbol !== '' ? symbol : session.defaultCommodity
})

const snapshot = (): RecordForm => ({ ...form, tags: form.tags.slice() })

const preview = computed<PostingDraft[]>(() => {
  if (accounts.value.length === 0) {
    return []
  }
  try {
    return buildPostings(snapshot(), accounts.value, session.defaultCommodity)
  } catch {
    return []
  }
})
const previewError = computed<string>(() => {
  if (accounts.value.length === 0) {
    return '账户列表加载中…'
  }
  try {
    buildPostings(snapshot(), accounts.value, session.defaultCommodity)
    return ''
  } catch (err) {
    return err instanceof Error ? err.message : '表单校验失败'
  }
})

const canSubmit = computed(
  () =>
    !submitting.value &&
    accounts.value.length > 0 &&
    session.ledgerId !== null &&
    !session.isViewer &&
    (advancedMode.value
      ? (form.advancedPostings?.length ?? 0) > 0
      : preview.value.length > 0),
)

const tagsText = computed(() => form.tags.join(','))
const categorySelectValue = computed(() =>
  form.categoryAccountId === null ? '' : String(form.categoryAccountId),
)
const fundValue = computed(() =>
  form.fundAccountId === null ? '' : String(form.fundAccountId),
)
const transferFromValue = computed(() =>
  form.transferFromAccountId === null ? '' : String(form.transferFromAccountId),
)
const transferToValue = computed(() =>
  form.transferToAccountId === null ? '' : String(form.transferToAccountId),
)

function resetIdempotencyKey(): void {
  idempotencyKey = null
  lastPayloadJson = ''
}

function clearDraftIfPossible(): void {
  if (session.ledgerId !== null) {
    clearDraft(session.ledgerId)
  }
}

/** 恢复本账本的草稿（不含任何凭据），逐字段做类型校验避免脏数据。 */
function restoreDraft(): void {
  if (session.ledgerId === null) {
    return
  }
  const draft = readDraft<Partial<RecordForm>>(session.ledgerId)
  if (draft === null) {
    return
  }
  if (draft.kind === 'expense' || draft.kind === 'income' || draft.kind === 'transfer') {
    form.kind = draft.kind
  }
  if (typeof draft.amountExpression === 'string') {
    form.amountExpression = draft.amountExpression
  }
  if (typeof draft.date === 'string' && draft.date !== '') {
    form.date = draft.date
  }
  if (typeof draft.description === 'string') {
    form.description = draft.description
  }
  if (Array.isArray(draft.tags)) {
    form.tags = draft.tags.filter((tag) => typeof tag === 'string')
  }
  if (typeof draft.categoryAccountId === 'number') {
    form.categoryAccountId = draft.categoryAccountId
  }
  if (typeof draft.fundAccountId === 'number') {
    form.fundAccountId = draft.fundAccountId
  }
  if (typeof draft.transferFromAccountId === 'number') {
    form.transferFromAccountId = draft.transferFromAccountId
  }
  if (typeof draft.transferToAccountId === 'number') {
    form.transferToAccountId = draft.transferToAccountId
  }
  if (Array.isArray(draft.advancedPostings)) {
    const rows: Array<{ accountId: number | null; amount: string }> = []
    for (const row of draft.advancedPostings) {
      if (
        typeof row === 'object' &&
        row !== null &&
        typeof (row as { accountId?: unknown }).accountId === 'number' &&
        typeof (row as { amount?: unknown }).amount === 'string'
      ) {
        rows.push({
          accountId: (row as { accountId: number }).accountId,
          amount: (row as { amount: string }).amount,
        })
      }
    }
    form.advancedPostings = rows
  }
  notice.value = '已恢复上次未提交的表单草稿'
}

// ---------- §14.5 #5：默认账户预选 / 回写（批 3） ----------

/** 按 uuid 反查可用账户 id；已删除 / 已关户返回 null，由调用方回退首个可用账户。 */
function defaultAccountId(uuid: string | undefined): number | null {
  if (uuid === undefined || uuid === '') {
    return null
  }
  for (const account of accounts.value) {
    if (account.uuid === uuid && (account.close_date === undefined || account.close_date === '')) {
      return account.id
    }
  }
  return null
}

/** 账户 id → uuid（旧测试固件的账户可能没有 uuid，容错为空串）。 */
function uuidOf(accountId: number | null): string {
  if (accountId === null) {
    return ''
  }
  for (const account of accounts.value) {
    if (account.id === accountId && typeof account.uuid === 'string') {
      return account.uuid
    }
  }
  return ''
}

/** 读取默认账户；草稿已指定的字段优先，失效记忆回退首个可用账户并提示。 */
function applyDefaultAccounts(): void {
  const id = session.ledgerId
  if (id === null) {
    return
  }
  const defaults = readDefaultAccounts(id)
  const categoryIds: number[] = categoryOptions.value.map((account) => account.id)
  const fundIds: number[] = fundAccounts.value.map((account) => account.id)
  let invalid = false

  const applyField = (
    field: 'categoryAccountId' | 'transferFromAccountId' | 'transferToAccountId',
    uuid: string | undefined,
    allowed: number[],
  ): void => {
    if (form[field] !== null || uuid === undefined || uuid === '') {
      return
    }
    const found = defaultAccountId(uuid)
    if (found !== null && allowed.indexOf(found) >= 0) {
      form[field] = found
    } else if (found === null) {
      invalid = true
    }
  }

  applyField('categoryAccountId', form.kind === 'income' ? defaults.income : defaults.expense, categoryIds)
  applyField('transferFromAccountId', defaults.transferOut, fundIds)
  applyField('transferToAccountId', defaults.transferIn, fundIds)
  if (invalid) {
    notice.value = '默认账户已失效，已回退为首个可用账户'
  }
}

/** 提交成功后记住本次选择（按账本命名空间，不写后端、不含凭据）。 */
function rememberDefaults(): void {
  const id = session.ledgerId
  if (id === null) {
    return
  }
  if (form.kind === 'transfer') {
    setDefaultAccount(id, 'transferOut', uuidOf(form.transferFromAccountId))
    setDefaultAccount(id, 'transferIn', uuidOf(form.transferToAccountId))
  } else {
    const kind: DefaultAccountKind = form.kind
    setDefaultAccount(id, kind, uuidOf(form.categoryAccountId))
  }
}

async function loadAccounts(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const res = await accountApi.list()
    accounts.value = Array.isArray(res.accounts) ? res.accounts : []
    applyDefaultAccounts()
    if (form.fundAccountId === null && fundAccounts.value.length > 0) {
      form.fundAccountId = fundAccounts.value[0].id
    }
    if (form.categoryAccountId === null && categoryOptions.value.length > 0) {
      form.categoryAccountId = categoryOptions.value[0].id
    }
    if (form.transferFromAccountId === null && fundAccounts.value.length > 0) {
      form.transferFromAccountId = fundAccounts.value[0].id
    }
    if (form.transferToAccountId === null && fundAccounts.value.length > 1) {
      form.transferToAccountId = fundAccounts.value[1].id
    }
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '账户列表加载失败'
  } finally {
    loading.value = false
  }
}

function selectedId(event: Event): number | null {
  const target = event.target as HTMLSelectElement
  if (target.value === '') {
    return null
  }
  const id = Number(target.value)
  return Number.isSafeInteger(id) && id > 0 ? id : null
}

/**
 * 表单改动不直接丢弃幂等键：是否换新键由 submit 时的 payload 快照比较决定，
 * 只有「内容真的变了」才换键（避免同 key 不同 payload 触发 409），
 * 纯重试或改回原值都复用同一 key（AC-13 账本底线）。
 */
function onCategorySelect(event: Event): void {
  form.categoryAccountId = selectedId(event)
}

function onFundSelect(event: Event): void {
  form.fundAccountId = selectedId(event)
}

function onTransferFrom(event: Event): void {
  form.transferFromAccountId = selectedId(event)
}

function onTransferTo(event: Event): void {
  form.transferToAccountId = selectedId(event)
}

function onTagsChange(event: Event): void {
  const target = event.target as HTMLInputElement
  const tags: string[] = []
  const raw = target.value.split(',')
  for (const part of raw) {
    const trimmed = part.trim()
    if (trimmed !== '') {
      tags.push(trimmed)
    }
  }
  form.tags = tags
}

function selectCategory(id: number): void {
  form.categoryAccountId = id
}

function switchKind(kind: RecordKind): void {
  form.kind = kind
  if (kind === 'income' || kind === 'expense') {
    // §14.5 #5：切 Tab 时也读取对应类型的默认分类，失效则回退首个可用。
    const defaults =
      session.ledgerId === null ? {} : readDefaultAccounts(session.ledgerId)
    const preferred = defaultAccountId(kind === 'income' ? defaults.income : defaults.expense)
    const options = kind === 'income' ? incomeAccounts.value : expenseAccounts.value
    const allowed = preferred !== null && options.some((account) => account.id === preferred)
    form.categoryAccountId =
      allowed && preferred !== null
        ? preferred
        : options.length > 0
          ? options[0].id
          : null
  }
  submitError.value = ''
  notice.value = ''
}

function currentSegment(expr: string): string {
  let start = 0
  for (let i = expr.length - 1; i >= 0; i -= 1) {
    const ch = expr.charAt(i)
    if (ch === '+' || ch === '-') {
      start = i + 1
      break
    }
  }
  return expr.slice(start)
}

function pressKey(key: string): void {
  if (key === '保存') {
    void submit(false)
    return
  }
  if (key === '再记') {
    void submit(true)
    return
  }
  if (key === '⌫') {
    form.amountExpression = form.amountExpression.slice(0, -1)
    return
  }
  if (key === '＋' || key === '－') {
    const last = form.amountExpression.slice(-1)
    if (last === '+' || last === '-') {
      form.amountExpression = form.amountExpression.slice(0, -1)
    }
    if (form.amountExpression === '') {
      return
    }
    form.amountExpression += key === '＋' ? '+' : '-'
    return
  }
  const segment = currentSegment(form.amountExpression)
  if (key === '.') {
    if (segment.indexOf('.') >= 0) {
      return
    }
    form.amountExpression += segment === '' ? '0.' : '.'
    return
  }
  if (/^[0-9]$/.test(key)) {
    const dot = segment.indexOf('.')
    if (dot >= 0 && segment.length - dot - 1 >= 2) {
      return
    }
    form.amountExpression += key
  }
}

function keyClass(key: string): string {
  if (key === '保存') {
    return 'rs-key rs-key--save'
  }
  if (key === '再记') {
    return 'rs-key rs-key--again'
  }
  if (key === '⌫' || key === '＋' || key === '－') {
    return 'rs-key rs-key--op'
  }
  return 'rs-key'
}

function defaultDescription(): string {
  if (form.kind === 'transfer') {
    return '转账'
  }
  const category = categoryOptions.value.find(
    (account) => account.id === form.categoryAccountId,
  )
  return category === undefined ? '记一笔' : accountLabel(category)
}

function balanceText(account: AccountView): string {
  if (privacy.enabled) {
    return '••••'
  }
  const balances = account.balances
  if (balances === undefined) {
    return formatMoney('0.00', session.defaultCommodity)
  }
  const value = balances[session.defaultCommodity]
  return formatMoney(value === undefined ? '0.00' : value, session.defaultCommodity)
}

function catIcon(account: AccountView): string {
  return account.icon !== undefined && account.icon !== '' ? account.icon : '📁'
}

// ─────────── 批 2.5 修 5：高级双录（逐条编辑双录分录） ───────────

/** 进入高级模式时以简化模式生成的配平分录预填；退出即清空。 */
function toggleAdvanced(): void {
  advancedError.value = ''
  if (!advancedMode.value) {
    const current = preview.value
    form.advancedPostings =
      current.length > 0
        ? current.map((posting) => ({ accountId: posting.accountId, amount: posting.amount }))
        : defaultAdvancedRows()
  } else {
    form.advancedPostings = []
  }
  advancedMode.value = !advancedMode.value
}

function defaultAdvancedRows(): Array<{ accountId: number | null; amount: string }> {
  const rows: Array<{ accountId: number | null; amount: string }> = []
  for (const account of openAccounts.value.slice(0, 2)) {
    rows.push({ accountId: account.id, amount: '' })
  }
  while (rows.length < 2) {
    const first = openAccounts.value[0]
    rows.push({ accountId: first === undefined ? null : first.id, amount: '' })
  }
  return rows
}

function onAdvancedAccount(index: number, event: Event): void {
  const row = form.advancedPostings?.[index]
  if (row === undefined) {
    return
  }
  const raw = (event.target as HTMLSelectElement).value
  if (raw === '' || !/^\d+$/.test(raw)) {
    row.accountId = null
  } else {
    const id = Number(raw)
    row.accountId = Number.isSafeInteger(id) && id > 0 ? id : null
  }
  resetIdempotencyKey()
}

function onAdvancedAmount(index: number, event: Event): void {
  const row = form.advancedPostings?.[index]
  if (row === undefined) {
    return
  }
  row.amount = (event.target as HTMLInputElement).value
  resetIdempotencyKey()
}

function addAdvancedRow(): void {
  if (form.advancedPostings === undefined) {
    form.advancedPostings = []
  }
  const first = openAccounts.value[0]
  form.advancedPostings.push({ accountId: first === undefined ? null : first.id, amount: '' })
  resetIdempotencyKey()
}

function removeAdvancedRow(index: number): void {
  form.advancedPostings?.splice(index, 1)
  resetIdempotencyKey()
}

function resolveAccountCommodity(account: AccountView): string {
  return account.commodity_restriction !== undefined && account.commodity_restriction !== ''
    ? account.commodity_restriction
    : session.defaultCommodity
}

/**
 * 高级双录分录构造：账户必选 + 金额非空 + 按 commodity 分组合计为 0
 * （core.balanceDiffByCommodity，差额明细直出）。不平返回 null 并写 advancedError。
 */
function buildAdvancedPostings(): PostingDraft[] | null {
  advancedError.value = ''
  const rows = form.advancedPostings ?? []
  if (rows.length < 2) {
    advancedError.value = '高级双录至少需要 2 条分录'
    return null
  }
  const drafts: PostingDraft[] = []
  for (const row of rows) {
    if (row.accountId === null) {
      advancedError.value = '存在未选择账户的分录'
      return null
    }
    const account = accounts.value.find((item) => item.id === row.accountId)
    if (account === undefined) {
      advancedError.value = '分录账户不在当前账本'
      return null
    }
    if (row.amount.trim() === '') {
      advancedError.value = '存在空金额的分录'
      return null
    }
    drafts.push({
      accountId: row.accountId,
      accountName: accountLabel(account),
      commodity: resolveAccountCommodity(account),
      amount: row.amount.trim(),
    })
  }
  const diffs = balanceDiffByCommodity(drafts)
  if (diffs.length > 0) {
    const parts: string[] = []
    for (const diff of diffs) {
      parts.push(`${diff.commodity} 差额 ${diff.diff}`)
    }
    advancedError.value = `分录不配平：${parts.join('；')}`
    return null
  }
  return drafts
}

/**
 * 提交：生成本次幂等键（或复用同内容重试的键），postings 全为整数 account_id。
 * 失败保留表单与幂等键，供用户原样重试（AC-13 / AC-32）。
 */
async function submit(continueAfter: boolean): Promise<void> {
  if (submitting.value) {
    return
  }
  submitError.value = ''
  if (session.isViewer) {
    submitError.value = '只读成员不能记账'
    return
  }
  if (session.ledgerId === null) {
    submitError.value = '尚未选择账本，无法提交'
    return
  }

  let postings: PostingDraft[]
  if (advancedMode.value) {
    // 批 2.5 修 5：高级双录逐条编辑，按币种配平校验（差额明细给出）后
    // 走与简化模式**完全相同**的提交路径（幂等键 / 草稿语义不变）。
    const built = buildAdvancedPostings()
    if (built === null) {
      submitError.value = advancedError.value === '' ? '高级分录校验失败' : advancedError.value
      return
    }
    postings = built
  } else {
    try {
      postings = buildPostings(snapshot(), accounts.value, session.defaultCommodity)
    } catch (err) {
      submitError.value = err instanceof Error ? err.message : '表单校验失败'
      return
    }
  }
  const payload: CreateTransactionRequest = {
    date: form.date,
    flag: '*' as TxnFlag,
    description:
      form.description.trim() !== '' ? form.description.trim() : defaultDescription(),
    tags: form.tags.slice(),
    postings: postings.map((posting) => ({
      account_id: posting.accountId,
      commodity: posting.commodity,
      amount: posting.amount,
    })),
  }
  const payloadJson = JSON.stringify(payload)
  if (
    idempotencyKey === null ||
    (lastPayloadJson !== '' && lastPayloadJson !== payloadJson)
  ) {
    idempotencyKey = newIdempotencyKey()
  }
  lastPayloadJson = payloadJson

  submitting.value = true
  try {
    const result = await transactionApi.create(payload, idempotencyKey)
    clearDraftIfPossible()
    resetIdempotencyKey()
    lastResult.value = result
    rememberDefaults()
    sheet.notifySaved(result)
    emit('saved', result)
    if (continueAfter) {
      resetForm()
      notice.value = '已保存，可继续记一笔'
    } else {
      showSuccess.value = true
    }
  } catch (err) {
    submitError.value =
      err instanceof ApiError ? err.message : '保存失败，请检查网络后重试'
    if (
      err instanceof ApiError &&
      (err.code === 'idempotency_key_deleted' ||
        err.code === 'idempotency_payload_mismatch')
    ) {
      resetIdempotencyKey()
    }
  } finally {
    submitting.value = false
  }
}

function resetForm(): void {
  form.amountExpression = ''
  form.description = ''
  form.tags = []
  form.advancedPostings = []
  resetIdempotencyKey()
  showSuccess.value = false
  submitError.value = ''
  advancedError.value = ''
  clearDraftIfPossible()
}

function again(): void {
  resetForm()
  notice.value = '已清空金额，可继续记一笔'
}

function copyLast(): void {
  // 保留金额 / 账户 / 分类，但必须换新幂等键，否则服务端会命中上次交易。
  resetIdempotencyKey()
  showSuccess.value = false
  notice.value = '已复制上一笔，可修改后保存'
}

function close(): void {
  emit('close')
}

onMounted(() => {
  restoreDraft()
  void loadAccounts()
})

// 草稿按账本命名空间暂存，刷新可恢复（AC-32）。
watch(
  () => JSON.stringify(snapshot()),
  () => {
    if (session.ledgerId !== null && !showSuccess.value) {
      writeDraft(session.ledgerId, snapshot())
    }
  },
)

// E-08：切账本时清空账户缓存与幂等键，重新加载该账本的账户与草稿。
watch(
  () => session.ledgerId,
  () => {
    accounts.value = []
    resetIdempotencyKey()
    restoreDraft()
    void loadAccounts()
  },
)
</script>

<template>
  <div class="rs-mask" :class="containerClass" @click.self="close">
    <section class="rs-panel" role="dialog" aria-modal="true" aria-label="记一笔">
      <header class="rs-head">
        <h3>记一笔</h3>
        <button class="rs-close" type="button" aria-label="关闭" @click="close">✕</button>
      </header>

      <!-- 保存结果：返回 / 再记一笔 / 复制上一笔（AC-14） -->
      <div v-if="showSuccess" class="rs-success">
        <div class="rs-success__icon">✓</div>
        <h4>已保存</h4>
        <p class="rs-muted">
          {{ lastResult && lastResult.idempotent
            ? '本次为重复提交的幂等命中，未新增交易'
            : '交易已写入账本' }}
        </p>
        <div class="rs-success__actions">
          <button class="fb-btn-ghost" type="button" @click="close">返回</button>
          <button class="fb-btn-ghost" type="button" @click="copyLast">复制上一笔</button>
          <button class="fb-btn-primary" type="button" @click="again">再记一笔</button>
        </div>
      </div>

      <template v-else>
        <div class="rs-tabs" role="tablist">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            type="button"
            role="tab"
            :aria-selected="form.kind === tab.key"
            :class="{ on: form.kind === tab.key }"
            @click="switchKind(tab.key)"
          >{{ tab.label }}</button>
        </div>

        <div class="rs-amount">
          <span class="rs-amount__cur">{{ currency }}</span>
          <span class="rs-amount__val">{{ amountDisplay }}</span>
        </div>

        <div v-if="loadError" class="rs-alert rs-alert--error">
          {{ loadError }}
          <button type="button" @click="loadAccounts">重试</button>
        </div>
        <div v-if="submitError" class="rs-alert rs-alert--error">{{ submitError }}</div>
        <div v-if="notice" class="rs-alert rs-alert--ok">{{ notice }}</div>

        <div class="rs-body">
          <!-- 批 2.5 修 5：高级双录模式（逐条编辑分录，按币种配平） -->
          <template v-if="advancedMode">
            <div class="rs-advanced-head">
              <span class="rs-label">双录分录（按币种合计必须为 0）</span>
              <button type="button" class="rs-advanced" @click="toggleAdvanced">
                ‹ 返回简化模式
              </button>
            </div>
            <div v-for="(row, index) in form.advancedPostings" :key="index" class="rs-adv-row">
              <select
                class="fb-input rs-adv-account"
                :value="row.accountId === null ? '' : String(row.accountId)"
                @change="onAdvancedAccount(index, $event)"
              >
                <option value="">选择账户</option>
                <option v-for="acc in openAccounts" :key="acc.id" :value="String(acc.id)">
                  {{ accountLabel(acc) }}
                </option>
              </select>
              <input
                class="fb-input rs-adv-amount"
                :value="row.amount"
                placeholder="金额"
                @change="onAdvancedAmount(index, $event)"
              />
              <button type="button" class="rs-adv-remove" aria-label="删除分录" @click="removeAdvancedRow(index)">✕</button>
            </div>
            <button type="button" class="rs-advanced" @click="addAdvancedRow">＋ 添加分录</button>
            <div v-if="advancedError" class="rs-alert rs-alert--error" role="alert">{{ advancedError }}</div>
            <p class="rs-hint">自动带出账户币种；提交走与简化模式相同的幂等提交路径。</p>
          </template>

          <template v-else>
          <template v-if="form.kind !== 'transfer'">
            <div class="rs-label">分类（必选）</div>
            <div class="rs-cats">
              <button
                v-for="cat in categoryGrid"
                :key="cat.id"
                type="button"
                class="rs-cat"
                :class="{ on: form.categoryAccountId === cat.id }"
                @click="selectCategory(cat.id)"
              >
                <span class="rs-cat__icon">{{ catIcon(cat) }}</span>
                <span class="rs-cat__name">{{ accountLabel(cat) }}</span>
              </button>
              <p v-if="categoryGrid.length === 0" class="rs-muted">
                当前账本暂无{{ form.kind === 'income' ? '收入' : '支出' }}分类账户
              </p>
            </div>
            <select
              v-if="categoryOptions.length > 9"
              class="fb-input rs-select"
              :value="categorySelectValue"
              @change="onCategorySelect"
            >
              <option value="">全部分类…</option>
              <option v-for="cat in categoryOptions" :key="cat.id" :value="String(cat.id)">
                {{ accountLabel(cat) }}
              </option>
            </select>
            <div class="rs-label">资金账户（必选）</div>
            <select class="fb-input" :value="fundValue" @change="onFundSelect">
              <option value="">选择资金账户</option>
              <option v-for="acc in fundAccounts" :key="acc.id" :value="String(acc.id)">
                {{ accountLabel(acc) }}（{{ balanceText(acc) }}）
              </option>
            </select>
          </template>

          <template v-else>
            <div class="rs-label">转出账户（必选）</div>
            <select class="fb-input" :value="transferFromValue" @change="onTransferFrom">
              <option value="">选择转出账户</option>
              <option v-for="acc in fundAccounts" :key="acc.id" :value="String(acc.id)">
                {{ accountLabel(acc) }}（{{ balanceText(acc) }}）
              </option>
            </select>
            <div class="rs-transfer-arrow">↓</div>
            <div class="rs-label">转入账户（必选）</div>
            <select class="fb-input" :value="transferToValue" @change="onTransferTo">
              <option value="">选择转入账户</option>
              <option v-for="acc in fundAccounts" :key="acc.id" :value="String(acc.id)">
                {{ accountLabel(acc) }}（{{ balanceText(acc) }}）
              </option>
            </select>
          </template>

          <div class="rs-row">
            <label class="rs-field">
              <span>日期</span>
              <input class="fb-input" type="date" v-model="form.date" />
            </label>
            <label class="rs-field">
              <span>标签（逗号分隔）</span>
              <input
                class="fb-input"
                :value="tagsText"
                placeholder="如：日常,孩子"
                @change="onTagsChange"
              />
            </label>
          </div>
          <label class="rs-field">
            <span>描述 / 备注</span>
            <input class="fb-input" v-model="form.description" placeholder="如：超市采购" />
          </label>

          <div class="rs-preview">
            <span class="rs-preview__label">分录预览</span>
            <span v-for="(posting, index) in preview" :key="index" class="rs-preview__line">
              {{ posting.accountName }} {{ posting.amount }}
            </span>
            <span v-if="previewError" class="rs-preview__error">{{ previewError }}</span>
          </div>
          <button type="button" class="rs-advanced" @click="toggleAdvanced">
            高级模式（逐条编辑双录分录）›
          </button>
          </template>
        </div>

        <div v-if="!advancedMode" class="rs-keyboard">
          <button
            v-for="key in keyboardKeys"
            :key="key"
            type="button"
            :class="keyClass(key)"
            @click="pressKey(key)"
          >{{ key }}</button>
        </div>

        <div class="rs-foot">
          <button type="button" class="fb-btn-ghost" @click="close">取消</button>
          <button
            type="button"
            class="fb-btn-primary"
            :disabled="!canSubmit"
            @click="submit(false)"
          >{{ submitting ? '保存中…' : '保存' }}</button>
        </div>
      </template>
    </section>
  </div>
</template>

<style scoped>
.rs-mask {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background: rgba(20, 28, 48, 0.45);
  z-index: 60;
  display: flex;
}

/* 移动：底部 Sheet；PC：居中弹窗（同一组件，断点切换） */
.rs-mask--auto,
.rs-mask--sheet {
  align-items: flex-end;
  justify-content: center;
}

.rs-mask--dialog {
  align-items: center;
  justify-content: center;
}

.rs-mask--fullscreen {
  align-items: stretch;
  justify-content: stretch;
}

.rs-panel {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-height: 92vh;
  background: var(--fb-card);
  border-radius: 22px 22px 0 0;
  box-shadow: 0 10px 30px rgba(23, 32, 68, 0.18);
  overflow: hidden;
}

.rs-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--fb-line);
}

.rs-head h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
}

.rs-close {
  border: none;
  background: transparent;
  color: var(--fb-sub);
  font-size: 18px;
  cursor: pointer;
}

.rs-tabs {
  display: flex;
  gap: 24px;
  padding: 12px 18px 0;
}

.rs-tabs button {
  border: none;
  background: transparent;
  padding: 6px 0 8px;
  font-size: 14px;
  font-family: inherit;
  color: var(--fb-sub);
  cursor: pointer;
  border-bottom: 2px solid transparent;
}

.rs-tabs button.on {
  color: var(--fb-blue);
  font-weight: 700;
  border-bottom-color: var(--fb-blue);
}

.rs-amount {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 10px 18px 12px;
  border-bottom: 1px solid var(--fb-line);
}

.rs-amount__cur {
  font-size: 18px;
  color: var(--fb-sub);
}

.rs-amount__val {
  font-size: 30px;
  font-weight: 800;
  font-variant-numeric: var(--fb-font-numeric);
  letter-spacing: -0.02em;
}

.rs-alert {
  margin: 8px 18px 0;
  padding: 8px 12px;
  border-radius: var(--fb-radius-input);
  font-size: 12px;
  background: var(--fb-soft);
  color: var(--fb-ink);
}

.rs-alert--error {
  background: var(--fb-danger-soft);
  color: var(--fb-danger);
}

.rs-alert--ok {
  background: var(--fb-green-soft);
  color: var(--fb-green);
}

.rs-alert button {
  border: none;
  background: transparent;
  color: var(--fb-blue);
  cursor: pointer;
  font-family: inherit;
}

.rs-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 18px 16px;
}

.rs-label {
  font-size: 12px;
  color: var(--fb-sub);
  margin: 10px 0 6px;
}

.rs-cats {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 8px;
}

.rs-cat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 8px 2px;
  border: 1.5px solid transparent;
  border-radius: 12px;
  background: var(--fb-soft);
  cursor: pointer;
  font-family: inherit;
}

.rs-cat.on {
  border-color: var(--fb-blue);
  background: var(--fb-blue-soft);
}

.rs-cat__icon {
  width: 30px;
  height: 30px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--fb-card);
  font-size: 16px;
}

.rs-cat__name {
  font-size: 11px;
  color: var(--fb-ink);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rs-select {
  margin-top: 8px;
}

.rs-transfer-arrow {
  text-align: center;
  color: var(--fb-blue);
  font-size: 18px;
  padding: 6px 0;
}

.rs-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.rs-field {
  display: block;
  margin-top: 10px;
}

.rs-field > span {
  display: block;
  font-size: 12px;
  color: var(--fb-sub);
  margin-bottom: 6px;
}

.rs-preview {
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: var(--fb-radius-input);
  background: var(--fb-soft);
  font-size: 11px;
  color: var(--fb-sub);
}

.rs-preview__label {
  display: block;
  font-weight: 600;
  color: var(--fb-ink);
  margin-bottom: 4px;
}

.rs-preview__line {
  display: block;
  font-variant-numeric: var(--fb-font-numeric);
  line-height: 1.7;
}

.rs-preview__error {
  display: block;
  color: var(--fb-danger);
  margin-top: 4px;
}

.rs-advanced {
  margin-top: 8px;
  border: none;
  background: transparent;
  color: var(--fb-blue);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  padding: 0;
}

/* 批 2.5 修 5：高级双录面板 */
.rs-advanced-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.rs-adv-row {
  display: grid;
  grid-template-columns: 1fr 110px 34px;
  gap: 6px;
  margin-top: 8px;
  align-items: center;
}

.rs-adv-amount {
  font-variant-numeric: var(--fb-font-numeric);
}

.rs-adv-remove {
  border: 1px solid var(--fb-line);
  background: var(--fb-card);
  color: var(--fb-danger);
  border-radius: var(--fb-radius-input);
  height: 40px;
  font-size: 12px;
  cursor: pointer;
}

.rs-hint {
  margin: 10px 0 0;
  font-size: 11px;
  line-height: 1.7;
  color: var(--fb-sub);
}

.rs-keyboard {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1px;
  background: var(--fb-line);
  border-top: 1px solid var(--fb-line);
}

.rs-key {
  border: none;
  background: var(--fb-card);
  padding: 12px 0;
  font-size: 17px;
  font-family: inherit;
  font-variant-numeric: var(--fb-font-numeric);
  color: var(--fb-ink);
  cursor: pointer;
}

.rs-key--op {
  color: var(--fb-blue);
  font-size: 15px;
}

.rs-key--again {
  color: var(--fb-green);
  font-size: 13px;
  font-weight: 700;
}

.rs-key--save {
  background: var(--fb-blue);
  color: var(--fb-white);
  font-size: 14px;
  font-weight: 700;
}

.rs-foot {
  display: flex;
  gap: 10px;
  padding: 10px 18px calc(10px + env(safe-area-inset-bottom, 0px));
}

.rs-foot > button {
  flex: 1;
}

.rs-success {
  padding: 30px 18px 26px;
  text-align: center;
}

.rs-success__icon {
  width: 56px;
  height: 56px;
  margin: 0 auto 12px;
  border-radius: 50%;
  background: var(--fb-green);
  color: var(--fb-white);
  font-size: 30px;
  line-height: 56px;
}

.rs-success h4 {
  margin: 0 0 6px;
  font-size: 18px;
}

.rs-success__actions {
  display: flex;
  gap: 8px;
  margin-top: 20px;
}

.rs-success__actions > button {
  flex: 1;
}

.rs-muted {
  color: var(--fb-sub);
  font-size: 12px;
}

@media (min-width: 1024px) {
  .rs-mask--auto {
    align-items: center;
  }

  .rs-mask--auto .rs-panel,
  .rs-mask--dialog .rs-panel {
    width: 460px;
    max-width: 92vw;
    max-height: 88vh;
    border-radius: 18px;
  }

  .rs-cats {
    grid-template-columns: repeat(5, 1fr);
  }
}

.rs-mask--fullscreen .rs-panel {
  width: 100%;
  height: 100vh;
  max-height: 100vh;
  border-radius: 0;
}
</style>
