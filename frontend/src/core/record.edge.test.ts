/**
 * QA 独立补充金额边界回归（Round 1，严过关 / Yan）
 *
 * 目标：证伪「定点求值、严格 ΣAssets + ΣLiabilities、不 abs、负号保留、
 * 三 Tab 配平」等声明。所有期望值均用 core/Money 独立复算，
 * 不复用被测函数的中间结果，避免「自己证明自己」。
 */
import { describe, expect, it } from 'vitest'

import { Money } from './Money'
import {
  buildPostings,
  computeBalanceSummary,
  evaluateAmountExpression,
  postingsBalanced,
  type RecordForm,
  type SummaryAccount,
} from './record'

function account(
  id: number,
  type: string,
  balances?: Record<string, string>,
  commodityRestriction?: string,
): SummaryAccount {
  return {
    id,
    name: `${type}:A${id}`,
    display_name: `账户${id}`,
    type,
    commodity_restriction: commodityRestriction,
    balances,
  }
}

function form(overrides: Partial<RecordForm> = {}): RecordForm {
  return {
    kind: 'expense',
    amountExpression: '38.50',
    date: '2026-11-10',
    description: '午餐',
    tags: [],
    categoryAccountId: 2,
    fundAccountId: 1,
    transferFromAccountId: null,
    transferToAccountId: null,
    ...overrides,
  }
}

const baseAccounts: SummaryAccount[] = [
  account(1, 'Assets', { CNY: '12000.00' }),
  account(2, 'Expenses', { CNY: '38.50' }),
  account(3, 'Income', { CNY: '-9500.00' }),
  account(4, 'Liabilities', { CNY: '-2340.50' }),
  account(5, 'Assets', { CNY: '-38.50' }),
]

describe('边界 A：金额表达式定点求值（AC-22）', () => {
  it('0.1+0.2 与长链小数运算全程无浮点尾差', () => {
    expect(evaluateAmountExpression('0.1+0.2').toString()).toBe('0.30')
    expect(evaluateAmountExpression('0.1+0.2+0.3').toString()).toBe('0.60')
    expect(evaluateAmountExpression('0.1+0.2+0.3+0.4+0.5').toString()).toBe('1.50')
    expect(evaluateAmountExpression('12.5-2.5').toString()).toBe('10.00')
    expect(evaluateAmountExpression('0.3-0.1').toString()).toBe('0.20')
    // 99999999.99 + 0.01 不得出现 100000000.00000001 之类尾差
    expect(evaluateAmountExpression('99999999.99+0.01').toString()).toBe('100000000.00')
  })

  it('表达式结果取绝对值（方向由三 Tab 决定），键盘输入形态可识别', () => {
    expect(evaluateAmountExpression('1-2').toString()).toBe('1.00')
    expect(evaluateAmountExpression('5-8+1').toString()).toBe('2.00')
    expect(evaluateAmountExpression('.5').toString()).toBe('0.50')
    expect(evaluateAmountExpression('12.').toString()).toBe('12.00')
  })

  it('非法表达式一律抛 RangeError，绝不静默按 0 处理', () => {
    const illegal = [
      '1e3',
      '1..2',
      '1.2.3',
      '1+',
      '+1',
      '-1',
      '1-',
      '',
      'abc',
      '1*2',
      ' 0.1 + 0.2 ', // 含空格：字符集校验拒绝
      '1--2',
    ]
    for (const expr of illegal) {
      expect(() => evaluateAmountExpression(expr), `表达式 ${expr} 应被拒绝`).toThrow(
        RangeError,
      )
    }
  })
})

describe('边界 B：三 Tab 分录符号与配平恒等式', () => {
  it('支出 / 收入 / 转账三条路径都生成两条且 Σpostings 恒为 0', () => {
    const cases: Array<RecordForm> = [
      form({ kind: 'expense' }),
      form({ kind: 'income', categoryAccountId: 3 }),
      form({
        kind: 'transfer',
        transferFromAccountId: 1,
        transferToAccountId: 5,
      }),
    ]
    for (const f of cases) {
      const postings = buildPostings(f, baseAccounts, 'CNY')
      expect(postings).toHaveLength(2)
      expect(postingsBalanced(postings)).toBe(true)
      // 独立复算：两条分录金额定点求和必须为 0。
      let sum = Money.zero()
      for (const p of postings) {
        sum = sum.plus(Money.from(p.amount))
      }
      expect(sum.isZero()).toBe(true)
      // 一正一负，方向由 kind 决定。
      const a = Money.from(postings[0].amount)
      const b = Money.from(postings[1].amount)
      expect(a.negated().toString()).toBe(b.toString())
    }
  })

  it('支出 Expenses 为正、资金账户为负；收入 Income 为负、资金账户为正', () => {
    const expense = buildPostings(form({ kind: 'expense' }), baseAccounts, 'CNY')
    expect(Money.from(expense[0].amount).isNegative()).toBe(false)
    expect(Money.from(expense[1].amount).isNegative()).toBe(true)

    const income = buildPostings(
      form({ kind: 'income', categoryAccountId: 3 }),
      baseAccounts,
      'CNY',
    )
    expect(Money.from(income[0].amount).isNegative()).toBe(true)
    expect(Money.from(income[1].amount).isNegative()).toBe(false)
  })

  it('postingsBalanced 能识别不配平的分录', () => {
    expect(
      postingsBalanced([
        { accountId: 1, accountName: 'A', commodity: 'CNY', amount: '10.00' },
        { accountId: 2, accountName: 'B', commodity: 'CNY', amount: '-9.99' },
      ]),
    ).toBe(false)
  })

  it('同账户转账拒绝', () => {
    expect(() =>
      buildPostings(
        form({ kind: 'transfer', transferFromAccountId: 1, transferToAccountId: 1 }),
        baseAccounts,
        'CNY',
      ),
    ).toThrow(RangeError)
  })

  it('跨币种拒绝：转账两端、分类与资金账户币种不一致都拒绝', () => {
    const mixed: SummaryAccount[] = [
      account(1, 'Assets', undefined, 'CNY'),
      account(5, 'Assets', undefined, 'USD'),
      account(2, 'Expenses', undefined, 'USD'),
    ]
    expect(() =>
      buildPostings(
        form({
          kind: 'transfer',
          transferFromAccountId: 1,
          transferToAccountId: 5,
        }),
        mixed,
        'CNY',
      ),
    ).toThrow(RangeError)
    expect(() =>
      buildPostings(
        form({ kind: 'expense', categoryAccountId: 2, fundAccountId: 1 }),
        mixed,
        'CNY',
      ),
    ).toThrow(RangeError)
  })

  it('零金额 / 缺账户 / 账户不在账本内一律拒绝', () => {
    expect(() => buildPostings(form({ amountExpression: '0' }), baseAccounts, 'CNY')).toThrow(
      RangeError,
    )
    expect(() =>
      buildPostings(form({ amountExpression: '0.001' }), baseAccounts, 'CNY'),
    ).toThrow(RangeError)
    expect(() => buildPostings(form({ fundAccountId: null }), baseAccounts, 'CNY')).toThrow(
      RangeError,
    )
    expect(() => buildPostings(form({ categoryAccountId: 999 }), baseAccounts, 'CNY')).toThrow(
      RangeError,
    )
  })
})

