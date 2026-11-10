/**
 * 记账 / 汇总的金额与符号口径（批 1，纯逻辑层）
 *
 * 硬约束：
 * - 一切金额运算走 `core/Money`（decimal.js 定点），禁止裸 + - * /、parseFloat；
 * - 净资产口径统一 `ΣAssets + ΣLiabilities`，后端返回的账户余额本身带符号
 *   （Liabilities / Income 为负），禁止按类型 abs() 或乘系数（§6.4 / AC-12）；
 * - 在线 postings 只用账户整数 id，金额用定点十进制字符串。
 */
import { Money } from './Money'

/** 账户余额视图所需的最小结构（避免 core 层反向依赖 api 层）。 */
export interface SummaryAccount {
  id: number
  name: string
  display_name?: string
  type: string
  commodity_restriction?: string
  balances?: Record<string, string>
  /** 关户守门用（后端 close_date，非空即已关闭）。 */
  close_date?: string
  /** 分类拖拽排序（0.4-C，同类型内升序）。 */
  sort_order?: number
}

/** 记账类型（C-01 三 Tab）。 */
export type RecordKind = 'expense' | 'income' | 'transfer'

/** 一条在线分录草稿（account_id 为整数）。 */
export interface PostingDraft {
  accountId: number
  accountName: string
  commodity: string
  amount: string
}

/** 记一笔表单的持久化形状（草稿不含任何凭据）。 */
export interface RecordForm {
  kind: RecordKind
  /** 数字键盘拼出的表达式，如 `12.50+3.25`；空串表示未输入。 */
  amountExpression: string
  date: string
  description: string
  tags: string[]
  /** 支出 → Expenses 账户；收入 → Income 账户。 */
  categoryAccountId: number | null
  /** 支出 / 收入的资金账户（Assets / Liabilities）。 */
  fundAccountId: number | null
  transferFromAccountId: number | null
  transferToAccountId: number | null
}

/** 两位补零（不依赖 padStart / toISOString，避免跨时区跨日）。 */
function twoDigits(value: number): string {
  return value < 10 ? `0${value}` : String(value)
}

/** 今日日期 `YYYY-MM-DD`（本地时区）。 */
export function todayString(now: Date = new Date()): string {
  return `${String(now.getFullYear())}-${twoDigits(now.getMonth() + 1)}-${twoDigits(now.getDate())}`
}

/** 把 `12.` / `.5` 这类键盘输入规范成 Money.from 可接受的普通十进制串。 */
function normalizeNumberToken(token: string): string {
  let result = token
  if (result.charAt(0) === '.') {
    result = `0${result}`
  }
  if (result.charAt(result.length - 1) === '.') {
    result = `${result}0`
  }
  return result
}

/**
 * 定点求值金额表达式（数字键盘的 ＋ / － 会拼出 `12.5+3.25`）。
 * 只接受数字、小数点和加减号；结果取绝对值（方向由三 Tab 决定）。
 * 全程 Money 定点运算，杜绝 `0.1 + 0.2` 浮点尾差（AC-22）。
 */
export function evaluateAmountExpression(expression: string): Money {
  const expr = expression.trim()
  if (expr === '' || !/^[0-9.]+(?:[+-][0-9.]+)*$/.test(expr)) {
    throw new RangeError('金额只能包含数字、小数点与加减号')
  }

  const parts: Array<{ minus: boolean; value: string }> = []
  let buffer = ''
  let minus = false
  for (let i = 0; i < expr.length; i += 1) {
    const ch = expr.charAt(i)
    if (ch === '+' || ch === '-') {
      if (buffer === '') {
        throw new RangeError('金额表达式的运算符位置不正确')
      }
      parts.push({ minus, value: normalizeNumberToken(buffer) })
      buffer = ''
      minus = ch === '-'
    } else {
      buffer += ch
    }
  }
  if (buffer === '') {
    throw new RangeError('金额表达式不能以运算符结尾')
  }
  parts.push({ minus, value: normalizeNumberToken(buffer) })

  let total = Money.zero()
  for (const part of parts) {
    const value = Money.from(part.value)
    total = part.minus ? total.minus(value) : total.plus(value)
  }
  return total.abs()
}

/** 账户展示名（优先 display_name）。 */
export function accountLabel(account: SummaryAccount): string {
  return account.display_name !== undefined && account.display_name !== ''
    ? account.display_name
    : account.name
}

function resolveCommodity(account: SummaryAccount, fallback: string): string {
  const restriction = account.commodity_restriction
  return restriction !== undefined && restriction !== '' ? restriction : fallback
}

/** 复式恒等式：所有分录金额（定点）求和必须为 0。 */
export function postingsBalanced(postings: PostingDraft[]): boolean {
  let sum = Money.zero()
  for (const posting of postings) {
    sum = sum.plus(Money.from(posting.amount))
  }
  return sum.isZero()
}

