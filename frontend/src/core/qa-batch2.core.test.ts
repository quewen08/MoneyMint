/**
 * QA 批 2 独立证伪（Round 1，严过关 / Yan）
 *
 * 期望值全部独立手工复算 / 对照旧版 Flutter 实现，不复用工程师断言：
 *  - CSV：对照 lib/screens/import_export/import_export_screen.dart:53-75 逐列复算；
 *  - 日历：对照 lib/widgets/calendar_view.dart:82-112（转账剔除 + 主导币种）；
 *  - AC-29：affectedTransactionCount 同笔多分录只计一次；
 *  - 期初：FR-03 边界 + hold/owe 符号（FR-05）；
 *  - 筛选：query 往返不漂移；排序：合并语义。
 */
import { describe, expect, it } from 'vitest'

import {
  affectedTransactionCount,
  isClosed,
  recordFundAccounts,
  type SummaryAccount,
} from './accounts'
import { buildMonthGrid, dailyTotals } from './calendar'
import { buildTransactionsCsv, csvQuoted, type CsvRow } from './csv'
import {
  applyFilters,
  emptyFilters,
  filtersFromQuery,
  filtersToQuery,
  hasActiveFilters,
} from './filters'
import { moveItem, sortBySortOrder } from './ordering'
import { openingSignedAmount, validateOpeningForm } from './opening'
import { transactionKind } from './record'

// ---------------------------------------------------------------------------
// 1. CSV 与旧版 1:1（期望值手工复算，不调用任何被测函数生成期望）
// ---------------------------------------------------------------------------
describe('批2-A：CSV 逐列对照旧版（手工期望值）', () => {
  const rows: CsvRow[] = [
    {
      date: '2026-11-10',
      description: '午餐,含饮料',
      flag: '*',
      postings: [
        { account: 'Expenses:Food', commodity: 'CNY', amount: '38.50' },
        { account: 'Assets:Cash', commodity: 'CNY', amount: '-38.50' },
      ],
    },
    {
      date: '2026-11-11',
      description: '说"你好"\n第二行',
      flag: '!',
      postings: [
        { account: 'Income:Salary', commodity: 'CNY', amount: '-9500.00' },
      ],
    },
  ]

  it('表头 / 列序 / 逗号与引号与换行转义 / 每分录一行 / 行尾单换行全部与旧版一致', () => {
    const csv = buildTransactionsCsv(rows)
    const lines = csv.split('\n')
    // 旧版 import_export_screen.dart:56 表头逐字符一致
    expect(lines[0]).toBe('date,description,flag,account,commodity,amount')
    // description 含逗号：引号包裹（旧版 :60）；account 列旧版同样双引号包裹（旧版 :63）
    expect(lines[1]).toBe('2026-11-10,"午餐,含饮料",*,"Expenses:Food",CNY,38.50')
    expect(lines[2]).toBe('2026-11-10,"午餐,含饮料",*,"Assets:Cash",CNY,-38.50')
    // description 含双引号 → 翻倍（前后各一个引号都翻倍）；含换行 → 引号内合法换行（旧版 replaceAll('"','""') 一致）
    expect(csv).toContain('"说""你好""\n第二行",!,"Income:Salary",CNY,-9500.00')
    // 行尾：旧版 writeln → 单个结尾换行，不得有空行
    expect(csv.endsWith('\n')).toBe(true)
    expect(csv.endsWith('\n\n')).toBe(false)
  })

  it('金额列原样透传后端定点字符串，绝不补零或数值化（旧版 p.amount 原样）', () => {
    const csv = buildTransactionsCsv([
      { date: '2026-11-10', description: 'x', flag: '*', postings: [
        { account: 'A', commodity: 'CNY', amount: '38.5' },
      ] },
    ])
    expect(csv).toContain(',38.5\n') // 不得被格式化成 38.50
    expect(csv).not.toContain('38.50')
  })

  it('csvQuoted：双引号包裹 + 内部引号翻倍（等价旧版 replaceAll）', () => {
    expect(csvQuoted('a"b')).toBe('"a""b"')
    expect(csvQuoted('plain')).toBe('"plain"')
    expect(csvQuoted('"')).toBe('""""')
  })

  it('【已修复验证】buildCsvRowsFromTransactions：account 列 display_name 优先回退全名（等价旧版 ledger.accountName）', async () => {
    const { buildCsvRowsFromTransactions } = await import('./csv')
    const accounts: SummaryAccount[] = [
      { id: 1, name: 'Expenses:Food', display_name: '食品', type: 'Expenses' },
      { id: 2, name: 'Assets:NoDisplay', type: 'Assets' }, // 无 display_name → 回退全名
    ]
    const rows = buildCsvRowsFromTransactions(
      [
        {
          date: '2026-11-10',
          description: 'x',
          flag: '*',
          postings: [
            { account_id: 1, account: 'Expenses:Food', commodity: 'CNY', amount: '1.00' },
            { account_id: 2, account: 'Assets:NoDisplay', commodity: 'CNY', amount: '-1.00' },
            { account_id: 99, account: 'Ghost', commodity: 'CNY', amount: '0.00' }, // 映射未命中回退原值
          ],
        },
      ],
      accounts,
    )
    expect(rows[0].postings.map((p) => p.account)).toEqual([
      '食品',
      'Assets:NoDisplay',
      'Ghost',
    ])
    // 金额仍原样定点字符串
    expect(rows[0].postings.map((p) => p.amount)).toEqual(['1.00', '-1.00', '0.00'])
  })
})

