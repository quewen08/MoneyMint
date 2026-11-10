/**
 * 会话状态（Pinia，批 1）
 *
 * D-7：会话凭据只由 httpOnly Cookie 承担，本 store **不保存 token**。
 * 刷新页面后通过 `/api/auth/me` 恢复登录态（AC-32），再拉取账本清单并校验
 * 持久化的账本 id 是否仍在成员列表内，不在则回退首个账本（E-08）。
 */
import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { ApiError } from '@/api/errors'
import { authApi, ledgerApi, setLedgerIdProvider } from '@/api/client'
import type { ChangePasswordRequest, LedgerView, UserInfo } from '@/api/types'
import { readLedgerId, writeLedgerId } from '@/core/storage'

export type SessionStatus = 'unknown' | 'loading' | 'authenticated' | 'guest'

export const useSessionStore = defineStore('session', () => {
  const status = ref<SessionStatus>('unknown')
  const user = ref<UserInfo | null>(null)
  const ledgers = ref<LedgerView[]>([])
  const ledgerId = ref<number | null>(readLedgerId())
  const errorMessage = ref('')

  // API 客户端每次请求都从 store 读取账本 id，保证单一真相源（E-08）。
  setLedgerIdProvider(() => ledgerId.value)

  const isAuthenticated = computed(
    () => status.value === 'authenticated' && user.value !== null,
  )
  const mustChangePassword = computed(
    () => user.value !== null && user.value.must_change_password === true,
  )
  const needsInvite = computed(
    () => isAuthenticated.value && ledgers.value.length === 0,
  )
  const isViewer = computed(() => user.value !== null && user.value.role === 'viewer')
  const ledgerName = computed(() => {
    if (ledgerId.value === null) {
      return '未选择账本'
    }
    for (const ledger of ledgers.value) {
      if (ledger.id === ledgerId.value) {
        return ledger.name
      }
    }
    return '未选择账本'
  })
  const defaultCommodity = computed(() => {
    for (const ledger of ledgers.value) {
      if (ledger.id === ledgerId.value && ledger.default_commodity) {
        return ledger.default_commodity
      }
    }
    return 'CNY'
  })

  function setUser(next: UserInfo | null): void {
    user.value = next
  }

  /** 清空本地会话态（不触碰 Cookie，登出接口负责清 Cookie）。 */
  function markGuest(): void {
    status.value = 'guest'
    user.value = null
    ledgers.value = []
    ledgerId.value = null
    errorMessage.value = ''
    writeLedgerId(null)
  }

  /**
   * 拉取账本清单并校验持久化账本 id。
   * 428（强制改密态）/ 403 / 5xx 一律保守视为「暂无账本」，保持登录态，
   * 由路由守卫优先引导到改密页；只有 401 才是真正的会话失效。
   */
  async function refreshLedgers(): Promise<void> {
    try {
      const res = await ledgerApi.list()
      ledgers.value = Array.isArray(res.ledgers) ? res.ledgers : []
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        throw err
      }
      ledgers.value = []
      return
    }
    const ids: number[] = []
    for (const ledger of ledgers.value) {
      ids.push(ledger.id)
    }
    const persisted = ledgerId.value
    if (persisted !== null && ids.indexOf(persisted) >= 0) {
      return
    }
    const next = ledgers.value.length > 0 ? ledgers.value[0].id : null
    ledgerId.value = next
    writeLedgerId(next)
  }

  async function runBootstrap(): Promise<boolean> {
    status.value = 'loading'
    errorMessage.value = ''
    try {
      const me = await authApi.me()
      user.value = me.user
      await refreshLedgers()
      status.value = 'authenticated'
      return true
    } catch (err) {
      user.value = null
      ledgers.value = []
      status.value = 'guest'
      errorMessage.value =
        err instanceof ApiError ? err.message : '无法恢复登录状态，请重新登录'
      return false
    }
  }

  let bootstrapTask: Promise<boolean> | null = null

  /** 恢复登录态；并发调用复用同一任务，避免重复请求。 */
  function bootstrap(): Promise<boolean> {
    if (bootstrapTask === null) {
      const task = runBootstrap()
      bootstrapTask = task.then(
        (ok) => {
          bootstrapTask = null
          return ok
        },
        (err) => {
          bootstrapTask = null
          throw err
        },
      )
    }
    return bootstrapTask
  }

  /** 用户名密码登录；成功后同步用户与账本清单。 */
  async function login(username: string, password: string): Promise<void> {
    errorMessage.value = ''
    const result = await authApi.login({ username, password })
    user.value = result.user
    status.value = 'authenticated'
    await refreshLedgers()
  }

  /** 登出：调用后端清 Cookie，无论成败都清空本地态。 */
  async function logout(): Promise<void> {
    try {
      await authApi.logout()
    } catch {
      // 网络失败也要清本地态，避免界面停留在已登录态。
    }
    markGuest()
  }

  /** 修改密码；成功后清除强制改密标记并重新同步会话。 */
  async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
    const body: ChangePasswordRequest = {
      old_password: oldPassword,
      new_password: newPassword,
    }
    await authApi.changePassword(body)
    if (user.value !== null) {
      user.value = {
        ...user.value,
        must_change_password: false,
      }
    }
    await bootstrap()
  }

  /** 切换账本；批 1 的缓存（首页数据 / 草稿）由各视图按 ledgerId 重新加载。 */
  function selectLedger(id: number): void {
    let found = false
    for (const ledger of ledgers.value) {
      if (ledger.id === id) {
        found = true
      }
    }
    if (!found) {
      return
    }
    ledgerId.value = id
    writeLedgerId(id)
  }

  return {
    status,
    user,
    ledgers,
    ledgerId,
    errorMessage,
    isAuthenticated,
    mustChangePassword,
    needsInvite,
    isViewer,
    ledgerName,
    defaultCommodity,
    setUser,
    markGuest,
    bootstrap,
    refreshLedgers,
    login,
    logout,
    changePassword,
    selectLedger,
  }
})
