<script setup lang="ts">
/**
 * P-02 待邀请（已登录但尚未加入任何账本）
 *
 * 后端对非账本成员的业务端点返回 403；账本清单为空时由路由守卫落到这里。
 * 提供「重新检查」与「退出登录」，避免用户卡在空白页。
 */
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()
const checking = ref(false)
const message = ref('')

async function refresh(): Promise<void> {
  checking.value = true
  message.value = ''
  try {
    await session.bootstrap()
    if (!session.needsInvite && session.isAuthenticated) {
      await router.replace('/')
      return
    }
    message.value = '仍未收到邀请，请稍后再试'
  } catch {
    message.value = '检查失败，请稍后重试'
  } finally {
    checking.value = false
  }
}

async function logout(): Promise<void> {
  await session.logout()
  await router.replace({ name: 'login' })
}
</script>

<template>
  <div class="invite">
    <div class="invite__hero fb-hero">
      <div class="invite__icon">📨</div>
      <h1>等待账本邀请</h1>
      <p>账号已登录，但还没有加入任何账本</p>
    </div>

    <section class="fb-card invite__card">
      <p class="invite__hello" v-if="session.user">
        你好，{{ session.user.display_name || session.user.username }}
      </p>
      <p class="invite__text">
        请联系账本 owner 在「成员管理」中把你加入账本。加入后点击下方的
        「重新检查」即可进入首页开始记账。
      </p>
      <p v-if="message" class="invite__message" role="status">{{ message }}</p>
      <div class="invite__actions">
        <button class="fb-btn-primary" type="button" :disabled="checking" @click="refresh">
          {{ checking ? '检查中…' : '重新检查' }}
        </button>
        <button class="fb-btn-ghost" type="button" @click="logout">退出登录</button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.invite {
  min-height: 100vh;
  background: var(--fb-bg);
}

.invite__hero {
  padding: calc(80px + env(safe-area-inset-top, 0px)) 22px 56px;
  text-align: center;
  border-radius: 0 0 30px 30px;
}

.invite__icon {
  font-size: 46px;
  line-height: 1;
}

.invite__hero h1 {
  margin: 12px 0 0;
  font-size: 21px;
  font-weight: 700;
}

.invite__hero p {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--fb-on-hero-muted);
}

.invite__card {
  max-width: 420px;
  margin: -26px auto 0;
  padding: 20px 18px;
  position: relative;
}

.invite__hello {
  margin: 0 0 10px;
  font-size: 15px;
  font-weight: 600;
}

.invite__text {
  margin: 0;
  font-size: 13px;
  line-height: 1.8;
  color: var(--fb-sub);
}

.invite__message {
  margin: 12px 0 0;
  font-size: 12px;
  color: var(--fb-blue);
}

.invite__actions {
  display: flex;
  gap: 10px;
  margin-top: 20px;
}

.invite__actions > button {
  flex: 1;
}
</style>
