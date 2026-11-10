/**
 * 日历网格与每日小计（P-04 日历视图，对照 mobile-redesign-v2.html 账单页）
 *
 * - 周一为第一列（原型列头 一二三四五六日）；
 * - 全部日期计算用本地时区 Date 构造，不用 toISOString（避免跨时区跨日）；
 * - 每日小计按资金分录正负分桶（负=支出、正=收入），core/Money 定点累加。
 */
import { Money } from './Money'
import type { SummaryAccount } from './record'

export interface DayTotal {
  expense: string
  income: string
}

/** 可聚合交易的最小结构（与 api/types 的 PostingView 结构兼容）。 */
export interface TotalableTxn {
  date: string
  postings: Array<{ account_id: number; amount: string; commodity?: string }>
}

function findType(accounts: SummaryAccount[], accountId: number): string {
  for (const account of accounts) {
    if (account.id === accountId) {
      return account.type
    }
  }
  return ''
}

function addMoney(bucket: { [date: string]: Money }, date: string, amount: string): void {
  let value: Money
  try {
    value = Money.from(amount)
  } catch {
    return
  }
  const previous = bucket[date]
  bucket[date] = previous === undefined ? value : previous.plus(value)
}

/**
 * 按日聚合收支（P-04 日历视图与列表日小计共用口径；批 2.5 改为**分录级聚合**，
 * 对齐旧版 calendar_view.dart:83-110 与 fava income statement）：
 *
 * - **逐分录判账户类型**：Income 分录金额（原值，fava 口径下为负）计入收入桶；
 *   Expenses 分录金额（原值，借方为正）计入支出桶；
 *   Assets / Liabilities / Equity 分录不计入——转账没有 Income/Expenses 分录，
 *   「转账不计入收支」的对外语义天然保持；期初（Equity）不计入收支；
 * - 金额**原样入桶，禁止 abs / 乘系数**（符号由展示层 MoneyText 规则处理）；
 * - `mainCommodity` 给定时只统计该币种分录（旧版只统计主导币种）。
 */
export function dailyTotals(
  txns: TotalableTxn[],
  accounts: SummaryAccount[],
  mainCommodity?: string,
): { [date: string]: DayTotal } {
  const expense: { [date: string]: Money } = {}
  const income: { [date: string]: Money } = {}
  for (const txn of txns) {
    for (const posting of txn.postings) {
      const type = findType(accounts, posting.account_id)
      if (type !== 'Income' && type !== 'Expenses') {
        continue
      }
      if (mainCommodity !== undefined && posting.commodity !== mainCommodity) {
        continue
      }
      addMoney(type === 'Expenses' ? expense : income, txn.date, posting.amount)
    }
  }
  const result: { [date: string]: DayTotal } = {}
  const dates = Object.keys(expense)
  for (const date of Object.keys(income)) {
    if (dates.indexOf(date) < 0) {
      dates.push(date)
    }
  }
  for (const date of dates) {
    const expenseValue = expense[date] === undefined ? Money.zero() : expense[date]
    const incomeValue = income[date] === undefined ? Money.zero() : income[date]
    result[date] = { expense: expenseValue.toString(), income: incomeValue.toString() }
  }
  return result
}

export interface CalendarDay {
  /** `YYYY-MM-DD`（含前后月补位日）。 */
  key: string
  day: number
  inMonth: boolean
  expense: string
  income: string
}

function twoDigits(value: number): string {
  return value < 10 ? `0${value}` : String(value)
}

function dateKey(year: number, month: number, day: number): string {
  return `${String(year)}-${twoDigits(month)}-${twoDigits(day)}`
}

/**
 * 构建某月 6 行 × 7 列网格（周一开头，前后月补位补满 42 格）。
 * `month` 取 1~12；`totals` 来自 `dailyTotals`。
 */
export function buildMonthGrid(
  year: number,
  month: number,
  totals: { [date: string]: DayTotal },
): CalendarDay[] {
  const firstWeekday = new Date(year, month - 1, 1).getDay()
  const offset = (firstWeekday + 6) % 7 // 周一开头
  const cells: CalendarDay[] = []
  for (let i = 0; i < 42; i += 1) {
    const date = new Date(year, month - 1, 1 - offset + i)
    const key = dateKey(date.getFullYear(), date.getMonth() + 1, date.getDate())
    const total = totals[key]
    cells.push({
      key,
      day: date.getDate(),
      inMonth: date.getMonth() + 1 === month && date.getFullYear() === year,
      expense: total === undefined ? '0.00' : total.expense,
      income: total === undefined ? '0.00' : total.income,
    })
  }
  return cells
}

/** 月标题，如 `2026年11月`。 */
export function monthLabel(year: number, month: number): string {
  return `${String(year)}年${twoDigits(month)}月`
}
