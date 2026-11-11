/**
 * 批 3 页面共用的展示口径（纯函数，可单测）
 *
 * 覆盖：备份错误文案、导出不平差额、相对时间、服务端时间格式化、
 * 文件体积、角色标签。全部不依赖 DOM / Vue，便于在 Vitest 直接断言。
 *
 * Safari 14.1 兼容：不用 Intl / padStart / `.at()` / toISOString。
 */
import { formatAmount } from './Money'
import type { LedgerRole } from '@/api/types'

/** 按币种的恒等式差额（后端 identity/check 的 `sum` 与导出头统一成 amount）。 */
export interface DiffItem {
  commodity: string
  amount: string
}

function twoDigits(value: number): string {
  return value < 10 ? `0${value}` : String(value)
}

/**
 * 解析导出响应头 `X-Ledger-Diffs`（`CNY=0.01;USD=-0.02`）。
 * 空项、缺 `=` 项静默跳过，与旧版 ledger_api.dart 的容错一致。
 */
export function parseIdentityHeader(raw: string): DiffItem[] {
  const out: DiffItem[] = []
  if (raw === '') {
    return out
  }
  for (const part of raw.split(';')) {
    const eq = part.indexOf('=')
    if (eq <= 0 || eq === part.length - 1) {
      continue
    }
    const commodity = part.slice(0, eq).trim()
    const amount = part.slice(eq + 1).trim()
    if (commodity !== '' && amount !== '') {
      out.push({ commodity, amount })
    }
  }
  return out
}

/** 把 identity/check 的 `{commodity, sum}` 明细转成统一 DiffItem。 */
export function fromIdentityDiffs(
  diffs: Array<{ commodity: string; sum: string }>,
): DiffItem[] {
  const out: DiffItem[] = []
  for (const diff of diffs) {
    out.push({ commodity: diff.commodity, amount: diff.sum })
  }
  return out
}

/** `CNY +0.01、USD -0.02` —— 正数显式加号，负数保留负号，零不加符号。 */
export function formatIdentityDiffs(diffs: DiffItem[]): string {
  const parts: string[] = []
  for (const diff of diffs) {
    const text = formatAmount(diff.amount)
    const sign = text.charAt(0) === '-' || text === '0.00' ? '' : '+'
    parts.push(`${diff.commodity} ${sign}${text}`)
  }
  return parts.join('、')
}

/** 备份页错误文案（与旧版 backup_card.dart 的 _mapError 一致）。 */
export function mapBackupError(status: number, message: string): string {
  switch (status) {
    case 401:
      return '登录已失效，请重新登录'
    case 403:
      return '需要管理员权限'
    case 409:
      return '已有备份正在执行，请稍后再试'
    case 503:
      return '服务端未启用备份功能'
    case 0:
      return '网络异常，请检查连接后重试'
    default:
      return message === '' ? '操作失败，请稍后重试' : message
  }
}

/**
 * 解析服务端时间：兼容 `YYYY-MM-DD HH:mm:ss`（SQLite datetime，UTC）与 RFC3339。
 * 无时区后缀时按 UTC 处理（服务端 M3 已统一 UTC 存储）。
 */
export function parseServerTime(raw: string): Date | null {
  if (raw === '') {
    return null
  }
  let text = raw.indexOf(' ') >= 0 ? raw.replace(' ', 'T') : raw
  if (!/(Z|[+-]\d{2}:?\d{2})$/.test(text)) {
    text += 'Z'
  }
  const parsed = new Date(text)
  return isNaN(parsed.getTime()) ? null : parsed
}

/** 会话列表用：本地时间 `YYYY-MM-DD HH:mm`（只到分钟）。 */
export function formatSessionTime(raw: string): string {
  const date = parseServerTime(raw)
  if (date === null) {
    return '—'
  }
  return `${date.getFullYear()}-${twoDigits(date.getMonth() + 1)}-${twoDigits(date.getDate())} ` +
    `${twoDigits(date.getHours())}:${twoDigits(date.getMinutes())}`
}

/** 相对时间：刚刚 / N 分钟前 / N 小时前 / N 天前 / 日期。 */
export function relativeTime(raw: string, now: Date = new Date()): string {
  const date = parseServerTime(raw)
  if (date === null) {
    return '—'
  }
  const diff = now.getTime() - date.getTime()
  if (diff < 0) {
    return formatSessionTime(raw)
  }
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) {
    return '刚刚'
  }
  if (minutes < 60) {
    return `${minutes} 分钟前`
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `${hours} 小时前`
  }
  const days = Math.floor(hours / 24)
  if (days < 7) {
    return `${days} 天前`
  }
  return `${date.getFullYear()}-${twoDigits(date.getMonth() + 1)}-${twoDigits(date.getDate())}`
}

/** 文件体积：≥1MB 用一位小数 MB，否则整数 KB。 */
export function formatBackupSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 KB'
  }
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }
  return `${Math.ceil(bytes / 1024)} KB`
}

/** 备份记录是否成功（error 为空即成功，失败记录仍保留在清单里）。 */
export function backupSucceeded(item: { error?: string }): boolean {
  return item.error === undefined || item.error === ''
}

/** 角色中文标签。 */
export function roleLabel(role: LedgerRole | ''): string {
  switch (role) {
    case 'owner':
      return '所有者'
    case 'editor':
      return '编辑'
    case 'viewer':
      return '只读'
    default:
      return '未加入账本'
  }
}

/** 触发浏览器下载 Blob（非浏览器环境静默跳过）。 */
export function downloadBlob(filename: string, blob: Blob): void {
  if (typeof Blob === 'undefined' || typeof document === 'undefined') {
    return
  }
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/** 触发浏览器下载文本（CSV / beancount 文本）。 */
export function downloadText(filename: string, text: string, mime: string): void {
  if (typeof Blob === 'undefined') {
    return
  }
  downloadBlob(filename, new Blob([text], { type: mime }))
}
