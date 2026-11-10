/**
 * 分类拖拽顺序持久化用例（§14.5 #2：写 / 读往返、按账本命名空间、损坏容错）
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  categoryOrderKey,
  readCategoryOrder,
  writeCategoryOrder,
} from './storage'

let store: Map<string, string>

beforeEach(() => {
  store = new Map<string, string>()
  const getItem = vi.fn((key: string) => (store.has(key) ? store.get(key) ?? null : null))
  const setItem = vi.fn((key: string, value: string) => {
    store.set(key, value)
  })
  const removeItem = vi.fn((key: string) => {
    store.delete(key)
  })
  vi.stubGlobal('localStorage', { getItem, setItem, removeItem })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('分类顺序持久化（localStorage，按账本命名空间）', () => {
  it('写 / 读往返：分组 id 序列完整还原', () => {
    writeCategoryOrder(7, { Expenses: [12, 11, 13], Income: [21] })
    expect(readCategoryOrder(7)).toEqual({ Expenses: [12, 11, 13], Income: [21] })
    // 键按账本命名空间隔离。
    expect(categoryOrderKey(7)).toBe('fambean.category_order.7')
    expect(store.has('fambean.category_order.7')).toBe(true)
  })

  it('不同账本互不干扰', () => {
    writeCategoryOrder(1, { Expenses: [1] })
    writeCategoryOrder(2, { Expenses: [2] })
    expect(readCategoryOrder(1).Expenses).toEqual([1])
    expect(readCategoryOrder(2).Expenses).toEqual([2])
  })

  it('损坏 JSON 容错为空对象，不抛错', () => {
    store.set(categoryOrderKey(7), '{broken json')
    expect(readCategoryOrder(7)).toEqual({})
  })

  it('非对象 / 数组形状容错为空对象', () => {
    store.set(categoryOrderKey(7), JSON.stringify([1, 2, 3]))
    expect(readCategoryOrder(7)).toEqual({})
    store.set(categoryOrderKey(7), JSON.stringify('oops'))
    expect(readCategoryOrder(7)).toEqual({})
  })

  it('非法 id（负数 / 小数 / 非数字）被过滤，合法项保留', () => {
    store.set(
      categoryOrderKey(7),
      JSON.stringify({ Expenses: [3, -1, 2.5, 'x', 5, 0] }),
    )
    expect(readCategoryOrder(7)).toEqual({ Expenses: [3, 5] })
  })

  it('无数据返回空对象（不写 localStorage）', () => {
    expect(readCategoryOrder(9)).toEqual({})
    expect(store.size).toBe(0)
  })
})
