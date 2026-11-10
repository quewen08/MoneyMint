/**
 * CSV 导出（§14.5 #1 Must，功能与旧版 import_export_screen.dart 对齐）
 *
 * 表头与字段顺序与旧版 1:1：`date,description,flag,account,commodity,amount`，
 * 每条分录一行；description 与 account 用双引号包裹并把内部 `"` 转义为 `""`；
 * date / flag / commodity / amount 原样输出（后端定点字符串，不经数值运算）。
 * 编码：Blob 默认 UTF-8（等价旧版 Utf8Encoder），不加 BOM。
 */
import { accountLabel, type SummaryAccount } from './record'

export interface CsvRow {
  date: string
  description: string
  flag: string
  postings: Array<{ account: string; commodity: string; amount: string }>
}

/**
 * 可导出交易的最小结构（与 api/types 的 TransactionView.postings 兼容）。
 */
export interface CsvExportTxn {
  date: string
  description: string
  flag: string
  postings: Array<{ account_id: number; account: string; commodity: string; amount: string }>
}

/**
 * §14.5 #1：从交易列表构造导出行。
 * account 列 = `display_name` 优先、回退 Beancount 全名（等价旧版
 * `ledger.accountName(uuid)` 语义，P1-B 修复）；按 account_id 从账户列表映射。
 */
export function buildCsvRowsFromTransactions(
  txns: CsvExportTxn[],
  accounts: SummaryAccount[],
): CsvRow[] {
  const rows: CsvRow[] = []
  for (const txn of txns) {
    const postings: CsvRow['postings'] = []
    for (const posting of txn.postings) {
      let label = posting.account
      for (const account of accounts) {
        if (account.id === posting.account_id) {
          label = accountLabel(account)
          break
        }
      }
      postings.push({
        account: label,
        commodity: posting.commodity,
        amount: posting.amount,
      })
    }
    rows.push({ date: txn.date, description: txn.description, flag: txn.flag, postings })
  }
  return rows
}

/** 与旧版一致：双引号包裹 + 内部双引号翻倍转义。 */
export function csvQuoted(value: string): string {
  return `"${value.split('"').join('""')}"`
}

/** 生成 CSV 文本（行尾带换行，等价旧版 writeln 行为）。 */
export function buildTransactionsCsv(rows: CsvRow[]): string {
  const lines: string[] = ['date,description,flag,account,commodity,amount']
  for (const row of rows) {
    for (const posting of row.postings) {
      lines.push(
        [
          row.date,
          csvQuoted(row.description),
          row.flag,
          csvQuoted(posting.account),
          posting.commodity,
          posting.amount,
        ].join(','),
      )
    }
  }
  return `${lines.join('\n')}\n`
}

/** 触发浏览器下载；非浏览器环境（Vitest）静默跳过。 */
export function downloadCsv(filename: string, content: string): void {
  if (typeof Blob === 'undefined' || typeof document === 'undefined') {
    return
  }
  const blob = new Blob([content], { type: 'text/csv' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
