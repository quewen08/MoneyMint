/**
 * M2 纯逻辑回归（core/m2.ts）：预检拒绝文案、覆盖量对比、错误行号格式化。
 * 全部为纯函数单测，不挂载组件。
 */
import { describe, expect, it } from 'vitest'

import type { ImportPreviewResult, RestorePreviewResult } from '@/api/types'
import {
  formatImportErrors,
  importSummaryRows,
  mapImportError,
  mapRestoreError,
  RESTORE_CONFIRM_WORD,
  restoreCompareRows,
  restoreRejectionText,
} from './m2'

describe('M2-A：恢复预检拒绝文案', () => {
  const base: RestorePreviewResult = {
    source: 'a.db',
    valid: false,
    backup: { users: 0, ledgers: 0, accounts: 0, transactions: 0 },
    current: { users: 0, ledgers: 0, accounts: 0, transactions: 0 },
    schema_version: 0,
    current_version: 14,
    warnings: [],
  }

  it('有 error 时优先展示服务端原因', () => {
    expect(restoreRejectionText({ ...base, reason: 'not_sqlite', error: '不是有效的 SQLite 数据库文件' })).toBe(
      '不是有效的 SQLite 数据库文件',
    )
  })

  it('reason 码映射到中文文案（5 类全覆盖）', () => {
    expect(restoreRejectionText({ ...base, reason: 'not_sqlite' })).toContain('SQLite')
    expect(restoreRejectionText({ ...base, reason: 'integrity_failed' })).toContain('损坏')
    expect(restoreRejectionText({ ...base, reason: 'not_our_backup' })).toContain('不是本系统的备份')
    expect(restoreRejectionText({ ...base, reason: 'future_version' })).toContain('降级')
    expect(restoreRejectionText({ ...base, reason: 'file_too_large' })).toContain('512MB')
  })

  it('未知 reason 给通用兜底文案', () => {
    expect(restoreRejectionText({ ...base, reason: 'whatever' })).toBe('备份未通过预检')
  })

  it('确认词与后端硬约束一致', () => {
    expect(RESTORE_CONFIRM_WORD).toBe('RESTORE')
  })
})

describe('M2-B：恢复错误映射与对比行', () => {
  it('mapRestoreError 覆盖 401/403/400/5xx', () => {
    expect(mapRestoreError(401, '')).toContain('重新登录')
    expect(mapRestoreError(403, '')).toContain('系统管理员')
    expect(mapRestoreError(400, '上传文件过大或表单解析失败')).toBe('上传文件过大或表单解析失败')
    expect(mapRestoreError(500, '')).toContain('数据未改动')
    expect(mapRestoreError(0, '')).toContain('稍后重试')
  })

  it('restoreCompareRows 固定输出用户/账本/账户/交易四行', () => {
    const rows = restoreCompareRows(
      { users: 3, ledgers: 2, accounts: 48, transactions: 1263 },
      { users: 2, ledgers: 1, accounts: 36, transactions: 890 },
    )
    expect(rows.map((r) => r.label)).toEqual(['用户', '账本', '账户', '交易'])
    expect(rows[3]).toEqual({ label: '交易', backup: 1263, current: 890 })
  })
})

describe('M2-C：导入文案', () => {
  const preview: ImportPreviewResult = {
    valid: false,
    title: 'x',
    ledger_name: 'x',
    accounts: 0,
    transactions: 0,
    prices: 0,
    commodities: 0,
    duplicate: false,
    errors: [
      { line: 3, msg: '不支持的指令 balance' },
      { line: 9, msg: '不支持的语法 cost 注解' },
    ],
    error: '文件含不支持的语法，已整体拒绝',
  }

  it('formatImportErrors 一次列出全部错误行（不截断）', () => {
    const lines = formatImportErrors(preview.errors ?? [])
    expect(lines).toEqual(['第 3 行：不支持的指令 balance', '第 9 行：不支持的语法 cost 注解'])
  })

  it('importSummaryRows 输出账户/交易/价格/币种', () => {
    const rows = importSummaryRows({ ...preview, valid: true, accounts: 2, transactions: 5, prices: 1, commodities: 1 })
    expect(rows.map((r) => `${r.label}${r.value}`)).toEqual(['账户2', '交易5', '价格1', '币种1'])
  })

  it('mapImportError 覆盖 401/400/5xx', () => {
    expect(mapImportError(401, '')).toContain('重新登录')
    expect(mapImportError(400, 'text 不能为空')).toBe('text 不能为空')
    expect(mapImportError(503, '')).toContain('未产生任何账本')
  })
})
