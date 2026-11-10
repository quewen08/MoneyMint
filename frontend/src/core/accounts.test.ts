/**
 * 账户视图辅助用例（分组余额 / AC-28 不可选 / AC-29 受影响笔数 / 关户守门口径）
 */
import { describe, expect, it } from 'vitest'

import {
  affectedTransactionCount,
  balanceOf,
  groupAccountsByType,
  isClosed,
  openAccounts,
  recordFundAccounts,
} from './accounts'
import type { SummaryAccount } from './record'

const accounts: SummaryAccount[] = [
  {
    id: 1,
    name: 'Assets:Cash',
    display_name: '招商储蓄',
    type: 'Assets',
    balances: { CNY: '12000.00' },
  },
  {
    id: 2,
    name: 'Assets:Old',
    display_name: '旧账户（已关）',
    type: 'Assets',
    close_date: '2026-06-01',
    balances: { CNY: '0.00' },
  },
  {
    id: 3,
    name: 'Liabilities:Card',
    display_name: '信用卡',
    type: 'Liabilities',
    balances: { CNY: '-2340.50' },
  },
  {
    id: 4,
    name: 'Expenses:Food',
    display_name: '食品饮料',
    type: 'Expenses',
    balances: { CNY: '4520.00' },
  },
]

describe('关户守门（AC-28）', () => {
  it('close_date 非空即视为已关闭', () => {
    expect(isClosed(accounts[1])).toBe(true)
    expect(isClosed(accounts[0])).toBe(false)
  })

  it('记一笔可选资金账户过滤掉已关户与非资金类型', () => {
    const selectable = recordFundAccounts(accounts)
    expect(selectable.map((account) => account.id)).toEqual([1, 3])
  })

  it('openAccounts 只保留未关闭账户', () => {
    expect(openAccounts(accounts).map((account) => account.id)).toEqual([1, 3, 4])
  })

  it('balanceOf 缺数据 / 缺币种按 0.00，不吞负号', () => {
    // accounts[2] 是 Liabilities:Card（信用卡，余额 -2340.50）。
    expect(balanceOf(accounts[2], 'CNY')).toBe('-2340.50')
    expect(balanceOf(accounts[0], 'USD')).toBe('0.00')
    expect(balanceOf({ id: 9, name: 'x', type: 'Assets' }, 'CNY')).toBe('0.00')
  })
})

describe('分组小计（P-07）', () => {
  it('按类型分组、小计按币种定点求和，负债组为负', () => {
    const groups = groupAccountsByType(accounts, ['Assets', 'Liabilities'])
    expect(groups).toHaveLength(2)
    expect(groups[0].label).toBe('资产')
    expect(groups[0].items.map((account) => account.id)).toEqual([1, 2])
    expect(groups[0].subtotal.CNY).toBe('12000.00')
    expect(groups[1].label).toBe('负债')
    expect(groups[1].items.map((account) => account.id)).toEqual([3])
    expect(groups[1].subtotal.CNY).toBe('-2340.50')
  })

  it('分类页复用：Expenses / Income 分组', () => {
    const groups = groupAccountsByType(accounts, ['Expenses', 'Income'])
    expect(groups[0].label).toBe('支出分类')
    expect(groups[0].items.map((account) => account.id)).toEqual([4])
    expect(groups[1].items).toEqual([])
    expect(groups[1].subtotal).toEqual({})
  })

  it('非法余额按 0 容错，不影响其余币种', () => {
    const broken: SummaryAccount[] = [
      { id: 1, name: 'Assets:A', type: 'Assets', balances: { CNY: 'oops', USD: '1.00' } },
      { id: 2, name: 'Assets:B', type: 'Assets', balances: { CNY: '0.20' } },
    ]
    const groups = groupAccountsByType(broken, ['Assets'])
    expect(groups[0].subtotal.CNY).toBe('0.20')
    expect(groups[0].subtotal.USD).toBe('1.00')
  })
})

describe('受影响交易笔数（AC-29）', () => {
  const txns = [
    { postings: [{ account_id: 1 }, { account_id: 4 }] },
    { postings: [{ account_id: 4 }, { account_id: 1 }] },
    { postings: [{ account_id: 3 }, { account_id: 2 }] },
  ]

  it('同一笔交易命中多分录只计一次', () => {
    expect(affectedTransactionCount(1, txns)).toBe(2)
    expect(affectedTransactionCount(4, txns)).toBe(2)
    expect(affectedTransactionCount(3, txns)).toBe(1)
    expect(affectedTransactionCount(999, txns)).toBe(0)
  })
})
