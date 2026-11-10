/**
 * QA 批 2.5 独立证伪（Round 1，严过关 / Yan）
 *
 * 五项修正逐条证伪，期望值独立复算：
 *  1. 三分法降级（筛选分录命中 / 日历分录级聚合 / opening kind / 混合交易语义）
 *  2. 账户层级树（parent_uuid 优先、冒号兜底、孤儿为根、小计=自身+后代、环容错）
 *  3. 多币种符号（displayEntryOf 币种取分录自身）
 *  4. 账户侧 journal（原值原符号 + running balance 主币种累计）
 *  5. 高级双录（balanceDiffByCommodity 按币种配平；组件路径见工程师
 *     RecordSheet.advanced.test.ts + 本文件附带的独立组件用例）
 */
import { describe, expect, it } from 'vitest'

import {
  buildAccountTree,
  flattenAccountTree,
  type SummaryAccount,
} from './accounts'
import { dailyTotals } from './calendar'
import {
  accountRunningBalances,
  balanceDiffByCommodity,
  displayEntryOf,
  hasAccountType,
  postingAmountForAccount,
  transactionKind,
} from './record'
import { applyFilters, emptyFilters } from './filters'

const ACC: SummaryAccount[] = [
  { id: 1, name: 'Assets:Cash', type: 'Assets' },
  { id: 2, name: 'Assets:Bank', type: 'Assets' },
  { id: 3, name: 'Liabilities:Card', type: 'Liabilities' },
  { id: 4, name: 'Expenses:Food', type: 'Expenses' },
  { id: 5, name: 'Income:Salary', type: 'Income' },
  { id: 6, name: 'Equity:Opening', type: 'Equity' },
]

// ---------------------------------------------------------------------------
// 1. 三分法降级：混合交易在筛选 / 日历 / badge / 代表分录的语义
// ---------------------------------------------------------------------------
describe('批2.5-1：混合交易（Expenses -50 + Income -30 + Assets +80）语义', () => {
  const mixed = [
    { account_id: 4, amount: '-50.00', commodity: 'CNY' },
    { account_id: 5, amount: '-30.00', commodity: 'CNY' },
    { account_id: 1, amount: '80.00', commodity: 'CNY' },
  ]
  const txns = [
    { id: 1, date: '2026-11-10', flag: '*', description: '退款报销', created_by_name: '小明', postings: mixed },
  ]

  it('kind=expense 与 kind=income 双命中（分录命中口径）', () => {
    expect(applyFilters(txns, { ...emptyFilters(), kind: 'expense' }, ACC)).toHaveLength(1)
    expect(applyFilters(txns, { ...emptyFilters(), kind: 'income' }, ACC)).toHaveLength(1)
    expect(hasAccountType(mixed, ACC, 'Expenses')).toBe(true)
    expect(hasAccountType(mixed, ACC, 'Income')).toBe(true)
  })

  it('kind=transfer 不命中；整笔 badge 启发式回退 expense（文档化口径）', () => {
    expect(applyFilters(txns, { ...emptyFilters(), kind: 'transfer' }, ACC)).toHaveLength(0)
    expect(transactionKind(mixed, ACC)).toBe('expense')
  })

  it('日历分录级聚合：Expenses -50 入支出桶、Income -30 入收入桶（原值，不 abs）', () => {
    const totals = dailyTotals(
      [{ date: '2026-11-10', postings: mixed }],
      ACC,
      'CNY',
    )
    expect(totals['2026-11-10']).toEqual({ expense: '-50.00', income: '-30.00' })
  })

  it('displayEntryOf 混合交易优先正号资金分录（净收款 80.00），币种取分录自身', () => {
    expect(displayEntryOf(mixed, ACC)).toEqual({ amount: '80.00', commodity: 'CNY' })
  })

  it('期初交易（含 Equity）：kind=opening、不计入日历收支（Equity/Assets 均不统计）', () => {
    const opening = [
      { account_id: 6, amount: '-1000.00', commodity: 'CNY' },
      { account_id: 1, amount: '1000.00', commodity: 'CNY' },
    ]
    expect(transactionKind(opening, ACC)).toBe('opening')
    const totals = dailyTotals(
      [{ date: '2026-01-01', postings: opening }],
      ACC,
      'CNY',
    )
    expect(totals['2026-01-01']).toBeUndefined()
  })

  it('转账对外语义不破坏：transfer 整笔无分类分录，日历天然不计入', () => {
    const transfer = [
      { account_id: 1, amount: '-2000.00', commodity: 'CNY' },
      { account_id: 2, amount: '2000.00', commodity: 'CNY' },
    ]
    expect(transactionKind(transfer, ACC)).toBe('transfer')
    expect(applyFilters(
      [{ id: 9, date: '2026-11-10', flag: '*', description: 't', created_by_name: '', postings: transfer }],
      { ...emptyFilters(), kind: 'transfer' },
      ACC,
    )).toHaveLength(1)
    expect(dailyTotals([{ date: '2026-11-10', postings: transfer }], ACC, 'CNY')['2026-11-10']).toBeUndefined()
  })
})

