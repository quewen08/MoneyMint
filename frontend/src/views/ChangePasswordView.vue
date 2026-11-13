<script setup lang="ts">
/**
 * 强制改密落点（428 / P-16 复用同一表单）
 *
 * 后端 `authMiddleware` 在 must_change_password 态对业务端点返回 428，
 * API 客户端统一跳到这里；改密成功后 Cookie 换发、标记清零并回首页。
 */
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { ApiError } from '@/api/errors'
import PasswordInput from '@/components/PasswordInput.vue'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const busy = ref(false)
const error = ref('')

async function submit(): Promise<void> {
  if (busy.value) {
    return
  }
  error.value = ''
  if (oldPassword.value === '') {
    error.value = '请输入当前密码'
    return
  }
  if (newPassword.value.length < 6) {
    error.value = '新密码至少 6 位'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = '两次输入的新密码不一致'
    return
  }
  busy.value = true
  try {
    await session.changePassword(oldPassword.value, newPassword.value)
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    await router.replace('/')
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : '修改失败，请稍后重试'
  } finally {
    busy.value = false
  }
}

async function logout(): Promise<void> {
  await session.logout()
  await router.replace({ name: 'login' })
}
</script>

<template>
  <div class="pwd">
    <div class="pwd__hero fb-hero">
      <h1>修改密码</h1>
      <p>保存后其他设备将自动退出，需重新登录</p>
    </div>

    <form class="fb-card pwd__card" @submit.prevent="submit">
      <p v-if="session.mustChangePassword" class="pwd__banner">
        首次登录或密码被重置，请先修改密码后才能使用账本功能。
      </p>

      <label class="pwd__field">
        <span>当前密码</span>
        <PasswordInput
          v-model="oldPassword"
          autocomplete="current-password"
          placeholder="••••••••"
        />
      </label>
      <label class="pwd__field">
        <span>新密码（至少 6 位）</span>
        <PasswordInput
          v-model="newPassword"
          autocomplete="new-password"
          placeholder="••••••••"
        />
      </label>
      <label class="pwd__field">
        <span>确认新密码</span>
        <PasswordInput
          v-model="confirmPassword"
          autocomplete="new-password"
          placeholder="再输一遍"
        />
      </label>

      <p v-if="error" class="pwd__error" role="alert">{{ error }}</p>

      <button class="fb-btn-primary pwd__submit" type="submit" :disabled="busy">
        {{ busy ? '保存中…' : '保存新密码' }}
      </button>
      <button class="pwd__logout" type="button" @click="logout">退出登录</button>

      <p class="pwd__note">密码用于登录与账本数据确认，请勿与其他平台重复</p>
    </form>
  </div>
</template>

<style scoped>
.pwd {
  min-height: 100vh;
  background: var(--fb-bg);
}

.pwd__hero {
  padding: calc(80px + env(safe-area-inset-top, 0px)) 22px 56px;
  text-align: center;
  border-radius: 0 0 30px 30px;
}

.pwd__hero h1 {
  margin: 0;
  font-size: 21px;
  font-weight: 700;
}

.pwd__hero p {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--fb-on-hero-muted);
}

.pwd__card {
  max-width: 420px;
  margin: -26px auto 0;
  padding: 20px 18px;
  position: relative;
}

.pwd__banner {
  margin: 0 0 14px;
  padding: 10px 12px;
  border-radius: var(--fb-radius-input);
  background: var(--fb-warn-soft);
  color: var(--fb-warn);
  font-size: 12px;
  line-height: 1.6;
}

.pwd__field {
  display: block;
  margin-bottom: 14px;
}

.pwd__field > span {
  display: block;
  font-size: 12px;
  color: var(--fb-sub);
  margin-bottom: 6px;
}

.pwd__submit {
  width: 100%;
  height: 48px;
  font-size: 16px;
}

.pwd__error {
  margin: 0 0 12px;
  padding: 8px 12px;
  border-radius: var(--fb-radius-input);
  background: var(--fb-danger-soft);
  color: var(--fb-danger);
  font-size: 13px;
}

.pwd__logout {
  display: block;
  width: 100%;
  margin: 12px 0 0;
  border: none;
  background: transparent;
  color: var(--fb-sub);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
}

.pwd__note {
  margin: 14px 0 0;
  text-align: center;
  font-size: 11px;
  line-height: 1.7;
  color: var(--fb-sub);
}
</style>
