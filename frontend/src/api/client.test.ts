/**
 * API 客户端契约测试（AC-24 / AC-25 / D-7）
 *
 * 覆盖：并发 401 single-flight、401 重试仍失败统一登出、428 跳改密、
 * X-CSRF 自动注入、X-Ledger-Id 注入与例外清单、缺失即抛错、幂等键透传。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from './errors'
import { API_ROUTES } from './routes'
import {
  getLedgerId,
  newIdempotencyKey,
  request,
  setApiClientHooks,
  setFetchImplementation,
  setLedgerId,
  transactionApi,
} from './client'

interface FetchCall {
  url: string
  init: RequestInit
}

function makeResponse(status: number, body: unknown): Response {
  const text =
    body === undefined
      ? ''
      : typeof body === 'string'
        ? body
        : JSON.stringify(body)
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => text,
  } as unknown as Response
}

function headersOf(call: FetchCall): Record<string, string> {
  return call.init.headers as unknown as Record<string, string>
}

let calls: FetchCall[] = []

beforeEach(() => {
  setFetchImplementation(null)
  calls = []
  setApiClientHooks({})
  setLedgerId(null)
})

afterEach(() => {
  setFetchImplementation(null)
})

describe('E-07：401 single-flight 刷新队列', () => {
  it('并发 4 个 401 只触发 1 次 refresh，其余挂起复用结果并成功重试（AC-24）', async () => {
    setLedgerId(7)
    let refreshCount = 0
    let refreshed = false
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      if (url === API_ROUTES.refresh) {
        refreshCount += 1
        // 制造并发窗口：4 个业务请求都会先拿到 401 再一起进入刷新队列。
        await new Promise((resolve) => setTimeout(resolve, 10))
        refreshed = true
        return makeResponse(200, { token: 'new-token' })
      }
      if (url === API_ROUTES.accounts) {
        return refreshed
          ? makeResponse(200, { accounts: [] })
          : makeResponse(401, { error: '登录已失效，请重新登录' })
      }
      return makeResponse(404, { error: 'not found' })
    })

    const results = await Promise.all([
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
    ])

    expect(refreshCount).toBe(1)
    expect(results).toHaveLength(4)
    const accountCalls = calls.filter((call) => call.url === API_ROUTES.accounts)
    // 4 次首次 401 + 4 次刷新后重试。
    expect(accountCalls.length).toBe(8)
    expect(calls[0].init.credentials).toBe('include')
  })

  it('refresh 本身走裸 fetch，不带账本头且不递归', async () => {
    setLedgerId(7)
    let refreshed = false
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      if (url === API_ROUTES.refresh) {
        refreshed = true
        return makeResponse(200, {})
      }
      return refreshed
        ? makeResponse(200, { accounts: [] })
        : makeResponse(401, { error: '未登录' })
    })

    await request(API_ROUTES.accounts)
    const refreshCall = calls.find((call) => call.url === API_ROUTES.refresh)
    expect(refreshCall).toBeDefined()
    expect(headersOf(refreshCall as FetchCall)['X-Ledger-Id']).toBeUndefined()
    expect(headersOf(refreshCall as FetchCall)['X-CSRF']).toBe('1')
  })

  it('refresh 失败或重试仍 401 时统一登出并抛 ApiError', async () => {
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(401, { error: '登录已失效，请重新登录' })
    })

    await expect(request(API_ROUTES.accounts)).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
    // refresh 1 次 + 业务请求 1 次，不做多余重试。
    expect(calls.length).toBe(2)
  })

  it('并发 401 且 refresh 失败：全部 reject、refresh 只 1 次、统一登出只 1 次', async () => {
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      // 业务端点与 refresh 一律 401：模拟会话真正失效。
      return makeResponse(401, { error: '登录已失效，请重新登录' })
    })

    const settled = await Promise.allSettled([
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
    ])

    expect(settled).toHaveLength(3)
    for (const item of settled) {
      expect(item.status).toBe('rejected')
      if (item.status === 'rejected') {
        expect(item.reason).toBeInstanceOf(ApiError)
        expect((item.reason as ApiError).status).toBe(401)
      }
    }
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(1)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })

  it('不同刷新周期各自统一登出一次（无全局门闩，不依赖成功响应复位）', async () => {
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    let phase: 'expired' | 'recovered' | 'expired-again' = 'expired'
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return phase === 'recovered'
        ? makeResponse(200, { ok: true })
        : makeResponse(401, { error: '登录已失效，请重新登录' })
    })

    await expect(request(API_ROUTES.accounts)).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)

    phase = 'recovered'
    await request(API_ROUTES.accounts) // 成功响应不复位任何状态；下一次 401 会新建 refreshTask 独立通知
    expect(onUnauthorized).toHaveBeenCalledTimes(1)

    phase = 'expired-again'
    await expect(request(API_ROUTES.accounts)).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(2)
  })

  it('5xx 最多重试 3 次，4xx 不重试', async () => {
    setLedgerId(7)
    let count = 0
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      count += 1
      return makeResponse(503, { error: '服务暂不可用' })
    })
    await expect(request(API_ROUTES.accounts)).rejects.toMatchObject({ status: 503 })
    expect(count).toBe(4)

    count = 0
    calls = []
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      count += 1
      return makeResponse(400, { error: '参数错误', code: 'invalid' })
    })
    await expect(request(API_ROUTES.accounts)).rejects.toMatchObject({ code: 'invalid' })
    expect(count).toBe(1)
  })
})

describe('E-07：428 强制改密', () => {
  it('业务请求返回 428 时触发改密跳转而非通用错误（AC-24）', async () => {
    setLedgerId(7)
    const onPasswordChangeRequired = vi.fn()
    setApiClientHooks({ onPasswordChangeRequired })
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(428, {
        error: '首次登录需要先修改密码',
        code: 'password_change_required',
      })
    })

    await expect(request(API_ROUTES.accounts)).rejects.toMatchObject({
      status: 428,
      code: 'password_change_required',
      message: '首次登录需要先修改密码',
    })
    expect(onPasswordChangeRequired).toHaveBeenCalledTimes(1)
  })
})

describe('D-7：Cookie 会话与 CSRF 头', () => {
  it('写操作自动带 X-CSRF，GET 不带；所有请求 credentials=include', async () => {
    setLedgerId(7)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { ok: true })
    })

    await request(API_ROUTES.accounts)
    await request(API_ROUTES.transactions, { method: 'POST', body: { description: 'x' } })

    expect(headersOf(calls[0])['X-CSRF']).toBeUndefined()
    expect(headersOf(calls[1])['X-CSRF']).toBe('1')
    expect(calls[0].init.credentials).toBe('include')
    expect(calls[1].init.credentials).toBe('include')
  })
})

describe('E-08：X-Ledger-Id 注入与断言', () => {
  it('业务端点注入当前账本 id，例外端点不注入（AC-25）', async () => {
    setLedgerId(9)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { ok: true })
    })

    await request(API_ROUTES.accounts)
    await request(API_ROUTES.me)

    expect(headersOf(calls[0])['X-Ledger-Id']).toBe('9')
    expect(headersOf(calls[1])['X-Ledger-Id']).toBeUndefined()
    expect(getLedgerId()).toBe(9)
  })

  it('未取到账本 id 时业务请求直接抛错，绝不静默回退 id=1（AC-25）', async () => {
    setLedgerId(null)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { ok: true })
    })

    await expect(request(API_ROUTES.transactions)).rejects.toMatchObject({
      code: 'ledger_id_required',
    })
    expect(calls.length).toBe(0)
  })

  it('账本清单 / 认证端点属例外清单，未选账本也可访问', async () => {
    setLedgerId(null)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { ledgers: [] })
    })
    await expect(request(API_ROUTES.ledgers)).resolves.toEqual({ ledgers: [] })
    expect(calls.length).toBe(1)
  })
})

describe('G-04：幂等键', () => {
  it('创建交易透传 Idempotency-Key 头', async () => {
    setLedgerId(3)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(201, { id: 1, uuid: 'u-1', date: '2026-11-09', postings: 2 })
    })

    await transactionApi.create(
      {
        date: '2026-11-09',
        description: '午餐',
        postings: [
          { account_id: 2, commodity: 'CNY', amount: '38.50' },
          { account_id: 1, commodity: 'CNY', amount: '-38.50' },
        ],
      },
      'key-abc',
    )

    expect(headersOf(calls[0])['Idempotency-Key']).toBe('key-abc')
    expect(headersOf(calls[0])['X-Ledger-Id']).toBe('3')
  })

  it('每次生成不同的 UUID v4 形态幂等键（不使用 crypto.randomUUID）', () => {
    const first = newIdempotencyKey()
    const second = newIdempotencyKey()
    expect(first).not.toBe(second)
    expect(first).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    )
  })
})
