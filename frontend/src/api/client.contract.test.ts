/**
 * QA 独立补充契约回归（Round 1，严过关 / Yan）
 *
 * 目的：以 fresh eyes 证伪 / 确认 client.ts 的六条关键声明，重点覆盖
 * 现有 client.test.ts 未触及的边界：
 *  - 并发 401 的「挂起复用」时序证据、refresh 失败时挂起请求是否全部收敛；
 *  - 例外清单与 Go middleware 实际豁免端点的逐路径契约矩阵；
 *  - 4xx / 5xx / 428 / skipRefresh 的重试与回调副作用次数；
 *  - 裸 fetch refresh 的完整请求形态（X-CSRF / credentials / 无账本头）。
 *
 * 所有断言只依赖公开 API（request / api.* / set*），不触达模块私有状态。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from './errors'
import { API_ROUTES, isNoLedgerPath } from './routes'
import {
  authApi,
  getLedgerId,
  request,
  setApiClientHooks,
  setFetchImplementation,
  setLedgerId,
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

/** 记录调用顺序，用于证明「挂起复用」而非「各自刷新」。 */
let calls: FetchCall[] = []

beforeEach(() => {
  calls = []
  setFetchImplementation(null)
  setApiClientHooks({})
  setLedgerId(null)
})

afterEach(() => {
  // 复位所有模块级单例，避免同文件内测试间状态泄漏。
  setFetchImplementation(null)
  setApiClientHooks({})
  setLedgerId(null)
  vi.restoreAllMocks()
})

// ---------------------------------------------------------------------------
// A. 401 single-flight
// ---------------------------------------------------------------------------
describe('契约 A：401 single-flight 刷新队列', () => {
  it('并发 3 个 401 → refresh 只 1 次，3 个请求挂起复用结果并重试成功（AC-24）', async () => {
    // Arrange
    setLedgerId(7)
    let refreshCount = 0
    let refreshed = false
    const order: string[] = []
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      order.push(url)
      if (url === API_ROUTES.refresh) {
        refreshCount += 1
        // 制造并发窗口：让另外两个 401 在 refresh 完成前进入队列。
        await new Promise((resolve) => setTimeout(resolve, 10))
        refreshed = true
        return makeResponse(200, { token: 'new-token' })
      }
      return refreshed
        ? makeResponse(200, { accounts: [] })
        : makeResponse(401, { error: '登录已失效，请重新登录' })
    })

    // Act
    const results = await Promise.all([
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
    ])

    // Assert
    expect(refreshCount).toBe(1)
    expect(results).toHaveLength(3)
    expect(results[0]).toEqual({ accounts: [] })
    // 时序证据：3 个业务 401 全部在唯一一次 refresh 之前发出，
    // refresh 之后才出现 3 次重试（说明挂起复用，而非各刷各的）。
    expect(order).toEqual([
      API_ROUTES.accounts,
      API_ROUTES.accounts,
      API_ROUTES.accounts,
      API_ROUTES.refresh,
      API_ROUTES.accounts,
      API_ROUTES.accounts,
      API_ROUTES.accounts,
    ])
    // 重试请求标记不应污染对外头；每个请求都必须是 Cookie 会话。
    for (const call of calls) {
      expect(call.init.credentials).toBe('include')
    }
  })

  it('refresh 失败 → 3 个挂起请求全部 reject（不悬挂），且登出只触发 1 次', async () => {
    // Arrange
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      // 业务与 refresh 全部 401：模拟会话真正失效。
      return makeResponse(401, { error: '登录已失效，请重新登录' })
    })

    // Act：用 allSettled 确保「不悬挂」可被观测（不会永久 pending）。
    const settled = await Promise.allSettled([
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
      request(API_ROUTES.accounts),
    ])

    // Assert
    expect(settled).toHaveLength(3)
    for (const item of settled) {
      expect(item.status).toBe('rejected')
      if (item.status === 'rejected') {
        expect(item.reason).toBeInstanceOf(ApiError)
        expect((item.reason as ApiError).status).toBe(401)
      }
    }
    // refresh 只发 1 次，3 个业务请求各 1 次，无多余重试。
    const refreshCalls = calls.filter((call) => call.url === API_ROUTES.refresh)
    expect(refreshCalls).toHaveLength(1)
    // 关键契约：并发失败只应统一登出一次，不能每个挂起请求各触发一次。
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })

  it('refresh 裸 fetch 自带 X-CSRF / credentials，且不带 X-Ledger-Id、不递归', async () => {
    // Arrange
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

    // Act
    await request(API_ROUTES.accounts)

    // Assert
    const refreshCalls = calls.filter((call) => call.url === API_ROUTES.refresh)
    expect(refreshCalls).toHaveLength(1)
    const refreshCall = refreshCalls[0]
    const headers = headersOf(refreshCall)
    expect(refreshCall.init.method).toBe('POST')
    expect(refreshCall.init.credentials).toBe('include')
    expect(headers['X-CSRF']).toBe('1')
    expect(headers['X-Ledger-Id']).toBeUndefined()
    expect(headers['Authorization']).toBeUndefined()
    // 裸 fetch 不应触发第二次 refresh（无递归）。
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(1)
  })

  it('401 重试一次仍 401 → 登出且不无限循环', async () => {
    // Arrange
    setLedgerId(7)
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    let refreshCount = 0
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      if (url === API_ROUTES.refresh) {
        refreshCount += 1
        return makeResponse(200, { token: 'new' })
      }
      // 刷新后业务依旧 401：必须走 refreshed 标记收敛。
      return makeResponse(401, { error: '登录已失效' })
    })

    // Act
    await expect(request(API_ROUTES.accounts)).rejects.toMatchObject({ status: 401 })

    // Assert
    expect(refreshCount).toBe(1)
    const accountCalls = calls.filter((call) => call.url === API_ROUTES.accounts)
    expect(accountCalls).toHaveLength(2) // 首次 + 一次重试
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })
})

