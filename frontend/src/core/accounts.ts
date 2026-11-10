/**
 * 账户视图辅助（P-07 分组余额 / AC-28 记一笔不可选过滤 / AC-29 受影响笔数 / 关户守门）
 *
 * 小计与差额一律 core/Money 定点，禁止裸浮点；符号口径与首页一致：
 * 后端余额本身带符号（负债为负），禁止 abs / 按类型乘系数。
 */
import { Money } from './Money'

// SummaryAccount 由 core/record 定义（QA 契约测试从本模块 import 该类型）。
export type { SummaryAccount } from './record'
import type { SummaryAccount } from './record'

/** 关户守门：close_date 非空即已关闭（AC-28 不可选、P-08 只读）。 */
export function isClosed<T extends SummaryAccount>(account: T): boolean {
  return account.close_date !== undefined && account.close_date !== ''
}

export function openAccounts<T extends SummaryAccount>(accounts: readonly T[]): T[] {
  return accounts.filter((account) => !isClosed(account))
}

/**
 * AC-28：记一笔可选的资金账户 = 未关闭且类型为 Assets / Liabilities。
 * 已关户账户在 C-01 的账户下拉里不可选。
 */
export function recordFundAccounts<T extends SummaryAccount>(accounts: readonly T[]): T[] {
  return openAccounts(accounts).filter(
    (account) => account.type === 'Assets' || account.type === 'Liabilities',
  )
}

/** 指定币种余额（缺数据按 0.00），原样保留后端带符号值。 */
export function balanceOf(account: SummaryAccount, commodity: string): string {
  const balances = account.balances
  if (balances === undefined) {
    return '0.00'
  }
  const value = balances[commodity]
  return value === undefined ? '0.00' : value
}

export interface AccountGroup<T extends SummaryAccount = SummaryAccount> {
  type: string
  label: string
  items: T[]
  /** 按币种小计（符号保留，负债组为负）。 */
  subtotal: Record<string, string>
}

const GROUP_LABELS: { [type: string]: string } = {
  Assets: '资产',
  Liabilities: '负债',
  Income: '收入分类',
  Expenses: '支出分类',
  Equity: '权益',
}

interface MoneyBucket {
  [code: string]: Money
}

function bucketToStrings(bucket: MoneyBucket): Record<string, string> {
  const result: Record<string, string> = {}
  for (const code of Object.keys(bucket)) {
    result[code] = bucket[code].toString()
  }
  return result
}

/**
 * 按账户类型分组并小计（P-07 / 分类页共用）。
 * 只产出 `types` 中出现的组，空组也会返回（供空态渲染）。
 */
export function groupAccountsByType<T extends SummaryAccount>(
  accounts: readonly T[],
  types: readonly string[],
): AccountGroup<T>[] {
  const buckets: { [type: string]: { items: T[]; money: MoneyBucket } } = {}
  for (const type of types) {
    buckets[type] = { items: [], money: {} }
  }
  for (const account of accounts) {
    const bucket = buckets[account.type]
    if (bucket === undefined) {
      continue
    }
    bucket.items.push(account)
    const balances = account.balances === undefined ? {} : account.balances
    for (const code of Object.keys(balances)) {
      let value: Money
      try {
        value = Money.from(balances[code])
      } catch {
        value = Money.zero()
      }
      const previous = bucket.money[code]
      bucket.money[code] = previous === undefined ? value : previous.plus(value)
    }
  }
  const groups: AccountGroup<T>[] = []
  for (const type of types) {
    const bucket = buckets[type]
    groups.push({
      type,
      label: GROUP_LABELS[type] === undefined ? type : GROUP_LABELS[type],
      items: bucket.items,
      subtotal: bucketToStrings(bucket.money),
    })
  }
  return groups
}

/**
 * AC-29：关户 / 删除前统计「引用该账户的未删除交易笔数」，
 * 二次确认弹窗必须列出该笔数。
 */
export function affectedTransactionCount(
  accountId: number,
  txns: ReadonlyArray<{ postings: ReadonlyArray<{ account_id: number }> }>,
): number {
  let count = 0
  for (const txn of txns) {
    for (const posting of txn.postings) {
      if (posting.account_id === accountId) {
        count += 1
        break
      }
    }
  }
  return count
}
