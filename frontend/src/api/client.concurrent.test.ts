/**
 * QA Round 2 边界回归：11:07 重写后的 client.ts 在「不走刷新队列」路径的并发登出次数。
 *
 * 新实现把去重门闩收窄为单次 refreshTask 的局部闭包（client.ts:219-226），
 * 解决了模块级门闩在 skipRefresh 路径的永久锁死（Round 2 原 Known Issue）；
 * 但 client.ts:307-313 对 skipRefresh:true / refreshed:true 两条路径
 * 改为「各自 notifyUnauthorized()」，并发时不再去重 → 重复登出。
 *
 * 这两条均为 P2 边界项，不在 AC-24「业务请求并发 + refresh 成功」口径内；
 * 断言按 E-07「统一登出」语义写为 1 次，实测 3 次，作为 Known Issues 保留。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { API_ROUTES } from './routes'
import {
  authApi,
  request,
  setApiClientHooks,
  setFetchImplementation,
  setLedgerId,
} from './client'

function makeResponse(status: number, body: unknown): Response {
  const text = typeof body === 'string' ? body : JSON.stringify(body)
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => text,
  } as unknown as Response
}

beforeEach(() => {
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

describe('Round 2 边界回归：不走 refreshSession 的 401 路径并发登出次数', () => {
  it('并发 3 个 skipRefresh 探测 401 应只统一登出一次', async () => {
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    setFetchImplementation(async () => makeResponse(401, { error: '未登录' }))

    const settled = await Promise.allSettled([authApi.me(), authApi.me(), authApi.me()])

    expect(settled.every((s) => s.status === 'rejected')).toBe(true)
    expect(settled).toHaveLength(3)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })

  it('并发 3 个业务 401、refresh 成功但重试(refreshed:true)仍 401 应只登出一次', async () => {
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    setFetchImplementation(async (url: string) =>
      url === API_ROUTES.refresh
        ? makeResponse(200, { token: 'new' })
        : makeResponse(401, { error: '登录已失效' }),
    )

    const settled = await Promise.allSettled([
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
    ])

    expect(settled.every((s) => s.status === 'rejected')).toBe(true)
    expect(settled).toHaveLength(3)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })
})
