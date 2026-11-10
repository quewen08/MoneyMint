<script setup lang="ts">
/**
 * P-01 登录（沿用 mobile-redesign-v2.html ⑦ 登录屏）
 *
 * D-7：登录成功由后端下发 httpOnly Cookie，前端不保存 token；
 * 登录后按「强制改密 / 等待邀请 / 正常」三态分流。
 */
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/errors'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')

async function redirectAfterLogin(): Promise<void> {
  if (session.mustChangePassword) {
    await router.replace({ name: 'change-password' })
    return
  }
  if (session.needsInvite) {
    await router.replace({ name: 'await-invite' })
    return
  }
  const redirect = route.query.redirect
  const target = typeof redirect === 'string' && redirect !== '' ? redirect : '/'
  await router.replace(target)
}

async function submit(): Promise<void> {
  if (busy.value) {
    return
  }
  error.value = ''
  if (username.value.trim() === '' || password.value === '') {
    error.value = '请输入用户名和密码'
    return
  }
  busy.value = true
  try {
    await session.login(username.value.trim(), password.value)
    password.value = ''
    await redirectAfterLogin()
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : '登录失败，请稍后重试'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth">
    <div class="auth__hero fb-hero">
      <div class="auth__logo">🫘</div>
      <h1>FamBean</h1>
      <p>家庭记账 · 每一笔都清清楚楚</p>
    </div>

    <form class="fb-card auth__card" @submit.prevent="submit">
      <label class="auth__field">
        <span>用户名</span>
        <input
          class="fb-input"
          v-model="username"
          type="text"
          autocomplete="username"
          placeholder="用户名"
        />
      </label>
      <label class="auth__field">
        <span>密码</span>
        <input
          class="fb-input"
          v-model="password"
          type="password"
          autocomplete="current-password"
          placeholder="••••••••"
        />
      </label>

      <p v-if="error" class="auth__error" role="alert">{{ error }}</p>

      <button class="fb-btn-primary auth__submit" type="submit" :disabled="busy">
        {{ busy ? '登录中…' : '登 录' }}
      </button>

      <p class="auth__note">
        忘记密码？请联系管理员（owner）在成员管理中重置。<br />
        新账号需由 owner 邀请加入账本后才能记账。
      </p>
    </form>
  </div>
</template>

<style scoped>
.auth {
  min-height: 100vh;
  background: var(--fb-bg);
  padding-bottom: env(safe-area-inset-bottom, 0px);
}

.auth__hero {
  padding: calc(80px + env(safe-area-inset-top, 0px)) 22px 56px;
  text-align: center;
  border-radius: 0 0 30px 30px;
}

.auth__logo {
  font-size: 46px;
  line-height: 1;
}

.auth__hero h1 {
  margin: 10px 0 0;
  font-size: 21px;
  font-weight: 700;
}

.auth__hero p {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--fb-on-hero-muted);
}

.auth__card {
  max-width: 420px;
  margin: -26px auto 0;
  padding: 20px 18px;
  position: relative;
}

.auth__field {
  display: block;
  margin-bottom: 14px;
}

.auth__field > span {
  display: block;
  font-size: 12px;
  color: var(--fb-sub);
  margin-bottom: 6px;
}

.auth__submit {
  width: 100%;
  height: 48px;
  font-size: 16px;
}

.auth__error {
  margin: 0 0 12px;
  padding: 8px 12px;
  border-radius: var(--fb-radius-input);
  background: var(--fb-danger-soft);
  color: var(--fb-danger);
  font-size: 13px;
}

.auth__note {
  margin: 16px 0 0;
  text-align: center;
  font-size: 11px;
  line-height: 1.7;
  color: var(--fb-sub);
}
</style>
