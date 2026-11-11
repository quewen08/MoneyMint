/**
 * P-11 报表聚合（批 3，纯逻辑层）
 *
 * 批 2.5 铁律（勿删）：
 * - **一切聚合显式携带 commodity 维度**，禁止跨币种求和、禁止折算成单值；
 * - 收支统计一律**分录级**，按账户 type 判定 Income / Expenses；
 *   转账（全是 Assets / Liabilities 分录）天然不计入收支；
 * - 金额运算只走 `core/Money`（decimal.js 定点），禁止 JS 裸 +/-、parseFloat；
 *   占比 / 比率仅用于绘图宽度，允许 Number，但绝不回写为金额；
 * - 净资产口径 ΣAssets + ΣLiabilities 由 `computeBalanceSummary` 统一提供，
 *   本模块不自行按账户类型取绝对值或乘系数。
 *
 * Safari 14.1 兼容：不用 `.at()` / findLast / Object.hasOwn / padStart / toISOString。
 */
import { Money } from './Money'
import { accountLabel, type SummaryAccount } from './record'

/** 报表所需的最小分录结构（与 api/types 的 PostingView 兼容）。 */
export interface ReportPosting {
  account_id: number
  commodity: string
  amount: string
}

/** 报表所需的最小交易结构。 */
export interface ReportTxn {
  id: number
  date: string
  description: string
  created_by_name: string
  tags?: string[]
  postings: ReportPosting[]
}

export type ReportPeriod = 'week' | 'month' | 'year'

/** 一个时间桶：key 用于与交易日期比对，label 用于图表刻度。 */
export interface PeriodBucket {
  key: string
  label: string
}

function twoDigits(value: number): string {
  return value < 10 ? `0${value}` : String(value)
}

/** 本地时区的 `YYYY-MM-DD`（不用 toISOString，避免 UTC 跨日）。 */
export function dateString(date: Date): string {
  return `${date.getFullYear()}-${twoDigits(date.getMonth() + 1)}-${twoDigits(date.getDate())}`
}

/** 某月天数（用 Date 溢出法，不依赖 Intl）。 */
function daysInMonth(year: number, month: number): number {
  return new Date(year, month + 1, 0).getDate()
}

/**
 * 生成时间桶：周 = 周一~周日 7 天；月 = 当月每一天；年 = 12 个月。
 * cursor 决定落在哪一周期，切换周期时由视图重置为今天。
 */
export function periodBuckets(period: ReportPeriod, cursor: Date): PeriodBucket[] {
  const year = cursor.getFullYear()
  const month = cursor.getMonth()
  const out: PeriodBucket[] = []

  if (period === 'year') {
    for (let m = 0; m < 12; m += 1) {
      out.push({ key: `${year}-${twoDigits(m + 1)}`, label: `${m + 1}月` })
    }
    return out
  }

  if (period === 'month') {
    const total = daysInMonth(year, month)
    for (let day = 1; day <= total; day += 1) {
      out.push({
        key: `${year}-${twoDigits(month + 1)}-${twoDigits(day)}`,
        label: `${twoDigits(month + 1)}/${twoDigits(day)}`,
      })
    }
    return out
  }

  // 周：以周一为起点（getDay(): 0=周日）。
  const weekday = cursor.getDay()
  const offset = weekday === 0 ? 6 : weekday - 1
  const monday = new Date(year, month, cursor.getDate() - offset)
  const weekLabels = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']
  for (let i = 0; i < 7; i += 1) {
    const day = new Date(monday.getFullYear(), monday.getMonth(), monday.getDate() + i)
    out.push({ key: dateString(day), label: weekLabels[i] })
  }
  return out
}

/** 交易日期是否落在桶内（年用 `YYYY-MM` 前缀，周/月用完整日期）。 */
export function inBuckets(date: string, buckets: PeriodBucket[], period: ReportPeriod): boolean {
  for (const bucket of buckets) {
    if (period === 'year' ? date.indexOf(bucket.key) === 0 : date === bucket.key) {
      return true
    }
  }
  return false
}

/** 区间可读标签，用于顶部「上一周 / 下一月」中间的文案。 */
export function periodLabel(period: ReportPeriod, cursor: Date): string {
  const year = cursor.getFullYear()
  if (period === 'year') {
    return `${year} 年`
  }
  if (period === 'month') {
    return `${year} 年 ${cursor.getMonth() + 1} 月`
  }
  const buckets = periodBuckets('week', cursor)
  return `${buckets[0].key} ~ ${buckets[6].key}`
}

