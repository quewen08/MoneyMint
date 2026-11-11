/**
 * 批 3 报表聚合回归（P-11）
 *
 * 守住四条：分录级统计、单币种隔离、转账剔除、定点金额无浮点尾差。
 * 与旧版差异（转账按整笔启发式判定）已在批 2.5 裁决为「统计一律分录级」，
 * 这里用混合交易与多币种分录把口径钉死。
 */
import { describe, expect, it } from 'vitest'

import type { SummaryAccount } from './record'
import {
  categoryShares,
  collectCommodities,
  collectTags,
  dateString,
  filterReportTransactions,
  incomeExpenseTotals,
  inBuckets,
  memberContributions,
  periodBuckets,
  pickCommodity,
  shiftPeriod,
  trialBalanceRows,
  trendSeries,
  type ReportTxn,
} from './reports'

const ACCOUNTS: SummaryAccount[] = [
  { id: 1, name: 'Assets:Cash', display_name: '现金', type: 'Assets', balances: { CNY: '1000.00' } },
  { id: 2, name: 'Expenses:Food', display_name: '食品', type: 'Expenses' },
  { id: 3, name: 'Income:Salary', display_name: '薪资', type: 'Income' },
  { id: 4, name: 'Assets:Bank', display_name: '银行', type: 'Assets' },
  { id: 5, name: 'Expenses:Travel', display_name: '交通', type: 'Expenses' },
  { id: 6, name: 'Liabilities:Card', type: 'Liabilities', balances: { CNY: '-120.00' } },
]

function txn(
  id: number,
  date: string,
  who: string,
  postings: Array<{ account_id: number; commodity: string; amount: string }>,
  tags: string[] = [],
): ReportTxn {
  return { id, date, description: `交易${id}`, created_by_name: who, tags, postings }
}

const TXNS: ReportTxn[] = [
  // 支出 10.00 CNY + 浮点尾差用例 0.10 / 0.20
  txn(1, '2026-11-10', '阿文', [
    { account_id: 2, commodity: 'CNY', amount: '10.00' },
    { account_id: 1, commodity: 'CNY', amount: '-10.00' },
  ], ['餐饮']),
  txn(2, '2026-11-10', '阿文', [
    { account_id: 2, commodity: 'CNY', amount: '0.10' },
    { account_id: 5, commodity: 'CNY', amount: '0.20' },
    { account_id: 1, commodity: 'CNY', amount: '-0.30' },
  ], ['餐饮']),
  // 收入 100.00 CNY
  txn(3, '2026-11-11', '小林', [
    { account_id: 3, commodity: 'CNY', amount: '-100.00' },
    { account_id: 1, commodity: 'CNY', amount: '100.00' },
  ]),
  // 纯转账：全部 Assets 分录，收支都不计
  txn(4, '2026-11-11', '小林', [
    { account_id: 1, commodity: 'CNY', amount: '-50.00' },
    { account_id: 4, commodity: 'CNY', amount: '50.00' },
  ]),
  // 外币支出：CNY 统计必须完全忽略
  txn(5, '2026-11-11', '阿文', [
    { account_id: 2, commodity: 'USD', amount: '7.00' },
    { account_id: 1, commodity: 'USD', amount: '-7.00' },
  ]),
]

