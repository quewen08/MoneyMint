// @vitest-environment jsdom
/**
 * P-05 软删走通自测（批 2 追加）：列表透出交易 uuid 后，
 * 详情页软删入口解除置灰，确认后 DELETE 恰好以该 uuid 调用一次并返回列表。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import TransactionDetailView from '@/views/TransactionDetailView.vue'

const h = vi.hoisted(() => ({
  remove: vi.fn(),
  listTxns: vi.fn(),
  listAccounts: vi.fn(),
  push: vi.fn(),
  isViewer: false,
}))

vi.mock('@/api/client', () => ({
  accountApi: { list: h.listAccounts },
  transactionApi: { list: h.listTxns, remove: h.remove },
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: {}, query: { id: '11' } }),
  useRouter: () => ({ back: vi.fn(), push: h.push, replace: vi.fn() }),
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    isViewer: h.isViewer,
    defaultCommodity: 'CNY',
    ledgers: [],
  }),
}))

vi.mock('@/stores/privacy', () => ({
  usePrivacyStore: () => ({ enabled: false }),
}))

const ACCOUNTS = {
  accounts: [
    { id: 1, uuid: 'a-1', name: 'Assets:Cash', display_name: '招商储蓄', type: 'Assets', open_date: '2026-01-01', sort_order: 0 },
    { id: 2, uuid: 'a-2', name: 'Expenses:Food', display_name: '食品饮料', type: 'Expenses', open_date: '2026-01-01', sort_order: 0 },
  ],
}

const TXNS = {
  transactions: [
    {
      id: 11,
      uuid: 't-11',
      date: '2026-11-09',
      flag: '*',
      description: '午餐',
      created_by_name: '阿文',
      postings: [
        { account_id: 2, account: 'Expenses:Food', commodity: 'CNY', amount: '38.50' },
        { account_id: 1, account: 'Assets:Cash', commodity: 'CNY', amount: '-38.50' },
      ],
    },
  ],
}

beforeEach(() => {
  h.isViewer = false
  h.remove.mockReset()
  h.remove.mockResolvedValue({ ok: true })
  h.listTxns.mockReset()
  h.listTxns.mockResolvedValue(TXNS)
  h.listAccounts.mockReset()
  h.listAccounts.mockResolvedValue(ACCOUNTS)
  h.push.mockReset()
})

describe('P-05 软删（uuid 解锁后）', () => {
  it('软删入口可用；确认后 remove 恰好以交易 uuid 调用一次并返回列表', async () => {
    const wrapper = mount(TransactionDetailView, { attachTo: document.body })
    await flushPromises()

    expect(wrapper.text()).toContain('午餐')
    const deleteBtn = wrapper.findAll('button').find((b) => b.text().includes('删除这笔交易'))
    expect(deleteBtn?.attributes('disabled')).toBeUndefined()

    await deleteBtn?.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('确认删除')

    const confirmBtn = wrapper.findAll('button').find((b) => b.text() === '确认删除')
    await confirmBtn?.trigger('click')
    await flushPromises()

    expect(h.remove).toHaveBeenCalledTimes(1)
    expect(h.remove).toHaveBeenCalledWith('t-11')
    expect(h.push).toHaveBeenCalledWith({ name: 'transactions' })
  })

  it('只读成员：删除入口禁用且点击不发起请求', async () => {
    h.isViewer = true
    const wrapper = mount(TransactionDetailView, { attachTo: document.body })
    await flushPromises()

    const deleteBtn = wrapper.findAll('button').find((b) => b.text().includes('删除这笔交易'))
    expect(deleteBtn?.attributes('disabled')).toBeDefined()
    await deleteBtn?.trigger('click')
    await flushPromises()
    expect(h.remove).not.toHaveBeenCalled()
  })
})