/** 按周期平移 cursor（上一期 / 下一期）。 */
export function shiftPeriod(period: ReportPeriod, cursor: Date, delta: number): Date {
  const next = new Date(cursor.getFullYear(), cursor.getMonth(), cursor.getDate())
  if (period === 'week') {
    next.setDate(next.getDate() + delta * 7)
  } else if (period === 'month') {
    next.setMonth(next.getMonth() + delta)
  } else {
    next.setFullYear(next.getFullYear() + delta)
  }
  return next
}

/** 从交易收集去重标签（供 #tag 下钻）。 */
export function collectTags(txns: ReportTxn[]): string[] {
  const tags: string[] = []
  for (const txn of txns) {
    if (txn.tags === undefined) {
      continue
    }
    for (const tag of txn.tags) {
      if (tag !== '' && tags.indexOf(tag) < 0) {
        tags.push(tag)
      }
    }
  }
  return tags.sort()
}

/** 时间区间 + 标签过滤；tag 为空串表示全部。 */
export function filterReportTransactions(
  txns: ReportTxn[],
  buckets: PeriodBucket[],
  period: ReportPeriod,
  tag: string,
): ReportTxn[] {
  const out: ReportTxn[] = []
  for (const txn of txns) {
    if (!inBuckets(txn.date, buckets, period)) {
      continue
    }
    if (tag !== '') {
      const tags = txn.tags === undefined ? [] : txn.tags
      if (tags.indexOf(tag) < 0) {
        continue
      }
    }
    out.push(txn)
  }
  return out
}

function safeMoney(value: string): Money {
  try {
    return Money.from(value)
  } catch {
    return Money.zero()
  }
}

function accountById(accounts: SummaryAccount[], id: number): SummaryAccount | undefined {
  for (const account of accounts) {
    if (account.id === id) {
      return account
    }
  }
  return undefined
}

/** 是否命中指定账户类型与币种（跨币种分录一律跳过）。 */
function matches(
  posting: ReportPosting,
  accounts: SummaryAccount[],
  type: 'Income' | 'Expenses',
  commodity: string,
): boolean {
  if (posting.commodity !== commodity) {
    return false
  }
  const account = accountById(accounts, posting.account_id)
  return account !== undefined && account.type === type
}

/**
 * 选定主币种：取支出分录绝对额总和最大的币种；无数据回退 defaultCommodity。
 * 这里只比较各币种**各自的**合计大小来决定展示币种，不产生跨币种金额。
 */
export function pickCommodity(
  txns: ReportTxn[],
  accounts: SummaryAccount[],
  fallback: string,
): string {
  const totals: { [code: string]: Money } = {}
  for (const txn of txns) {
    for (const posting of txn.postings) {
      const account = accountById(accounts, posting.account_id)
      if (account === undefined || account.type !== 'Expenses') {
        continue
      }
      const code = posting.commodity
      const value = safeMoney(posting.amount).abs()
      totals[code] = totals[code] === undefined ? value : totals[code].plus(value)
    }
  }
  let best = ''
  // 仅用于「比较哪个币种金额更大」以决定展示币种，不产出跨币种金额；
  // 定点字符串字典序无法比较位数不同的数（"10.00" < "9.00"），故转 Number 比较。
  let bestMagnitude = -1
  for (const code of Object.keys(totals)) {
    const magnitude = Number(totals[code].toString())
    if (magnitude > bestMagnitude) {
      best = code
      bestMagnitude = magnitude
    }
  }
  return best === '' ? (fallback === '' ? 'CNY' : fallback) : best
}

/** 全部出现过的币种（供币种切换器，按字母序）。 */
export function collectCommodities(txns: ReportTxn[], fallback: string): string[] {
  const codes: string[] = [fallback]
  for (const txn of txns) {
    for (const posting of txn.postings) {
      if (posting.commodity !== '' && codes.indexOf(posting.commodity) < 0) {
        codes.push(posting.commodity)
      }
    }
  }
  return codes.sort()
}

/** 收支合计（幅度值，非负）。 */
export interface IncomeExpenseTotals {
  income: string
  expense: string
}

export function incomeExpenseTotals(
  txns: ReportTxn[],
  accounts: SummaryAccount[],
  commodity: string,
): IncomeExpenseTotals {
  let income = Money.zero()
  let expense = Money.zero()
  for (const txn of txns) {
    for (const posting of txn.postings) {
      const value = safeMoney(posting.amount).abs()
      if (matches(posting, accounts, 'Income', commodity)) {
        income = income.plus(value)
      } else if (matches(posting, accounts, 'Expenses', commodity)) {
        expense = expense.plus(value)
      }
    }
  }
  return { income: income.toString(), expense: expense.toString() }
}