/**
 * 依据三 Tab 生成两条配平分录。
 * - 支出：Expenses +amount / 资金账户 -amount
 * - 收入：Income -amount / 资金账户 +amount
 * - 转账：转出 -amount / 转入 +amount
 * 后端余额口径即「postings 直接累加」，因此这里不按账户类型乘系数。
 */
export function buildPostings(
  form: RecordForm,
  accounts: SummaryAccount[],
  defaultCommodity: string,
): PostingDraft[] {
  const amount = evaluateAmountExpression(form.amountExpression)
  if (amount.isDisplayZero()) {
    throw new RangeError('请输入大于 0 的金额')
  }

  const byId = new Map<number, SummaryAccount>()
  for (const account of accounts) {
    byId.set(account.id, account)
  }
  const resolve = (id: number | null, label: string): SummaryAccount => {
    if (id === null) {
      throw new RangeError(`请选择${label}`)
    }
    const found = byId.get(id)
    if (found === undefined) {
      throw new RangeError(`${label}不在当前账本账户列表中`)
    }
    return found
  }

  const positive = amount.toString()
  const negative = amount.negated().toString()
  const postings: PostingDraft[] = []

  if (form.kind === 'transfer') {
    const from = resolve(form.transferFromAccountId, '转出账户')
    const to = resolve(form.transferToAccountId, '转入账户')
    if (from.id === to.id) {
      throw new RangeError('转出与转入账户不能相同')
    }
    const fromCommodity = resolveCommodity(from, defaultCommodity)
    const toCommodity = resolveCommodity(to, defaultCommodity)
    if (fromCommodity !== toCommodity) {
      throw new RangeError('转出与转入账户币种需一致')
    }
    postings.push({
      accountId: from.id,
      accountName: accountLabel(from),
      commodity: fromCommodity,
      amount: negative,
    })
    postings.push({
      accountId: to.id,
      accountName: accountLabel(to),
      commodity: toCommodity,
      amount: positive,
    })
  } else {
    const category = resolve(
      form.categoryAccountId,
      form.kind === 'income' ? '收入分类' : '支出分类',
    )
    const fund = resolve(form.fundAccountId, '资金账户')
    const categoryCommodity = resolveCommodity(category, defaultCommodity)
    const fundCommodity = resolveCommodity(fund, defaultCommodity)
    if (categoryCommodity !== fundCommodity) {
      throw new RangeError('分类与资金账户币种需一致')
    }
    const isExpense = form.kind === 'expense'
    postings.push({
      accountId: category.id,
      accountName: accountLabel(category),
      commodity: categoryCommodity,
      amount: isExpense ? positive : negative,
    })
    postings.push({
      accountId: fund.id,
      accountName: accountLabel(fund),
      commodity: fundCommodity,
      amount: isExpense ? negative : positive,
    })
  }

  if (!postingsBalanced(postings)) {
    throw new RangeError('分录未配平，请检查金额')
  }
  return postings
}

/** 首页余额汇总（按币种分组；主币种单列便于大数字展示）。 */
export interface BalanceSummary {
  assets: Record<string, string>
  liabilities: Record<string, string>
  netWorth: Record<string, string>
  assetTotal: string
  liabilityTotal: string
  netWorthTotal: string
  commodity: string
}

interface MoneyBuckets {
  [code: string]: Money
}

function safeMoney(value: string): Money {
  try {
    return Money.from(value)
  } catch {
    return Money.zero()
  }
}

function addToBucket(bucket: MoneyBuckets, code: string, value: string): void {
  const previous = bucket[code]
  bucket[code] = previous === undefined
    ? safeMoney(value)
    : previous.plus(safeMoney(value))
}

function bucketToStrings(bucket: MoneyBuckets): Record<string, string> {
  const result: Record<string, string> = {}
  const codes = Object.keys(bucket)
  for (const code of codes) {
    result[code] = bucket[code].toString()
  }
  return result
}

/**
 * 统一净资产口径：ΣAssets + ΣLiabilities（后端余额本身带符号，负债为负）。
 * 禁止 abs() / 按类型乘系数；净资产为负时原样保留负号（AC-12）。
 */
export function computeBalanceSummary(
  accounts: SummaryAccount[],
  defaultCommodity: string,
): BalanceSummary {
  const assets: MoneyBuckets = {}
  const liabilities: MoneyBuckets = {}

  for (const account of accounts) {
    if (account.type !== 'Assets' && account.type !== 'Liabilities') {
      continue
    }
    const bucket = account.type === 'Assets' ? assets : liabilities
    const balances = account.balances === undefined ? {} : account.balances
    const codes = Object.keys(balances)
    for (const code of codes) {
      addToBucket(bucket, code, balances[code])
    }
  }

  const net: MoneyBuckets = {}
  const assetCodes = Object.keys(assets)
  for (const code of assetCodes) {
    net[code] = assets[code]
  }
  const liabilityCodes = Object.keys(liabilities)
  for (const code of liabilityCodes) {
    const asset = net[code]
    net[code] = asset === undefined ? liabilities[code] : asset.plus(liabilities[code])
  }

  const allCodes = Object.keys(net)
  // 账本默认币种若无任何余额数据，退回第一个实际有数据的币种，避免首页误显示 0.00。
  let main = defaultCommodity === '' ? 'CNY' : defaultCommodity
  if (net[main] === undefined && allCodes.length > 0) {
    main = allCodes[0]
  }
  const assetMain = assets[main] === undefined ? Money.zero() : assets[main]
  const liabilityMain = liabilities[main] === undefined ? Money.zero() : liabilities[main]
  const netMain = net[main] === undefined ? Money.zero() : net[main]

  return {
    assets: bucketToStrings(assets),
    liabilities: bucketToStrings(liabilities),
    netWorth: bucketToStrings(net),
    assetTotal: assetMain.toString(),
    liabilityTotal: liabilityMain.toString(),
    netWorthTotal: netMain.toString(),
    commodity: main,
  }
}

