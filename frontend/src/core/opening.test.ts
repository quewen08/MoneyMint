/**
 * 期初表单校验用例（绝对值 / 方向 / open_date 约束 / 类型限制 / 符号预览）
 */
import { describe, expect, it } from 'vitest'

import { openingSignedAmount, validateOpeningForm } from './opening'

const base = {
  amount: '3200',
  direction: 'hold' as const,
  accountType: 'Assets',
  accountOpenDate: '2026-01-01',
  ledgerOpeningDate: '2026-01-01',
}

describe('期初表单校验（opening-balance-prd §7）', () => {
  it('合法输入通过并给出两位定点绝对值', () => {
    const result = validateOpeningForm(base)
    expect(result.ok).toBe(true)
    expect(result.errors).toEqual([])
    expect(result.absAmount).toBe('3200.00')
  })

  it('金额取绝对值口径：负数输入被拒绝（方向由「持有/欠款」表达）', () => {
    const result = validateOpeningForm({ ...base, amount: '-3200' })
    expect(result.ok).toBe(false)
    expect(result.errors.join('')).toContain('非负')
    expect(result.absAmount).toBeNull()
  })

  it('欠款方向映射为负、持有为正（FR-05 落库符号预览）', () => {
    expect(openingSignedAmount('3200.00', 'owe')).toBe('-3200.00')
    expect(openingSignedAmount('3200.00', 'hold')).toBe('3200.00')
  })

  it('open_date 晚于账本期初日期被拒绝（FR-03）', () => {
    const result = validateOpeningForm({
      ...base,
      accountOpenDate: '2026-02-01',
      ledgerOpeningDate: '2026-01-01',
    })
    expect(result.ok).toBe(false)
    expect(result.errors.join('')).toContain('不得晚于账本期初日期')
  })

  it('open_date 早于账本期初日期允许（FR-24 迁移修正后的合法形态）', () => {
    const result = validateOpeningForm({
      ...base,
      accountOpenDate: '2025-06-01',
      ledgerOpeningDate: '2026-01-01',
    })
    expect(result.ok).toBe(true)
  })

  it('仅 Assets / Liabilities 可设期初（FR-04）', () => {
    const result = validateOpeningForm({ ...base, accountType: 'Expenses' })
    expect(result.ok).toBe(false)
    expect(result.errors.join('')).toContain('仅资产 / 负债账户')
  })

  it('金额格式与空值校验', () => {
    expect(validateOpeningForm({ ...base, amount: '' }).errors).toContain('请输入期初金额')
    expect(validateOpeningForm({ ...base, amount: '1e3' }).errors.join('')).toContain('格式不正确')
    expect(validateOpeningForm({ ...base, amount: 'abc' }).errors.join('')).toContain('格式不正确')
  })

  it('日期格式不合法直接报错（金额校验独立，absAmount 只反映金额合法性）', () => {
    const result = validateOpeningForm({
      ...base,
      accountOpenDate: '2026/01/01',
      ledgerOpeningDate: '20260101',
    })
    expect(result.ok).toBe(false)
    expect(result.errors.join('')).toContain('格式不合法')
    expect(result.errors.join('')).toContain('账本期初日期')
  })
})