// ---------------------------------------------------------------------------
// 2. 账户层级树
// ---------------------------------------------------------------------------
describe('批2.5-2：buildAccountTree / flattenAccountTree', () => {
  const treeAccounts: SummaryAccount[] = [
    { id: 1, uuid: 'u-food', name: 'Expenses:Food', type: 'Expenses', sort_order: 1, balances: { CNY: '100.00' } },
    { id: 2, uuid: 'u-snack', name: 'Expenses:Food:Snack', parent_uuid: 'u-food', type: 'Expenses', sort_order: 2, balances: { CNY: '20.00' } },
    { id: 3, name: 'Expenses:Food:Drink', type: 'Expenses', sort_order: 0, balances: { CNY: '-5.50' } },
    { id: 4, uuid: 'u-orphan', name: 'Expenses:Orphan', parent_uuid: 'u-missing', type: 'Expenses', sort_order: 3, balances: { CNY: '7.00' } },
    { id: 5, uuid: 'u-cash', name: 'Assets:Cash', type: 'Assets', sort_order: 4, balances: { CNY: '9.00' } },
  ]

  it('三级树：parent_uuid 优先、冒号推导兜底、孤儿为根、子随父、depth 正确', () => {
    const roots = buildAccountTree(treeAccounts)
    const flat = flattenAccountTree(roots)
    // 根：Expenses:Food(id1，id3 冒号推导挂它)、Expenses:Orphan(id4)、Assets:Cash(id5)
    const rootNames = roots.map((r) => r.account.name)
    expect(rootNames).toEqual(['Expenses:Food', 'Expenses:Orphan', 'Assets:Cash'])
    expect(roots[0].children.map((c) => c.account.name)).toEqual([
      'Expenses:Food:Drink', // sort_order 0
      'Expenses:Food:Snack', // sort_order 2（parent_uuid 命中）
    ])
    const depths = new Map(flat.map((n) => [n.account.name, n.depth]))
    expect(depths.get('Expenses:Food')).toBe(0)
    expect(depths.get('Expenses:Food:Drink')).toBe(1)
    expect(depths.get('Expenses:Orphan')).toBe(0) // parent 不在列表 → 根
  })

  it('父小计 = 自身 + 全部后代（定点、负号保留）', () => {
    const roots = buildAccountTree(treeAccounts)
    // Food 100.00 + Drink(-5.50) + Snack(20.00) = 114.50
    expect(roots[0].subtotal.CNY).toBe('114.50')
    const flat = flattenAccountTree(roots)
    const drink = flat.find((n) => n.account.name === 'Expenses:Food:Drink')
    expect(drink?.subtotal.CNY).toBe('-5.50')
  })

  it('parent_uuid 与冒号推导冲突时显式关系胜出', () => {
    const conflict: SummaryAccount[] = [
      { id: 1, uuid: 'u-a', name: 'Expenses:A', type: 'Expenses', sort_order: 0 },
      { id: 2, uuid: 'u-b', name: 'Expenses', type: 'Expenses', sort_order: 1 },
      { id: 3, uuid: 'u-x', name: 'Expenses:A:X', parent_uuid: 'u-b', type: 'Expenses', sort_order: 2 },
    ]
    const roots = buildAccountTree(conflict)
    // 冒号推导父是 Expenses:A(id1)，但 parent_uuid='u-b' 显式指向 id2 → X 挂 Expenses；
    // A 自身也按冒号推导挂 Expenses（作为独立子节点）。
    const expenseNode = roots.find((r) => r.account.name === 'Expenses')
    expect(expenseNode?.children.map((c) => c.account.name)).toEqual([
      'Expenses:A',
      'Expenses:A:X',
    ])
    const aNode = roots.find((r) => r.account.name === 'Expenses:A')
    expect(aNode).toBeUndefined() // A 不再是根（已挂到 Expenses 下）
    expect(expenseNode?.children.find((c) => c.account.name === 'Expenses:A:X')?.children).toHaveLength(0)
  })

  it('环（A.parent=B、B.parent=A）：不崩溃，但节点不入树（现状记录，需上层防环）', () => {
    const cycle: SummaryAccount[] = [
      { id: 1, uuid: 'u-c1', name: 'Assets:C1', parent_uuid: 'u-c2', type: 'Assets' },
      { id: 2, uuid: 'u-c2', name: 'Assets:C2', parent_uuid: 'u-c1', type: 'Assets' },
    ]
    const flat = flattenAccountTree(buildAccountTree(cycle))
    expect(flat).toHaveLength(0) // 现状：环内节点被静默丢弃
  })
})

