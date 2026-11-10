/**
 * 筛选 query 化用例（AC-16：写 / 读往返还原、非法值容错、筛选应用）
 */
import { describe, expect, it } from 'vitest'

import { applyFilters, emptyFilters, filtersFromQuery, filtersToQuery, hasActiveFilters } from './filters'
import type { SummaryAccount } from './record'

const accounts: SummaryAccount[] = [
  { id: 1, name: 'Assets:Cash', display_name: '招商储蓄', type: 'Assets' },
  { id: 2, name: 'Assets:Bank', display_name: '工商银行', type: 'Assets' },
  { id: 4, name: 'Expenses:Food', display_name: '食品饮料', type: 'Expenses' },
  { id: 5, name: 'Income:Salary', display_name: '薪资收入', type: 'Income' },
]

const txns = [
  {
    id: 1,
    date: '2026-11-09',
    flag: '*',
    description: '午餐',
    created_by_name: '阿文',
    postings: [
      { account_id: 4, amount: '38.50' },
      { account_id: 1, amount: '-38.50' },
    ],
  },
  {
    id: 2,
    date: '2026-11-01',
    flag: '*',
    description: '十月工资',
    created_by_name: '太太',
    postings: [
      { account_id: 5, amount: '-9500.00' },
      { account_id: 2, amount: '9500.00' },
    ],
  },
  {
    id: 3,
    date: '2026-11-05',
    flag: '*',
    description: '转账',
    created_by_name: '阿文',
    postings: [
      { account_id: 1, amount: '-2000.00' },
      { account_id: 2, amount: '2000.00' },
    ],
  },
]

describe('筛选 query 化（AC-16）', () => {
  it('写 / 读往返还原：全部字段都进 query 并能完整还原', () => {
    const filters = {
      from: '2026-11-01',
      to: '2026-11-09',
      kind: 'transfer' as const,
      account: 2,
      category: 4,
      member: '阿文',
    }
    const query = filtersToQuery(filters)
    expect(query).toEqual({
      from: '2026-11-01',
      to: '2026-11-09',
      kind: 'transfer',
      account: '2',
      category: '4',
      member: '阿文',
    })
    expect(filtersFromQuery(query)).toEqual(filters)
  })

  it('空筛选产出干净 query（空值省略），还原回默认值', () => {
    const filters = emptyFilters()
    expect(filtersToQuery(filters)).toEqual({})
    expect(filtersFromQuery({})).toEqual(filters)
    expect(hasActiveFilters(filters)).toBe(false)
  })

  it('非法值容错：坏日期 / 坏枚举 / 坏 id 一律回退默认，不抛错', () => {
    const restored = filtersFromQuery({
      from: '2026/11/01',
      to: 'not-a-date',
      kind: 'hack',
      account: '-3',
      category: 'abc',
    })
    expect(restored).toEqual(emptyFilters())
  })

  it('vue-router 数组形状的 query 取第一个值', () => {
    expect(filtersFromQuery({ member: ['阿文', '太太'] }).member).toBe('阿文')
    expect(filtersFromQuery({ account: ['2', '9'] }).account).toBe(2)
  })

  it('hasActiveFilters 随任一字段生效而变化', () => {
    const filters = emptyFilters()
    filters.kind = 'expense'
    expect(hasActiveFilters(filters)).toBe(true)
    filters.kind = 'all'
    filters.from = '2026-11-01'
    expect(hasActiveFilters(filters)).toBe(true)
  })
})

describe('筛选应用', () => {
  it('日期范围（字典序含端点）', () => {
    const result = applyFilters(txns, { ...emptyFilters(), from: '2026-11-01', to: '2026-11-05' }, accounts)
    expect(result.map((txn) => txn.id)).toEqual([2, 3])
  })

  it('kind 判定与首页口径一致（转账 / 支出 / 收入）', () => {
    expect(applyFilters(txns, { ...emptyFilters(), kind: 'transfer' }, accounts).map((t) => t.id)).toEqual([3])
    expect(applyFilters(txns, { ...emptyFilters(), kind: 'expense' }, accounts).map((t) => t.id)).toEqual([1])
    expect(applyFilters(txns, { ...emptyFilters(), kind: 'income' }, accounts).map((t) => t.id)).toEqual([2])
  })

  it('账户 / 分类命中任一分录即保留；成员精确匹配', () => {
    expect(applyFilters(txns, { ...emptyFilters(), account: 2 }, accounts).map((t) => t.id)).toEqual([2, 3])
    expect(applyFilters(txns, { ...emptyFilters(), category: 4 }, accounts).map((t) => t.id)).toEqual([1])
    expect(applyFilters(txns, { ...emptyFilters(), member: '太太' }, accounts).map((t) => t.id)).toEqual([2])
  })
})
