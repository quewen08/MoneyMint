/**
 * 记账 / 汇总的金额与符号口径（批 1，纯逻辑层）
 *
 * 硬约束：
 * - 一切金额运算走 `core/Money`（decimal.js 定点），禁止裸 + - * /、parseFloat；
 * - 净资产口径统一 `ΣAssets + ΣLiabilities`，后端返回的账户余额本身带符号
 *   （Liabilities / Income 为负），禁止按类型 abs() 或乘系数（§6.4 / AC-12）；
 * - 在线 postings 只用账户整数 id，金额用定点十进制字符串。
 *
 * 批 3 前置铁律（老板拍板「基础核心必须 fava/beancount 一致」，勿删）：
 * - 任何聚合函数签名必须**显式携带 commodity 维度**（如 dailyTotals 的
 *   mainCommodity、账户余额的 balances[commodity]）；
 * - **禁止跨币种求和**、禁止输出跨币种折算后的单值；
 * - 报表（批 3）将带币种切换器，聚合一律在选定币种内进行。
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
  /** 账户 uuid（批 2 起列表透出；层级树 parent 关系优先用它）。 */
  uuid?: string
  /** 父账户 uuid（三级分类结构；批 2.5 账户层级树用）。 */
  parent_uuid?: string
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
  /** 高级双录模式的分录行（批 2.5；accountId 允许 null 表示未选择；旧草稿无此字段）。 */
  advancedPostings?: Array<{ accountId: number | null; amount: string }>
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

/**
 * 交易展示类型（整笔启发式）。
 *
 * ⚠️ 文档化口径（批 2.5 三分法降级）：本判定是**整笔启发式**，仅供
 * 列表 badge 展示与 kind 筛选旧值兼容（transfer 项），**禁止作为任何
 * 统计依据**——收支统计一律分录级（见 calendar.dailyTotals），筛选的
 * expense / income 已改为分录命中（见 filters.applyFilters）。
 */
export type TransactionKind = 'transfer' | 'expense' | 'income' | 'opening'