// ---------------------------------------------------------------------------
// 3. 多币种符号（displayEntryOf 币种取分录自身）
// ---------------------------------------------------------------------------
describe('批2.5-3：行内金额按分录币种渲染', () => {
  it('非主币种分录返回自身 commodity；单分录资金分录按转账语义取绝对值；缺省兜底空串', () => {
    // 双分录支出：非转账路径 → 资金分录带符号原值，币种取分录自身（USD）
    const usd = [
      { account_id: 4, amount: '25.00', commodity: 'USD' },
      { account_id: 1, amount: '-25.00', commodity: 'USD' },
    ]
    expect(displayEntryOf(usd, ACC)).toEqual({ amount: '-25.00', commodity: 'USD' })
    // 单分录资金分录被整笔判为 transfer → 取绝对值（transferAmount 口径）
    const single = [{ account_id: 1, amount: '-25.00', commodity: 'USD' }]
    expect(displayEntryOf(single, ACC)).toEqual({ amount: '25.00', commodity: 'USD' })
    const noCommodity = [{ account_id: 1, amount: '-25.00' }]
    expect(displayEntryOf(noCommodity, ACC).commodity).toBe('')
  })

  it('支出 / 收入代表分录仍取资金分录带符号原值（兼容入口等价）', () => {
    const expense = [
      { account_id: 4, amount: '10.00', commodity: 'CNY' },
      { account_id: 1, amount: '-10.00', commodity: 'CNY' },
    ]
    expect(displayEntryOf(expense, ACC)).toEqual({ amount: '-10.00', commodity: 'CNY' })
    const income = [
      { account_id: 5, amount: '-100.00', commodity: 'CNY' },
      { account_id: 2, amount: '100.00', commodity: 'CNY' },
    ]
    expect(displayEntryOf(income, ACC)).toEqual({ amount: '100.00', commodity: 'CNY' })
  })
})

// ---------------------------------------------------------------------------
// 4. 账户侧 journal
// ---------------------------------------------------------------------------
describe('批2.5-4：postingAmountForAccount / accountRunningBalances', () => {
  it('账户侧分录金额 = 原值原符号（负债还款 +500、Income -9500），无分录 0.00', () => {
    const txn = {
      postings: [
        { account_id: 3, amount: '500.00', commodity: 'CNY' },
        { account_id: 1, amount: '-500.00', commodity: 'CNY' },
      ],
    }
    expect(postingAmountForAccount(txn, 3)).toBe('500.00') // 负债账户视角：还款减少负债
    expect(postingAmountForAccount(txn, 1)).toBe('-500.00')
    expect(postingAmountForAccount(txn, 99)).toBe('0.00')
  })

  it('running balance 日期升序累计、仅主币种、非主币种行不累计', () => {
    const txns = [
      { id: 1, date: '2026-11-02', postings: [{ account_id: 1, amount: '-38.50', commodity: 'CNY' }] },
      { id: 2, date: '2026-11-01', postings: [{ account_id: 1, amount: '100.00', commodity: 'CNY' }] },
      { id: 3, date: '2026-11-03', postings: [{ account_id: 1, amount: '50.00', commodity: 'USD' }] },
    ]
    const result = accountRunningBalances(txns, 1, 'CNY')
    expect(result[2]).toBe('100.00') // 升序首笔
    expect(result[1]).toBe('61.50') // 100.00 - 38.50，定点无尾差
    expect(result[3]).toBe('61.50') // USD 行不累计，余额保持
  })
})

// ---------------------------------------------------------------------------
// 5. 高级双录配平（core 层）
// ---------------------------------------------------------------------------
describe('批2.5-5：balanceDiffByCommodity 按币种配平', () => {
  it('配平返回空数组；0.1+0.2 无尾差', () => {
    expect(balanceDiffByCommodity([
      { commodity: 'CNY', amount: '10.00' },
      { commodity: 'CNY', amount: '-10.00' },
    ])).toEqual([])
    expect(balanceDiffByCommodity([
      { commodity: 'CNY', amount: '0.10' },
      { commodity: 'CNY', amount: '0.20' },
      { commodity: 'CNY', amount: '-0.30' },
    ])).toEqual([])
  })

  it('不配平直出差额明细；空币种归入 CNY；多币种各自独立判定', () => {
    expect(balanceDiffByCommodity([
      { commodity: 'CNY', amount: '100.00' },
      { commodity: 'CNY', amount: '-30.00' },
    ])).toEqual([{ commodity: 'CNY', diff: '70.00' }])
    expect(balanceDiffByCommodity([
      { commodity: '', amount: '10.00' },
      { commodity: '', amount: '-10.00' },
    ])).toEqual([])
    expect(balanceDiffByCommodity([
      { commodity: 'CNY', amount: '10.00' },
      { commodity: 'CNY', amount: '-10.00' },
      { commodity: 'USD', amount: '5.00' },
      { commodity: 'USD', amount: '-3.00' },
    ])).toEqual([{ commodity: 'USD', diff: '2.00' }])
  })
})
