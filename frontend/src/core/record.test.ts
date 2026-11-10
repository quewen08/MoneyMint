/**
 * 记一笔金额平衡与净资产符号口径用例（AC-12 / AC-22 / AC-23 前置）
 *
 * 全部金额走 core/Money 定点运算，禁止裸浮点参与。
 */
import { describe, expect, it } from 'vitest'

import {
  buildPostings,
  computeBalanceSummary,
  evaluateAmountExpression,
  postingsBalanced,
  todayString,
  transactionKind,
  transferAmount,
  transferRoute,
  type PostingLike,
  type RecordForm,
  type SummaryAccount,
} from './record'

const accounts: SummaryAccount[] = [
  {
    id: 1,
    name: 'Assets:Cash',
    display_name: '招商储蓄',
    type: 'Assets',
    balances: { CNY: '-38.50' },
  },
  {
    id: 2,
    name: 'Expenses:Food',
    display_name: '食品饮料',
    type: 'Expenses',
    balances: { CNY: '38.50' },
  },
  {
    id: 3,
    name: 'Income:Salary',
    display_name: '薪资收入',
    type: 'Income',
    balances: { CNY: '-9500.00' },
  },
  {
    id: 4,
    name: 'Liabilities:CreditCard',
    display_name: '信用卡',
    type: 'Liabilities',
    balances: { CNY: '-2340.50' },
  },
  {
    id: 5,
    name: 'Assets:Bank',
    display_name: '工商银行',
    type: 'Assets',
    balances: { CNY: '12000.00' },
  },
]

function form(overrides: Partial<RecordForm> = {}): RecordForm {
  return {
    kind: 'expense',
    amountExpression: '38.50',
    date: '2026-11-09',
    description: '午餐',
    tags: [],
    categoryAccountId: 2,
    fundAccountId: 1,
    transferFromAccountId: null,
    transferToAccountId: null,
    ...overrides,
  }
}

describe('金额表达式定点求值', () => {
  it('0.1+0.2 无浮点尾差（AC-22）', () => {
    expect(evaluateAmountExpression('0.1+0.2').toString()).toBe('0.30')
    expect(evaluateAmountExpression('12.5-2.5').toString()).toBe('10.00')
    expect(evaluateAmountExpression('.5+12.').toString()).toBe('12.50')
    expect(evaluateAmountExpression('0.1+0.2+0.3').toString()).toBe('0.60')
  })

  it('非法表达式直接抛错，绝不静默按 0 处理', () => {
    expect(() => evaluateAmountExpression('1e3')).toThrow(RangeError)
    expect(() => evaluateAmountExpression('1+')).toThrow(RangeError)
    expect(() => evaluateAmountExpression('+1')).toThrow(RangeError)
    expect(() => evaluateAmountExpression('')).toThrow(RangeError)
  })
})

describe('记一笔分录平衡与符号口径', () => {
  it('支出：Expenses +amount / 资金账户 -amount，且配平', () => {
    const postings = buildPostings(form(), accounts, 'CNY')
    expect(postings).toHaveLength(2)
    expect(postings[0].accountId).toBe(2)
    expect(postings[0].amount).toBe('38.50')
    expect(postings[1].accountId).toBe(1)
    expect(postings[1].amount).toBe('-38.50')
    expect(postingsBalanced(postings)).toBe(true)
  })

  it('收入：Income -amount / 资金账户 +amount，且配平', () => {
    const postings = buildPostings(
      form({ kind: 'income', amountExpression: '100.00', categoryAccountId: 3 }),
      accounts,
      'CNY',
    )
    expect(postings[0].amount).toBe('-100.00')
    expect(postings[1].amount).toBe('100.00')
    expect(postingsBalanced(postings)).toBe(true)
  })

  it('转账：转出 -amount / 转入 +amount，且配平', () => {
    const postings = buildPostings(
      form({
        kind: 'transfer',
        amountExpression: '2000.00',
        transferFromAccountId: 1,
        transferToAccountId: 5,
      }),
      accounts,
      'CNY',
    )
    expect(postings[0].accountId).toBe(1)
    expect(postings[0].amount).toBe('-2000.00')
    expect(postings[1].accountId).toBe(5)
    expect(postings[1].amount).toBe('2000.00')
    expect(postingsBalanced(postings)).toBe(true)
  })

  it('拒绝同一账户转账、跨币种转账与零金额', () => {
    expect(() =>
      buildPostings(
        form({ kind: 'transfer', transferFromAccountId: 1, transferToAccountId: 1 }),
        accounts,
        'CNY',
      ),
    ).toThrow(RangeError)
    const mixed: SummaryAccount[] = [
      { id: 1, name: 'Assets:Cash', type: 'Assets', commodity_restriction: 'CNY' },
      { id: 5, name: 'Assets:USD', type: 'Assets', commodity_restriction: 'USD' },
    ]
    expect(() =>
      buildPostings(
        form({ kind: 'transfer', transferFromAccountId: 1, transferToAccountId: 5 }),
        mixed,
        'CNY',
      ),
    ).toThrow(RangeError)
    expect(() => buildPostings(form({ amountExpression: '0' }), accounts, 'CNY')).toThrow(
      RangeError,
    )
  })

  it('缺账户 / 账户不在当前账本时给出可定位的校验错误', () => {
    expect(() => buildPostings(form({ fundAccountId: null }), accounts, 'CNY')).toThrow(
      RangeError,
    )
    expect(() => buildPostings(form({ categoryAccountId: 999 }), accounts, 'CNY')).toThrow(
      RangeError,
    )
  })
})

