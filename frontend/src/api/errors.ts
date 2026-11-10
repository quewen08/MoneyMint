/**
 * API 错误封装（批 0）
 *
 * 后端错误体约定（`internal/handler/handler.go` 的 writeError）：
 * - `{ "error": "文案", "code"?: "稳定错误码", ...明细 }`
 * - 关闭账户余额非 0 时额外带 `account_uuid` 与 `balances`；
 * - 限流时额外带 `retry_after_seconds`；
 * - 未知错误回退 `{ "error": "..." }`。
 *
 * 前端保留 `message`（用户可读文案）与 `balances`（结构化差额），
 * 其余明细收进 `details`，便于页面按需读取而不丢信息。
 */
import type { BalanceDiff } from './types'

/** 错误响应体（宽松解析，字段都可能缺失）。 */
export interface ApiErrorBody {
  error?: string
  message?: string
  code?: string
  balances?: BalanceDiff[]
  details?: unknown
  [key: string]: unknown
}

/** 构造 ApiError 的选项。 */
export interface ApiErrorOptions {
  code?: string
  details?: Record<string, unknown>
  balances?: BalanceDiff[]
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isBalanceDiff(value: unknown): value is BalanceDiff {
  return (
    isRecord(value) &&
    typeof value.commodity === 'string' &&
    typeof value.amount === 'string'
  )
}

/** 统一的 API 错误：HTTP 状态 + 稳定错误码 + 结构化明细。 */
export class ApiError extends Error {
  /** HTTP 状态码。 */
  readonly status: number
  /** 后端稳定错误码（如 `account_balance_not_zero`），无则为 undefined。 */
  readonly code?: string
  /** 其余明细（如 `account_uuid`、`retry_after_seconds`）。 */
  readonly details?: Record<string, unknown>
  /** 关闭账户时的按币种差额明细。 */
  readonly balances?: BalanceDiff[]

  constructor(status: number, message: string, options: ApiErrorOptions = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = options.code
    this.details = options.details
    this.balances = options.balances
  }

  /**
   * 从 HTTP 状态与响应体解析错误。
   * 文案优先级：`error` → `message` → `HTTP ${status}`（未知码回退）。
   */
  static fromBody(status: number, body: unknown): ApiError {
    if (typeof body === 'string' && body.trim() !== '') {
      return new ApiError(status, body.trim())
    }
    if (!isRecord(body)) {
      return new ApiError(status, `HTTP ${status}`)
    }

    const message =
      typeof body.error === 'string' && body.error !== ''
        ? body.error
        : typeof body.message === 'string' && body.message !== ''
          ? body.message
          : `HTTP ${status}`

    const code = typeof body.code === 'string' && body.code !== '' ? body.code : undefined

    const balances = Array.isArray(body.balances)
      ? body.balances.filter(isBalanceDiff)
      : undefined

    // 把除已知字段外的键收进 details（含后端平铺的 account_uuid 等）。
    const { error: _error, message: _message, code: _code, balances: _balances, ...rest } = body
    const details = Object.keys(rest).length > 0 ? rest : undefined

    return new ApiError(status, message, {
      code,
      details,
      balances: balances && balances.length > 0 ? balances : undefined,
    })
  }
}
