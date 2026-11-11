// @vitest-environment jsdom
/**
 * 批 3 组件回归：P-01 登录页消费注册开关（§14.5 #4）
 *
 * 守住：关闭注册时不切注册态并给出说明、开放时可注册、公开端点查询失败
 * 按开放处理（避免服务端抖动误锁注册入口）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import LoginView from './LoginView.vue'

const h = vi.hoisted(() => ({
  registration: vi.fn(),
  replace: vi.fn(),
  login: vi.fn(),
  bootstrap: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  authApi: { register: vi.fn().mockResolvedValue({ token: '', user: {} }) },
  settingsApi: { registration: h.registration },
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({
    mustChangePassword: false,
    needsInvite: false,
    login: h.login,
    bootstrap: h.bootstrap,
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ replace: h.replace }),
  useRoute: () => ({ query: {} }),
}))

beforeEach(() => {
  h.registration.mockReset()
  h.replace.mockReset()
  h.login.mockReset()
  h.bootstrap.mockReset()
  h.registration.mockResolvedValue({ enabled: true })
  h.login.mockResolvedValue(undefined)
  h.bootstrap.mockResolvedValue(true)
})

function switchButton(wrapper: ReturnType<typeof mount>) {
  return wrapper.find('.auth__switch')
}

describe('批3-J：登录页注册开关', () => {
  it('关闭注册时不切注册态，给出联系管理员的说明', async () => {
    h.registration.mockResolvedValue({ enabled: false })
    const wrapper = mount(LoginView)
    await flushPromises()
    await switchButton(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('注册已关闭，请联系管理员在设置中开放')
    expect(wrapper.text()).toContain('登 录')
    expect(wrapper.text()).not.toContain('首个注册的账号自动成为系统管理员')
    wrapper.unmount()
  })

  it('开放注册时切到注册态并展示说明框', async () => {
    const wrapper = mount(LoginView)
    await flushPromises()
    await switchButton(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('注 册')
    expect(wrapper.text()).toContain('首个注册的账号自动成为系统管理员')
    expect(wrapper.text()).toContain('显示名（可选）')
    wrapper.unmount()
  })

  it('公开端点查询失败按开放处理，不锁死注册入口', async () => {
    h.registration.mockRejectedValue(new Error('network'))
    const wrapper = mount(LoginView)
    await flushPromises()
    await switchButton(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('注 册')
    wrapper.unmount()
  })
})
