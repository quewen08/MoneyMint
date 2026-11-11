// @vitest-environment jsdom
/**
 * QA 独立补充：RecordSheet 幂等键生命周期（AC-13 / §7 冲突矩阵，Round 1）
 *
 * 用真实的 `<script setup>` submit 逻辑（只 mock 外部依赖：API / stores / storage），
 * 通过点击键盘与保存按钮走真实用户路径，证伪 / 确认声明 5：
 *  - submit 时按 payload 快照比较：纯重试 / 改回原值复用同键，内容真变才换键；
 *  - 命中 409 idempotency_key_deleted / idempotency_payload_mismatch 后换键重提；
 *  - 保存成功后换键（避免「复制上一笔」命中旧交易）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import { ApiError } from '@/api/errors'
import RecordSheet from '@/components/RecordSheet.vue'

const h = vi.hoisted(() => ({
  create: vi.fn(),
  list: vi.fn(),
  seq: 0,
}))

vi.mock('@/api/client', () => ({
  accountApi: { list: h.list },
  transactionApi: { create: h.create },
  newIdempotencyKey: () => {
    h.seq += 1
    return `QA-KEY-${h.seq}`
  },
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    isViewer: false,
    defaultCommodity: 'CNY',
  }),
}))

vi.mock('@/stores/recordSheet', () => ({
  useRecordSheetStore: () => ({ notifySaved: vi.fn() }),
}))

vi.mock('@/stores/privacy', () => ({
  usePrivacyStore: () => ({ enabled: false }),
}))

vi.mock('@/core/storage', () => ({
  readDraft: () => null,
  writeDraft: () => undefined,
  clearDraft: () => undefined,
  // 批 3 §14.5 #5：默认账户预选 / 回写；本文件返回空映射，保持原有用例语义。
  readDefaultAccounts: () => ({}),
  setDefaultAccount: () => undefined,
}))

const ACCOUNTS = [
  { id: 1, name: 'Assets:Cash', type: 'Assets', balances: { CNY: '1000.00' } },
  { id: 2, name: 'Expenses:Food', type: 'Expenses' },
  { id: 3, name: 'Income:Salary', type: 'Income' },
  { id: 5, name: 'Assets:Bank', type: 'Assets' },
]

const OK_RESULT = {
  id: 11,
  uuid: 'u-qa-1',
  date: '2026-11-10',
  postings: 2,
  idempotent: false,
}

function keyOf(callIndex: number): string {
  const call = h.create.mock.calls[callIndex]
  expect(call, `第 ${callIndex + 1} 次 create 未发生`).toBeDefined()
  return call[1] as string
}

function payloadOf(callIndex: number): { postings: Array<{ amount: string }> } {
  return h.create.mock.calls[callIndex][0] as {
    postings: Array<{ amount: string }>
  }
}

/** 点击数字键盘上的某个键（按可见文本定位）。 */
async function press(wrapper: VueWrapper, label: string): Promise<void> {
  const buttons = wrapper.findAll('.rs-key')
  const target = buttons.find((button) => button.text() === label)
  if (target === undefined) {
    throw new Error(`键盘按键 ${label} 未找到`)
  }
  await target.trigger('click')
}

/** 点击键盘「保存」键（绕过 disabled，保证一定进入 submit）。 */
async function save(wrapper: VueWrapper): Promise<void> {
  await press(wrapper, '保存')
  await flushPromises()
}

/** 输入金额 100（三个键），并等待响应式更新。 */
async function enterAmount100(wrapper: VueWrapper): Promise<void> {
  await press(wrapper, '1')
  await press(wrapper, '0')
  await press(wrapper, '0')
  await flushPromises()
}

async function mountSheet(): Promise<VueWrapper> {
  const wrapper = mount(RecordSheet)
  await flushPromises() // 等待 onMounted 的 loadAccounts
  return wrapper
}

beforeEach(() => {
  h.create.mockReset()
  h.list.mockReset()
  h.seq = 0
  h.list.mockResolvedValue({ accounts: ACCOUNTS })
})

