// @vitest-environment jsdom
/**
 * AC-27 自测（批 2 追加）：修改期初保存成功后重试同一操作，
 * 前端两次上送的请求参数必须完全一致（uuid + body）——
 * 「冲正 + 新建」成对交易的唯一性由服务端 opening_key 幂等保证，
 * 前端的义务是不引入任何会破坏幂等的变化（不换键、不改参数、不重复置位）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AccountView } from '@/api/types'
import AccountForm from '@/components/AccountForm.vue'

const h = vi.hoisted(() => ({
  get: vi.fn(),
  set: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  accountApi: { create: vi.fn(), close: vi.fn(), list: vi.fn() },
  openingApi: { get: h.get, set: h.set },
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

const ACCOUNT: AccountView = {
  id: 1,
  uuid: 'a-1',
  name: 'Assets:Cash',
  display_name: '招商储蓄',
  type: 'Assets',
  open_date: '2026-01-01',
  sort_order: 0,
  balances: { CNY: '12500.00' },
}

async function mountOpeningForm(): Promise<ReturnType<typeof mount>> {
  const wrapper = mount(AccountForm, {
    props: { mode: 'opening', account: ACCOUNT },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

async function clickSave(wrapper: ReturnType<typeof mount>): Promise<void> {
  const save = wrapper.findAll('button').find((b) => b.text() === '保存')
  expect(save).toBeDefined()
  await save?.trigger('click')
  await flushPromises()
}

beforeEach(() => {
  h.set.mockReset()
  h.get.mockReset()
  h.get.mockResolvedValue({
    has_opening: true,
    account_uuid: 'a-1',
    amount: '12500.00',
    direction: 'hold',
    commodity: 'CNY',
    date: '2026-01-01',
    version: 1,
    signed_amount: '12500.00',
  })
  h.set.mockResolvedValue({
    status: 'created',
    account_uuid: 'a-1',
    transaction_uuid: 'opening-1',
    version: 2,
    signed_amount: '13000.00',
    commodity: 'CNY',
    date: '2026-01-01',
  })
})

describe('AC-27：修改期初的重试幂等（前端容忍逻辑）', () => {
  it('预填既有期初，修改后两次提交的请求参数完全一致（后端 opening_key 幂等只生成一对）', async () => {
    const wrapper = await mountOpeningForm()

    // 预填既有期初（amount 为非负绝对值，方向 hold）。
    expect(h.get).toHaveBeenCalledWith('a-1')
    const amountInput = wrapper.findAll('input').find((i) => (i.element as HTMLInputElement).value === '12500.00')
    expect(amountInput).toBeDefined()

    // 修改为 13000 并第一次保存。
    await amountInput?.setValue('13000')
    await clickSave(wrapper)
    expect(h.set).toHaveBeenCalledTimes(1)

    // 不改任何内容直接重试第二次保存（弱网重试场景）。
    await clickSave(wrapper)
    expect(h.set).toHaveBeenCalledTimes(2)

    const calls = h.set.mock.calls
    expect(calls[0][0]).toBe('a-1')
    expect(calls[1][0]).toBe('a-1')
    // 两次 body 逐字段一致：前端不会引入任何破坏幂等的变化。
    expect(calls[1][1]).toEqual(calls[0][1])
    expect(calls[0][1]).toEqual({
      amount: '13000.00',
      direction: 'hold',
      commodity: 'CNY',
      date: '2026-01-01',
    })
  })

  it('欠款方向映射为负（FR-05 落库符号由前端方向表达，服务端落库）', async () => {
    const wrapper = await mountOpeningForm()
    // 先改金额再切方向，避免依赖默认预填值。
    const amountInput = wrapper.findAll('input').find((i) => (i.element as HTMLInputElement).value === '12500.00')
    await amountInput?.setValue('13000')
    await wrapper.findAll('select')[0].setValue('owe')
    await clickSave(wrapper)
    expect(h.set).toHaveBeenCalledWith('a-1', {
      amount: '13000.00',
      direction: 'owe',
      commodity: 'CNY',
      date: '2026-01-01',
    })
  })
})
