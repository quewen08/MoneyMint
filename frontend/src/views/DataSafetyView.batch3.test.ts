// @vitest-environment jsdom
/**
 * 批 3 组件回归：P-15 数据安全
 *
 * 核心守住 §14.5 #3：导出前恒等式检查不平即**阻断**（旧版是导出后告警、
 * 不阻断），且展示按币种差额；备份区仅系统管理员可见。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import DataSafetyView from './DataSafetyView.vue'

const h = vi.hoisted(() => ({
  isAdmin: true as boolean,
  check: vi.fn(),
  exportBeancount: vi.fn(),
  exportArchive: vi.fn(),
  backupList: vi.fn(),
  backupRun: vi.fn(),
  backupDownload: vi.fn(),
  txnList: vi.fn(),
  accountList: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  accountApi: { list: h.accountList },
  transactionApi: { list: h.txnList },
  backupApi: { list: h.backupList, run: h.backupRun, download: h.backupDownload },
  exportApi: { beancount: h.exportBeancount, archive: h.exportArchive },
  identityApi: { check: h.check },
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    ledgerName: '家用账本',
    user: { id: 1, username: 'boss', role: 'owner', is_admin: h.isAdmin, must_change_password: false },
  }),
}))

// 保留 core/batch3 的全部纯函数实现，只把浏览器下载替换成 spy。
vi.mock('@/core/batch3', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/core/batch3')>()
  return {
    ...actual,
    downloadBlob: vi.fn(),
    downloadText: vi.fn(),
  }
})

beforeEach(() => {
  h.isAdmin = true
  h.check.mockReset()
  h.exportBeancount.mockReset()
  h.exportArchive.mockReset()
  h.backupList.mockReset()
  h.backupRun.mockReset()
  h.backupDownload.mockReset()
  h.txnList.mockReset()
  h.accountList.mockReset()
  h.backupList.mockResolvedValue({ items: [] })
  h.check.mockResolvedValue({ balanced: true, diffs: [] })
  h.exportBeancount.mockResolvedValue({ blob: new Blob(['x']), balanced: true, diffs: [] })
  h.exportArchive.mockResolvedValue({ blob: new Blob(['x']), balanced: true, diffs: [] })
  h.txnList.mockResolvedValue({ transactions: [] })
  h.accountList.mockResolvedValue({ accounts: [] })
})

function exportButton(wrapper: ReturnType<typeof mount>) {
  return wrapper.findAll('.ds__card')[0].findAll('button')[0]
}

describe('批3-H：导出前不平阻断（§14.5 #3 / AC-31）', () => {
  it('恒等式不平时阻断导出、列出差额，且不调用导出端点', async () => {
    h.check.mockResolvedValue({ balanced: false, diffs: [{ commodity: 'CNY', sum: '0.01' }] })
    const wrapper = mount(DataSafetyView)
    await flushPromises()
    await exportButton(wrapper).trigger('click')
    await flushPromises()

    expect(h.check).toHaveBeenCalledTimes(1)
    expect(h.exportBeancount).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('账本不平，已阻断导出')
    expect(wrapper.text()).toContain('CNY +0.01')
    wrapper.unmount()
  })

  it('恒等式通过才导出，并显示成功文案', async () => {
    const wrapper = mount(DataSafetyView)
    await flushPromises()
    await exportButton(wrapper).trigger('click')
    await flushPromises()
    expect(h.exportBeancount).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('已导出 family.beancount')
    wrapper.unmount()
  })

  it('导出后响应头仍不平 → 二次提示（双保险）', async () => {
    h.exportBeancount.mockResolvedValue({
      blob: new Blob(['x']),
      balanced: false,
      diffs: [{ commodity: 'USD', amount: '-0.02' }],
    })
    const wrapper = mount(DataSafetyView)
    await flushPromises()
    await exportButton(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('响应头提示账本不平')
    expect(wrapper.text()).toContain('USD -0.02')
    wrapper.unmount()
  })
})

describe('批3-I：备份区权限', () => {
  it('非系统管理员完全不渲染备份区，也不请求备份清单', async () => {
    h.isAdmin = false
    const wrapper = mount(DataSafetyView)
    await flushPromises()
    expect(wrapper.text()).not.toContain('数据备份')
    expect(h.backupList).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('系统管理员可见备份区并加载清单', async () => {
    const wrapper = mount(DataSafetyView)
    await flushPromises()
    expect(wrapper.text()).toContain('数据备份')
    expect(h.backupList).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
})