describe('幂等键：submit 时按 payload 快照比较', () => {
  it('纯网络失败重试（payload 不变）复用同一个键', async () => {
    // Arrange
    const wrapper = await mountSheet()
    await enterAmount100(wrapper)
    h.create.mockRejectedValueOnce(new Error('network down'))
    h.create.mockResolvedValueOnce(OK_RESULT)

    // Act：连续两次提交同一 payload
    await save(wrapper)
    await save(wrapper)

    // Assert
    expect(h.create).toHaveBeenCalledTimes(2)
    expect(h.create.mock.calls[0][0]).toEqual(h.create.mock.calls[1][0])
    expect(keyOf(0)).toBe(keyOf(1))
    expect(keyOf(0)).toMatch(/^QA-KEY-/)
    wrapper.unmount()
  })

  it('改回原值仍复用同键（键生命周期不受中间编辑干扰）', async () => {
    // Arrange
    const wrapper = await mountSheet()
    await enterAmount100(wrapper)
    h.create.mockRejectedValueOnce(new Error('network down'))
    h.create.mockRejectedValueOnce(new Error('network down'))
    h.create.mockResolvedValueOnce(OK_RESULT)

    // Act：A 失败 → 中途改成 1000 → 再改回 100 → 再提交
    await save(wrapper) // A = 100
    await press(wrapper, '0') // 1000
    await flushPromises()
    await press(wrapper, '⌫') // 100（改回原值）
    await flushPromises()
    await save(wrapper) // A 再提交

    // Assert
    expect(h.create).toHaveBeenCalledTimes(2)
    expect(payloadOf(0)).toEqual(payloadOf(1))
    expect(keyOf(0)).toBe(keyOf(1))
    wrapper.unmount()
  })

  it('内容真的变化（金额从 100 改 1000）才换键', async () => {
    // Arrange
    const wrapper = await mountSheet()
    await enterAmount100(wrapper)
    h.create.mockRejectedValueOnce(new Error('network down'))
    h.create.mockResolvedValueOnce(OK_RESULT)

    // Act
    await save(wrapper)
    await press(wrapper, '0')
    await flushPromises()
    await save(wrapper)

    // Assert
    expect(h.create).toHaveBeenCalledTimes(2)
    expect(payloadOf(0)).not.toEqual(payloadOf(1))
    expect(keyOf(0)).not.toBe(keyOf(1))
    wrapper.unmount()
  })
})

describe('幂等键：409 冲突后换新键重提', () => {
  it('命中 idempotency_key_deleted 后换键，用户重提成功', async () => {
    // Arrange
    const wrapper = await mountSheet()
    await enterAmount100(wrapper)
    h.create.mockRejectedValueOnce(
      new ApiError(409, '该幂等键对应的交易已被删除', {
        code: 'idempotency_key_deleted',
      }),
    )
    h.create.mockResolvedValueOnce(OK_RESULT)

    // Act
    await save(wrapper)
    const errorText = wrapper.find('.rs-alert--error').text()
    await save(wrapper) // 用户看到错误后再点一次保存

    // Assert
    expect(errorText).toContain('已被删除')
    expect(h.create).toHaveBeenCalledTimes(2)
    expect(keyOf(0)).not.toBe(keyOf(1))
    expect(payloadOf(0)).toEqual(payloadOf(1))
    wrapper.unmount()
  })

  it('命中 idempotency_payload_mismatch 后同样换键', async () => {
    // Arrange
    const wrapper = await mountSheet()
    await enterAmount100(wrapper)
    h.create.mockRejectedValueOnce(
      new ApiError(409, '该幂等键已用于另一笔内容不同的交易', {
        code: 'idempotency_payload_mismatch',
      }),
    )
    h.create.mockResolvedValueOnce(OK_RESULT)

    // Act
    await save(wrapper)
    await save(wrapper)

    // Assert
    expect(h.create).toHaveBeenCalledTimes(2)
    expect(keyOf(0)).not.toBe(keyOf(1))
    wrapper.unmount()
  })

  it('普通 5xx 失败不换键（失败保留表单与键供原样重试）', async () => {
    // Arrange
    const wrapper = await mountSheet()
    await enterAmount100(wrapper)
    h.create.mockRejectedValueOnce(new ApiError(503, '服务暂不可用'))
    h.create.mockResolvedValueOnce(OK_RESULT)

    // Act
    await save(wrapper)
    await save(wrapper)

    // Assert
    expect(h.create).toHaveBeenCalledTimes(2)
    expect(keyOf(0)).toBe(keyOf(1))
    wrapper.unmount()
  })
})