// ---------------------------------------------------------------------------
// B. X-Ledger-Id 断言与例外清单契约矩阵
// ---------------------------------------------------------------------------

/** 把路由模板里的占位符替换成真实值，得到可请求的具体路径。 */
function concretePath(template: string): string {
  return template
    .replace('{uuid}', 'u-0001')
    .replace('{sid}', 'sid-0001')
    .replace('{id}', '9')
    .replace('{userID}', '42')
}

/**
 * 依据 `internal/server/middleware.go` 实际「不解析账本上下文」的分支：
 *  - L95  公开：health / auth/register / auth/login
 *  - L101 settings/registration (GET)
 *  - L129 auth/sessions*
 *  - L135 me / logout / change-password / refresh
 *  - L153 ledgers / ledgers/*
 *  - L160 backup/*
 *  - L166 settings/*
 * 其余 /api/* 一律走 L172 ledgerIDFromRequest（业务端点，必须带 X-Ledger-Id）。
 */
const MIDDLEWARE_EXEMPT: readonly string[] = [
  'health',
  'register',
  'login',
  'logout',
  'me',
  'refresh',
  'changePassword',
  'sessions',
  'sessionsRevokeOthers',
  'sessionBySid',
  'ledgers',
  'ledgerById',
  'backupList',
  'backupDownload',
  'backupRun',
  'settingsRegistration',
]

