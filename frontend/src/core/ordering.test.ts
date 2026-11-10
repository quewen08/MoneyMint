/**
 * 拖拽排序逻辑层用例（P-09，至少逻辑层）
 */
import { describe, expect, it } from 'vitest'

import { moveItem, sortBySortOrder } from './ordering'
import type { SummaryAccount } from './record'

const items = ['a', 'b', 'c', 'd']

describe('moveItem', () => {
  it('后移：a(0) → c(2) 后，b 补到 a 原位置', () => {
    expect(moveItem(items, 0, 2)).toEqual(['b', 'c', 'a', 'd'])
  })

  it('前移：d(3) → a(0)', () => {
    expect(moveItem(items, 3, 0)).toEqual(['d', 'a', 'b', 'c'])
  })

  it('原数组不可变（返回新数组）', () => {
    const copy = moveItem(items, 0, 1)
    expect(items).toEqual(['a', 'b', 'c', 'd'])
    expect(copy).not.toBe(items)
  })

  it('越界 / 相同位置 / 非法索引原样返回副本', () => {
    expect(moveItem(items, -1, 2)).toEqual(items)
    expect(moveItem(items, 0, 4)).toEqual(items)
    expect(moveItem(items, 2, 2)).toEqual(items)
    expect(moveItem(items, Number.NaN, 1)).toEqual(items)
  })
})

describe('sortBySortOrder', () => {
  const accounts: SummaryAccount[] = [
    { id: 3, name: 'Expenses:C', type: 'Expenses', sort_order: 2 },
    { id: 1, name: 'Expenses:A', type: 'Expenses', sort_order: 1 },
    { id: 4, name: 'Expenses:D', type: 'Expenses', sort_order: 2 },
    { id: 2, name: 'Expenses:B', type: 'Expenses', sort_order: 0 },
  ]

  it('按 sort_order 升序，同序号保持原相对顺序（稳定）', () => {
    expect(sortBySortOrder(accounts, (account) => account.sort_order ?? 0).map((a) => a.id)).toEqual([
      2, 1, 3, 4,
    ])
  })

  it('不修改原数组', () => {
    sortBySortOrder(accounts, (account) => account.sort_order ?? 0)
    expect(accounts.map((a) => a.id)).toEqual([3, 1, 4, 2])
  })
})
