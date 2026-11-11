// @vitest-environment jsdom
/**
 * 批 3 组件回归：P-12 成员管理
 *
 * 守住 owner 权限门（AC-33）、邀请与重置密码的输入校验（不达标不发请求）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import MembersView from './MembersView.vue'

const h = vi.hoisted(() => ({
  role: 'owner' as string,
  list: vi.fn(),
  add: vi.fn(),
  updateRole: vi.fn(),
  remove: vi.fn(),
  resetPassword: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  memberApi: {
    list: h.list,
    add: h.add,
    updateRole: h.updateRole,
    remove: h.remove,
    resetPassword: h.resetPassword,
  },
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    ledgerId: 7,
    ledgers: [{ id: 7, name: '家用账本', role: h.role, opening_date: '2026-01-01' }],
    user: { id: 1, username: 'boss', role: h.role, is_admin: true, must_change_password: false },
  }),
}))

const MEMBERS = [
  { user_id: 1, username: 'boss', display_name: '老板', role: 'owner' },
  { user_id: 2, username: 'alice', display_name: '小林', role: 'viewer' },
]

beforeEach(() => {
  h.role = 'owner'
  h.list.mockReset()
  h.add.mockReset()
  h.updateRole.mockReset()
  h.remove.mockReset()
  h.resetPassword.mockReset()
  h.list.mockResolvedValue({ members: MEMBERS })
  h.add.mockResolvedValue({ ok: true, user_id: 3, role: 'viewer' })
  h.updateRole.mockResolvedValue({ ok: true, user_id: 2, role: 'editor' })
  h.remove.mockResolvedValue({ ok: true })
  h.resetPassword.mockResolvedValue({ ok: true })
})

describe('批3-F：成员管理权限门', () => {
  it('owner 加载列表，且 owner 行不显示任何操作按钮', async () => {
    const wrapper = mount(MembersView)
    await flushPromises()
    expect(h.list).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('小林')
    expect(wrapper.text()).toContain('共 2 位成员')
    const ownerRow = wrapper.findAll('.mem__row')[0]
    expect(ownerRow.findAll('button').length).toBe(0)
    wrapper.unmount()
  })

  it('非 owner 直接显示权限提示，且不发起成员列表请求', async () => {
    h.role = 'editor'
    const wrapper = mount(MembersView)
    await flushPromises()
    expect(wrapper.text()).toContain('仅账本所有者（owner）可以管理成员')
    expect(h.list).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})

describe('批3-G：成员操作输入校验', () => {
  it('用户名为空时邀请不发请求并给出文案', async () => {
    const wrapper = mount(MembersView)
    await flushPromises()
    await wrapper.find('.mem__toolbar button').trigger('click')
    await wrapper.findAll('.mem__panel .fb-btn-primary')[0].trigger('click')
    await flushPromises()
    expect(h.add).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('请输入用户名（对方需先注册）')
    wrapper.unmount()
  })

  it('重置密码少于 6 位不发请求', async () => {
    const wrapper = mount(MembersView)
    await flushPromises()
    const resetButtons = wrapper.findAll('.mem__row button')
    // 第二行（非 owner）依次为：设为编辑、重置密码、移除
    await resetButtons[1].trigger('click')
    const panel = wrapper.find('.mem__panel')
    await panel.find('input[type="password"]').setValue('123')
    await panel.findAll('.fb-btn-primary')[0].trigger('click')
    await flushPromises()
    expect(h.resetPassword).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('新密码至少 6 位')
    wrapper.unmount()
  })
})
