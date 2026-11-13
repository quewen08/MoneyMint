<script setup lang="ts">
/**
 * P-12 成员管理（批 3，仅账本 owner）
 *
 * 与旧版 members_screen.dart 1:1：邀请（用户名需先注册 + 角色默认 viewer）、
 * 角色翻转（无二次确认）、移除（二次确认）、重置密码（>=6 位，成功后该成员
 * 全部会话失效）。owner 行隐藏所有操作按钮。
 * 权限判定取「当前账本」的角色，不取 `user.role`（那是默认账本角色，
 * 切账本后可能不准）。
 */
import { computed, onMounted, ref, watch } from 'vue'

import { memberApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { LedgerRole, MemberView } from '@/api/types'
import PasswordInput from '@/components/PasswordInput.vue'
import { roleLabel } from '@/core/batch3'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const members = ref<MemberView[]>([])
const loading = ref(false)
const loadError = ref('')
const busy = ref(false)
const notice = ref('')
const actionError = ref('')

const inviteOpen = ref(false)
const inviteUsername = ref('')
const inviteRole = ref<LedgerRole>('viewer')

const resetTarget = ref<MemberView | null>(null)
const resetPassword = ref('')
const removeTarget = ref<MemberView | null>(null)

const currentRole = computed<LedgerRole | ''>(() => {
  for (const ledger of session.ledgers) {
    if (ledger.id === session.ledgerId) {
      return ledger.role
    }
  }
  return session.user === null ? '' : session.user.role
})
const isOwner = computed(() => currentRole.value === 'owner')

function messageOf(err: unknown): string {
  return err instanceof ApiError ? err.message : '操作失败，请稍后重试'
}

async function load(): Promise<void> {
  if (!isOwner.value) {
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const res = await memberApi.list()
    members.value = Array.isArray(res.members) ? res.members : []
  } catch (err) {
    loadError.value = messageOf(err)
  } finally {
    loading.value = false
  }
}

async function run(action: () => Promise<unknown>, success: string): Promise<void> {
  if (busy.value) {
    return
  }
  busy.value = true
  actionError.value = ''
  notice.value = ''
  try {
    await action()
    notice.value = success
    await load()
  } catch (err) {
    actionError.value = messageOf(err)
  } finally {
    busy.value = false
  }
}

function openInvite(): void {
  inviteOpen.value = true
  inviteUsername.value = ''
  inviteRole.value = 'viewer'
  actionError.value = ''
}

function submitInvite(): void {
  const username = inviteUsername.value.trim()
  if (username === '') {
    actionError.value = '请输入用户名（对方需先注册）'
    return
  }
  void run(async () => {
    await memberApi.add({ username, role: inviteRole.value })
    inviteOpen.value = false
  }, `已邀请「${username}」`)
}

function toggleRole(member: MemberView): void {
  const next: LedgerRole = member.role === 'viewer' ? 'editor' : 'viewer'
  void run(
    () => memberApi.updateRole(member.user_id, { role: next }),
    `已将「${member.display_name || member.username}」改为${roleLabel(next)}`,
  )
}

function confirmRemove(): void {
  const target = removeTarget.value
  if (target === null) {
    return
  }
  void run(async () => {
    await memberApi.remove(target.user_id)
    removeTarget.value = null
  }, `已移除「${target.display_name || target.username}」`)
}

function openReset(member: MemberView): void {
  resetTarget.value = member
  resetPassword.value = ''
  actionError.value = ''
}

function submitReset(): void {
  const target = resetTarget.value
  if (target === null) {
    return
  }
  if (resetPassword.value.length < 6) {
    actionError.value = '新密码至少 6 位'
    return
  }
  void run(async () => {
    await memberApi.resetPassword(target.user_id, { new_password: resetPassword.value })
    resetTarget.value = null
    resetPassword.value = ''
  }, `已重置密码，该成员的全部会话已失效`)
}

watch(() => session.ledgerId, () => { void load() })
onMounted(() => { void load() })
</script>

<template>
  <section class="mem">
    <header class="mem__hero fb-hero">
      <span class="mem__title">成员管理</span>
      <span class="mem__sub">成员通过用户名加入，需先注册账号</span>
    </header>

    <div v-if="!isOwner" class="fb-card mem__state">
      仅账本所有者（owner）可以管理成员。
    </div>

    <template v-else>
      <div class="fb-card mem__toolbar">
        <span class="fb-muted">共 {{ members.length }} 位成员</span>
        <button class="fb-btn-primary mem__btn" type="button" :disabled="busy" @click="openInvite">
          邀请成员
        </button>
      </div>

      <p v-if="notice" class="mem__notice mem__notice--ok" role="status">{{ notice }}</p>
      <p v-if="actionError" class="mem__notice mem__notice--err" role="alert">{{ actionError }}</p>

      <div v-if="inviteOpen" class="fb-card mem__panel">
        <h2>邀请成员</h2>
        <label class="mem__field">
          <span>用户名（对方需先注册）</span>
          <input v-model="inviteUsername" class="fb-input" type="text" placeholder="如 alice" />
        </label>
        <label class="mem__field">
          <span>角色</span>
          <select v-model="inviteRole" class="fb-input">
            <option value="viewer">只读：仅查看</option>
            <option value="editor">编辑：可记账改账</option>
          </select>
        </label>
        <div class="mem__actions">
          <button class="fb-btn-ghost" type="button" @click="inviteOpen = false">取消</button>
          <button class="fb-btn-primary" type="button" :disabled="busy" @click="submitInvite">确认邀请</button>
        </div>
      </div>

      <div v-if="resetTarget" class="fb-card mem__panel">
        <h2>重置「{{ resetTarget.display_name || resetTarget.username }}」的密码</h2>
        <p class="fb-muted">成功后该成员的全部登录会话立即失效，需重新登录。</p>
        <label class="mem__field">
          <span>新密码（至少 6 位）</span>
          <PasswordInput v-model="resetPassword" placeholder="至少 6 位" />
        </label>
        <div class="mem__actions">
          <button class="fb-btn-ghost" type="button" @click="resetTarget = null">取消</button>
          <button class="fb-btn-primary" type="button" :disabled="busy" @click="submitReset">确认重置</button>
        </div>
      </div>

      <div v-if="removeTarget" class="fb-card mem__panel mem__panel--danger">
        <h2>移除成员？</h2>
        <p>移除后「{{ removeTarget.display_name || removeTarget.username }}」将无法访问共享账本，其本地数据仍保留。</p>
        <div class="mem__actions">
          <button class="fb-btn-ghost" type="button" @click="removeTarget = null">取消</button>
          <button class="fb-btn-primary" type="button" :disabled="busy" @click="confirmRemove">确认移除</button>
        </div>
      </div>

      <div v-if="loadError" class="fb-card mem__state mem__state--err">
        <p>加载失败：{{ loadError }}</p>
        <button class="fb-btn-ghost" type="button" @click="load">重新加载</button>
      </div>
      <div v-else-if="loading" class="fb-card mem__state">正在加载成员…</div>
      <div v-else-if="members.length === 0" class="fb-card mem__state">还没有成员。</div>
      <div v-else class="fb-card mem__list">
        <div v-for="member in members" :key="member.user_id" class="mem__row">
          <span class="mem__avatar">{{ (member.display_name || member.username).charAt(0) }}</span>
          <span class="mem__name">
            {{ member.display_name || member.username }}
            <small>@{{ member.username }} · {{ roleLabel(member.role) }}</small>
          </span>
          <template v-if="member.role !== 'owner'">
            <button class="fb-btn-ghost mem__btn" type="button" :disabled="busy" @click="toggleRole(member)">
              {{ member.role === 'viewer' ? '设为编辑' : '设为只读' }}
            </button>
            <button class="fb-btn-ghost mem__btn" type="button" :disabled="busy" @click="openReset(member)">重置密码</button>
            <button class="fb-btn-ghost mem__btn mem__btn--danger" type="button" :disabled="busy" @click="removeTarget = member">移除</button>
          </template>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.mem { padding-bottom: 20px; }
.mem__hero { padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px; border-radius: 0 0 24px 24px; }
.mem__title { display: block; font-size: 16px; font-weight: 700; }
.mem__sub { display: block; margin-top: 4px; font-size: 12px; color: var(--fb-on-hero-muted); }
.mem__toolbar { margin-top: 14px; padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 10px; font-size: 12px; }
.mem__btn { padding: 6px 12px; font-size: 12px; }
.mem__btn--danger { color: var(--fb-danger); }
.mem__notice { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; }
.mem__notice--ok { background: var(--fb-green-soft); color: var(--fb-green); }
.mem__notice--err { background: var(--fb-danger-soft); color: var(--fb-danger); }
.mem__panel { margin-top: 12px; padding: 14px 16px; }
.mem__panel h2 { margin: 0 0 10px; font-size: 14px; }
.mem__panel p { margin: 0 0 10px; font-size: 12px; }
.mem__panel--danger { border: 1px solid var(--fb-danger); }
.mem__field { display: block; margin-bottom: 10px; font-size: 12px; color: var(--fb-sub); }
.mem__field span { display: block; margin-bottom: 4px; }
.mem__field .fb-input { width: 100%; }
.mem__actions { display: flex; justify-content: flex-end; gap: 8px; }
.mem__list { margin-top: 12px; padding: 6px 14px; }
.mem__row { display: flex; align-items: center; gap: 10px; padding: 10px 0; border-bottom: 1px solid var(--fb-line); flex-wrap: wrap; }
.mem__row:last-child { border-bottom: none; }
.mem__avatar { width: 32px; height: 32px; border-radius: 50%; background: var(--fb-blue-soft); color: var(--fb-blue); display: flex; align-items: center; justify-content: center; font-weight: 700; }
.mem__name { flex: 1; min-width: 120px; font-size: 13px; }
.mem__name small { display: block; font-size: 11px; color: var(--fb-sub); }
.mem__state { margin-top: 14px; padding: 28px 16px; text-align: center; font-size: 13px; color: var(--fb-sub); }
.mem__state--err { color: var(--fb-danger); }
.mem__state p { margin: 0 0 12px; }
</style>
