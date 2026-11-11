/**
 * M2 恢复 / 导入的纯逻辑（批 4，P-19 / P-20）
 *
 * 与 `core/batch3.ts` 同一口径：视图只做编排，判定与文案全部收口在这里，
 * 便于 Vitest 直接单测（不挂载组件）。
 *
 * 关键口径（与 docs/m2-plan.md 对齐）：
 * - 恢复是全库替换：预检摘要必须展示「备份 vs 当前」覆盖量对比，
 *   执行需输入确认词 RESTORE 解锁，成功后提示用备份中的账号重新登录；
 * - 导入是新建账本：预检 valid=false 时整体拒绝并列出全部错误行号
 *   （RK-M2-04：跳过等于悄悄丢账）。
 */
import type {
  BeancountParseError,
  ImportPreviewResult,
  RestorePreviewResult,
  RestoreStats,
} from '@/api/types'

/** 执行还原所需的二次确认词（后端 service.restore 同值硬校验）。 */
export const RESTORE_CONFIRM_WORD = 'RESTORE'

/** 预检拒绝原因码 → 人类可读文案（error 兜底，未知原因给通用文案）。 */
export function restoreRejectionText(result: RestorePreviewResult): string {
  if (result.error !== undefined && result.error !== '') {
    return result.error
  }
  const byReason: Record<string, string> = {
    not_sqlite: '不是有效的 SQLite 数据库文件',
    integrity_failed: '备份文件已损坏，无法通过完整性校验',
    not_our_backup: '缺少核心表，不是本系统的备份',
    future_version: '备份来自更新的版本，降级还原会丢列',
    file_too_large: '备份文件超过 512MB 上限',
  }
  if (result.reason !== undefined && byReason[result.reason] !== undefined) {
    return byReason[result.reason]
  }
  return '备份未通过预检'
}

/** 恢复端点错误（4xx/5xx）→ 文案。 */
export function mapRestoreError(status: number, message: string): string {
  if (status === 401) return '登录状态已失效，请重新登录后再试'
  if (status === 403) return '仅系统管理员可以还原备份'
  if (status === 400) return message !== '' ? message : '上传失败：请确认选择的是 .db 备份文件'
  if (status >= 500) return '服务端还原失败，数据未改动；请稍后重试或联系管理员'
  return message !== '' ? message : '还原请求失败，请稍后重试'
}

/** 导入端点错误（4xx/5xx）→ 文案。 */
export function mapImportError(status: number, message: string): string {
  if (status === 401) return '登录状态已失效，请重新登录后再试'
  if (status === 400) return message !== '' ? message : '请求无效：请确认文件内容为 Beancount 文本'
  if (status >= 500) return '服务端导入失败，未产生任何账本；请稍后重试'
  return message !== '' ? message : '导入请求失败，请稍后重试'
}

/** 预检对比卡的一行：核心表名 + 备份行数 + 当前行数。 */
export interface RestoreCompareRow {
  label: string
  backup: number
  current: number
}

/** 构造「备份 vs 当前」覆盖量对比行（顺序固定：用户/账本/账户/交易）。 */
export function restoreCompareRows(backup: RestoreStats, current: RestoreStats): RestoreCompareRow[] {
  return [
    { label: '用户', backup: backup.users, current: current.users },
    { label: '账本', backup: backup.ledgers, current: current.ledgers },
    { label: '账户', backup: backup.accounts, current: current.accounts },
    { label: '交易', backup: backup.transactions, current: current.transactions },
  ]
}

/**
 * 解析错误列表 → 逐行文案（`第 N 行：原因`）。
 * 后端保证收集全部错误后才返回，这里保序展示，不做去重截断。
 */
export function formatImportErrors(errors: BeancountParseError[]): string[] {
  return errors.map((e) => `第 ${e.line} 行：${e.msg}`)
}

/** 导入预检的摘要行（账户/交易/价格/币种计数）。 */
export interface ImportSummaryRow {
  label: string
  value: number
}

export function importSummaryRows(prev: ImportPreviewResult): ImportSummaryRow[] {
  return [
    { label: '账户', value: prev.accounts },
    { label: '交易', value: prev.transactions },
    { label: '价格', value: prev.prices },
    { label: '币种', value: prev.commodities },
  ]
}