/** 趋势序列：与 buckets 等长，逐桶收入 / 支出幅度。 */
export interface TrendSeries {
  labels: string[]
  income: string[]
  expense: string[]
}

export function trendSeries(
  txns: ReportTxn[],
  accounts: SummaryAccount[],
  commodity: string,
  buckets: PeriodBucket[],
  period: ReportPeriod,
): TrendSeries {
  const labels: string[] = []
  const income: string[] = []
  const expense: string[] = []
  for (const bucket of buckets) {
    const inRange: ReportTxn[] = []
    for (const txn of txns) {
      const hit =
        period === 'year'
          ? txn.date.indexOf(bucket.key) === 0
          : txn.date === bucket.key
      if (hit) {
        inRange.push(txn)
      }
    }
    const totals = incomeExpenseTotals(inRange, accounts, commodity)
    labels.push(bucket.label)
    income.push(totals.income)
    expense.push(totals.expense)
  }
  return { labels, income, expense }
}

/** 分类占比 / 排行条目；ratio 仅用于绘图宽度（0~1），不是金额。 */
export interface CategoryShare {
  label: string
  amount: string
  ratio: number
}

export function categoryShares(
  txns: ReportTxn[],
  accounts: SummaryAccount[],
  commodity: string,
  limit = 8,
): CategoryShare[] {
  const totals: { [label: string]: Money } = {}
  for (const txn of txns) {
    for (const posting of txn.postings) {
      if (!matches(posting, accounts, 'Expenses', commodity)) {
        continue
      }
      const account = accountById(accounts, posting.account_id)
      const label = account === undefined ? '未分类' : accountLabel(account)
      const value = safeMoney(posting.amount).abs()
      totals[label] = totals[label] === undefined ? value : totals[label].plus(value)
    }
  }
  const rows: Array<{ label: string; amount: Money }> = []
  for (const label of Object.keys(totals)) {
    rows.push({ label, amount: totals[label] })
  }
  rows.sort((a, b) => (a.amount.toString() === b.amount.toString()
    ? (a.label < b.label ? -1 : 1)
    : (Number(a.amount.toString()) > Number(b.amount.toString()) ? -1 : 1)))

  let grand = Money.zero()
  for (const row of rows) {
    grand = grand.plus(row.amount)
  }
  const grandNumber = Number(grand.toString())
  const out: CategoryShare[] = []
  for (let i = 0; i < rows.length && i < limit; i += 1) {
    const row = rows[i]
    out.push({
      label: row.label,
      amount: row.amount.toString(),
      ratio: grandNumber > 0 ? Number(row.amount.toString()) / grandNumber : 0,
    })
  }
  return out
}

/** 成员贡献：按记账人聚合笔数与该币种支出金额。 */
export interface MemberContribution {
  name: string
  count: number
  amount: string
  ratio: number
}

export function memberContributions(
  txns: ReportTxn[],
  accounts: SummaryAccount[],
  commodity: string,
): MemberContribution[] {
  const buckets: { [name: string]: { count: number; amount: Money } } = {}
  for (const txn of txns) {
    const name = txn.created_by_name === '' ? '未署名' : txn.created_by_name
    let expense = Money.zero()
    for (const posting of txn.postings) {
      if (matches(posting, accounts, 'Expenses', commodity)) {
        expense = expense.plus(safeMoney(posting.amount).abs())
      }
    }
    const current = buckets[name]
    buckets[name] = current === undefined
      ? { count: 1, amount: expense }
      : { count: current.count + 1, amount: current.amount.plus(expense) }
  }
  let grand = Money.zero()
  for (const name of Object.keys(buckets)) {
    grand = grand.plus(buckets[name].amount)
  }
  const grandNumber = Number(grand.toString())
  const out: MemberContribution[] = []
  for (const name of Object.keys(buckets)) {
    const item = buckets[name]
    out.push({
      name,
      count: item.count,
      amount: item.amount.toString(),
      ratio: grandNumber > 0 ? Number(item.amount.toString()) / grandNumber : 0,
    })
  }
  out.sort((a, b) => (Number(b.amount) > Number(a.amount) ? 1 : -1))
  return out
}

/** 试算平衡行：账户 / 类型 / 该币种余额（原符号，不取绝对值）。 */
export interface TrialBalanceRow {
  label: string
  type: string
  balance: string
}

export function trialBalanceRows(
  accounts: SummaryAccount[],
  commodity: string,
): TrialBalanceRow[] {
  const out: TrialBalanceRow[] = []
  for (const account of accounts) {
    const balances = account.balances === undefined ? {} : account.balances
    const value = balances[commodity]
    if (value === undefined) {
      continue
    }
    out.push({ label: accountLabel(account), type: account.type, balance: value })
  }
  return out
}