/** 判定展示类型所需的最小分录结构（与 api/types 的 PostingView 结构兼容）。 */
export interface PostingLike {
  account_id: number
  amount: string
  /** 分录币种（列表行内金额按分录自身币种渲染；可选，兜底由调用方决定）。 */
  commodity?: string
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

/** postings 中是否含指定类型的账户分录（批 2.5：kind 筛选的分录命中口径）。 */
export function hasAccountType(
  postings: PostingLike[],
  accounts: SummaryAccount[],
  type: 'Expenses' | 'Income',
): boolean {
  for (const posting of postings) {
    const account = findAccount(accounts, posting.account_id)
    if (account !== undefined && account.type === type) {
      return true
    }
  }
  return false
}

/**
 * 判定一笔交易的整笔启发式类型：
 * - 含 Equity 分录 → opening（优先级最高：期初交易在每个账户都有分录，
 *   被标成「支出」是确定性错误）
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
  let hasEquity = false
  for (const posting of postings) {
    const account = findAccount(accounts, posting.account_id)
    const type = account === undefined ? '' : account.type
    if (type === 'Assets' || type === 'Liabilities') {
      fundCount += 1
    } else if (type === 'Expenses') {
      hasExpense = true
    } else if (type === 'Income') {
      hasIncome = true
    } else if (type === 'Equity') {
      hasEquity = true
    }
  }
  if (hasEquity) {
    return 'opening'
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

/** 行内金额的代表分录（含币种）。 */
export interface DisplayEntry {
  amount: string
  /** 分录自身币种（行内金额按此渲染，非主币种不得错标主币种符号）。 */
  commodity: string
}

/**
 * 用户视角的代表分录（首页 / 流水列表共用口径）：
 * - 转账取转出方（负值）资金分录的绝对值金额；
 * - **混合交易回退**（批 2.5）：同时含 Expenses 与 Income 分录（退款 / 报销类）
 *   时优先取**正号**资金分录——用户视角是净收款；
 * - 支出 / 收入取资金分录带符号原值（负=支出、正=收入）；
 * - 无资金分录取首条分录原值；空分录取 {0.00, ''}。
 * 币种始终取代表分录自身的 commodity（行内金额按分录币种渲染，批 2.5 修 3）。
 */
export function displayEntryOf(
  postings: PostingLike[],
  accounts: SummaryAccount[],
): DisplayEntry {
  if (postings.length === 0) {
    return { amount: '0.00', commodity: '' }
  }
  if (transactionKind(postings, accounts) === 'transfer') {
    for (const posting of postings) {
      const account = findAccount(accounts, posting.account_id)
      const type = account === undefined ? '' : account.type
      if (type !== 'Assets' && type !== 'Liabilities') {
        continue
      }
      if (posting.amount.charAt(0) === '-') {
        return { amount: amountAbs(posting.amount), commodity: posting.commodity ?? '' }
      }
    }
    for (const posting of postings) {
      const account = findAccount(accounts, posting.account_id)
      const type = account === undefined ? '' : account.type
      if (type === 'Assets' || type === 'Liabilities') {
        return { amount: amountAbs(posting.amount), commodity: posting.commodity ?? '' }
      }
    }
    return { amount: '0.00', commodity: '' }
  }

  let hasExpense = false
  let hasIncome = false
  for (const posting of postings) {
    const account = findAccount(accounts, posting.account_id)
    const type = account === undefined ? '' : account.type
    if (type === 'Expenses') {
      hasExpense = true
    } else if (type === 'Income') {
      hasIncome = true
    }
  }
  // 混合交易：优先正号资金分录（净收款）。
  if (hasExpense && hasIncome) {
    for (const posting of postings) {
      const account = findAccount(accounts, posting.account_id)
      const type = account === undefined ? '' : account.type
      if (
        (type === 'Assets' || type === 'Liabilities') &&
        posting.amount.charAt(0) !== '-'
      ) {
        return { amount: posting.amount, commodity: posting.commodity ?? '' }
      }
    }
  }
  for (const posting of postings) {
    const account = findAccount(accounts, posting.account_id)
    const type = account === undefined ? '' : account.type
    if (type === 'Assets' || type === 'Liabilities') {
      return { amount: posting.amount, commodity: posting.commodity ?? '' }
    }
  }
  return { amount: postings[0].amount, commodity: postings[0].commodity ?? '' }
}

/**
 * 用户视角金额（兼容入口）：等价 `displayEntryOf(...).amount`。
 * 需要币种时请改用 displayEntryOf（批 2.5 修 3：行内金额按分录币种渲染）。
 */
export function displayAmountOf(
  postings: PostingLike[],
  accounts: SummaryAccount[],
): string {
  return displayEntryOf(postings, accounts).amount
}

/**
 * 账户侧 journal（批 2.5 修 4）：某账户在交易中的分录金额（原值原符号）。
 * 负债 / Income 账户的分录符号与资金视角相反，必须按分录原值展示；
 * 找不到该账户分录时返回 '0.00'。
 */
export function postingAmountForAccount(
  txn: { postings: PostingLike[] },
  accountId: number,
): string {
  for (const posting of txn.postings) {
    if (posting.account_id === accountId) {
      return posting.amount
    }
  }
  return '0.00'
}

/**
 * 账户侧 running balance（批 2.5 修 4）：按日期升序累计「仅主币种」分录。
 *
 * 取舍说明：多币种账户的 running balance 只对 `commodity === mainCommodity`
 * 的分录累计（其余币种行余额显示 '-'）——旧版 journal 同款主导币种口径，
 * 跨币种混合累计违反批 3 铁律（禁止跨币种求和）。
 * 返回 map：txn.id → 累计后的余额（定点字符串，符号为账户视角原符号）。
 */
export function accountRunningBalances(
  txns: ReadonlyArray<{ id: number; date: string; postings: PostingLike[] }>,
  accountId: number,
  mainCommodity: string,
): { [txnId: number]: string } {
  const ordered = txns.slice().sort((a, b) => (a.date < b.date ? -1 : a.date > b.date ? 1 : 0))
  const result: { [txnId: number]: string } = {}
  let running = Money.zero()
  for (const txn of ordered) {
    for (const posting of txn.postings) {
      if (posting.account_id !== accountId) {
        continue
      }
      if (mainCommodity !== '' && posting.commodity !== mainCommodity) {
        continue
      }
      try {
        running = running.plus(Money.from(posting.amount))
      } catch {
        // 非法金额跳过（后端保证定点串，此处兜底）。
      }
      break
    }
    result[txn.id] = running.toString()
  }
  return result
}

/**
 * 高级双录的配平校验（批 2.5 修 5）：按币种分组合计，任一币种合计非 0 即不平。
 * 返回差额明细（币种 + 差额定点串），供 UI 直接渲染。
 */
export function balanceDiffByCommodity(
  postings: ReadonlyArray<{ commodity: string; amount: string }>,
): Array<{ commodity: string; diff: string }> {
  const buckets: { [code: string]: Money } = {}
  for (const posting of postings) {
    let value: Money
    try {
      value = Money.from(posting.amount)
    } catch {
      value = Money.zero()
    }
    const code = posting.commodity === '' ? 'CNY' : posting.commodity
    const previous = buckets[code]
    buckets[code] = previous === undefined ? value : previous.plus(value)
  }
  const diffs: Array<{ commodity: string; diff: string }> = []
  for (const code of Object.keys(buckets)) {
    if (!buckets[code].isZero()) {
      diffs.push({ commodity: code, diff: buckets[code].toString() })
    }
  }
  return diffs
}