describe('首页最近流水展示语义（转账 / 支出 / 收入）', () => {
  const viewAccounts: SummaryAccount[] = [
    { id: 1, name: 'Assets:Cash', display_name: '招商储蓄', type: 'Assets' },
    { id: 2, name: 'Assets:Bank', display_name: '工商银行', type: 'Assets' },
    { id: 3, name: 'Liabilities:Card', display_name: '信用卡', type: 'Liabilities' },
    { id: 4, name: 'Expenses:Food', display_name: '食品饮料', type: 'Expenses' },
    { id: 5, name: 'Income:Salary', display_name: '薪资收入', type: 'Income' },
    { id: 6, name: 'Equity:Opening', display_name: '期初余额', type: 'Equity' },
  ]
  const posting = (account_id: number, amount: string): PostingLike => ({
    account_id,
    amount,
  })

  it('转账：分录全为资金账户 → transfer，金额取转出方绝对值且为正', () => {
    const postings = [posting(1, '-100.00'), posting(2, '100.00')]
    expect(transactionKind(postings, viewAccounts)).toBe('transfer')
    expect(transferAmount(postings, viewAccounts)).toBe('100.00')
    expect(transferAmount(postings, viewAccounts).charAt(0)).not.toBe('-')
    expect(transferRoute(postings, viewAccounts)).toBe('招商储蓄 → 工商银行')
  })

  it('转账：信用卡（Liabilities）转出同样按绝对值展示', () => {
    const postings = [posting(3, '-38.50'), posting(2, '38.50')]
    expect(transactionKind(postings, viewAccounts)).toBe('transfer')
    expect(transferAmount(postings, viewAccounts)).toBe('38.50')
    expect(transferRoute(postings, viewAccounts)).toBe('信用卡 → 工商银行')
  })

  it('支出：含 Expenses → expense（与转账明确区分）', () => {
    const postings = [posting(4, '38.50'), posting(1, '-38.50')]
    expect(transactionKind(postings, viewAccounts)).toBe('expense')
  })

  it('收入：含 Income → income', () => {
    const postings = [posting(5, '-9500.00'), posting(1, '9500.00')]
    expect(transactionKind(postings, viewAccounts)).toBe('income')
  })

  it('混合 / 异常回退 expense（未知账户类型、空分录、账户未加载）', () => {
    expect(
      transactionKind([posting(6, '1.00'), posting(1, '-1.00')], viewAccounts),
    ).toBe('expense')
    expect(transactionKind([], viewAccounts)).toBe('expense')
    expect(transactionKind([posting(999, '-1.00')], viewAccounts)).toBe('expense')
  })

  it('转账金额无负值分录时回退第一条资金分录绝对值，非法金额容错为 0.00', () => {
    expect(
      transferAmount([posting(1, '100.00'), posting(2, '-100.00')], viewAccounts),
    ).toBe('100.00')
    expect(transferAmount([posting(1, 'oops')], viewAccounts)).toBe('0.00')
    expect(transferAmount([], viewAccounts)).toBe('0.00')
  })
})

describe('首页净资产口径', () => {
  it('ΣAssets + ΣLiabilities，负债与净资产为负时保留负号（AC-12 / §6.4）', () => {
    const summary = computeBalanceSummary(accounts, 'CNY')
    // Assets: -38.50 + 12000.00 = 11961.50；Liabilities: -2340.50。
    expect(summary.assetTotal).toBe('11961.50')
    expect(summary.liabilityTotal).toBe('-2340.50')
    expect(summary.netWorthTotal).toBe('9621.00')
    expect(summary.netWorth.CNY).toBe('9621.00')

    const negative = computeBalanceSummary(
      [
        { id: 1, name: 'Assets:Cash', type: 'Assets', balances: { CNY: '100.00' } },
        { id: 2, name: 'Liabilities:Loan', type: 'Liabilities', balances: { CNY: '-500.00' } },
      ],
      'CNY',
    )
    expect(negative.netWorthTotal).toBe('-400.00')
    expect(negative.netWorthTotal.charAt(0)).toBe('-')
  })

  it('账本默认币种无余额时回退到首个有数据的币种，避免首页误显示 0.00', () => {
    const summary = computeBalanceSummary(
      [
        {
          id: 1,
          name: 'Assets:Cash',
          type: 'Assets',
          balances: { USD: '12.00' },
        },
      ],
      'CNY',
    )
    expect(summary.commodity).toBe('USD')
    expect(summary.netWorthTotal).toBe('12.00')
  })

  it('多币种分组求和、非法余额按 0 容错，Expenses / Income 不计入净资产', () => {
    const summary = computeBalanceSummary(
      [
        { id: 1, name: 'Assets:Cash', type: 'Assets', balances: { CNY: '0.10', USD: '1.00' } },
        { id: 2, name: 'Assets:Bank', type: 'Assets', balances: { CNY: '0.20' } },
        { id: 3, name: 'Liabilities:Card', type: 'Liabilities', balances: { CNY: 'oops' } },
        { id: 4, name: 'Expenses:Food', type: 'Expenses', balances: { CNY: '999.00' } },
        { id: 5, name: 'Income:Salary', type: 'Income', balances: { CNY: '-999.00' } },
      ],
      'CNY',
    )
    expect(summary.assetTotal).toBe('0.30')
    expect(summary.liabilityTotal).toBe('0.00')
    expect(summary.netWorthTotal).toBe('0.30')
    expect(summary.netWorth.USD).toBe('1.00')
  })
})

describe('日期', () => {
  it('按本地时区补零，不依赖 toISOString 造成跨日', () => {
    expect(todayString(new Date(2026, 10, 9, 23, 30))).toBe('2026-11-09')
    expect(todayString(new Date(2026, 0, 5, 8, 0))).toBe('2026-01-05')
  })
})
