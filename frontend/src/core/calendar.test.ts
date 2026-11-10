/**
 * 日历网格与每日小计用例（批 2.5 分录级聚合口径 / 周一开头 / 补位 / 定点聚合）
 *
 * 口径（批 2.5 三分法降级，对齐旧版 calendar_view.dart:83-110 与 fava）：
 * - 逐分录判账户类型：Income 分录（原值为负）计入收入桶、Expenses 分录
 *   （借方为正）计入支出桶、Assets/Liabilities/Equity 分录不计入——
 *   转账与期初（Equity）天然不计入收支，无需整笔判定；
 * - 金额原样入桶（不 abs、不乘系数，符号由展示层处理）；
 * - 只统计主导币种（mainCommodity）。
 */
import { describe, expect, it } from 'vitest'

import { buildMonthGrid, dailyTotals, monthLabel } from './calendar'
import type { SummaryAccount } from './record'

const accounts: SummaryAccount[] = [
  { id: 1, name: 'Assets:Cash', display_name: '招商储蓄', type: 'Assets' },
  { id: 4, name: 'Expenses:Food', display_name: '食品饮料', type: 'Expenses' },
  { id: 5, name: 'Income:Salary', display_name: '薪资收入', type: 'Income' },
  { id: 9, name: 'Equity:Opening-Balances', type: 'Equity' },
]

describe('每日小计（dailyTotals，分录级聚合）', () => {
  it('Expenses 正分录入支出桶（金额原样，借方为正）', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-02',
          postings: [
            { account_id: 4, amount: '38.50', commodity: 'CNY' },
            { account_id: 1, amount: '-38.50', commodity: 'CNY' },
          ],
        },
        {
          date: '2026-11-02',
          postings: [
            { account_id: 4, amount: '0.10', commodity: 'CNY' },
            { account_id: 1, amount: '-0.10', commodity: 'CNY' },
          ],
        },
      ],
      accounts,
      'CNY',
    )
    expect(totals['2026-11-02']).toEqual({ expense: '38.60', income: '0.00' })
  })

  it('Income 负分录入收入桶；退款（Expenses 负分录）自然落入支出桶', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-05',
          postings: [
            { account_id: 5, amount: '-9500.00', commodity: 'CNY' },
            { account_id: 1, amount: '9500.00', commodity: 'CNY' },
          ],
        },
        {
          // 退款：Expenses 负分录（借方冲回）→ 支出桶负值。
          date: '2026-11-06',
          postings: [
            { account_id: 4, amount: '-20.00', commodity: 'CNY' },
            { account_id: 1, amount: '20.00', commodity: 'CNY' },
          ],
        },
      ],
      accounts,
      'CNY',
    )
    expect(totals['2026-11-05']).toEqual({ expense: '0.00', income: '-9500.00' })
    expect(totals['2026-11-06'].expense).toBe('-20.00')
  })

  it('转账（资产→资产 / 资产→负债）与期初（Equity）不计入当日收支', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-03',
          postings: [
            { account_id: 1, amount: '-2000.00', commodity: 'CNY' },
            { account_id: 1, amount: '2000.00', commodity: 'CNY' },
          ],
        },
        {
          date: '2026-11-04',
          postings: [
            { account_id: 9, amount: '12500.00', commodity: 'CNY' },
            { account_id: 1, amount: '12500.00', commodity: 'CNY' },
          ],
        },
      ],
      accounts,
      'CNY',
    )
    expect(totals['2026-11-03']).toBeUndefined()
    expect(totals['2026-11-04']).toBeUndefined()
  })

  it('指定主导币种时其余币种分录不计入；非法金额容错为跳过', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-06',
          postings: [
            { account_id: 4, amount: '150.00', commodity: 'CNY' },
            { account_id: 1, amount: '-50.00', commodity: 'USD' },
          ],
        },
        {
          date: '2026-11-07',
          postings: [
            { account_id: 4, amount: 'oops', commodity: 'CNY' },
            { account_id: 4, amount: '7.00', commodity: 'CNY' },
            { account_id: 1, amount: '-7.00', commodity: 'CNY' },
          ],
        },
      ],
      accounts,
      'CNY',
    )
    expect(totals['2026-11-06']).toEqual({ expense: '150.00', income: '0.00' })
    expect(totals['2026-11-07']).toEqual({ expense: '7.00', income: '0.00' })
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
            { account_id: 4, amount: '38.50', commodity: 'CNY' },
            { account_id: 1, amount: '-38.50', commodity: 'CNY' },
          ],
        },
      ],
      accounts,
      'CNY',
    )
    const cells = buildMonthGrid(2026, 11, totals)
    expect(cells[7]).toMatchObject({ key: '2026-11-02', expense: '38.50', income: '0.00' })
    expect(cells[8].expense).toBe('0.00')
  })

  it('月标题补零', () => {
    expect(monthLabel(2026, 11)).toBe('2026年11月')
    expect(monthLabel(2026, 1)).toBe('2026年01月')
  })
})
