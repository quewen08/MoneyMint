<script setup lang="ts">
/**
 * P-14 设置（批 3）
 *
 * 与旧版 settings_screen.dart 1:1：身份卡（系统管理员 / 账本角色两个分层徽章）、
 * 默认账户卡、设置行（账本 / 成员 / 登录设备 / 改密 / 数据安全 / 报表 / 同步状态）、
 * 开放注册开关（仅 isAdmin 渲染）、退出登录。
 * §14.5 #4/#5 不降级项落在本页：注册开关、默认账户查看与清除。
 */
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { accountApi, settingsApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { AccountView } from '@/api/types'
import { roleLabel } from '@/core/batch3'
import {
  DEFAULT_ACCOUNT_KINDS,
  DEFAULT_ACCOUNT_LABELS,
  clearDefaultAccounts,
  readDefaultAccounts,
  type DefaultAccountKind,
} from '@/core/storage'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()

const registrationEnabled = ref<boolean | null>(null)
const registrationBusy = ref(false)
const actionError = ref('')
const notice = ref('')
const accounts = ref<AccountView[]>([])
const defaults = ref<Partial<Record<DefaultAccountKind, string>>>({})
const accountsLoaded = ref(false)

const isAdmin = computed(() => session.user !== null && session.user.is_admin === true)
const currentRole = computed(() => {
  for (const ledger of session.ledgers) {
    if (ledger.id === session.ledgerId) {
      return ledger.role
    }
  }
  return session.user === null ? '' : session.user.role
})

interface LinkItem {
  key: string
  label: string
  hint: string
  to: string
  disabled?: boolean
}

const links = computed<LinkItem[]>(() => [
  { key: 'ledgers', label: '账本管理', hint: `${session.ledgerName}（当前）`, to: '/ledgers' },
  {
    key: 'members',
    label: '成员管理',
    hint: currentRole.value === 'owner' ? '所有者' : '仅 owner',
    to: '/members',
    disabled: currentRole.value !== 'owner',
  },
  { key: 'sessions', label: '登录设备', hint: '查看 / 吊销', to: '/sessions' },
  { key: 'password', label: '修改密码', hint: '', to: '/change-password' },
  { key: 'data', label: '数据安全', hint: '备份 / 导出', to: '/data-safety' },
  { key: 'reports', label: '报表分析', hint: '趋势 / 占比', to: '/reports' },
  { key: 'sync', label: '同步状态', hint: '服务端水位', to: '/sync' },
])

function go(item: LinkItem): void {
  if (item.disabled === true) {
    notice.value = '仅账本所有者（owner）可以管理成员'
    return
  }
  void router.push(item.to)
}

async function loadRegistration(): Promise<void> {
  if (!isAdmin.value) {
    return
  }
  try {
    const res = await settingsApi.registration()
    registrationEnabled.value = res.enabled === true
  } catch {
    // GET 是公开端点，查询失败按「开放」处理，避免误锁注册入口（与旧版一致）。
    registrationEnabled.value = true
  }
}

async function toggleRegistration(): Promise<void> {
  if (registrationBusy.value || registrationEnabled.value === null) {
    return
  }
  registrationBusy.value = true
  actionError.value = ''
  const next = !registrationEnabled.value
  try {
    const res = await settingsApi.setRegistration({ enabled: next })
    registrationEnabled.value = res.enabled === true
    notice.value = next ? '已开放注册' : '已关闭注册'
  } catch (err) {
    actionError.value =
      err instanceof ApiError ? `切换失败（需系统管理员）：${err.message}` : '切换失败，请稍后重试'
  } finally {
    registrationBusy.value = false
  }
}

async function loadDefaults(): Promise<void> {
  const id = session.ledgerId
  if (id === null) {
    return
  }
  defaults.value = readDefaultAccounts(id)
  try {
    const res = await accountApi.list()
    accounts.value = Array.isArray(res.accounts) ? res.accounts : []
  } catch {
    accounts.value = []
  }
  accountsLoaded.value = true
}

function defaultLabel(kind: DefaultAccountKind): string {
  const uuid = defaults.value[kind]
  if (uuid === undefined || uuid === '') {
    return '未设置'
  }
  for (const account of accounts.value) {
    if (account.uuid === uuid) {
      return account.display_name || account.name
    }
  }
  return '（已删除）'
}

function clearDefaults(): void {
  const id = session.ledgerId
  if (id === null) {
    return
  }
  clearDefaultAccounts(id)
  defaults.value = {}
  notice.value = '已清除全部默认账户'
}

async function logout(): Promise<void> {
  await session.logout()
  await router.replace({ name: 'login' })
}

watch(() => session.ledgerId, () => { void loadDefaults() })
onMounted(() => {
  void loadRegistration()
  void loadDefaults()
})
</script>

<template>
  <section class="set">
    <header class="set__hero fb-hero">
      <span class="set__title">设置</span>
      <span class="set__sub">账号、账本与数据</span>
    </header>

    <div class="fb-card set__identity">
      <span class="set__avatar">
        {{ session.user ? (session.user.display_name || session.user.username).charAt(0) : '?' }}
      </span>
      <span class="set__who">
        <strong>{{ session.user ? session.user.display_name || session.user.username : '未登录' }}</strong>
        <small>@{{ session.user ? session.user.username : '—' }}</small>
      </span>
      <span class="set__badges">
        <em v-if="isAdmin" class="set__badge set__badge--admin">系统管理员</em>
        <em class="set__badge" :class="{ 'set__badge--owner': currentRole === 'owner' }">
          账本{{ roleLabel(currentRole) }}
        </em>
      </span>
    </div>

    <p v-if="notice" class="set__notice set__notice--ok" role="status">{{ notice }}</p>
    <p v-if="actionError" class="set__notice set__notice--err" role="alert">{{ actionError }}</p>

    <div class="fb-card set__card">
      <div class="set__card-head">
        <h2>默认账户</h2>
        <button
          v-if="Object.keys(defaults).length > 0"
          class="fb-btn-ghost set__btn set__btn--danger"
          type="button"
          @click="clearDefaults"
        >清除全部默认</button>
      </div>
      <p v-if="!accountsLoaded" class="set__state">正在读取默认账户…</p>
      <dl v-else class="set__defaults">
        <template v-for="kind in DEFAULT_ACCOUNT_KINDS" :key="kind">
          <dt>{{ DEFAULT_ACCOUNT_LABELS[kind] }}</dt>
          <dd :class="{ 'set__missing': defaultLabel(kind) === '（已删除）' }">{{ defaultLabel(kind) }}</dd>
        </template>
      </dl>
      <p class="set__hint">记一笔会记住上次选择；账户被删除或关户后自动回退为未选。</p>
    </div>

    <div class="fb-card set__card set__links">
      <button
        v-for="item in links"
        :key="item.key"
        type="button"
        class="set__link"
        :disabled="item.disabled"
        @click="go(item)"
      >
        <span>{{ item.label }}</span>
        <span class="fb-muted">{{ item.hint }} ›</span>
      </button>
    </div>

    <div v-if="isAdmin" class="fb-card set__card set__registration">
      <div>
        <strong>开放注册</strong>
        <p class="fb-muted">关闭后登录页不显示注册入口；仅系统管理员可切换。</p>
      </div>
      <button
        class="set__switch"
        type="button"
        role="switch"
        :aria-checked="registrationEnabled === true"
        :disabled="registrationEnabled === null || registrationBusy"
        @click="toggleRegistration"
      >
        <i :class="{ on: registrationEnabled === true }" />
        <span>{{ registrationBusy ? '切换中…' : registrationEnabled === null ? '…' : registrationEnabled ? '开放' : '关闭' }}</span>
      </button>
    </div>

    <div class="fb-card set__card">
      <button class="set__link set__link--danger" type="button" @click="logout">
        <span>退出登录</span><span>›</span>
      </button>
    </div>
  </section>
</template>

<style scoped>
.set { padding-bottom: 20px; }
.set__hero { padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px; border-radius: 0 0 24px 24px; }
.set__title { display: block; font-size: 16px; font-weight: 700; }
.set__sub { display: block; margin-top: 4px; font-size: 12px; color: var(--fb-on-hero-muted); }
.set__identity { margin-top: 14px; padding: 14px 16px; display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.set__avatar { width: 40px; height: 40px; border-radius: 50%; background: var(--fb-blue-soft); color: var(--fb-blue); display: flex; align-items: center; justify-content: center; font-size: 18px; font-weight: 700; }
.set__who { flex: 1; min-width: 120px; }
.set__who strong { display: block; font-size: 14px; }
.set__who small { font-size: 11px; color: var(--fb-sub); }
.set__badges { display: flex; gap: 6px; flex-wrap: wrap; }
.set__badge { padding: 3px 9px; border-radius: var(--fb-radius-pill); background: var(--fb-soft); color: var(--fb-sub); font-size: 11px; font-style: normal; }
.set__badge--admin { background: var(--fb-blue-soft); color: var(--fb-blue); }
.set__badge--owner { background: var(--fb-green-soft); color: var(--fb-green); }
.set__notice { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; }
.set__notice--ok { background: var(--fb-green-soft); color: var(--fb-green); }
.set__notice--err { background: var(--fb-danger-soft); color: var(--fb-danger); }
.set__card { margin-top: 12px; padding: 12px 16px; }
.set__card-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.set__card h2 { margin: 0 0 8px; font-size: 14px; }
.set__card-head h2 { margin-bottom: 0; }
.set__btn { padding: 5px 10px; font-size: 11px; }
.set__btn--danger { color: var(--fb-danger); }
.set__defaults { display: grid; grid-template-columns: 1fr 1fr; gap: 6px 12px; margin: 10px 0 0; font-size: 12px; }
.set__defaults dt { color: var(--fb-sub); }
.set__defaults dd { margin: 0; text-align: right; }
.set__missing { color: var(--fb-danger); }
.set__hint, .set__registration p { margin: 10px 0 0; font-size: 11px; color: var(--fb-sub); line-height: 1.6; }
.set__state { margin: 10px 0 0; font-size: 12px; color: var(--fb-sub); }
.set__links { padding: 4px 16px; }
.set__link { width: 100%; display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 12px 0; border: none; border-bottom: 1px solid var(--fb-line); background: transparent; color: var(--fb-ink); font-size: 13px; font-family: inherit; text-align: left; cursor: pointer; }
.set__link:last-child { border-bottom: none; }
.set__link:disabled { opacity: 0.5; cursor: default; }
.set__link--danger { color: var(--fb-danger); }
.set__registration { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.set__registration strong { font-size: 13px; }
.set__switch { display: flex; align-items: center; gap: 8px; border: none; background: transparent; color: var(--fb-sub); font-size: 12px; font-family: inherit; cursor: pointer; }
.set__switch i { display: block; width: 34px; height: 20px; border-radius: 10px; background: var(--fb-line); position: relative; }
.set__switch i::after { content: ''; position: absolute; top: 2px; left: 2px; width: 16px; height: 16px; border-radius: 50%; background: var(--fb-white); }
.set__switch i.on { background: var(--fb-blue); }
.set__switch i.on::after { left: 16px; }
</style>
