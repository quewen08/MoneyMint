/**
 * 日历网格与每日小计用例（转账剔除 / 主导币种 / 周一开头 / 补位 / 定点聚合）
 *
 * 口径（AC-31 含「转账剔除」；旧版 calendar_view.dart:103）：
 * 资产/负债/权益之间的转移不计入当日收支；只统计主导币种。
 */
import { describe, expect, it } from 'vitest'

import { buildMonthGrid, dailyTotals, monthLabel } from './calendar'
import type { SummaryAccount } from './record'

const accounts: SummaryAccount[] = [
  { id: 1, name: 'Assets:Cash', display_name: '招商储蓄', type: 'Assets' },
  { id: 4, name: 'Expenses:Food', display_name: '食品饮料', type: 'Expenses' },
  { id: 5, name: 'Income:Salary', display_name: '薪资收入', type: 'Income' },
]

describe('每日小计（dailyTotals，转账剔除口径）', () => {
  it('支出 / 收入按资金分录统计，0.1 类小数无浮点尾差', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-02',
          postings: [
            { account_id: 4, amount: '38.50' },
            { account_id: 1, amount: '-12.50' },
          ],
        },
        {
          date: '2026-11-02',
          postings: [
            { account_id: 4, amount: '0.00' },
            { account_id: 1, amount: '-0.10' },
          ],
        },
      ],
      accounts,
    )
    expect(totals['2026-11-02']).toEqual({ expense: '-12.60', income: '0.00' })
  })

  it('转账（资产→资产 / 资产→负债）整笔不计入当日收支（AC-31 / 旧版剔除）', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-03',
          postings: [
            { account_id: 1, amount: '-2000.00' },
            { account_id: 1, amount: '2000.00' },
          ],
        },
      ],
      accounts,
    )
    // 全资金分录 → transfer → 整笔剔除，当日不产生任何小计。
    expect(totals['2026-11-03']).toBeUndefined()
  })

  it('非法金额容错为跳过，收入与支出分桶互不污染', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-05',
          postings: [
            { account_id: 4, amount: 'oops' },
            { account_id: 5, amount: '-9500.00' },
            { account_id: 1, amount: '9500.00' },
          ],
        },
      ],
      accounts,
    )
    expect(totals['2026-11-05']).toEqual({ expense: '0.00', income: '9500.00' })
  })

  it('指定主导币种时，其余币种的资金分录不计入当日收支（多币种不得混加）', () => {
    const txns = [
      {
        date: '2026-11-06',
        postings: [
          { account_id: 4, amount: '150.00' },
          { account_id: 1, amount: '-100.00', commodity: 'CNY' },
          { account_id: 1, amount: '-50.00', commodity: 'USD' },
        ],
      },
    ]
    expect(dailyTotals(txns, accounts, 'CNY')['2026-11-06'].expense).toBe('-100.00')
    expect(dailyTotals(txns, accounts, 'USD')['2026-11-06'].expense).toBe('-50.00')
    // 兼容模式（未指定主导币种）：不做币种过滤（调用方应传主导币种）。
    expect(dailyTotals(txns, accounts)['2026-11-06'].expense).toBe('-150.00')
  })
})

describe('月网格（buildMonthGrid）', () => {
  it('周一开头：2026-11-01（周日）应补位在第 1 行第 7 列', () => {
    const cells = buildMonthGrid(2026, 11, {})
    expect(cells).toHaveLength(42)
    // 2026-11-01 是周日 → 周一开头偏移 6。
    expect(cells[6]).toMatchObject({ key: '2026-11-01', day: 1, inMonth: true })
    expect(cells[5]).toMatchObject({ key: '2026-10-31', inMonth: false })
    expect(cells[7]).toMatchObject({ key: '2026-11-02', inMonth: true })
  })

  it('每月 1 号为周一时无前补位，网格含下月补位日', () => {
    const cells = buildMonthGrid(2026, 12, {})
    // 2026-12-01 是周二 → 偏移 1。
    expect(cells[0]).toMatchObject({ key: '2026-11-30', inMonth: false })
    expect(cells[1]).toMatchObject({ key: '2026-12-01', inMonth: true })
    expect(cells[41].inMonth).toBe(false)
  })

  it('网格带出每日小计，缺数据的日子为 0.00', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-02',
          postings: [
            { account_id: 4, amount: '38.50' },
            { account_id: 1, amount: '-38.50' },
          ],
        },
      ],
      accounts,
    )
    const cells = buildMonthGrid(2026, 11, totals)
    expect(cells[7]).toMatchObject({ key: '2026-11-02', expense: '-38.50', income: '0.00' })
    expect(cells[8].expense).toBe('0.00')
  })

  it('月标题补零', () => {
    expect(monthLabel(2026, 11)).toBe('2026年11月')
    expect(monthLabel(2026, 1)).toBe('2026年01月')
  })
})