describe('契约 B：X-Ledger-Id 断言与 middleware 豁免端点矩阵（AC-25 / E-08）', () => {
  it('31 条端点逐路径与 middleware 豁免集合完全一致', () => {
    const routeNames = Object.keys(API_ROUTES) as Array<keyof typeof API_ROUTES>
    // 覆盖完整性自检：路由常量确实全部纳入判定。
    expect(routeNames.length).toBe(31)

    for (const name of routeNames) {
      const expected = MIDDLEWARE_EXEMPT.indexOf(name) >= 0
      const path = concretePath(API_ROUTES[name])
      expect(
        isNoLedgerPath(path),
        `路由 ${name} (${path}) 的例外判定与 middleware 不一致`,
      ).toBe(expected)
    }
  })

  it('缺失账本 id 时所有业务端点零 fetch 直接抛 ledger_id_required', async () => {
    // Arrange
    setLedgerId(null)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { ok: true })
    })
    const routeNames = Object.keys(API_ROUTES) as Array<keyof typeof API_ROUTES>
    const businessPaths = routeNames
      .filter((name) => MIDDLEWARE_EXEMPT.indexOf(name) < 0)
      .map((name) => concretePath(API_ROUTES[name]))

    // Act + Assert
    for (const path of businessPaths) {
      await expect(request(path)).rejects.toMatchObject({
        status: 0,
        code: 'ledger_id_required',
      })
    }
    // 一个请求都不应该发出去（绝不静默回退 id=1）。
    expect(calls).toHaveLength(0)
  })

  it('账本 id <= 0 一律视为未选择，同样零 fetch 抛错', async () => {
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { ok: true })
    })
    for (const bad of [0, -1]) {
      setLedgerId(bad)
      await expect(request(API_ROUTES.transactions)).rejects.toMatchObject({
        code: 'ledger_id_required',
      })
    }
    expect(getLedgerId()).toBe(-1)
    expect(calls).toHaveLength(0)
  })

  it('所有例外端点在未选账本时可正常发出，且响应头注入不含 X-Ledger-Id', async () => {
    // Arrange
    setLedgerId(null)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { ok: true })
    })
    const routeNames = Object.keys(API_ROUTES) as Array<keyof typeof API_ROUTES>
    const exemptPaths = routeNames
      .filter((name) => MIDDLEWARE_EXEMPT.indexOf(name) >= 0)
      .map((name) => concretePath(API_ROUTES[name]))

    // Act
    for (const path of exemptPaths) {
      await expect(request(path)).resolves.toEqual({ ok: true })
    }

    // Assert
    expect(calls).toHaveLength(exemptPaths.length)
    for (const call of calls) {
      expect(headersOf(call)['X-Ledger-Id']).toBeUndefined()
    }
  })

  it('GET 探测端点（me）401 不触发 refresh，直接抛 ApiError', async () => {
    // Arrange
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onUnauthorized })
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(401, { error: '未登录' })
    })

    // Act
    await expect(authApi.me()).rejects.toBeInstanceOf(ApiError)

    // Assert：skipRefresh 生效，绝不发出 refresh 请求。
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(0)
    expect(calls).toHaveLength(1)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })
})

// ---------------------------------------------------------------------------
// C. 重试策略与 428 副作用
// ---------------------------------------------------------------------------
describe('契约 C：4xx / 5xx / 428 的重试与回调次数', () => {
  it.each([400, 401, 403, 404, 409, 422])('4xx (%i) 不重试，只发 1 次请求', async (status) => {
    // Arrange
    setLedgerId(7)
    // 401 会走 refresh 分支，这里用 skipRefresh 隔离「4xx 不重试」语义。
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(status, { error: 'client error' })
    })

    // Act
    await expect(
      request(API_ROUTES.accounts, { skipRefresh: true }),
    ).rejects.toMatchObject({ status })

    // Assert
    expect(calls).toHaveLength(1)
  })

  it('5xx 最多重试 3 次（共 4 次请求），仍失败则抛 ApiError', async () => {
    // Arrange
    setLedgerId(7)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(503, { error: '服务暂不可用' })
    })

    // Act
    await expect(request(API_ROUTES.accounts)).rejects.toMatchObject({ status: 503 })

    // Assert
    expect(calls).toHaveLength(4)
  })

  it('5xx 重试过程中转 200 时提前成功，不再多发', async () => {
    // Arrange
    setLedgerId(7)
    let count = 0
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      count += 1
      return count < 3
        ? makeResponse(500, { error: 'temp' })
        : makeResponse(200, { ok: true })
    })

    // Act
    const result = await request<{ ok: boolean }>(API_ROUTES.accounts)

    // Assert
    expect(result).toEqual({ ok: true })
    expect(calls).toHaveLength(3)
  })

  it('428 只跳一次、不重试、不触发 refresh / 登出，并保留后端 code', async () => {
    // Arrange
    setLedgerId(7)
    const onPasswordChangeRequired = vi.fn()
    const onUnauthorized = vi.fn()
    setApiClientHooks({ onPasswordChangeRequired, onUnauthorized })
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(428, {
        error: '首次登录需要先修改密码',
        code: 'password_change_required',
      })
    })

    // Act
    await expect(request(API_ROUTES.accounts)).rejects.toMatchObject({
      status: 428,
      code: 'password_change_required',
    })

    // Assert
    expect(calls).toHaveLength(1) // 不重试、不 refresh
    expect(calls.filter((call) => call.url === API_ROUTES.refresh)).toHaveLength(0)
    expect(onPasswordChangeRequired).toHaveBeenCalledTimes(1)
    expect(onUnauthorized).not.toHaveBeenCalled()
  })
})
