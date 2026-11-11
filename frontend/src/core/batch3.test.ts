// @vitest-environment jsdom
/**
 * 批 3 展示口径与默认账户持久化回归
 *
 * 覆盖：导出不平差额解析 / 文案、备份错误映射（409/503/403）、
 * 服务端 UTC 时间解析、相对时间、体积、角色标签、
 * 默认账户按账本命名空间读写与非法数据容错（§14.5 #5）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  backupSucceeded,
  formatBackupSize,
  formatIdentityDiffs,
  formatSessionTime,
  fromIdentityDiffs,
  mapBackupError,
  parseIdentityHeader,
  parseServerTime,
  relativeTime,
  roleLabel,
} from './batch3'
import {
  clearDefaultAccounts,
  defaultAccountKey,
  readDefaultAccounts,
  setDefaultAccount,
  writeDefaultAccounts,
} from './storage'

describe('批3-A：导出不平差额（§14.5 #3）', () => {
  it('解析 X-Ledger-Diffs，跳过空项与缺 = 项', () => {
    expect(parseIdentityHeader('CNY=0.01;USD=-0.02')).toEqual([
      { commodity: 'CNY', amount: '0.01' },
      { commodity: 'USD', amount: '-0.02' },
    ])
    expect(parseIdentityHeader('CNY=0.01;;BROKEN;=1;USD=')).toEqual([
      { commodity: 'CNY', amount: '0.01' },
    ])
    expect(parseIdentityHeader('')).toEqual([])
  })

  it('文案正数加 +、负数保留负号、零不加符号', () => {
    expect(formatIdentityDiffs([
      { commodity: 'CNY', amount: '0.01' },
      { commodity: 'USD', amount: '-0.02' },
      { commodity: 'EUR', amount: '0' },
    ])).toBe('CNY +0.01、USD -0.02、EUR 0.00')
  })

  it('identity/check 的 sum 字段映射为统一 amount', () => {
    expect(fromIdentityDiffs([{ commodity: 'CNY', sum: '1.25' }])).toEqual([
      { commodity: 'CNY', amount: '1.25' },
    ])
    expect(formatIdentityDiffs(fromIdentityDiffs([{ commodity: 'CNY', sum: '-1.25' }]))).toBe('CNY -1.25')
  })
})

describe('批3-B：备份错误与展示口径', () => {
  it('409 / 503 / 403 / 401 / 0 各有可读文案', () => {
    expect(mapBackupError(409, 'busy')).toBe('已有备份正在执行，请稍后再试')
    expect(mapBackupError(503, '')).toBe('服务端未启用备份功能')
    expect(mapBackupError(403, 'forbidden')).toBe('需要管理员权限')
    expect(mapBackupError(401, '')).toBe('登录已失效，请重新登录')
    expect(mapBackupError(0, '')).toBe('网络异常，请检查连接后重试')
    expect(mapBackupError(500, '服务异常')).toBe('服务异常')
  })

  it('SQLite datetime 字符串按 UTC 解析，RFC3339 原样解析', () => {
    expect(parseServerTime('2026-11-11 03:00:00')?.toISOString()).toBe('2026-11-11T03:00:00.000Z')
    expect(parseServerTime('2026-11-11T03:00:00Z')?.toISOString()).toBe('2026-11-11T03:00:00.000Z')
    expect(parseServerTime('')).toBeNull()
    expect(parseServerTime('not-a-date')).toBeNull()
  })

  it('formatSessionTime / relativeTime 边界', () => {
    const now = new Date('2026-11-11T05:00:00Z')
    expect(relativeTime('2026-11-11 04:59:30', now)).toBe('刚刚')
    expect(relativeTime('2026-11-11 04:30:00', now)).toBe('30 分钟前')
    expect(relativeTime('2026-11-11 02:00:00', now)).toBe('3 小时前')
    expect(relativeTime('2026-11-09 05:00:00', now)).toBe('2 天前')
    expect(relativeTime('2026-10-01 05:00:00', now)).toBe('2026-10-01')
    expect(formatSessionTime('bad')).toBe('—')
    // 服务端 UTC → 本地时区，只断言格式不断言具体小时（CI 时区不固定）。
    expect(formatSessionTime('2026-11-11 03:04:00')).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/)
  })

  it('体积、成功判定、角色标签', () => {
    expect(formatBackupSize(2048)).toBe('2 KB')
    expect(formatBackupSize(1024 * 1024 * 2)).toBe('2.0 MB')
    expect(formatBackupSize(0)).toBe('0 KB')
    expect(backupSucceeded({ error: '' })).toBe(true)
    expect(backupSucceeded({ error: '磁盘满' })).toBe(false)
    expect(roleLabel('owner')).toBe('所有者')
    expect(roleLabel('')).toBe('未加入账本')
  })
})

describe('批3-C：默认账户按账本命名空间（§14.5 #5）', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('按账本隔离读写，互不串账', () => {
    setDefaultAccount(7, 'expense', 'uuid-a')
    setDefaultAccount(7, 'transferOut', 'uuid-b')
    setDefaultAccount(8, 'expense', 'uuid-c')
    expect(readDefaultAccounts(7)).toEqual({ expense: 'uuid-a', transferOut: 'uuid-b' })
    expect(readDefaultAccounts(8)).toEqual({ expense: 'uuid-c' })
  })

  it('非法 kind / 损坏 JSON / 空 uuid 一律容错', () => {
    localStorage.setItem(defaultAccountKey(7), '{broken')
    expect(readDefaultAccounts(7)).toEqual({})
    localStorage.setItem(defaultAccountKey(7), JSON.stringify({ expense: 123, bad: 'x' }))
    expect(readDefaultAccounts(7)).toEqual({})
    setDefaultAccount(7, 'income', '')
    expect(readDefaultAccounts(7).income).toBeUndefined()
  })

  it('清除全部默认后为空', () => {
    writeDefaultAccounts(7, { expense: 'uuid-a', income: 'uuid-b' })
    clearDefaultAccounts(7)
    expect(readDefaultAccounts(7)).toEqual({})
  })

  it('localStorage 不可用时静默降级（不抛错）', () => {
    const stub = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('quota')
    })
    expect(() => readDefaultAccounts(7)).not.toThrow()
    expect(readDefaultAccounts(7)).toEqual({})
    stub.mockRestore()
  })
})