// ---------------------------------------------------------------------------
// 2. 日历每日收支口径（对照旧版 calendar_view.dart:82-112）
//    旧版口径：只统计分类科目（Expenses→负、Income→正），
//    「资产/负债/权益之间的转移不计入当日收支」，且只统计主导币种。
// ---------------------------------------------------------------------------
const calAccounts: SummaryAccount[] = [
  { id: 1, name: 'Assets:Cash', type: 'Assets' },
  { id: 2, name: 'Assets:Bank', type: 'Assets' },
  { id: 3, name: 'Liabilities:Card', type: 'Liabilities' },
  { id: 4, name: 'Expenses:Food', type: 'Expenses' },
  { id: 5, name: 'Income:Salary', type: 'Income' },
]

describe('批2-B：dailyTotals 口径（转账剔除 / 币种）', () => {
  it('支出 / 收入按分类分录统计符号正确（批 2.5 分录级聚合：Expenses 借方正、Income 原值负）', () => {
    const totals = dailyTotals(
      [
        { date: '2026-11-10', postings: [
          { account_id: 4, amount: '38.50', commodity: 'CNY' },
          { account_id: 1, amount: '-38.50', commodity: 'CNY' },
        ] },
        { date: '2026-11-11', postings: [
          { account_id: 5, amount: '-9500.00', commodity: 'CNY' },
          { account_id: 2, amount: '9500.00', commodity: 'CNY' },
        ] },
      ],
      calAccounts,
      'CNY',
    )
    // Expenses 借方分录原值为正 → 支出桶 38.50；Income 分录原值为负 → 收入桶 -9500.00
    // （fava income statement 口径，符号由展示层规则处理）。
    expect(totals['2026-11-10']).toEqual({ expense: '38.50', income: '0.00' })
    expect(totals['2026-11-11']).toEqual({ expense: '0.00', income: '-9500.00' })
  })

  it('【P1 证据】转账（资产→资产）不得计入当日收支（旧版明确剔除）', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-10',
          postings: [
            { account_id: 1, amount: '-2000.00' }, // 转出
            { account_id: 2, amount: '2000.00' }, // 转入
          ],
        },
      ],
      calAccounts,
    )
    const day = totals['2026-11-10']
    expect(
      day === undefined || (day.expense === '0.00' && day.income === '0.00'),
    ).toBe(true)
  })

  it('【P1 证据】还款类转账（资产→负债）同样不得计入当日收支', () => {
    const totals = dailyTotals(
      [
        {
          date: '2026-11-10',
          postings: [
            { account_id: 1, amount: '-500.00' }, // Assets 转出
            { account_id: 3, amount: '500.00' }, // Liabilities 还款
          ],
        },
      ],
      calAccounts,
    )
    const day = totals['2026-11-10']
    expect(
      day === undefined || (day.expense === '0.00' && day.income === '0.00'),
    ).toBe(true)
  })

  it('【已修复验证】三参 mainCommodity 只统计该币种资金分录，跨币种不混加', () => {
    const totals = dailyTotals(
      [
        { date: '2026-11-10', postings: [
          { account_id: 4, amount: '100.00', commodity: 'CNY' },
          { account_id: 1, amount: '-100.00', commodity: 'CNY' },
        ] },
        { date: '2026-11-10', postings: [
          { account_id: 4, amount: '50.00', commodity: 'USD' },
          { account_id: 2, amount: '-50.00', commodity: 'USD' },
        ] },
      ],
      calAccounts,
      'CNY',
    )
    // 只统计 CNY 分类分录；USD +50 不得混入（对齐旧版主导币种口径）。
    expect(totals['2026-11-10']?.expense).toBe('100.00')
  })

  it('两参兼容模式不过滤币种（现状锁定，calendar.ts:50 注释已声明调用方责任）', () => {
    const totals = dailyTotals(
      [
        { date: '2026-11-10', postings: [
          { account_id: 4, amount: '100.00', commodity: 'CNY' },
          { account_id: 1, amount: '-100.00', commodity: 'CNY' },
        ] },
        { date: '2026-11-10', postings: [
          { account_id: 4, amount: '50.00', commodity: 'USD' },
          { account_id: 2, amount: '-50.00', commodity: 'USD' },
        ] },
      ],
      calAccounts,
    )
    // 兼容模式下混加仍然发生；当前唯一调用处 TransactionsView.vue:84 已传三参。
    expect(totals['2026-11-10']?.expense).toBe('150.00')
  })

  it('buildMonthGrid：周一开头 6×7=42 格，补位日 inMonth 正确、key 为真实日期', () => {
    // 2026-11-01 是周日 → offset=6，首格为 10-26（周一）
    const cells = buildMonthGrid(2026, 11, {})
    expect(cells).toHaveLength(42)
    expect(cells[0].key).toBe('2026-10-26')
    expect(cells[0].inMonth).toBe(false)
    const firstInMonth = cells.find((c) => c.key === '2026-11-01')
    expect(firstInMonth?.inMonth).toBe(true)
    expect(cells[41].key).toBe('2026-12-06')
    expect(cells[41].inMonth).toBe(false)
    // 每列周一起始：第 7k 格的 key 也必须是周一
    for (let i = 0; i < 42; i += 7) {
      expect(new Date(`${cells[i].key}T00:00:00`).getDay()).toBe(1)
    }
  })
})

