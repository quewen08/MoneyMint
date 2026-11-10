/**
 * CSV 导出用例（§14.5 #1：字段 / 表头 / 转义 / 金额格式与旧版一致）
 */
import { describe, expect, it } from 'vitest'

import { buildCsvRowsFromTransactions, buildTransactionsCsv, csvQuoted } from './csv'
import type { SummaryAccount } from './record'

describe('CSV 导出', () => {
  it('表头与字段顺序与旧版一致，每条分录一行', () => {
    const csv = buildTransactionsCsv([
      {
        date: '2026-11-09',
        description: '午餐',
        flag: '*',
        postings: [
          { account: 'Expenses:食品饮料:食品', commodity: 'CNY', amount: '38.50' },
          { account: 'Assets:招商储蓄', commodity: 'CNY', amount: '-38.50' },
        ],
      },
    ])
    const lines = csv.split('\n')
    expect(lines[0]).toBe('date,description,flag,account,commodity,amount')
    expect(lines[1]).toBe('2026-11-09,"午餐",*,"Expenses:食品饮料:食品",CNY,38.50')
    expect(lines[2]).toBe('2026-11-09,"午餐",*,"Assets:招商储蓄",CNY,-38.50')
    // 末行带换行（等价旧版 writeln）。
    expect(csv.endsWith('\n')).toBe(true)
    expect(lines[3]).toBe('')
  })

  it('描述 / 账户内的双引号与逗号被正确转义包裹', () => {
    expect(csvQuoted('他说"你好"')).toBe('"他说""你好"""')
    const csv = buildTransactionsCsv([
      {
        date: '2026-11-09',
        description: '超市,采购',
        flag: '!',
        postings: [{ account: 'Assets:A"B', commodity: 'CNY', amount: '0.10' }],
      },
    ])
    expect(csv).toContain('"超市,采购"')
    expect(csv).toContain('"Assets:A""B"')
  })

  it('金额保持后端定点字符串原样，不发生任何数值运算', () => {
    const csv = buildTransactionsCsv([
      {
        date: '2026-01-01',
        description: '大额',
        flag: '*',
        postings: [{ account: 'Assets:Bank', commodity: 'CNY', amount: '1000000.00' }],
      },
    ])
    expect(csv).toContain(',1000000.00')
  })

  it('account 列优先 display_name、回退 Beancount 全名（P1-B：等价旧版 ledger.accountName 语义）', () => {
    const accounts: SummaryAccount[] = [
      { id: 1, name: 'Assets:Cash', display_name: '招商储蓄', type: 'Assets' },
      { id: 2, name: 'Expenses:Food', display_name: '食品饮料', type: 'Expenses' },
    ]
    const rows = buildCsvRowsFromTransactions(
      [
        {
          date: '2026-11-10',
          description: '午餐',
          flag: '*',
          postings: [
            { account_id: 2, account: 'Expenses:Food', commodity: 'CNY', amount: '38.50' },
            { account_id: 1, account: 'Assets:Cash', commodity: 'CNY', amount: '-38.50' },
          ],
        },
      ],
      accounts,
    )
    const csv = buildTransactionsCsv(rows)
    expect(csv).toContain('"食品饮料",CNY,38.50')
    expect(csv).toContain('"招商储蓄",CNY,-38.50')
    // display_name 已存在时全名不得出现在 account 列。
    expect(csv).not.toContain('"Expenses:Food"')
    expect(csv).not.toContain('"Assets:Cash"')
  })
})
