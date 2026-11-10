/**
 * 本地存储安全封装（批 1）
 *
 * D-7：会话凭据只走 httpOnly Cookie，**禁止**把 token 写入 localStorage。
 * 本模块只持久化账本 id / 记账草稿 / 隐私开关等非凭据数据，并在启动时
 * 清理旧版 Flutter 前端遗留的 `fl_token`（§13 D-7 实施清单 #7）。
 *
 * 所有访问都带 typeof 守卫与 try/catch，保证在 Node（Vitest）环境、
 * Safari 隐私模式、配额耗尽等场景下静默降级而不抛错。
 */

/** 旧版 Flutter 前端把 token 存在这个键里，Cookie 化后必须清理。 */
const LEGACY_TOKEN_KEY = 'fl_token'
/** 当前账本 id（仅数字，不含任何凭据）。 */
export const LEDGER_ID_KEY = 'fambean.ledger_id'
/** 金额隐私遮挡开关。 */
export const PRIVACY_KEY = 'fambean.privacy'
/** 记账草稿键前缀，按账本命名空间隔离。 */
export const DRAFT_KEY_PREFIX = 'fambean.record_draft.'

function getStorage(): Storage | null {
  try {
    if (typeof localStorage === 'undefined') {
      return null
    }
    return localStorage
  } catch {
    return null
  }
}

/** 读取字符串键；不存在或不可用时返回 null。 */
export function readString(key: string): string | null {
  const store = getStorage()
  if (store === null) {
    return null
  }
  try {
    return store.getItem(key)
  } catch {
    return null
  }
}

/** 写入字符串键；失败时静默降级（草稿丢失不影响在线记账主流程）。 */
export function writeString(key: string, value: string): void {
  const store = getStorage()
  if (store === null) {
    return
  }
  try {
    store.setItem(key, value)
  } catch {
    // 隐私模式 / 配额耗尽：静默降级。
  }
}

/** 删除指定键。 */
export function removeKey(key: string): void {
  const store = getStorage()
  if (store === null) {
    return
  }
  try {
    store.removeItem(key)
  } catch {
    // 静默降级。
  }
}

/** 清理旧版 localStorage token，Cookie 化后不再需要。 */
export function clearLegacyToken(): void {
  removeKey(LEGACY_TOKEN_KEY)
}

/** 读取持久化的账本 id；非法值一律视为未选择（绝不回退默认账本 id=1）。 */
export function readLedgerId(): number | null {
  const raw = readString(LEDGER_ID_KEY)
  if (raw === null || raw === '' || !/^\d+$/.test(raw)) {
    return null
  }
  const id = Number(raw)
  return Number.isSafeInteger(id) && id > 0 ? id : null
}

/** 持久化账本 id；null 表示清除。 */
export function writeLedgerId(id: number | null): void {
  if (id === null) {
    removeKey(LEDGER_ID_KEY)
  } else {
    writeString(LEDGER_ID_KEY, String(id))
  }
}

/** 读取金额隐私开关。 */
export function readPrivacy(): boolean {
  return readString(PRIVACY_KEY) === '1'
}

/** 持久化金额隐私开关。 */
export function writePrivacy(enabled: boolean): void {
  writeString(PRIVACY_KEY, enabled ? '1' : '0')
}

/** 按账本命名空间生成草稿键。 */
export function draftKey(ledgerId: number): string {
  return `${DRAFT_KEY_PREFIX}${ledgerId}`
}

/** 读取指定账本的草稿；解析失败或不存在返回 null。 */
export function readDraft<T>(ledgerId: number): T | null {
  const raw = readString(draftKey(ledgerId))
  if (raw === null || raw === '') {
    return null
  }
  try {
    return JSON.parse(raw) as T
  } catch {
    return null
  }
}

/** 写入指定账本的草稿（不含任何凭据）。 */
export function writeDraft<T>(ledgerId: number, draft: T): void {
  writeString(draftKey(ledgerId), JSON.stringify(draft))
}

/** 提交成功后清除草稿。 */
export function clearDraft(ledgerId: number): void {
  removeKey(draftKey(ledgerId))
}