// ---------------------------------------------------------------------------
// 3. AC-29 笔数口径 + AC-28 过滤器
// ---------------------------------------------------------------------------
describe('批2-C：affectedTransactionCount（AC-29）与关户过滤（AC-28）', () => {
  it('同一笔交易 3 条分录指向同一账户只计 1 次；跨账户重复分录不误计', () => {
    const txns = [
      { postings: [{ account_id: 5 }, { account_id: 5 }, { account_id: 5 }] },
      { postings: [{ account_id: 6 }, { account_id: 7 }, { account_id: 5 }] },
      { postings: [{ account_id: 8 }] },
    ]
    expect(affectedTransactionCount(5, txns)).toBe(2) // 两笔交易，非 4 条分录
    expect(affectedTransactionCount(6, txns)).toBe(1)
    expect(affectedTransactionCount(99, txns)).toBe(0)
  })

  it('isClosed / recordFundAccounts：空串 close_date 视为未关闭，与 RecordSheet 的 !close_date 口径一致', () => {
    const accounts: SummaryAccount[] = [
      { id: 1, name: 'A', type: 'Assets' },
      { id: 2, name: 'B', type: 'Assets', close_date: '' },
      { id: 3, name: 'C', type: 'Assets', close_date: '2026-11-01' },
      { id: 4, name: 'D', type: 'Expenses' },
      { id: 5, name: 'E', type: 'Liabilities' },
    ]
    expect(isClosed(accounts[2])).toBe(true)
    expect(isClosed(accounts[1])).toBe(false)
    const funds = recordFundAccounts(accounts)
    expect(funds.map((a) => a.id)).toEqual([1, 2, 5]) // 关户 3 与分类 4 被排除
  })

  it('【已修复验证】buildPostings 前端不拦关户账户（由后端 409 account_closed 兜底，AC-28 双防线）', async () => {
    const { buildPostings } = await import('./record')
    const accounts: SummaryAccount[] = [
      { id: 1, name: 'Assets:Cash', type: 'Assets' },
      { id: 2, name: 'Expenses:Food', type: 'Expenses', close_date: '2026-11-01' },
    ]
    const postings = buildPostings(
      {
        kind: 'expense',
        amountExpression: '10',
        date: '2026-11-10',
        description: 'x',
        tags: [],
        categoryAccountId: 2, // 已关户
        fundAccountId: 1,
        transferFromAccountId: null,
        transferToAccountId: null,
      },
      accounts,
      'CNY',
    )
    expect(postings).toHaveLength(2) // 关户账户照常生成可提交分录 → 只剩后端兜底
  })
})

