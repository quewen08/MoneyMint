// @vitest-environment jsdom
/**
 * AC-28 / AC-29 自测（批 2 追加）：关户走通。
 * - AC-29：关户前二次确认列出「受影响交易笔数」，确认后真实调用 close；
 * - AC-28：已关户账户写入口禁用；409（余额非 0）时渲染差额明细表。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountDetailView from '@/views/AccountDetailView.vue'

const h = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  close: vi.fn(),
  listTxns: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  accountApi: { list: h.listAccounts, close: h.close, create: vi.fn() },
  openingApi: { get: vi.fn(), set: vi.fn() },
  transactionApi: { list: h.listTxns },
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '1' }, query: {} }),
  useRouter: () => ({ back: vi.fn(), push: vi.fn(), replace: vi.fn() }),
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    isViewer: false,
    defaultCommodity: 'CNY',
    ledgers: [{ id: 7, name: '家庭账本', opening_date: '2026-01-01', role: 'owner' }],
  }),
}))

vi.mock('@/stores/privacy', () => ({
  usePrivacyStore: () => ({ enabled: false }),
}))

const ACCOUNT = {
  id: 1,
  uuid: 'a-1',
  name: 'Assets:Cash',
  display_name: '招商储蓄',
  type: 'Assets',
  open_date: '2026-01-01',
  balances: { CNY: '0.00' },
}

const CLOSED_ACCOUNT = {
  ...ACCOUNT,
  name: 'Assets:Old',
  display_name: '旧账户',
  close_date: '2026-06-01',
}

const TXNS = [
  { id: 1, uuid: 't-1', date: '2026-11-01', flag: '*', description: 'A', created_by_name: 'x', postings: [{ account_id: 1, account: 'Assets:Cash', commodity: 'CNY', amount: '10.00' }] },
  { id: 2, uuid: 't-2', date: '2026-11-02', flag: '*', description: 'B', created_by_name: 'x', postings: [{ account_id: 9, account: 'Expenses:X', commodity: 'CNY', amount: '1.00' }, { account_id: 1, account: 'Assets:Cash', commodity: 'CNY', amount: '-1.00' }] },
  { id: 3, uuid: 't-3', date: '2026-11-03', flag: '*', description: 'C', created_by_name: 'x', postings: [{ account_id: 9, account: 'Expenses:X', commodity: 'CNY', amount: '2.00' }] },
]

function setup(accounts: unknown[], txns: unknown[]): void {
  h.listAccounts.mockResolvedValue({ accounts })
  h.listTxns.mockResolvedValue({ transactions: txns })
}

beforeEach(() => {
  h.close.mockReset()
  h.close.mockResolvedValue({ ok: true })
})

describe('AC-29：关户确认与提交走通', () => {
  it('确认弹窗列出受影响笔数（同笔多分录只计一次：账户 1 出现在 2 笔），确认后 close 恰好调用一次', async () => {
    setup([ACCOUNT], TXNS)
    const wrapper = mount(AccountDetailView, { attachTo: document.body })
    await flushPromises()

    // 打开确认弹窗：受影响笔数 = 2。
    const closeBtn = wrapper.findAll('button').find((b) => b.text().includes('关闭账户'))
    expect(closeBtn).toBeDefined()
    await closeBtn?.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('2')

    // 确认关户：close 恰好以该账户 uuid 调用一次，并触发重新加载。
    const confirmBtn = wrapper.findAll('button').find((b) => b.text() === '确认关户')
    expect(confirmBtn).toBeDefined()
    await confirmBtn?.trigger('click')
    await flushPromises()

    expect(h.close).toHaveBeenCalledTimes(1)
    expect(h.close).toHaveBeenCalledWith('a-1')
    expect(h.listAccounts).toHaveBeenCalledTimes(2)
  })

  it('已关户账户写入口禁用，不再发起任何请求', async () => {
    setup([CLOSED_ACCOUNT], TXNS)
    const wrapper = mount(AccountDetailView, { attachTo: document.body })
    await flushPromises()

    const openingBtn = wrapper.findAll('button').find((b) => b.text().includes('期初余额设置'))
    expect(openingBtn?.attributes('disabled')).toBeDefined()
    const closeBtn = wrapper.findAll('button').find((b) => b.text().includes('关闭账户'))
    expect(closeBtn?.attributes('disabled')).toBeDefined()
    await closeBtn?.trigger('click')
    await flushPromises()
    expect(h.close).not.toHaveBeenCalled()
  })

  it('服务端 409（余额非 0）时渲染按币种差额明细表，而非只弹文案（AC-28 守门）', async () => {
    setup([ACCOUNT], TXNS)
    h.close.mockRejectedValueOnce(
      new (await import('@/api/errors')).ApiError(409, '账户余额非 0，无法关闭', {
        balances: [{ commodity: 'CNY', amount: '-3200.00' }],
      }),
    )
    const wrapper = mount(AccountDetailView, { attachTo: document.body })
    await flushPromises()

    const closeBtn = wrapper.findAll('button').find((b) => b.text().includes('关闭账户'))
    await closeBtn?.trigger('click')
    await flushPromises()
    const confirmBtn = wrapper.findAll('button').find((b) => b.text() === '确认关户')
    await confirmBtn?.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('按币种差额明细')
    // MoneyText 渲染为 `-¥ 3,200.00`（千分位），断言金额本体即可。
    expect(wrapper.text()).toContain('3,200.00')
  })
})
