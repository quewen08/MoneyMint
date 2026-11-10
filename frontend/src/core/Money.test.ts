import { describe, expect, it } from 'vitest'

import { Money, formatAmount, formatMoney } from './Money'

describe('Money 定点金额', () => {
  it('detail: -0.001 舍入后归一为 0.00，不出现负零', () => {
    expect(formatAmount('-0.001')).toBe('0.00')
    expect(Money.from('-0.001').toString()).toBe('0.00')
  })

  it('detail: 0.005 按 ROUND_HALF_UP 进位为 0.01', () => {
    expect(formatAmount('0.005')).toBe('0.01')
    expect(Money.from('0.005').toString()).toBe('0.01')
  })

  it('detail: 1e21 量级不输出指数记数法', () => {
    const money = Money.from('1000000000000000000000')
    expect(money.toString()).toBe('1000000000000000000000.00')
    expect(money.formatAmount()).toBe('1,000,000,000,000,000,000,000.00')
  })

  it('detail: 大额千分位', () => {
    expect(formatAmount('1234567.891')).toBe('1,234,567.89')
    expect(formatAmount('123456789.5')).toBe('123,456,789.50')
  })

  it('detail: 负号前置到币种符号之前，金额本体取绝对值', () => {
    expect(formatMoney('-1234.5', 'CNY')).toBe('-¥ 1,234.50')
    expect(formatMoney('1234.5', 'USD', { signed: true })).toBe('+$ 1,234.50')
    expect(formatMoney('-1234.5', 'XYZ')).toBe('-1,234.50 XYZ')
  })

  it('detail: 展示为零时不显示任何符号', () => {
    expect(formatMoney('-0.001', 'CNY')).toBe('¥ 0.00')
    expect(formatMoney('0.001', 'CNY', { signed: true })).toBe('¥ 0.00')
  })

  it('detail: 构造只接受普通十进制字符串', () => {
    expect(() => Money.from('1e21')).toThrow(RangeError)
    expect(() => Money.from('abc')).toThrow(RangeError)
    expect(Money.from(' 12.30 ').toString()).toBe('12.30')
  })

  it('detail: 不可变运算', () => {
    const a = Money.from('0.10')
    const b = Money.from('0.20')
    expect(a.plus(b).toString()).toBe('0.30')
    expect(a.minus(b).toString()).toBe('-0.10')
    expect(a.negated().toString()).toBe('-0.10')
    expect(a.abs().toString()).toBe('0.10')
    expect(a.isZero()).toBe(false)
    expect(Money.zero().isZero()).toBe(true)
  })
})