describe('P-11：时间桶与筛选', () => {
  it('周桶固定 7 天且以周一为起点', () => {
    const buckets = periodBuckets('week', new Date(2026, 10, 11))
    expect(buckets.length).toBe(7)
    expect(buckets[0].label).toBe('周一')
    expect(buckets[6].label).toBe('周日')
    expect(buckets[0].key).toBe('2026-11-09')
    expect(buckets[6].key).toBe('2026-11-15')
  })

  it('月桶按当月天数（2026 年 2 月 28 天）、年桶 12 个月', () => {
    expect(periodBuckets('month', new Date(2026, 1, 1)).length).toBe(28)
    expect(periodBuckets('year', new Date(2026, 1, 1)).length).toBe(12)
  })

  it('年桶用 YYYY-MM 前缀比较，周/月用完整日期', () => {
    const year = periodBuckets('year', new Date(2026, 0, 1))
    const month = periodBuckets('month', new Date(2026, 10, 1))
    expect(inBuckets('2026-11-11', year, 'year')).toBe(true)
    expect(inBuckets('2025-11-11', year, 'year')).toBe(false)
    expect(inBuckets('2026-11-11', month, 'month')).toBe(true)
    expect(inBuckets('2026-12-01', month, 'month')).toBe(false)
  })

  it('dateString 用本地时区，shiftPeriod 按周期平移', () => {
    expect(dateString(new Date(2026, 10, 11))).toBe('2026-11-11')
    expect(dateString(shiftPeriod('week', new Date(2026, 10, 11), -1))).toBe('2026-11-04')
    expect(dateString(shiftPeriod('month', new Date(2026, 0, 15), 1))).toBe('2026-02-15')
  })

  it('标签下钻与时间区间同时生效', () => {
    const month = periodBuckets('month', new Date(2026, 10, 1))
    expect(filterReportTransactions(TXNS, month, 'month', '').length).toBe(5)
    expect(filterReportTransactions(TXNS, month, 'month', '餐饮').length).toBe(2)
    expect(filterReportTransactions(TXNS, periodBuckets('month', new Date(2026, 9, 1)), 'month', '').length).toBe(0)
    expect(collectTags(TXNS)).toEqual(['餐饮'])
  })
})

describe('P-11：分录级收支与币种隔离', () => {
  it('CNY 收入 100.00、支出 10.30，纯转账不计入', () => {
    const totals = incomeExpenseTotals(TXNS, ACCOUNTS, 'CNY')
    expect(totals.income).toBe('100.00')
    expect(totals.expense).toBe('10.30')
  })

  it('0.10 + 0.20 = 0.30，无浮点尾差（AC-22）', () => {
    const onlySmall = [TXNS[1]]
    expect(incomeExpenseTotals(onlySmall, ACCOUNTS, 'CNY').expense).toBe('0.30')
  })

  it('USD 统计只含 USD 分录，绝不跨币种求和', () => {
    const usd = incomeExpenseTotals(TXNS, ACCOUNTS, 'USD')
    expect(usd.expense).toBe('7.00')
    expect(usd.income).toBe('0.00')
    expect(incomeExpenseTotals(TXNS, ACCOUNTS, 'CNY').expense).toBe('10.30')
  })

  it('主币种选支出绝对额最大的币种，无数据时回退默认', () => {
    expect(pickCommodity(TXNS, ACCOUNTS, 'CNY')).toBe('CNY')
    expect(pickCommodity([], ACCOUNTS, 'CNY')).toBe('CNY')
    expect(collectCommodities(TXNS, 'CNY')).toEqual(['CNY', 'USD'])
  })

  it('趋势序列与桶等长，且按日期归桶', () => {
    const month = periodBuckets('month', new Date(2026, 10, 1))
    const series = trendSeries(TXNS, ACCOUNTS, 'CNY', month, 'month')
    expect(series.income.length).toBe(30)
    expect(series.income[9]).toBe('0.00')
    expect(series.expense[9]).toBe('10.30')
    expect(series.income[10]).toBe('100.00')
  })
})

describe('P-11：分类 / 成员 / 试算平衡', () => {
  it('分类占比按金额降序，比例合计约 1', () => {
    const shares = categoryShares(TXNS, ACCOUNTS, 'CNY')
    expect(shares[0].label).toBe('食品')
    expect(shares[0].amount).toBe('10.10')
    expect(shares[1].amount).toBe('0.20')
    const total = shares.reduce((sum, item) => sum + item.ratio, 0)
    expect(total).toBeCloseTo(1, 6)
  })

  it('成员贡献按记账人聚合笔数与支出金额', () => {
    const members = memberContributions(TXNS, ACCOUNTS, 'CNY')
    const a = members.find((item) => item.name === '阿文')
    const b = members.find((item) => item.name === '小林')
    expect(a?.count).toBe(3)
    expect(a?.amount).toBe('10.30')
    expect(b?.count).toBe(2)
    expect(b?.amount).toBe('0.00')
  })

  it('试算平衡只列该币种有余额的账户，负债保留负号', () => {
    const rows = trialBalanceRows(ACCOUNTS, 'CNY')
    expect(rows.length).toBe(2)
    expect(rows[0].label).toBe('现金')
    expect(rows[1].balance).toBe('-120.00')
    expect(trialBalanceRows(ACCOUNTS, 'USD')).toEqual([])
  })
})
