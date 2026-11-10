/**
 * 账户视图辅助用例（分组余额 / AC-28 不可选 / AC-29 受影响笔数 / 关户守门口径）
 */
import { describe, expect, it } from 'vitest'

import {
  affectedTransactionCount,
  balanceOf,
  buildAccountTree,
  flattenAccountTree,
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

describe('账户层级树（批 2.5 修 2）', () => {
  // 三级：Food 挂在 FoodGroup（parent_uuid 显式），FoodGroup 冒号推导到根 Expenses。
  const treeAccounts: SummaryAccount[] = [
    {
      id: 1,
      uuid: 'u-root',
      name: 'Expenses:Food',
      display_name: '食品饮料',
      type: 'Expenses',
      sort_order: 0,
      balances: { CNY: '100.00' },
    },
    {
      id: 2,
      uuid: 'u-food',
      name: 'Expenses:Food:Snacks',
      display_name: '零食',
      type: 'Expenses',
      parent_uuid: 'u-root',
      sort_order: 0,
      balances: { CNY: '30.00' },
    },
    {
      id: 3,
      uuid: 'u-drink',
      name: 'Expenses:Food:Drinks',
      display_name: '饮料',
      type: 'Expenses',
      parent_uuid: 'u-root',
      sort_order: 1,
      balances: { CNY: '12.50' },
    },
    {
      // 孤儿：parent_uuid 指向不存在的节点 → 作为根。
      id: 4,
      uuid: 'u-orphan',
      name: 'Expenses:Ghost',
      display_name: '孤儿',
      type: 'Expenses',
      parent_uuid: 'u-missing',
      sort_order: 2,
      balances: { CNY: '5.00' },
    },
  ]

  it('三级树：parent_uuid 优先建树，depth 逐层递增，子随父排序', () => {
    const roots = buildAccountTree(treeAccounts)
    expect(roots.map((node) => node.account.id)).toEqual([1, 4])
    expect(roots[0].children.map((node) => node.account.id)).toEqual([2, 3])
    const flat = flattenAccountTree(roots)
    expect(flat.map((row) => row.account.id)).toEqual([1, 2, 3, 4])
    expect(flat.map((row) => row.depth)).toEqual([0, 1, 1, 0])
    expect(flat[1].childCount).toBe(0)
    expect(roots[0].children.length).toBe(2)
  })

  it('父行小计 = 自身 + 全部后代（定点聚合，fava 折叠小计语义）', () => {
    const roots = buildAccountTree(treeAccounts)
    expect(roots[0].subtotal.CNY).toBe('142.50') // 100 + 30 + 12.50
    expect(roots[1].subtotal.CNY).toBe('5.00')
  })

  it('parent_uuid 缺失时按 name 冒号层级推导兜底', () => {
    const colonAccounts: SummaryAccount[] = [
      { id: 1, name: 'Expenses:Food', type: 'Expenses' },
      { id: 2, name: 'Expenses:Food:Snacks', type: 'Expenses' },
    ]
    const flat = flattenAccountTree(buildAccountTree(colonAccounts))
    expect(flat.map((row) => row.account.id)).toEqual([1, 2])
    expect(flat[1].depth).toBe(1)
  })

  it('parent 与冒号同时可用时 parent_uuid 胜出（显式关系优先，取舍已写入注释）', () => {
    // id=3 冒号推导父应为 id=1，但显式 parent_uuid 指向 id=2。
    const conflict: SummaryAccount[] = [
      { id: 1, uuid: 'u-1', name: 'Expenses:Food', type: 'Expenses' },
      { id: 2, uuid: 'u-2', name: 'Expenses:Other', type: 'Expenses' },
      {
        id: 3,
        uuid: 'u-3',
        name: 'Expenses:Food:Snacks',
        type: 'Expenses',
        parent_uuid: 'u-2',
      },
    ]
    const roots = buildAccountTree(conflict)
    expect(roots.map((node) => node.account.id)).toEqual([1, 2])
    expect(roots[1].children.map((node) => node.account.id)).toEqual([3])
  })

  it('compare 注入：本地拖拽 order 对同层排序生效（供 CategoriesView 传入）', () => {
    // order [3,2]：子层（id1 下）顺序反转为 [3, 2]；孤儿 4 不在 order → 落末尾。
    const order = [3, 2]
    const flat = flattenAccountTree(
      buildAccountTree(treeAccounts, (a, b) => {
        const indexA = order.indexOf(a.id)
        const indexB = order.indexOf(b.id)
        if (indexA === -1 && indexB === -1) return (a.sort_order ?? 0) - (b.sort_order ?? 0)
        if (indexA === -1) return 1
        if (indexB === -1) return -1
        return indexA - indexB
      }),
    )
    expect(flat.map((row) => row.account.id)).toEqual([1, 3, 2, 4])
  })

  it('compare 注入：order 外的新增分类落同层末尾', () => {
    // order [4, 1]：根层 4 提前、1 靠后；id5 不在 order → 落同层末尾。
    const order = [4, 1]
    const withNew: SummaryAccount[] = [
      ...treeAccounts,
      { id: 5, uuid: 'u-new', name: 'Expenses:New', type: 'Expenses', sort_order: 3 },
    ]
    const flat = flattenAccountTree(
      buildAccountTree(withNew, (a, b) => {
        const indexA = order.indexOf(a.id)
        const indexB = order.indexOf(b.id)
        if (indexA === -1 && indexB === -1) return (a.sort_order ?? 0) - (b.sort_order ?? 0)
        if (indexA === -1) return 1
        if (indexB === -1) return -1
        return indexA - indexB
      }),
    )
    // flatten 顺序：根 [4, 1, 5]，id1 的子 [2, 3] 随父。
    expect(flat.map((row) => row.account.id)).toEqual([4, 1, 2, 3, 5])
  })
})
