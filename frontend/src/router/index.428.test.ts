// @vitest-environment jsdom
/**
 * 428 强制改密跳转守卫回归（BugFix：跳转被 needsInvite 改道吃掉）
 *
 * 复现链路：sysadmin 重置 awen 密码 → awen 用新密码登录成功
 * （200 + user.must_change_password=true + Cookie），随后
 * `session.login()` / `bootstrap()` 内的 GET /api/ledgers 命中强改密态
 * 428。`client.ts` 的 onPasswordChangeRequired 与
 * `LoginView.redirectAfterLogin` 都把导航指向 /change-password，但
 * `refreshLedgers()` 的 428 catch 会把 ledgers 清空，使 needsInvite
 * 变成 true；旧守卫在目标已是 change-password 时因
 * `mustChangePassword && to.name !== 'change-password'` 为假而继续下探，
 * 落到 `needsInvite && to.name !== 'await-invite'` 改道 /await-invite，
 * /await-invite 又被 mustChangePassword 改道回 /change-password，
 * 形成无限重定向（导航 abort，页面卡在登录态，表现为「登不进去」）。
 *
 * 本文件加载真实 router（含真实 beforeEach 守卫），仅把 session store
 * 换成可控快照、把 11 个静态视图换成占位组件，避免引入无关依赖。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { RouteLocationRaw } from 'vue-router'

import router from '@/router'

const h = vi.hoisted(() => ({
  state: {
    isAuthenticated: true,
    mustChangePassword: true,
    needsInvite: true,
    user: {
      id: 2,
      username: 'awen',
      display_name: '阿文',
      role: 'member',
      is_admin: false,
      must_change_password: true,
    },
    bootstrap: vi.fn(async () => true),
    markGuest: vi.fn(),
  },
  hooks: {} as {
    onUnauthorized?: () => void
    onPasswordChangeRequired?: () => void
  },
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => h.state,
}))

vi.mock('@/api/client', () => ({
  setApiClientHooks: (next: typeof h.hooks): void => {
    h.hooks = next
  },
}))

// router/index.ts 顶层静态导入的 11 个视图：本用例只测守卫，全部用占位组件
// 顶替，避免加载视图内部的 api / 子组件依赖。懒加载的 9 个视图不会在
// 模块加载期执行，也未被本文件导航到，无需 mock。
vi.mock('@/views/AccountsView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/AccountDetailView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/AwaitInviteView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/CategoriesView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/ChangePasswordView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/CommoditiesView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/HomeView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/LoginView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/TrashView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/TransactionDetailView.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('@/views/TransactionsView.vue', () => ({ default: { template: '<div/>' } }))

// 无限重定向 / 导航 abort 会让 push 的 Promise reject；注册空 handler 消除
// 未处理的 rejection 噪音，真实落点一律由下方断言判定。
router.onError(() => {
  // 导航错误不外抛：是否发生循环由 navigate 的返回值暴露。
})

/**
 * 发起导航并返回「是否发生无限重定向」。
 *
 * Vue Router 4 的 guard redirect 循环是纯 Promise 微任务链，会饿死所有
 * setTimeout / 测试超时定时器：`vue-router.mjs:1307-1309` 的 30 次熔断
 * 只在「重定向目标与当前目标相同」时生效，而 change-password 与
 * await-invite 两个不同路由来回震荡恰好命中不了该保护。因此这里用同样
 * 跑在微任务队列的看门狗与之交替计数：超过阈值就把会话标记置回正常态，
 * 让守卫自然收敛——既暴露循环，又不拖死测试进程（防止日后代码回退时
 * 整个测试套件被饿死）。
 */
async function navigate(target: RouteLocationRaw): Promise<boolean> {
  let ticks = 0
  let done = false
  let looped = false
  const watchdog = (): void => {
    if (done) {
      return
    }
    ticks += 1
    if (ticks > 200) {
      looped = true
      h.state.mustChangePassword = false
      h.state.needsInvite = false
      return
    }
    queueMicrotask(watchdog)
  }
  queueMicrotask(watchdog)
  try {
    await router.push(target)
  } catch {
    // 导航 abort / reject 本身即缺陷证据，由 looped 与落点断言暴露。
  } finally {
    done = true
  }
  return looped
}

beforeEach(async () => {
  // 先把 currentRoute 复位到 login 这个中性落点：临时关闭全部会话标记，
  // public 分支会直接放行。这样后续 push 不会因「重复导航」跳过守卫。
  h.state.isAuthenticated = false
  h.state.mustChangePassword = false
  h.state.needsInvite = false
  await router.replace({ name: 'login' })

  // 装配本用例：已登录 + 强改密 + 账本列表为空（428 catch 后的最坏状态）。
  h.state.isAuthenticated = true
  h.state.mustChangePassword = true
  h.state.needsInvite = true
  h.state.user.must_change_password = true
  h.state.bootstrap.mockClear()
  h.state.markGuest.mockClear()
})

describe('BugFix 428：强改密态在守卫中拥有最高优先级', () => {
  it('已登录 + must_change_password + 账本为空：导航 /change-password 直接放行，不改道 await-invite', async () => {
    const looped = await navigate({ name: 'change-password' })

    expect(looped).toBe(false)
    expect(router.currentRoute.value.name).toBe('change-password')
    expect(router.currentRoute.value.name).not.toBe('await-invite')
  })

  it('强改密态从业务页 / 出发：一律收敛到 /change-password，不落入 needsInvite 分支', async () => {
    const looped = await navigate('/')

    expect(looped).toBe(false)
    expect(router.currentRoute.value.name).toBe('change-password')
    expect(h.state.bootstrap).not.toHaveBeenCalled()
  })

  it('强改密态直达 /await-invite：仍收敛到 /change-password，杜绝来回震荡', async () => {
    const looped = await navigate({ name: 'await-invite' })

    expect(looped).toBe(false)
    expect(router.currentRoute.value.name).toBe('change-password')
  })

  it('强改密态 + 账本非空：业务页仍收敛到 /change-password', async () => {
    h.state.needsInvite = false
    const looped = await navigate('/transactions')

    expect(looped).toBe(false)
    expect(router.currentRoute.value.name).toBe('change-password')
  })

  it('回归保护：非强改密 + 账本为空时，待邀请改道语义保持不变', async () => {
    h.state.mustChangePassword = false
    h.state.user.must_change_password = false
    const looped = await navigate('/')

    expect(looped).toBe(false)
    expect(router.currentRoute.value.name).toBe('await-invite')
  })
})

describe('BugFix 428：428 回调注入仍然可用', () => {
  it('router 模块加载时注册了 onPasswordChangeRequired 回调（跳转链路存在）', () => {
    expect(typeof h.hooks.onPasswordChangeRequired).toBe('function')
    expect(typeof h.hooks.onUnauthorized).toBe('function')
  })
})
