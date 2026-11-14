/**
 * 期初余额表单校验（拍板口径）
 *
 * - 金额按「绝对值 + 方向」录入：只接受非负普通十进制，方向固定「持有 / 欠款」；
 * - FR-03：账户开户日不得晚于账本期初日期（前端同样校验）；
 * - 仅 Assets / Liabilities 可设期初（FR-04）；
 * - 落库符号由方向决定：持有 = 正、欠款 = 负（FR-05），前端只上送绝对值与方向。
 */
import { Money } from './Money'

export type OpeningDirection = 'hold' | 'owe'

export interface OpeningFormInput {
  amount: string
  direction: OpeningDirection
  accountType: string
  accountOpenDate: string
  ledgerOpeningDate: string
}

export interface OpeningValidation {
  ok: boolean
  errors: string[]
  /** 校验通过时的非负绝对值（两位定点字符串）；失败为 null。 */
  absAmount: string | null
}

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/

/** 按方向映射落库符号：持有 = 正、欠款 = 负（FR-05，展示用预览）。 */
export function openingSignedAmount(absAmount: string, direction: OpeningDirection): string {
  const value = Money.from(absAmount)
  return direction === 'owe' ? value.negated().toString() : value.toString()
}

/** 期初表单整体校验；全部通过时 `ok=true` 且给出两位定点绝对值。 */
export function validateOpeningForm(input: OpeningFormInput): OpeningValidation {
  const errors: string[] = []
  let absAmount: string | null = null

  if (input.accountType !== 'Assets' && input.accountType !== 'Liabilities') {
    errors.push('仅资产 / 负债账户可设置期初余额')
  }
  if (!DATE_RE.test(input.ledgerOpeningDate)) {
    errors.push('账本期初日期格式不合法')
  }
  if (!DATE_RE.test(input.accountOpenDate)) {
    errors.push('账户开户日格式不合法')
  } else if (
    DATE_RE.test(input.ledgerOpeningDate) &&
    input.accountOpenDate > input.ledgerOpeningDate
  ) {
    errors.push(
      `账户开户日（${input.accountOpenDate}）不得晚于账本期初日期（${input.ledgerOpeningDate}）`,
    )
  }

  const raw = input.amount.trim()
  if (raw === '') {
    errors.push('请输入期初金额')
  } else {
    try {
      const value = Money.from(raw)
      if (value.isNegative()) {
        errors.push('期初金额必须为非负数，方向请用「持有 / 欠款」表达')
      } else {
        absAmount = value.toString()
      }
    } catch {
      errors.push('期初金额格式不正确（只接受普通十进制数字）')
    }
  }

  if (input.direction !== 'hold' && input.direction !== 'owe') {
    errors.push('期初方向必须是「持有」或「欠款」')
  }

  return { ok: errors.length === 0, errors, absAmount }
}
