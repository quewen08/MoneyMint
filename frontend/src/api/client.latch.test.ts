/**
 * QA Round 2 补充：401「统一登出」去重门闩的生命周期证伪（严过关 / Yan）
 *
 * 背景：P2 修复把门闩复位点从「成功响应」改为「新建 refreshTask」（client.ts:239）
 * 与「重新注入 hooks」（:116）。主理人点名要证伪：
 *   「若某条 401 路径不创建 refreshTask，门闩是否会永久锁死」。
 *
 * 真实场景：`router.beforeEach` 的 bootstrap 走 `authApi.me()`（skipRefresh:true），
 * 这条 401 不进 refreshSession()、不创建 refreshTask，因此不会触发 :239 复位；
 * 而 `setApiClientHooks` 在真实应用里只在 router 模块加载时调用一次。
 *
 * 本文件所有用例在「单个测试内部只注入一次 hooks」，不靠 beforeEach 的
 * setApiClientHooks({}) 在事件之间偷偷复位门闩，以贴近真实运行时。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from './errors'
import { API_ROUTES } from './routes'
import {
  authApi,
  request,
  setApiClientHooks,
  setFetchImplementation,
  setLedgerId,
} from './client'

function makeResponse(status: number, body: unknown): Response {
  const text =
    body === undefined ? '' : typeof body === 'string' ? body : JSON.stringify(body)
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => text,
  } as unknown as Response
}

interface FetchCall {
  url: string
  init: RequestInit
}

let calls: FetchCall[] = []

beforeEach(() => {
  calls = []
  setFetchImplementation(null)
  setApiClientHooks({})
  setLedgerId(null)
})

afterEach(() => {
  setFetchImplementation(null)
  setApiClientHooks({})
  setLedgerId(null)
  vi.restoreAllMocks()
})

function recordFetch(impl: (url: string) => Promise<Response>): void {
  setFetchImplementation(async (url: string, init: RequestInit) => {
    calls.push({ url, init })
    return impl(url)
  })
}

describe('门闩回归：Round 1 修复项与既有 401 路径', () => {
  it('并发 3×401 且 refresh 失败：spy=1、3 个挂起全 reject、refresh 1 次、401 不重试', async () => {
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    recordFetch(async () => makeResponse(401, { error: '登录已失效' }))

    const settled = await Promise.allSettled([
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
    ])

    for (const item of settled) {
      expect(item.status).toBe('rejected')
      if (item.status === 'rejected') {
        expect(item.reason).toBeInstanceOf(ApiError)
      }
    }
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(1)
    // 3 次首次 401，无重试（401 不重试）。
    expect(calls.filter((call) => call.url === API_ROUTES.accounts)).toHaveLength(3)
  })

  it('单请求 401（skipRefresh 探测端点）仍触发一次登出，且不发 refresh', async () => {
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    recordFetch(async () => makeResponse(401, { error: '未登录' }))

    await expect(authApi.me()).rejects.toBeInstanceOf(ApiError)

    expect(onUnauthorized).toHaveBeenCalledTimes(1)
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(0)
  })

  it('refreshed:true（refresh 成功但重试仍 401）触发一次登出', async () => {
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    recordFetch(async (url) =>
      url === API_ROUTES.refresh
        ? makeResponse(200, { token: 'new' })
        : makeResponse(401, { error: '登录已失效' }),
    )

    await expect(request(API_ROUTES.accounts)).rejects.toMatchObject({ status: 401 })

    expect(onUnauthorized).toHaveBeenCalledTimes(1)
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(1)
    expect(calls.filter((call) => call.url === API_ROUTES.accounts)).toHaveLength(2)
  })

  it('428 只跳一次、不触发登出，也不污染门闩（后续 401 仍可登出）', async () => {
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    const onPasswordChangeRequired = vi.fn()
    setApiClientHooks({ onUnauthorized, onPasswordChangeRequired })
    let phase = '428'
    recordFetch(async () =>
      phase === '428'
        ? makeResponse(428, { error: '请先改密', code: 'password_change_required' })
        : makeResponse(401, { error: '登录已失效' }),
    )

    await expect(request(API_ROUTES.accounts)).rejects.toMatchObject({ status: 428 })
    expect(onPasswordChangeRequired).toHaveBeenCalledTimes(1)
    expect(onUnauthorized).not.toHaveBeenCalled()

    phase = '401'
    await expect(authApi.me()).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })
})

describe('门闩自愈：证伪「不创建 refreshTask 的 401 路径永久锁死」', () => {
  it('【重点】skipRefresh 401 → 重新登录成功(200) → skipRefresh 401，应再次统一登出', async () => {
    const onUnauthorized = vi.fn()
    // 只在开头注入一次 hooks，模拟真实应用 router 模块只注册一次回调。
    setApiClientHooks({ onUnauthorized })
    let phase: 'expired' | 'recovered' | 'expired-again' = 'expired'
    recordFetch(async () =>
      phase === 'recovered'
        ? makeResponse(200, { user: { id: 1 } })
        : makeResponse(401, { error: '登录已失效' }),
    )

    // 事件 1：Cookie 会话过期，bootstrap 的 /api/auth/me 返回 401。
    await expect(authApi.me()).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)

    // 事件 2：用户重新登录并恢复正常请求（成功响应不再复位门闩）。
    phase = 'recovered'
    await authApi.me()
    expect(onUnauthorized).toHaveBeenCalledTimes(1)

    // 事件 3：新会话再次过期，仍是 skipRefresh 的探测端点 401，
    // 该路径不进 refreshSession()，不会创建 refreshTask，门闩无复位点。
    phase = 'expired-again'
    await expect(authApi.me()).rejects.toBeInstanceOf(ApiError)

    // 期望：这是一次独立的会话过期，应再次统一登出。
    expect(onUnauthorized).toHaveBeenCalledTimes(2)
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(0)
  })

  it('对照：skipRefresh 401 → 成功 200 → 业务 401（创建 refreshTask）能自愈', async () => {
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    let phase = 'expired'
    recordFetch(async () =>
      phase === 'recovered'
        ? makeResponse(200, { ok: true })
        : makeResponse(401, { error: '登录已失效' }),
    )

    await expect(authApi.me()).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)

    phase = 'recovered'
    await authApi.me()

    phase = 'expired-again'
    await expect(request(API_ROUTES.accounts)).rejects.toBeInstanceOf(ApiError)
    // 业务 401 创建新 refreshTask → :239 复位门闩 → refresh 失败 → 再通知一次。
    expect(onUnauthorized).toHaveBeenCalledTimes(2)
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(1)
  })

  it('重新注入 hooks（独立会话/测试）能复位门闩，后续 401 可再次登出', async () => {
    const first = vi.fn()
    const second = vi.fn()
    setApiClientHooks({ onUnauthorized: first })
    recordFetch(async () => makeResponse(401, { error: '未登录' }))

    await expect(authApi.me()).rejects.toBeInstanceOf(ApiError)
    expect(first).toHaveBeenCalledTimes(1)

    setApiClientHooks({ onUnauthorized: second })
    await expect(authApi.me()).rejects.toBeInstanceOf(ApiError)
    expect(first).toHaveBeenCalledTimes(1)
    expect(second).toHaveBeenCalledTimes(1)
  })
})
