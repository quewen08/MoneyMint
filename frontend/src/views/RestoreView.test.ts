// @vitest-environment jsdom
/**
 * M2 组件回归：P-20 恢复向导（RestoreView）
 *
 * 守住 m2-plan §4 / RK-M2-01：预检摘要卡展示覆盖前后对比；
 * 未输入 RESTORE 确认词时「执行还原」禁用；成功后提示重新登录。
 * 路由层 adminOnly 守卫之外，视图对非管理员渲染无权卡（纵深防御）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import { ApiError } from '@/api/errors'
import RestoreView from './RestoreView.vue'

const h = vi.hoisted(() => ({
  isAdmin: true as boolean,
  markGuest: vi.fn(),
  preview: vi.fn(),
  execute: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  restoreApi: { preview: h.preview, execute: h.execute },
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    user: { id: 1, username: 'boss', role: 'owner', is_admin: h.isAdmin, must_change_password: false },
    markGuest: h.markGuest,
  }),
}))

// 视图用 useRouter 做「重新登录」跳转：memory router 挂 login 具名路由。
const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/login', name: 'login', component: { template: '<div/>' } }],
})

function mountView(): ReturnType<typeof mount> {
  return mount(RestoreView, { global: { plugins: [router] } })
}

function makeFile(name = 'ledger-20261111-030000.db'): File {
  return new File(['sqlite'], name)
}

function setFile(wrapper: ReturnType<typeof mount>, file: File): void {
  const input = wrapper.find('input[type="file"]').element as HTMLInputElement
  Object.defineProperty(input, 'files', { value: [file], configurable: true })
  input.dispatchEvent(new Event('change'))
}

beforeEach(() => {
  h.isAdmin = true
  h.markGuest.mockReset()
  h.preview.mockReset()
  h.execute.mockReset()
  h.preview.mockResolvedValue({
    source: 'a.db',
    valid: true,
    backup: { users: 3, ledgers: 2, accounts: 48, transactions: 1263 },
    current: { users: 2, ledgers: 1, accounts: 36, transactions: 890 },
    schema_version: 14,
    current_version: 14,
    warnings: ['还原后当前账号会消失（若备份里没有该账号），请用备份时期的账号重新登录'],
  })
  h.execute.mockResolvedValue({
    source: 'a.db',
    replaced: { users: 3, ledgers: 2, accounts: 48, transactions: 1263 },
    snapshot: 'ledger-20261111-155429.db',
    new_instance_id: '2344fe25-3096-4eb9-8440-cb8368012544',
    warnings: ['还原后当前账号会消失（若备份里没有该账号），请用备份时期的账号重新登录'],
  })
})

async function toPreviewStep(wrapper: ReturnType<typeof mount>): Promise<void> {
  setFile(wrapper, makeFile())
  await flushPromises()
  await wrapper.find('.rs__buttons button').trigger('click')
  await flushPromises()
}

describe('M2-H：恢复向导权限', () => {
  it('非管理员只渲染无权卡，不触发任何请求', async () => {
    h.isAdmin = false
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('仅系统管理员可还原备份')
    expect(wrapper.find('input[type="file"]').exists()).toBe(false)
    expect(h.preview).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})

describe('M2-I：预检摘要与拒绝路径', () => {
  it('预检通过：覆盖量对比 + 副作用警告齐全', async () => {
    const wrapper = mountView()
    await toPreviewStep(wrapper)
    const text = wrapper.text()
    expect(text).toContain('确认覆盖范围')
    expect(text).toContain('1263')
    expect(text).toContain('890')
    expect(text).toContain('schema 版本：备份 v14 / 当前 v14')
    expect(text).toContain('请用备份时期的账号重新登录')
    wrapper.unmount()
  })

  it('预检拒绝：展示原因且不出现确认词输入', async () => {
    h.preview.mockResolvedValue({
      source: 'a.db',
      valid: false,
      backup: { users: 0, ledgers: 0, accounts: 0, transactions: 0 },
      current: { users: 0, ledgers: 0, accounts: 0, transactions: 0 },
      schema_version: 0,
      current_version: 14,
      warnings: [],
      reason: 'not_sqlite',
      error: '不是有效的 SQLite 数据库文件',
    })
    const wrapper = mountView()
    await toPreviewStep(wrapper)
    expect(wrapper.text()).toContain('预检未通过')
    expect(wrapper.text()).toContain('不是有效的 SQLite 数据库文件')
    expect(wrapper.find('input.rs__input').exists()).toBe(false)
    wrapper.unmount()
  })

  it('预检接口 4xx：展示映射文案并留在第一步', async () => {
    h.preview.mockRejectedValue(new ApiError(403, '仅系统管理员可以还原备份'))
    const wrapper = mountView()
    setFile(wrapper, makeFile())
    await flushPromises()
    await wrapper.find('.rs__buttons button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('仅系统管理员可以还原备份')
    expect(wrapper.find('input[type="file"]').exists()).toBe(true)
    wrapper.unmount()
  })
})

describe('M2-J：确认词门禁与执行', () => {
  it('未输入 RESTORE 时执行按钮禁用；输入后解锁', async () => {
    const wrapper = mountView()
    await toPreviewStep(wrapper)
    const execBtn = wrapper.findAll('button').find((b) => b.text() === '执行还原')!
    expect(execBtn.attributes('disabled')).toBeDefined()

    const confirmInput = wrapper.find('input.rs__input')
    await confirmInput.setValue('restore') // 小写不放行
    expect(execBtn.attributes('disabled')).toBeDefined()

    await confirmInput.setValue('RESTORE')
    expect(execBtn.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('执行成功：结果卡含替换行数、快照名与新库世代', async () => {
    const wrapper = mountView()
    await toPreviewStep(wrapper)
    await wrapper.find('input.rs__input').setValue('RESTORE')
    await wrapper.findAll('button').find((b) => b.text() === '执行还原')!.trigger('click')
    await flushPromises()

    expect(h.execute).toHaveBeenCalledTimes(1)
    const text = wrapper.text()
    expect(text).toContain('还原完成')
    expect(text).toContain('1263')
    expect(text).toContain('ledger-20261111-155429.db')
    expect(text).toContain('2344fe25-3096-4eb9-8440-cb8368012544')
    wrapper.unmount()
  })

  it('结果卡「重新登录」仅本地登出（markGuest），不调任何端点', async () => {
    const wrapper = mountView()
    await toPreviewStep(wrapper)
    await wrapper.find('input.rs__input').setValue('RESTORE')
    await wrapper.findAll('button').find((b) => b.text() === '执行还原')!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text() === '重新登录')!.trigger('click')
    await flushPromises()
    expect(h.markGuest).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('执行失败（服务端 500）：展示映射文案，数据未改动的口径不丢', async () => {
    h.execute.mockRejectedValue(new ApiError(500, '服务端还原失败'))
    const wrapper = mountView()
    await toPreviewStep(wrapper)
    await wrapper.find('input.rs__input').setValue('RESTORE')
    await wrapper.findAll('button').find((b) => b.text() === '执行还原')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('服务端还原失败，数据未改动')
    wrapper.unmount()
  })
})
