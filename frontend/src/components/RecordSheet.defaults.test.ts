// @vitest-environment jsdom
/**
 * 批 3 组件回归：§14.5 #5 默认账户预选
 *
 * 守住：记一笔打开时按账本命名空间的默认账户预选分类；
 * 记忆的 uuid 已失效（账户被删除 / 关户）时回退首个可用分类并提示。
 * 本文件用真实 localStorage（只 mock API 与 stores），验证端到端读写链路。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RecordSheet from './RecordSheet.vue'
import { writeDefaultAccounts } from '@/core/storage'

const h = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  accountApi: { list: h.list },
  transactionApi: { create: h.create },
  newIdempotencyKey: () => 'QA-KEY',
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    isViewer: false,
    needsInvite: false,
    defaultCommodity: 'CNY',
  }),
}))

vi.mock('@/stores/recordSheet', () => ({
  useRecordSheetStore: () => ({ notifySaved: vi.fn() }),
}))

vi.mock('@/stores/privacy', () => ({
  usePrivacyStore: () => ({ enabled: false }),
}))

// 分类顺序固定：食品（id=2）、交通（id=3）。
const ACCOUNTS = [
  { id: 1, uuid: 'u-cash', name: 'Assets:Cash', display_name: '现金', type: 'Assets' },
  { id: 2, uuid: 'u-food', name: 'Expenses:Food', display_name: '食品', type: 'Expenses' },
  { id: 3, uuid: 'u-travel', name: 'Expenses:Travel', display_name: '交通', type: 'Expenses' },
  { id: 4, uuid: 'u-salary', name: 'Income:Salary', display_name: '薪资', type: 'Income' },
]

function selectedIndex(wrapper: ReturnType<typeof mount>): number {
  const cats = wrapper.findAll('.rs-cat')
  for (let i = 0; i < cats.length; i += 1) {
    if (cats[i].classes().indexOf('on') >= 0) {
      return i
    }
  }
  return -1
}

beforeEach(() => {
  localStorage.clear()
  h.list.mockReset()
  h.create.mockReset()
  h.list.mockResolvedValue({ accounts: ACCOUNTS })
})

describe('批3-K：默认账户预选（§14.5 #5）', () => {
  it('按 uuid 预选分类，而不是永远取第一个', async () => {
    writeDefaultAccounts(7, { expense: 'u-travel' })
    const wrapper = mount(RecordSheet)
    await flushPromises()
    expect(selectedIndex(wrapper)).toBe(1)
    wrapper.unmount()
  })

  it('记忆的账户已删除时回退首个可用分类并给出提示', async () => {
    writeDefaultAccounts(7, { expense: 'u-deleted' })
    const wrapper = mount(RecordSheet)
    await flushPromises()
    expect(selectedIndex(wrapper)).toBe(0)
    expect(wrapper.text()).toContain('默认账户已失效，已回退为首个可用账户')
    wrapper.unmount()
  })

  it('已关户账户不可作为默认账户', async () => {
    writeDefaultAccounts(7, { expense: 'u-travel' })
    const closed = ACCOUNTS.map((account) =>
      account.id === 3 ? { ...account, close_date: '2026-11-01' } : account,
    )
    h.list.mockResolvedValue({ accounts: closed })
    const wrapper = mount(RecordSheet)
    await flushPromises()
    expect(selectedIndex(wrapper)).toBe(0)
    wrapper.unmount()
  })

  it('没有记忆时保持原有「首个可用分类」行为', async () => {
    const wrapper = mount(RecordSheet)
    await flushPromises()
    expect(selectedIndex(wrapper)).toBe(0)
    expect(wrapper.text()).not.toContain('默认账户已失效')
    wrapper.unmount()
  })
})
