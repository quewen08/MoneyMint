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

/** 账户层级树节点（批 2.5 修 2）。 */
export interface AccountTreeNode<T extends SummaryAccount> {
  account: T
  children: AccountTreeNode<T>[]
  /** 树深度（根为 0，供渲染缩进）。 */
  depth: number
  /** 父账户自身 + 全部后代的按币种聚合小计（定点，符号保留）。 */
  subtotal: Record<string, string>
}

interface TreeMoneyBucket {
  [code: string]: Money
}

function mergeBucket(target: TreeMoneyBucket, source: TreeMoneyBucket): void {
  for (const code of Object.keys(source)) {
    const previous = target[code]
    target[code] = previous === undefined ? source[code] : previous.plus(source[code])
  }
}

function selfBucket(account: SummaryAccount): TreeMoneyBucket {
  const bucket: TreeMoneyBucket = {}
  const balances = account.balances === undefined ? {} : account.balances
  for (const code of Object.keys(balances)) {
    let value: Money
    try {
      value = Money.from(balances[code])
    } catch {
      value = Money.zero()
    }
    bucket[code] = value
  }
  return bucket
}

function subtreeBucket<T extends SummaryAccount>(node: AccountTreeNode<T>): TreeMoneyBucket {
  const bucket = selfBucket(node.account)
  for (const child of node.children) {
    mergeBucket(bucket, subtreeBucket(child))
  }
  return bucket
}

/**
 * 账户层级树（批 2.5 修 2；旧版 categories_screen.dart:45-48 同款 parent_uuid
 * 优先、ledger_controller.dart:180-194 子随父排序）：
 *
 * - **parent_uuid 显式关系优先**：parent 在列表内即挂为其子节点；
 * - parent_uuid 缺失时按 **name 冒号层级**推导兜底（`Expenses:食品饮料:食品`
 *   的父是列表内 name 为 `Expenses:食品饮料` 的账户）——两者同时可用时
 *   parent_uuid 胜出（显式关系优先，冒号推导只做兜底）；
 * - 孤儿节点（parent 不在列表 / 推导不出父）作为根；
 * - 同层按 `compare` 排序（缺省按 sort_order 升序、并列保持 name 字典序），
 *   子随父；
 * - 节点 subtotal = 自身 + 全部后代（fava 账户表折叠小计语义，定点、符号保留）。
 */
export function buildAccountTree<T extends SummaryAccount>(
  accounts: readonly T[],
  compare?: (a: T, b: T) => number,
): AccountTreeNode<T>[] {
  const byCompare =
    compare ??
    ((a: T, b: T) =>
      (a.sort_order ?? 0) - (b.sort_order ?? 0) || (a.name < b.name ? -1 : 1))
  const nodes = new Map<number, AccountTreeNode<T>>()
  const byUuid = new Map<string, T>()
  const byName = new Map<string, T>()
  for (const account of accounts) {
    nodes.set(account.id, { account, children: [], depth: 0, subtotal: {} })
    if (account.uuid !== undefined && account.uuid !== '') {
      byUuid.set(account.uuid, account)
    }
    byName.set(account.name, account)
  }

  // 父 id 解析：parent_uuid 优先，缺失时冒号层级推导（找不到父则视为根）。
  const parentIds = new Map<number, number | null>()
  for (const account of accounts) {
    let parentId: number | null = null
    const parentUuid = account.parent_uuid
    if (parentUuid !== undefined && parentUuid !== '') {
      const parentAccount = byUuid.get(parentUuid)
      if (parentAccount !== undefined && parentAccount.id !== account.id) {
        parentId = parentAccount.id
      }
    } else {
      const lastColon = account.name.lastIndexOf(':')
      if (lastColon > 0) {
        const parentName = account.name.slice(0, lastColon)
        const parent = byName.get(parentName)
        if (parent !== undefined && parent.id !== account.id) {
          parentId = parent.id
        }
      }
    }
    parentIds.set(account.id, parentId)
  }

  const roots: AccountTreeNode<T>[] = []
  for (const account of accounts) {
    const node = nodes.get(account.id)
    const parentId = parentIds.get(account.id)
    if (node === undefined) {
      continue
    }
    if (parentId == null) {
      roots.push(node)
      continue
    }
    const parent = nodes.get(parentId)
    if (parent === undefined) {
      roots.push(node)
    } else {
      parent.children.push(node)
    }
  }

  const attach = (node: AccountTreeNode<T>, depth: number): void => {
    node.depth = depth
    node.children.sort((a, b) => byCompare(a.account, b.account))
    for (const child of node.children) {
      attach(child, depth + 1)
    }
    node.subtotal = bucketToStrings(subtreeBucket(node))
  }
  roots.sort((a, b) => byCompare(a.account, b.account))
  for (const root of roots) {
    attach(root, 0)
  }
  return roots
}

/** 树的深度优先扁平化（渲染用：depth 供缩进，childCount 供父行小计展示）。 */
export function flattenAccountTree<T extends SummaryAccount>(
  roots: readonly AccountTreeNode<T>[],
): Array<{
  account: T
  depth: number
  subtotal: Record<string, string>
  childCount: number
}> {
  const out: Array<{
    account: T
    depth: number
    subtotal: Record<string, string>
    childCount: number
  }> = []
  const walk = (nodes: readonly AccountTreeNode<T>[]): void => {
    for (const node of nodes) {
      out.push({
        account: node.account,
        depth: node.depth,
        subtotal: node.subtotal,
        childCount: node.children.length,
      })
      walk(node.children)
    }
  }
  walk(roots)
  return out
}
