/**
 * 流水筛选与 query 化（AC-16：筛选状态写入 URL，刷新 / 分享可完整还原）
 *
 * hash 路由下用 vue-router 的 query；本模块只依赖 `Record<string, string|string[]|undefined>`
 * 形状的 query（与 vue-router LocationQuery 兼容），core 层不引入 vue-router。
 * 所有解析都容错：非法值一律回退默认，不抛错。
 */
import { transactionKind, type SummaryAccount } from './record'

/** 可筛选交易的最小结构（api/types 的 TransactionView 结构兼容）。 */
export interface FilterableTxn {
  id: number
  date: string
  flag: string
  description: string
  created_by_name: string
  postings: Array<{ account_id: number; amount: string }>
}

export type FilterKind = 'all' | 'expense' | 'income' | 'transfer'

export interface TransactionFilters {
  /** `YYYY-MM-DD` 或空串（含端点）。 */
  from: string
  to: string
  kind: FilterKind
  /** 资金 / 分类账户 id（posting 命中即算）。 */
  account: number | null
  category: number | null
  /** 记账人（created_by_name 精确匹配），空串表示全部。 */
  member: string
}

/** vue-router `LocationQuery` 兼容形状（数组元素可为 null）。 */
type QueryLike = Record<string, string | null | (string | null)[] | undefined>

const KINDS: readonly string[] = ['all', 'expense', 'income', 'transfer']
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/

export function emptyFilters(): TransactionFilters {
  return { from: '', to: '', kind: 'all', account: null, category: null, member: '' }
}

function firstQuery(value: string | null | (string | null)[] | undefined): string {
  if (Array.isArray(value)) {
    for (const item of value) {
      if (typeof item === 'string') {
        return item
      }
    }
    return ''
  }
  return typeof value === 'string' ? value : ''
}

function parseId(raw: string): number | null {
  if (raw === '' || !/^\d+$/.test(raw)) {
    return null
  }
  const id = Number(raw)
  return Number.isSafeInteger(id) && id > 0 ? id : null
}

/** 从 URL query 还原筛选；非法值回退默认（AC-16 读路径）。 */
export function filtersFromQuery(query: QueryLike): TransactionFilters {
  const filters = emptyFilters()
  const from = firstQuery(query.from)
  const to = firstQuery(query.to)
  if (DATE_RE.test(from)) {
    filters.from = from
  }
  if (DATE_RE.test(to)) {
    filters.to = to
  }
  const kind = firstQuery(query.kind)
  if (KINDS.indexOf(kind) >= 0) {
    filters.kind = kind as FilterKind
  }
  const account = parseId(firstQuery(query.account))
  if (account !== null) {
    filters.account = account
  }
  const category = parseId(firstQuery(query.category))
  if (category !== null) {
    filters.category = category
  }
  filters.member = firstQuery(query.member)
  return filters
}

/** 把筛选写回 query（空值省略，保证 URL 干净；AC-16 写路径）。 */
export function filtersToQuery(filters: TransactionFilters): Record<string, string> {
  const query: Record<string, string> = {}
  if (filters.from !== '') {
    query.from = filters.from
  }
  if (filters.to !== '') {
    query.to = filters.to
  }
  if (filters.kind !== 'all') {
    query.kind = filters.kind
  }
  if (filters.account !== null) {
    query.account = String(filters.account)
  }
  if (filters.category !== null) {
    query.category = String(filters.category)
  }
  if (filters.member !== '') {
    query.member = filters.member
  }
  return query
}

/** 是否存在生效筛选（供 UI 显示「清除筛选」）。 */
export function hasActiveFilters(filters: TransactionFilters): boolean {
  return (
    filters.from !== '' ||
    filters.to !== '' ||
    filters.kind !== 'all' ||
    filters.account !== null ||
    filters.category !== null ||
    filters.member !== ''
  )
}

/**
 * 应用筛选。日期用 `YYYY-MM-DD` 字典序比较（含端点）；
 * kind 用统一 `transactionKind` 口径判定，禁止各处自行 abs / 乘系数。
 * 泛型保留元素类型，调用方可直接得到 TransactionView[]。
 */
export function applyFilters<T extends FilterableTxn>(
  txns: T[],
  filters: TransactionFilters,
  accounts: SummaryAccount[],
): T[] {
  return txns.filter((txn) => {
    if (filters.from !== '' && txn.date < filters.from) {
      return false
    }
    if (filters.to !== '' && txn.date > filters.to) {
      return false
    }
    if (filters.kind !== 'all' && transactionKind(txn.postings, accounts) !== filters.kind) {
      return false
    }
    if (filters.account !== null) {
      let hit = false
      for (const posting of txn.postings) {
        if (posting.account_id === filters.account) {
          hit = true
        }
      }
      if (!hit) {
        return false
      }
    }
    if (filters.category !== null) {
      let hit = false
      for (const posting of txn.postings) {
        if (posting.account_id === filters.category) {
          hit = true
        }
      }
      if (!hit) {
        return false
      }
    }
    if (filters.member !== '' && txn.created_by_name !== filters.member) {
      return false
    }
    return true
  })
}
