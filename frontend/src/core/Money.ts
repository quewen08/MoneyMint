/**
 * 金额封装（E-01）
 *
 * 口径与旧版 `frontend/lib/utils/money.dart` 对齐：
 * - 金额一律为定点十进制字符串，禁止 JS 浮点参与运算或存储；
 * - 内部 Decimal 实例不对外暴露，构造只接受普通十进制字符串（不接受 1e21 之类指数记数法）；
 * - `toString()` 固定两位小数，绝不输出指数记数法；
 * - 展示层负号前置到币种符号之前，金额本体始终取绝对值，避免 `-¥ -1,234.50`。
 */
import Decimal from 'decimal.js'

// 全局精度与舍入：28 位有效数字 + 四舍五入（与旧版 package:decimal 展示一致）。
Decimal.set({ precision: 28, rounding: Decimal.ROUND_HALF_UP })

/** 普通十进制字符串（允许可选正负号，不接受指数记数法）。 */
const PLAIN_DECIMAL_RE = /^[+-]?\d+(?:\.\d+)?$/

/** 常见币种的显示符号；未知币种返回空串（直接显示代码）。 */
export function currencySymbol(commodity: string): string {
  switch (commodity) {
    case 'CNY':
      return '¥'
    case 'USD':
      return '$'
    case 'EUR':
      return '€'
    case 'JPY':
      return '¥'
    case 'GBP':
      return '£'
    case 'HKD':
      return 'HK$'
    default:
      return ''
  }
}

/** 千分位分组（只接受非负整数部分）。 */
function groupThousands(digits: string): string {
  let out = ''
  let count = 0
  for (let i = digits.length - 1; i >= 0; i -= 1) {
    if (count > 0 && count % 3 === 0) {
      out = `,${out}`
    }
    out = `${digits[i]}${out}`
    count += 1
  }
  return out
}

/** 把非负 Decimal 格成「千分位 + 两位小数」。 */
function formatMagnitude(value: Decimal): string {
  const [intPart, decPart] = value.abs().toFixed(2).split('.')
  return `${groupThousands(intPart)}.${decPart}`
}

/** 宽松解析：无法解析的输入按 0 处理（与旧版 formatAmount 行为一致）。 */
function parseOrZero(amount: string): Decimal {
  const trimmed = amount.trim()
  if (!PLAIN_DECIMAL_RE.test(trimmed)) {
    return new Decimal(0)
  }
  return new Decimal(trimmed)
}

/** 把定点十进制字符串格式化为带千分位、两位小数的展示串（保留符号）。 */
export function formatAmount(amount: string): string {
  const value = parseOrZero(amount)
  const magnitude = formatMagnitude(value)
  const negative = value.isNeg() && magnitude !== '0.00'
  return `${negative ? '-' : ''}${magnitude}`
}

export interface FormatMoneyOptions {
  /** 正数是否显式加 `+`（零不显示符号）。 */
  signed?: boolean
}

/**
 * 完整展示：符号 + 金额（+ 可选币种代码）。
 * 已知币种如 `-¥ 1,234.50`；未知币种如 `-1,234.50 XYZ`。
 */
export function formatMoney(
  amount: string,
  commodity: string,
  options: FormatMoneyOptions = {},
): string {
  return formatMoneyValue(parseOrZero(amount), commodity, options)
}

/** 已解析 Decimal 的格式化实现（避免大数经 toString() 变成指数记数法）。 */
function formatMoneyValue(
  value: Decimal,
  commodity: string,
  options: FormatMoneyOptions = {},
): string {
  const symbol = currencySymbol(commodity)
  const magnitude = formatMagnitude(value)
  const body = symbol !== '' ? `${symbol} ${magnitude}` : magnitude

  const isDisplayZero = magnitude === '0.00'
  const isNegative = value.isNeg() && !isDisplayZero
  const isPositive = value.isPos() && !isDisplayZero
  const sign = isNegative ? '-' : options.signed && isPositive ? '+' : ''
  const result = `${sign}${body}`

  if (symbol === '' && commodity !== '') {
    return `${result} ${commodity}`
  }
  return result
}

/**
 * 金额值对象。
 *
 * 构造只接受 string（`Money.from`），内部 Decimal 私有，避免外部绕过定点规则。
 */
export class Money {
  private readonly amount: Decimal

  private constructor(amount: Decimal) {
    this.amount = amount
  }

  /** 从普通十进制字符串构造；非法或指数记数法直接抛错。 */
  static from(input: string): Money {
    const trimmed = input.trim()
    if (!PLAIN_DECIMAL_RE.test(trimmed)) {
      throw new RangeError(`非法金额字符串（只接受普通十进制）: ${input}`)
    }
    return new Money(new Decimal(trimmed))
  }

  /** 零值。 */
  static zero(): Money {
    return new Money(new Decimal(0))
  }

  /** 固定两位小数，永不输出指数记数法；负零归一为 0.00。 */
  toString(): string {
    const fixed = this.amount.toFixed(2)
    return fixed === '-0.00' ? '0.00' : fixed
  }

  /** 绝对值。 */
  abs(): Money {
    return new Money(this.amount.abs())
  }

  /** 取反。 */
  negated(): Money {
    return new Money(this.amount.negated())
  }

  /** 相加（另一侧必须是 Money，禁止裸字符串混算）。 */
  plus(other: Money): Money {
    return new Money(this.amount.plus(other.amount))
  }

  /** 相减。 */
  minus(other: Money): Money {
    return new Money(this.amount.minus(other.amount))
  }

  /** 是否为 0（按原始值判断，不做展示舍入）。 */
  isZero(): boolean {
    return this.amount.isZero()
  }

  /** 是否严格小于 0（不做展示舍入）。 */
  isNegative(): boolean {
    return this.amount.isNeg()
  }

  /** 按两位小数展示判断是否为零（-0.001 视为 0.00）。 */
  isDisplayZero(): boolean {
    return this.amount.abs().toFixed(2) === '0.00'
  }

  /** 按两位小数展示的金额（千分位、保留符号）。 */
  formatAmount(): string {
    const magnitude = formatMagnitude(this.amount)
    const negative = this.amount.isNeg() && magnitude !== '0.00'
    return `${negative ? '-' : ''}${magnitude}`
  }

  /** 按两位小数展示的完整金额（符号前置、金额取绝对值）。 */
  formatMoney(commodity: string, options: FormatMoneyOptions = {}): string {
    return formatMoneyValue(this.amount, commodity, options)
  }
}