/** 交易展示类型（首页最近流水区分转账 / 支出 / 收入）。 */
export type TransactionKind = 'transfer' | 'expense' | 'income'

/** 判定展示类型所需的最小分录结构（与 api/types 的 PostingView 结构兼容）。 */
export interface PostingLike {
  account_id: number
  amount: string
}

function findAccount(
  accounts: SummaryAccount[],
  accountId: number,
): SummaryAccount | undefined {
  for (const account of accounts) {
    if (account.id === accountId) {
      return account
    }
  }
  return undefined
}

/**
 * 判定一笔交易的展示类型：
 * - postings 全部属于 Assets / Liabilities → transfer（同账本内转账）
 * - 含 Expenses → expense
 * - 含 Income → income
 * - 其余（未知账户类型、空 postings、账户未加载）→ expense 回退
 */
export function transactionKind(
  postings: PostingLike[],
  accounts: SummaryAccount[],
): TransactionKind {
  if (postings.length === 0) {
    return 'expense'
  }
  let fundCount = 0
  let hasExpense = false
  let hasIncome = false
  for (const posting of postings) {
    const account = findAccount(accounts, posting.account_id)
    const type = account === undefined ? '' : account.type
    if (type === 'Assets' || type === 'Liabilities') {
      fundCount += 1
    } else if (type === 'Expenses') {
      hasExpense = true
    } else if (type === 'Income') {
      hasIncome = true
    }
  }
  if (hasExpense) {
    return 'expense'
  }
  if (hasIncome) {
    return 'income'
  }
  if (fundCount === postings.length) {
    return 'transfer'
  }
  return 'expense'
}

function amountAbs(amount: string): string {
  try {
    return Money.from(amount).abs().toString()
  } catch {
    return '0.00'
  }
}

/**
 * 转账展示金额：取转出方（负值）资金分录的**绝对值**。
 *
 * 全程走 core/Money 的 decimal `abs()`，禁止 Math.abs / parseFloat；
 * 找不到负值分录时退回第一条资金分录的绝对值，非法金额容错为 0.00。
 */
export function transferAmount(
  postings: PostingLike[],
  accounts: SummaryAccount[],
): string {
  let fallback = ''
  for (const posting of postings) {
    const account = findAccount(accounts, posting.account_id)
    const type = account === undefined ? '' : account.type
    if (type !== 'Assets' && type !== 'Liabilities') {
      continue
    }
    if (posting.amount.charAt(0) === '-') {
      return amountAbs(posting.amount)
    }
    if (fallback === '') {
      fallback = posting.amount
    }
  }
  return fallback === '' ? '0.00' : amountAbs(fallback)
}

/** 转账方向标签：`转出账户 → 转入账户`（账户名优先 display_name）。 */
export function transferRoute(
  postings: PostingLike[],
  accounts: SummaryAccount[],
): string {
  let from = ''
  let to = ''
  for (const posting of postings) {
    const account = findAccount(accounts, posting.account_id)
    if (account === undefined) {
      continue
    }
    if (account.type !== 'Assets' && account.type !== 'Liabilities') {
      continue
    }
    const label = accountLabel(account)
    if (posting.amount.charAt(0) === '-') {
      if (from === '') {
        from = label
      }
    } else if (to === '') {
      to = label
    }
  }
  if (from !== '' && to !== '') {
    return `${from} → ${to}`
  }
  return from !== '' ? from : to
}

/**
 * 用户视角金额（首页 / 流水列表共用口径）：
 * - 转账取转出方绝对值（decimal abs，不显示负号）；
 * - 支出 / 收入取资金分录带符号原值（负=支出、正=收入）；
 * - 无资金分录取首条分录原值；空分录取 0.00。
 */
export function displayAmountOf(
  postings: PostingLike[],
  accounts: SummaryAccount[],
): string {
  if (transactionKind(postings, accounts) === 'transfer') {
    return transferAmount(postings, accounts)
  }
  for (const posting of postings) {
    const account = findAccount(accounts, posting.account_id)
    const type = account === undefined ? '' : account.type
    if (type === 'Assets' || type === 'Liabilities') {
      return posting.amount
    }
  }
  return postings.length > 0 ? postings[0].amount : '0.00'
}
