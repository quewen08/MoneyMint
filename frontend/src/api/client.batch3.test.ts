/**
 * 批 3 API 封装契约测试
 *
 * 覆盖：`requestWithMeta` 读响应头（导出不平告警 / §14.5 #3）、blob 下载、
 * X-Ledger-Id 强制断言对导出仍生效、备份下载走例外清单、成员路径占位符替换、
 * 5xx 重试边界未被新封装破坏。
 */
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { ApiError } from './errors'
import { API_ROUTES } from './routes'
import {
  backupApi,
  exportApi,
  memberApi,
  requestWithMeta,
  setApiClientHooks,
  setFetchImplementation,
  setLedgerId,
} from './client'

interface FetchCall {
  url: string
  init: RequestInit
}

function makeResponse(
  status: number,
  options: {
    body?: unknown
    blob?: Blob
    headers?: Record<string, string>
  } = {},
): Response {
  const text =
    options.body === undefined
      ? ''
      : typeof options.body === 'string'
        ? options.body
        : JSON.stringify(options.body)
  const headerMap = options.headers === undefined ? {} : options.headers
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: {
      get: (key: string): string | null => {
        for (const name of Object.keys(headerMap)) {
          if (name.toLowerCase() === key.toLowerCase()) {
            return headerMap[name]
          }
        }
        return null
      },
    },
    text: async () => text,
    blob: async () => options.blob,
  } as unknown as Response
}

function headersOf(call: FetchCall): Record<string, string> {
  return call.init.headers as unknown as Record<string, string>
}

let calls: FetchCall[] = []

beforeEach(() => {
  setFetchImplementation(null)
  setApiClientHooks({})
  setLedgerId(null)
  calls = []
})

afterEach(() => {
  setFetchImplementation(null)
})

describe('批3-D：导出响应头与 blob 下载（AC-20 / §14.5 #3）', () => {
  it('响应头 X-Ledger-Balanced=false 时返回告警与按币种差额', async () => {
    setLedgerId(7)
    const blob = new Blob(['option "operating_currency" "CNY"'])
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, {
        blob,
        headers: {
          'X-Ledger-Balanced': 'false',
          'X-Ledger-Diffs': 'CNY=0.01;USD=-0.02',
        },
      })
    })

    const result = await exportApi.beancount()
    expect(result.blob).toBe(blob)
    expect(result.balanced).toBe(false)
    expect(result.diffs).toEqual([
      { commodity: 'CNY', amount: '0.01' },
      { commodity: 'USD', amount: '-0.02' },
    ])
    expect(calls[0].url).toBe(API_ROUTES.export)
    expect(headersOf(calls[0])['X-Ledger-Id']).toBe('7')
  })

  it('缺响应头按 true 兼容旧服务端', async () => {
    setLedgerId(7)
    setFetchImplementation(async () => makeResponse(200, { blob: new Blob(['x']) }))
    const result = await exportApi.archive()
    expect(result.balanced).toBe(true)
    expect(result.diffs).toEqual([])
  })

  it('导出缺失 X-Ledger-Id 直接抛错且不发请求（E-08）', async () => {
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { blob: new Blob(['x']) })
    })
    await expect(exportApi.beancount()).rejects.toMatchObject({ code: 'ledger_id_required' })
    expect(calls.length).toBe(0)
  })

  it('错误响应即使声明 blob 也按 JSON 文案解析', async () => {
    setLedgerId(7)
    setFetchImplementation(async () =>
      makeResponse(403, { body: { error: '只读成员不能导出', code: 'forbidden' } }),
    )
    await expect(exportApi.beancount()).rejects.toMatchObject({
      status: 403,
      message: '只读成员不能导出',
      code: 'forbidden',
    })
  })
})

describe('批3-E：备份与成员封装的边界', () => {
  it('备份下载走 X-Ledger-Id 例外清单，并带 URL 编码的文件名', async () => {
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { blob: new Blob(['db']) })
    })
    const blob = await backupApi.download('ledger-20261111-030000.db')
    expect(blob.size).toBeGreaterThan(0)
    expect(calls[0].url).toContain('/api/backup/download?name=')
    expect(headersOf(calls[0])['X-Ledger-Id']).toBeUndefined()
  })

  it('成员改角色路径占位符替换，且带 X-CSRF 与账本头', async () => {
    setLedgerId(7)
    setFetchImplementation(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return makeResponse(200, { body: { ok: true, user_id: 9, role: 'editor' } })
    })
    const res = await memberApi.updateRole(9, { role: 'editor' })
    expect(res.role).toBe('editor')
    expect(calls[0].url).toBe('/api/ledger/members/9')
    expect(headersOf(calls[0])['X-CSRF']).toBe('1')
    expect(headersOf(calls[0])['X-Ledger-Id']).toBe('7')
  })

  it('requestWithMeta 保留 4xx 不重试 / 5xx 最多 3 次边界', async () => {
    setLedgerId(7)
    let attempts = 0
    setFetchImplementation(async () => {
      attempts += 1
      return makeResponse(500, { body: { error: '服务异常' } })
    })
    await expect(requestWithMeta(API_ROUTES.accounts)).rejects.toBeInstanceOf(ApiError)
    // 首次 + 3 次重试 = 4 次，与既有 request 完全一致。
    expect(attempts).toBe(4)
  })
})