// ---------------------------------------------------------------------------
// 4. 期初：FR-03 边界 + FR-05 符号
// ---------------------------------------------------------------------------
describe('批2-D：期初符号与 FR-03 边界', () => {
  it('owe（欠款）落库符号为负，hold 为正（FR-05 / 11-05 拍板）', () => {
    expect(openingSignedAmount('500.00', 'owe')).toBe('-500.00')
    expect(openingSignedAmount('500.00', 'hold')).toBe('500.00')
    expect(openingSignedAmount('0.30', 'owe')).toBe('-0.30') // 定点无尾差
  })

  it('FR-03：open_date == opening_date 允许（相等不报错）', () => {
    const v = validateOpeningForm({
      amount: '100',
      direction: 'hold',
      accountType: 'Assets',
      accountOpenDate: '2026-01-01',
      ledgerOpeningDate: '2026-01-01',
    })
    expect(v.ok).toBe(true)
    expect(v.absAmount).toBe('100.00')
  })

  it('FR-03：open_date 晚于 opening_date 拒绝', () => {
    const v = validateOpeningForm({
      amount: '100',
      direction: 'hold',
      accountType: 'Assets',
      accountOpenDate: '2026-01-02',
      ledgerOpeningDate: '2026-01-01',
    })
    expect(v.ok).toBe(false)
    expect(v.errors.join()).toContain('不得晚于')
  })

  it('空 opening_date：格式校验拒绝（调用方 AccountForm:53-60 已回退今日，见代码审查）', () => {
    const v = validateOpeningForm({
      amount: '100',
      direction: 'hold',
      accountType: 'Assets',
      accountOpenDate: '2026-01-01',
      ledgerOpeningDate: '',
    })
    expect(v.ok).toBe(false)
    expect(v.errors.join()).toContain('期初日期格式不合法')
  })

  it('负数金额 / 非资产类型 / 非法方向均拒绝；0 金额合法（后端跳过语义）', () => {
    expect(
      validateOpeningForm({
        amount: '-5',
        direction: 'hold',
        accountType: 'Assets',
        accountOpenDate: '2026-01-01',
        ledgerOpeningDate: '2026-01-01',
      }).ok,
    ).toBe(false)
    expect(
      validateOpeningForm({
        amount: '5',
        direction: 'hold',
        accountType: 'Equity',
        accountOpenDate: '2026-01-01',
        ledgerOpeningDate: '2026-01-01',
      }).ok,
    ).toBe(false)
    expect(
      validateOpeningForm({
        amount: '5',
        direction: 'sideways',
        accountType: 'Assets',
        accountOpenDate: '2026-01-01',
        ledgerOpeningDate: '2026-01-01',
      } as never).ok,
    ).toBe(false)
    expect(
      validateOpeningForm({
        amount: '0',
        direction: 'hold',
        accountType: 'Assets',
        accountOpenDate: '2026-01-01',
        ledgerOpeningDate: '2026-01-01',
      }).absAmount,
    ).toBe('0.00')
  })
})

