// @vitest-environment jsdom
/**
 * 批 2.5 修 5：RecordSheet 高级双录模式。
 * - 高级模式提交前按 commodity 分组做配平校验，差额明细直出；
 * - 不平衡的提交被拦截（transactionApi.create 不被调用）；
 * - 高级模式的 postings 走与简化模式**完全相同**的提交路径（幂等键 / 草稿语义不变）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RecordSheet from '@/components/RecordSheet.vue'

const h = vi.hoisted(() => ({
  create: vi.fn(),
  list: vi.fn(),
  seq: 0,
}))

vi.mock('@/api/client', () => ({
  accountApi: { list: h.list },
  transactionApi: { create: h.create, remove: vi.fn(), trash: vi.fn(), restore: vi.fn() },
  newIdempotencyKey: () => {
    h.seq += 1
    return `QA-ADV-${h.seq}`
  },
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    isViewer: false,
    defaultCommodity: 'CNY',
    ledgers: [{ id: 7, name: '家用账本', opening_date: '2026-01-01' }],
  }),
}))

vi.mock('@/stores/recordSheet', () => ({
  useRecordSheetStore: () => ({ notifySaved: vi.fn(), savedEpoch: 0, open: vi.fn() }),
}))

vi.mock('@/stores/privacy', () => ({
  usePrivacyStore: () => ({ enabled: false }),
}))

vi.mock('@/core/storage', () => ({
  readDraft: () => null,
  writeDraft: () => undefined,
  clearDraft: () => undefined,
}))

const ACCOUNTS = [
  { id: 1, name: 'Assets:Cash', display_name: '现金', type: 'Assets' },
  { id: 2, name: 'Assets:Bank', display_name: '银行', type: 'Assets', commodity_restriction: 'USD' },
  { id: 3, name: 'Expenses:Food', display_name: '食品', type: 'Expenses' },
]

async function mountAdvanced(): Promise<ReturnType<typeof mount>> {
  const wrapper = mount(RecordSheet)
  await flushPromises()
  const toggle = wrapper.findAll('button').find((b) => b.text().includes('高级模式'))
  await toggle?.trigger('click')
  await flushPromises()
  return wrapper
}

async function fillRow(
  wrapper: ReturnType<typeof mount>,
  index: number,
  accountId: string,
  amount: string,
): Promise<void> {
  const rows = wrapper.findAll('.rs-adv-row')
  const row = rows[index]
  await row.find('select').setValue(accountId)
  await row.find('input').setValue(amount)
}

function findSave(wrapper: ReturnType<typeof mount>): ReturnType<typeof wrapper.find> {
  const save = wrapper.findAll('button').find((b) => b.text() === '保存')
  expect(save).toBeDefined()
  // eslint 断言辅助：非空已检查
  return save as unknown as ReturnType<typeof wrapper.find>
}

beforeEach(() => {
  h.create.mockReset()
  h.list.mockReset()
  h.seq = 0
  h.list.mockResolvedValue({ accounts: ACCOUNTS })
})

describe('批 2.5 修 5：高级双录', () => {
  it('进入高级模式以简化模式分录预填；配平提交走与简化模式相同路径（幂等键）', async () => {
    h.create.mockResolvedValue({ id: 1, uuid: 't-1', date: '2026-11-10', postings: 2 })
    const wrapper = await mountAdvanced()

    // 预填：账户未加载完成前进入会兜底两条空行；这里直接填写两条配平分录。
    await fillRow(wrapper, 0, '3', '38.50')
    await fillRow(wrapper, 1, '1', '-38.50')

    await findSave(wrapper).trigger('click')
    await flushPromises()

    expect(h.create).toHaveBeenCalledTimes(1)
    const [payload, key] = h.create.mock.calls[0]
    expect(key).toBe('QA-ADV-1')
    expect(payload.postings).toEqual([
      { account_id: 3, commodity: 'CNY', amount: '38.50' },
      { account_id: 1, commodity: 'CNY', amount: '-38.50' },
    ])
  })

  it('不平衡（按币种合计非 0）提交被拦截，差额明细直出且不发起请求', async () => {
    const wrapper = await mountAdvanced()
    await fillRow(wrapper, 0, '3', '100.00')
    await fillRow(wrapper, 1, '1', '-30.00')

    await findSave(wrapper).trigger('click')
    await flushPromises()

    expect(h.create).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('分录不配平')
    expect(wrapper.text()).toContain('CNY 差额 70.00')
  })

  it('跨币种各自配平即可通过（USD 账户自动带出 commodity_restriction）', async () => {
    h.create.mockResolvedValue({ id: 2, uuid: 't-2', date: '2026-11-10', postings: 4 })
    const wrapper = await mountAdvanced()
    // 默认只有 2 行：添加两行做两组币种。
    const add = wrapper.findAll('button').find((b) => b.text().includes('添加分录'))
    await add?.trigger('click')
    await add?.trigger('click')
    await flushPromises()

    await fillRow(wrapper, 0, '1', '-100.00')
    await fillRow(wrapper, 1, '3', '100.00')
    await fillRow(wrapper, 2, '2', '-10.00')
    await fillRow(wrapper, 3, '2', '10.00')

    await findSave(wrapper).trigger('click')
    await flushPromises()

    expect(h.create).toHaveBeenCalledTimes(1)
    const [payload] = h.create.mock.calls[0]
    // USD 账户（commodity_restriction）自动带出 USD。
    expect(payload.postings).toEqual([
      { account_id: 1, commodity: 'CNY', amount: '-100.00' },
      { account_id: 3, commodity: 'CNY', amount: '100.00' },
      { account_id: 2, commodity: 'USD', amount: '-10.00' },
      { account_id: 2, commodity: 'USD', amount: '10.00' },
    ])
  })
})
