<script setup lang="ts">
/**
 * P-01 登录 / 注册（沿用 mobile-redesign-v2.html ⑦ 登录屏）
 *
 * D-7：登录成功由后端下发 httpOnly Cookie，前端不保存 token；
 * 登录后按「强制改密 / 等待邀请 / 正常」三态分流。
 *
 * 批 3 补齐 §14.5 #4：登录页消费 `GET /api/settings/registration`（公开端点）。
 * 关闭注册时不切到注册态并给出说明；查询失败按「开放」处理，避免服务端抖动
 * 误锁注册入口。首个注册用户自动创建默认账本并成为 owner。
 */
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { authApi, settingsApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import PasswordInput from '@/components/PasswordInput.vue'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()
const route = useRoute()

// 项目 base 为 './'，反代到子路径时必须用 BASE_URL 拼图标地址，不能用绝对 /icons。
const baseUrl = import.meta.env.BASE_URL

const username = ref('')
const password = ref('')
const displayName = ref('')
const busy = ref(false)
const error = ref('')
const mode = ref<'login' | 'register'>('login')
/** null = 尚未查询完成，按开放处理。 */
const registrationAllowed = ref<boolean | null>(null)

async function loadRegistration(): Promise<void> {
  try {
    const res = await settingsApi.registration()
    registrationAllowed.value = res.enabled === true
  } catch {
    // 公开端点查询失败按开放处理，与旧版 ledger_api.dart 一致。
    registrationAllowed.value = true
  }
}

function switchMode(): void {
  error.value = ''
  if (mode.value === 'login') {
    if (registrationAllowed.value === false) {
      error.value = '注册已关闭，请联系管理员在设置中开放，或由 owner 邀请加入'
      return
    }
    mode.value = 'register'
  } else {
    mode.value = 'login'
  }
}

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
  const name = username.value.trim()
  if (name === '' || password.value === '') {
    error.value = '请输入用户名和密码'
    return
  }
  if (mode.value === 'register' && password.value.length < 6) {
    error.value = '注册密码至少 6 位'
    return
  }
  busy.value = true
  try {
    if (mode.value === 'register') {
      await authApi.register({
        username: name,
        password: password.value,
        display_name: displayName.value.trim(),
      })
      // 注册接口已下发 Cookie，用 bootstrap 恢复登录态与账本清单。
      await session.bootstrap()
    } else {
      await session.login(name, password.value)
    }
    password.value = ''
    await redirectAfterLogin()
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : '操作失败，请稍后重试'
  } finally {
    busy.value = false
  }
}

onMounted(() => { void loadRegistration() })
</script>

<template>
  <div class="auth">
    <div class="auth__hero fb-hero">
      <img
        class="auth__logo"
        :src="`${baseUrl}icons/Icon-192.png`"
        alt="FamBean"
        width="56"
        height="56"
        loading="eager"
      />
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
      <label v-if="mode === 'register'" class="auth__field">
        <span>显示名（可选）</span>
        <input class="fb-input" v-model="displayName" type="text" placeholder="如 阿文" />
      </label>
      <label class="auth__field">
        <span>密码</span>
        <PasswordInput
          v-model="password"
          :autocomplete="mode === 'register' ? 'new-password' : 'current-password'"
          placeholder="密码"
        />
      </label>

      <p v-if="mode === 'register'" class="auth__hint" :class="{ 'auth__hint--warn': registrationAllowed === false }">
        <template v-if="registrationAllowed === false">
          注册已关闭：请由管理员在「设置 → 开放注册」中开启，或由账本 owner 通过成员管理邀请你加入。
        </template>
        <template v-else>
          首个注册的账号自动成为系统管理员并创建默认账本；之后的账号需管理员开放注册或由 owner 邀请加入账本。
        </template>
      </p>

      <p v-if="error" class="auth__error" role="alert">{{ error }}</p>

      <button class="fb-btn-primary auth__submit" type="submit" :disabled="busy">
        {{ busy ? '处理中…' : mode === 'register' ? '注 册' : '登 录' }}
      </button>

      <button class="auth__switch" type="button" @click="switchMode">
        {{ mode === 'login' ? '没有账号？注册' : '已有账号？返回登录' }}
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

/* 豆子图标：56x56 圆角块，极淡投影让 hero 上的品牌标识有轻微浮起感 */
.auth__logo {
  width: 56px;
  height: 56px;
  display: block;
  margin: 0 auto;
  border-radius: 14px;
  object-fit: cover;
  box-shadow: 0 2px 10px rgba(13, 22, 48, 0.18);
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

.auth__hint {
  margin: 0 0 12px;
  padding: 8px 12px;
  border-radius: var(--fb-radius-input);
  background: var(--fb-blue-soft);
  color: var(--fb-blue);
  font-size: 11px;
  line-height: 1.7;
}

.auth__hint--warn {
  background: var(--fb-danger-soft);
  color: var(--fb-danger);
}

.auth__switch {
  display: block;
  width: 100%;
  margin-top: 12px;
  border: none;
  background: transparent;
  color: var(--fb-blue);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
}

.auth__note {
  margin: 16px 0 0;
  text-align: center;
  font-size: 11px;
  line-height: 1.7;
  color: var(--fb-sub);
}
</style>