// ---------------------------------------------------------------------------
// 5. 筛选 query 往返不漂移（AC-16）
// ---------------------------------------------------------------------------
describe('批2-E：筛选 query 往返稳定性', () => {
  const queries: Array<Record<string, string | null | (string | null)[] | undefined>> = [
    {},
    { from: '2026-11-01', to: '2026-11-30', kind: 'expense', account: '3', category: '4', member: '小明' },
    { from: 'abc', to: '2026-13-99', kind: 'bogus', account: '-1', category: '0', member: '' },
    { from: ['2026-01-01', null], kind: ['income'], account: ['7'] },
    { view: 'calendar', from: '2026-02-30' }, // 视图标记不得混入筛选；格式合法日期原样保留
  ]

  it('fromQuery → toQuery → fromQuery 不漂移（含非法值 / 数组值 / 多余键）', () => {
    for (const q of queries) {
      const f1 = filtersFromQuery(q)
      const q2 = filtersToQuery(f1)
      const f2 = filtersFromQuery(q2)
      expect(f2).toEqual(f1)
    }
  })

  it('非法值回退默认：kind bogus→all、account -1→null、日期 abc→空', () => {
    const f = filtersFromQuery({ from: 'abc', kind: 'bogus', account: '-1' })
    expect(f).toEqual(emptyFilters())
  })

  it('applyFilters：日期含端点、kind 判定、account/category 命中、member 精确匹配', () => {
    const txns = [
      { id: 1, date: '2026-11-01', flag: '*', description: 'a', created_by_name: '小明',
        postings: [{ account_id: 4, amount: '10.00' }, { account_id: 1, amount: '-10.00' }] },
      { id: 2, date: '2026-11-30', flag: '*', description: 'b', created_by_name: '小红',
        postings: [{ account_id: 5, amount: '-20.00' }, { account_id: 2, amount: '20.00' }] },
      { id: 3, date: '2026-12-01', flag: '*', description: 'c', created_by_name: '小明',
        postings: [{ account_id: 1, amount: '-5.00' }, { account_id: 2, amount: '5.00' }] },
    ]
    // 含端点：11-01 与 11-30 都命中，12-01 排除
    expect(
      applyFilters(txns, { ...emptyFilters(), from: '2026-11-01', to: '2026-11-30' }, calAccounts).map((t) => t.id),
    ).toEqual([1, 2])
    // kind=transfer：交易 3 全部为资金分录
    expect(
      applyFilters(txns, { ...emptyFilters(), kind: 'transfer' }, calAccounts).map((t) => t.id),
    ).toEqual([3])
    // account 命中任一分录即可
    expect(
      applyFilters(txns, { ...emptyFilters(), account: 1 }, calAccounts).map((t) => t.id),
    ).toEqual([1, 3])
    // member 精确匹配
    expect(
      applyFilters(txns, { ...emptyFilters(), member: '小红' }, calAccounts).map((t) => t.id),
    ).toEqual([2])
    expect(hasActiveFilters(emptyFilters())).toBe(false)
    expect(hasActiveFilters({ ...emptyFilters(), member: 'x' })).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// 6. 排序逻辑层 + transactionKind 容错
// ---------------------------------------------------------------------------
describe('批2-F：排序逻辑与 kind 容错', () => {
  it('moveItem：越界 / 相同位置原样返回，正常移动不可变', () => {
    const base = [1, 2, 3, 4]
    expect(moveItem(base, 0, 2)).toEqual([2, 3, 1, 4])
    expect(moveItem(base, -1, 2)).toEqual(base)
    expect(moveItem(base, 1, 9)).toEqual(base)
    expect(moveItem(base, 2, 2)).toEqual(base)
    expect(base).toEqual([1, 2, 3, 4]) // 原数组不被修改
  })

  it('sortBySortOrder：同序号保持原相对顺序（稳定）', () => {
    const list = [
      { id: 'a', sort: 2 },
      { id: 'b', sort: 1 },
      { id: 'c', sort: 2 },
      { id: 'd', sort: 1 },
    ]
    expect(sortBySortOrder(list, (x) => x.sort).map((x) => x.id)).toEqual(['b', 'd', 'a', 'c'])
  })

  it('transactionKind：空分录 / 空 accounts 的容错行为（记录现状）', () => {
    expect(transactionKind([], calAccounts)).toBe('expense')
    // accounts 未加载时全部回退 expense（列表徽章短暂错标，见报告观察项）
    expect(transactionKind([{ account_id: 1, amount: '-1' }], [])).toBe('expense')
    expect(
      transactionKind(
        [{ account_id: 1, amount: '-1' }, { account_id: 2, amount: '1' }],
        calAccounts,
      ),
    ).toBe('transfer')
  })
})
