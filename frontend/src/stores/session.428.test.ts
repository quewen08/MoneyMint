/**
 * 428 强制改密态下 needsInvite 语义回归（BugFix 配套）
 *
 * `needsInvite` 表示「已登录但尚未加入任何账本，等待 owner 邀请」。
 * 强改密态下 GET /api/ledgers 会被后端 428 拦截，`refreshLedgers()` 的
 * catch 把 ledgers 清成空数组——这是 428 的副作用，不是「真的没有账本」。
 * 因此强改密态下 needsInvite 必须恒为 false，否则任何消费该标记的地方
 * （守卫、待邀请页、首页 / 布局提示）都会误判。
 *
 * 这里用真实 session store + 真实 ApiError，mock 掉 api/client 的三个入口，
 * 覆盖 login 与 bootstrap 两条恢复登录态路径。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { ApiError } from '@/api/errors'
import { useSessionStore } from '@/stores/session'

const h = vi.hoisted(() => ({
  login: vi.fn(),
  me: vi.fn(),
  list: vi.fn(),
  logout: vi.fn(),
  changePassword: vi.fn(),
  setLedgerIdProvider: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  authApi: {
    login: h.login,
    me: h.me,
    logout: h.logout,
    changePassword: h.changePassword,
  },
  ledgerApi: { list: h.list },
  setLedgerIdProvider: h.setLedgerIdProvider,
}))

interface UserFixture {
  id: number
  username: string
  display_name: string
  role: string
  is_admin: boolean
  must_change_password: boolean
}

function makeUser(mustChangePassword: boolean): UserFixture {
  return {
    id: 2,
    username: 'awen',
    display_name: '阿文',
    role: 'member',
    is_admin: false,
    must_change_password: mustChangePassword,
  }
}

function make428(): ApiError {
  return new ApiError(428, '首次登录需要先修改密码', {
    code: 'password_change_required',
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  h.login.mockReset()
  h.me.mockReset()
  h.list.mockReset()
  h.logout.mockReset()
  h.changePassword.mockReset()
  h.setLedgerIdProvider.mockReset()
})

describe('BugFix 428：强改密态 needsInvite 恒为 false', () => {
  it('login → /api/ledgers 返回 428：登录成功、账本为空，但 needsInvite=false', async () => {
    h.login.mockResolvedValue({ token: '', user: makeUser(true) })
    h.list.mockRejectedValue(make428())

    const session = useSessionStore()
    await session.login('awen', 'new-password')

    expect(session.isAuthenticated).toBe(true)
    expect(session.mustChangePassword).toBe(true)
    expect(session.ledgers).toEqual([])
    // 修复前为 true（会把守卫导航改道到 await-invite）。
    expect(session.needsInvite).toBe(false)
  })

  it('bootstrap（刷新页面恢复登录态）→ 428：needsInvite 同样恒为 false', async () => {
    h.me.mockResolvedValue({ user: makeUser(true) })
    h.list.mockRejectedValue(make428())

    const session = useSessionStore()
    const ok = await session.bootstrap()

    expect(ok).toBe(true)
    expect(session.isAuthenticated).toBe(true)
    expect(session.mustChangePassword).toBe(true)
    expect(session.needsInvite).toBe(false)
  })

  it('回归保护：非强改密 + 账本清单为空时 needsInvite 仍为 true', async () => {
    h.login.mockResolvedValue({ token: '', user: makeUser(false) })
    h.list.mockResolvedValue({ ledgers: [] })

    const session = useSessionStore()
    await session.login('awen', 'password')

    expect(session.mustChangePassword).toBe(false)
    expect(session.needsInvite).toBe(true)
  })
})
