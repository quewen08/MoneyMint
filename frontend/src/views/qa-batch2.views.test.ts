// @vitest-environment jsdom
/**
 * QA 批 2 组件级独立证伪（Round 1，严过关 / Yan）
 *
 *  - AC-28 真实链路：RecordSheet.vue 的资金 / 分类 / 转账下拉必须真的排除已关户账户
 *    （含「空串 close_date 视为未关闭」的口径一致性）；
 *  - §14.5 #2 合并语义：CategoriesView.vue 本地 order 与服务端 sort_order 合并时，
 *    「新增分类落末尾」「删除分类不产生幽灵项」必须成立。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import RecordSheet from '@/components/RecordSheet.vue'
import CategoriesView from '@/views/CategoriesView.vue'
import { writeCategoryOrder } from '@/core/storage'

const h = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  close: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  accountApi: { list: h.list, create: h.create, close: h.close },
  transactionApi: { create: vi.fn() },
  newIdempotencyKey: () => 'QA-KEY',
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    isViewer: false,
    defaultCommodity: 'CNY',
    ledgers: [{ id: 7, name: '家用账本', opening_date: '2026-01-01' }],
    needsInvite: false,
  }),
}))

vi.mock('@/stores/recordSheet', () => ({
  useRecordSheetStore: () => ({ notifySaved: vi.fn(), savedEpoch: 0, open: vi.fn() }),
}))

vi.mock('@/stores/privacy', () => ({
  usePrivacyStore: () => ({ enabled: false }),
}))

beforeEach(() => {
  h.list.mockReset()
  h.create.mockReset()
  h.close.mockReset()
  localStorage.clear()
})

// ---------------------------------------------------------------------------
// AC-28：RecordSheet 关户过滤真实链路
// ---------------------------------------------------------------------------
describe('批2-G：RecordSheet 下拉真的排除已关户账户（AC-28）', () => {
  const ACCOUNTS = [
    { id: 1, name: 'Assets:Cash', display_name: '现金', type: 'Assets' },
    { id: 2, name: 'Assets:OldCard', display_name: '已注销卡', type: 'Assets', close_date: '2026-11-01' },
    { id: 3, name: 'Liabilities:Loan', display_name: '房贷', type: 'Liabilities' },
    { id: 4, name: 'Expenses:Food', display_name: '食品', type: 'Expenses' },
    { id: 5, name: 'Assets:Empty', display_name: '空串关闭日期', type: 'Assets', close_date: '' },
  ]

  it('资金 / 转账 / 分类下拉全部不含已关户账户，空串 close_date 仍可选', async () => {
    h.list.mockResolvedValue({ accounts: ACCOUNTS })
    const wrapper = mount(RecordSheet)
    await flushPromises()

    const values: string[] = []
    for (const option of wrapper.findAll('option')) {
      values.push(option.element.value)
    }
    expect(values).toContain('1')
    expect(values).toContain('5') // close_date='' 与「未关闭」口径一致，仍可选
    expect(values).not.toContain('2') // 关户账户绝不可选
    wrapper.unmount()
  })

  it('「新建后立刻关户再记账」时序：重新打开组件会重新拉取并过滤', async () => {
    // 第一次打开：账户 2 未关户
    h.list.mockResolvedValueOnce({ accounts: ACCOUNTS.filter((a) => a.id !== 2) })
    const first = mount(RecordSheet)
    await flushPromises()
    const firstValues: string[] = []
    for (const option of first.findAll('option')) {
      firstValues.push(option.element.value)
    }
    // 此时若关户发生在别的视图，重新打开（重新 mount）必然重新拉取：
    h.list.mockResolvedValueOnce({
      accounts: ACCOUNTS.map((a) => (a.id === 2 ? { ...a, close_date: '2026-11-10' } : a)),
    })
    const second = mount(RecordSheet)
    await flushPromises()
    const secondValues: string[] = []
    for (const option of second.findAll('option')) {
      secondValues.push(option.element.value)
    }
    expect(secondValues).not.toContain('2')
    first.unmount()
    second.unmount()
  })
})

// ---------------------------------------------------------------------------
// §14.5 #2：CategoriesView 本地 order × 服务端 sort_order 合并语义
// ---------------------------------------------------------------------------
describe('批2-H：分类排序合并（新增落末尾 / 删除无幽灵项）', () => {
  function accountsFor(ids: number[]): { accounts: unknown[] } {
    const all: Record<number, unknown> = {
      1: { id: 1, uuid: 'u1', name: 'Expenses:Food', display_name: '食品', type: 'Expenses', sort_order: 0 },
      2: { id: 2, uuid: 'u2', name: 'Expenses:Transport', display_name: '交通', type: 'Expenses', sort_order: 1 },
      3: { id: 3, uuid: 'u3', name: 'Income:Salary', display_name: '薪资', type: 'Income', sort_order: 0 },
      4: { id: 4, uuid: 'u4', name: 'Expenses:New', display_name: '新分类', type: 'Expenses', sort_order: 2 },
    }
    return { accounts: ids.map((id) => all[id]) }
  }

  /** 按「分类名」提取 Expenses 组（支出分类 card）内的渲染顺序。 */
  function expensesOrder(wrapper: VueWrapper): string[] {
    const names: string[] = []
    for (const card of wrapper.findAll('.cats__card')) {
      const heading = card.find('h3')?.text() ?? ''
      if (!heading.startsWith('支出分类')) {
        continue
      }
      for (const row of card.findAll('.cats__row')) {
        const name = row.find('.cats__name')?.text() ?? ''
        if (name !== '') {
          names.push(name.trim().split('已关闭')[0].trim())
        }
      }
    }
    return names
  }

  it('本地 order [2,1] 时按本地序渲染（覆盖服务端 sort_order）', async () => {
    h.list.mockResolvedValue(accountsFor([1, 2, 3]))
    writeCategoryOrder(7, { Expenses: [2, 1] })
    const wrapper = mount(CategoriesView)
    await flushPromises()
    const names = expensesOrder(wrapper)
    expect(names.indexOf('交通')).toBeLessThan(names.indexOf('食品'))
    wrapper.unmount()
  })

  it('新增分类（本地 order 无记录）追加到该组末尾', async () => {
    h.list.mockResolvedValue(accountsFor([1, 2, 4, 3]))
    writeCategoryOrder(7, { Expenses: [2, 1] })
    const wrapper = mount(CategoriesView)
    await flushPromises()
    const names = expensesOrder(wrapper)
    expect(names).toEqual(['交通', '食品', '新分类'])
    wrapper.unmount()
  })

  it('删除分类后残留 id 不产生幽灵项，剩余项保持本地相对顺序', async () => {
    h.list.mockResolvedValue(accountsFor([1, 3])) // id=2 已删除
    writeCategoryOrder(7, { Expenses: [2, 1] })
    const wrapper = mount(CategoriesView)
    await flushPromises()
    const names = expensesOrder(wrapper)
    expect(names).toEqual(['食品'])
    expect(wrapper.text()).not.toContain('交通')
    wrapper.unmount()
  })
})