describe('边界 C：computeBalanceSummary 严格 ΣAssets + ΣLiabilities（AC-12）', () => {
  it('负债与净资产为负时保留负号，绝不 abs', () => {
    const summary = computeBalanceSummary(baseAccounts, 'CNY')
    // 12000.00 + (-38.50) = 11961.50；Liabilities = -2340.50
    expect(summary.assetTotal).toBe('11961.50')
    expect(summary.liabilityTotal).toBe('-2340.50')
    expect(summary.netWorthTotal).toBe('9621.00')
    expect(summary.liabilityTotal.charAt(0)).toBe('-')

    const negative = computeBalanceSummary(
      [
        account(1, 'Assets', { CNY: '100.00' }),
        account(2, 'Liabilities', { CNY: '-500.00' }),
      ],
      'CNY',
    )
    expect(negative.netWorthTotal).toBe('-400.00')
    expect(negative.netWorthTotal.charAt(0)).toBe('-')
    expect(negative.liabilityTotal).toBe('-500.00')
  })

  it('对每个币种独立满足 net === assets + liabilities 恒等式，且不含 Expenses/Income', () => {
    const accounts: SummaryAccount[] = [
      account(1, 'Assets', { CNY: '0.10', USD: '1.00', JPY: '-30.00' }),
      account(2, 'Assets', { CNY: '0.20', USD: '2.50' }),
      account(3, 'Liabilities', { CNY: '-0.05', USD: '-0.25' }),
      account(4, 'Liabilities', { CNY: '-1000.00', JPY: '5.00' }),
      account(5, 'Expenses', { CNY: '999.00', USD: '888.00' }),
      account(6, 'Income', { CNY: '-999.00', USD: '-888.00' }),
      account(7, 'Equity', { CNY: '777.00' }),
    ]
    const summary = computeBalanceSummary(accounts, 'CNY')

    // 0.1 + 0.2 无尾差
    expect(summary.assets.CNY).toBe('0.30')
    expect(summary.liabilities.CNY).toBe('-1000.05')
    expect(summary.netWorth.CNY).toBe('-999.75')

    const codes = new Set<string>([
      ...Object.keys(summary.assets),
      ...Object.keys(summary.liabilities),
      ...Object.keys(summary.netWorth),
    ])
    for (const code of codes) {
      const a = Money.from(summary.assets[code] === undefined ? '0' : summary.assets[code])
      const l = Money.from(
        summary.liabilities[code] === undefined ? '0' : summary.liabilities[code],
      )
      const expected = a.plus(l).toString()
      expect(
        summary.netWorth[code],
        `币种 ${code} 的净资产不等于 ΣAssets+ΣLiabilities`,
      ).toBe(expected)
    }
    // Expenses / Income / Equity 的余额不得混进任何汇总桶。
    expect(summary.assets.USD).toBe('3.50')
    expect(summary.liabilities.USD).toBe('-0.25')
  })

  it('空币种回退 CNY；默认币种无数据时回退首个有数据的币种', () => {
    const cnyOnly = computeBalanceSummary([account(1, 'Assets', { CNY: '12.34' })], '')
    expect(cnyOnly.commodity).toBe('CNY')
    expect(cnyOnly.netWorthTotal).toBe('12.34')

    const usdOnly = computeBalanceSummary([account(1, 'Assets', { USD: '9.87' })], '')
    expect(usdOnly.commodity).toBe('USD')
    expect(usdOnly.netWorthTotal).toBe('9.87')

    const empty = computeBalanceSummary([], '')
    expect(empty.commodity).toBe('CNY')
    expect(empty.netWorthTotal).toBe('0.00')
  })

  it('非法余额字符串按 0 容错，不抛错、不污染其它币种', () => {
    const summary = computeBalanceSummary(
      [
        account(1, 'Assets', { CNY: 'oops', USD: '5.00' }),
        account(2, 'Liabilities', { CNY: '1e5' }),
      ],
      'CNY',
    )
    expect(summary.assetTotal).toBe('0.00')
    expect(summary.liabilityTotal).toBe('0.00')
    expect(summary.netWorth.USD).toBe('5.00')
  })
})
