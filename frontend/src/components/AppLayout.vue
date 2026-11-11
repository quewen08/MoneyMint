<script setup lang="ts">
/**
 * L-01 单布局（3 个 Dart shell → 1 个组件）
 *
 * 一套模板 + CSS 断点（1024）：≥1024 显示 PC 侧边栏，<1024 显示移动底栏
 * （4 Tab + 中央 ＋）。沿用 pc-redesign.html / mobile-redesign-v2.html 的
 * 布局 token 与 IA，不重画信息架构。批 3 起 P-11~P-18 全部接入，导航项
 * 无占位死链；`ready` 保留用于将来临时下线某入口。
 */
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useSessionStore } from '@/stores/session'
import { usePrivacyStore } from '@/stores/privacy'
import { useRecordSheetStore } from '@/stores/recordSheet'
import RecordSheet from './RecordSheet.vue'

interface NavItem {
  key: string
  label: string
  icon: string
  to?: string
  ready: boolean
}

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const privacy = usePrivacyStore()
const sheet = useRecordSheetStore()
const notice = ref('')

// 批 3：P-11~P-18 全部接入，不再有「后续批次开放」的占位项。
// 顺序即 PC 侧栏分组顺序；移动底栏只取 4 项（含中央 ＋）。
const navItems: NavItem[] = [
  { key: 'home', label: '总览', icon: '▦', to: '/', ready: true },
  { key: 'transactions', label: '交易流水', icon: '≣', to: '/transactions', ready: true },
  { key: 'trash', label: '回收站', icon: '🗑', to: '/trash', ready: true },
  { key: 'accounts', label: '账户管理', icon: '▤', to: '/accounts', ready: true },
  { key: 'categories', label: '分类管理', icon: '▥', to: '/categories', ready: true },
  { key: 'commodities', label: '币种管理', icon: '💱', to: '/commodities', ready: true },
  { key: 'reports', label: '报表分析', icon: '📈', to: '/reports', ready: true },
  { key: 'members', label: '成员管理', icon: '◯', to: '/members', ready: true },
  { key: 'ledgers', label: '账本管理', icon: '▣', to: '/ledgers', ready: true },
  { key: 'sessions', label: '登录设备', icon: '▢', to: '/sessions', ready: true },
  { key: 'data-safety', label: '数据安全', icon: '⛁', to: '/data-safety', ready: true },
  { key: 'sync', label: '同步状态', icon: '↻', to: '/sync', ready: true },
  { key: 'settings', label: '设置', icon: '⚙', to: '/settings', ready: true },
]

const desktopGroups: Array<{ title: string; items: NavItem[] }> = [
  { title: '概览', items: [navItems[0], navItems[1]] },
  { title: '管理', items: [navItems[3], navItems[4], navItems[5], navItems[7], navItems[8]] },
  { title: '工具', items: [navItems[2], navItems[6], navItems[10]] },
  { title: '系统', items: [navItems[9], navItems[11], navItems[12]] },
]

const mobileNav = computed<NavItem[]>(() => [
  navItems[0],
  navItems[1],
  navItems[3],
  // 原型第 4 个 Tab 是「更多」；PC 侧栏对应项叫「设置」，IA 保持一致。
  { ...navItems[12], label: '更多' },
])

function isActive(item: NavItem): boolean {
  if (item.to === undefined) {
    return false
  }
  if (item.to === '/') {
    return route.path === '/'
  }
  return route.path === item.to || route.path.startsWith(`${item.to}/`)
}

function navigate(item: NavItem): void {
  if (!item.ready || item.to === undefined) {
    notice.value = `${item.label}将在后续批次开放`
    return
  }
  notice.value = ''
  void router.push(item.to)
}

function openRecord(): void {
  if (session.isViewer) {
    notice.value = '只读成员不能记账'
    return
  }
  if (session.needsInvite) {
    notice.value = '请先等待 owner 邀请加入账本'
    return
  }
  notice.value = ''
  sheet.open()
}

function togglePrivacy(): void {
  privacy.toggle()
}

async function logout(): Promise<void> {
  await session.logout()
  await router.replace({ name: 'login' })
}

function goChangePassword(): void {
  void router.push({ name: 'change-password' })
}
</script>

<template>
  <div class="layout">
    <!-- PC 侧边栏（≥1024） -->
    <aside class="layout__side">
      <div class="layout__brand">
        <span class="layout__logo">¥</span>
        <span>家庭记账</span>
      </div>
      <nav class="layout__nav" aria-label="主导航">
        <template v-for="group in desktopGroups" :key="group.title">
          <div class="layout__group">{{ group.title }}</div>
          <button
            v-for="item in group.items"
            :key="item.key"
            type="button"
            class="layout__nav-item"
            :class="{ on: isActive(item) }"
            @click="navigate(item)"
          >
            <span class="layout__nav-icon">{{ item.icon }}</span>{{ item.label }}
          </button>
        </template>
      </nav>
      <button class="fb-btn-primary layout__cta" type="button" @click="openRecord">
        ＋ 记一笔
      </button>
    </aside>

    <!-- 主区 -->
    <div class="layout__main">
      <header class="layout__top">
        <div class="layout__title">
          <strong>{{ session.ledgerName }}</strong>
          <span class="fb-muted" v-if="session.user">
            · {{ session.user.display_name || session.user.username }}
          </span>
        </div>
        <div class="layout__tools">
          <button
            type="button"
            class="layout__tool"
            :aria-pressed="privacy.enabled"
            @click="togglePrivacy"
          >{{ privacy.enabled ? '🙈 已遮挡' : '👁 显示金额' }}</button>
          <button
            v-if="session.mustChangePassword"
            type="button"
            class="layout__tool"
            @click="goChangePassword"
          >修改密码</button>
          <button type="button" class="layout__tool" @click="logout">退出</button>
        </div>
      </header>

      <main class="layout__content">
        <slot />
      </main>

      <p v-if="notice" class="layout__notice" role="status">{{ notice }}</p>
    </div>

    <!-- 移动底栏（<1024）：4 Tab + 中央 ＋ -->
    <nav class="layout__bottom" aria-label="底部导航">
      <button
        v-for="item in mobileNav.slice(0, 2)"
        :key="item.key"
        type="button"
        class="layout__tab"
        :class="{ on: isActive(item) }"
        @click="navigate(item)"
      >
        <span class="layout__tab-icon">{{ item.icon }}</span>
        <span>{{ item.label }}</span>
      </button>
      <button type="button" class="layout__fab" aria-label="记一笔" @click="openRecord">＋</button>
      <button
        v-for="item in mobileNav.slice(2)"
        :key="item.key"
        type="button"
        class="layout__tab"
        :class="{ on: isActive(item) }"
        @click="navigate(item)"
      >
        <span class="layout__tab-icon">{{ item.icon }}</span>
        <span>{{ item.label }}</span>
      </button>
    </nav>

    <!-- C-01 单实例：PC 弹窗 / 移动 Sheet / 全屏由同一组件按断点收敛 -->
    <RecordSheet v-if="sheet.isOpen" @close="sheet.close()" />
  </div>
</template>

<style scoped>
.layout {
  min-height: 100vh;
  display: flex;
  background: var(--fb-bg);
  color: var(--fb-ink);
}

/* ---------- PC 侧栏 ---------- */
.layout__side {
  display: none;
  width: 240px;
  flex: 0 0 240px;
  flex-direction: column;
  background: var(--fb-sidebar);
  color: var(--fb-side-fg);
}

.layout__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 20px;
  color: var(--fb-white);
  font-size: 16px;
  font-weight: 800;
}

.layout__logo {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  background: var(--fb-blue);
  display: flex;
  align-items: center;
  justify-content: center;
}

.layout__nav {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
}

.layout__group {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--fb-side-head);
  padding: 14px 12px 6px;
}

.layout__nav-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--fb-side-fg);
  font-size: 13px;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
}

.layout__nav-item.on {
  background: var(--fb-blue);
  color: var(--fb-white);
  font-weight: 700;
}

.layout__nav-icon {
  width: 19px;
  text-align: center;
}

.layout__cta {
  margin: 12px 16px 18px;
  width: calc(100% - 32px);
}

/* ---------- 主区 ---------- */
.layout__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.layout__top {
  min-height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--fb-line);
  background: var(--fb-card);
}

.layout__title {
  font-size: 15px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.layout__tools {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.layout__tool {
  border: 1px solid var(--fb-line);
  background: var(--fb-card);
  color: var(--fb-sub);
  border-radius: var(--fb-radius-pill);
  padding: 6px 10px;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
}

.layout__content {
  flex: 1;
  padding: 16px;
  padding-bottom: calc(84px + env(safe-area-inset-bottom, 0px));
}

.layout__notice {
  position: fixed;
  left: 50%;
  transform: translateX(-50%);
  bottom: calc(84px + env(safe-area-inset-bottom, 0px));
  margin: 0;
  padding: 8px 14px;
  border-radius: var(--fb-radius-pill);
  background: var(--fb-sidebar);
  color: var(--fb-white);
  font-size: 12px;
  z-index: 40;
}

/* ---------- 移动底栏 ---------- */
.layout__bottom {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  height: calc(68px + env(safe-area-inset-bottom, 0px));
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  align-items: center;
  background: var(--fb-card);
  border-top: 1px solid var(--fb-line);
  padding-bottom: env(safe-area-inset-bottom, 0px);
  z-index: 30;
}

.layout__tab {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  border: none;
  background: transparent;
  color: var(--fb-sub);
  font-size: 11px;
  font-family: inherit;
  cursor: pointer;
}

.layout__tab.on {
  color: var(--fb-blue);
  font-weight: 700;
}

.layout__tab-icon {
  font-size: 18px;
  line-height: 1;
}

.layout__fab {
  justify-self: center;
  width: 54px;
  height: 54px;
  margin-top: -24px;
  border: 4px solid var(--fb-card);
  border-radius: 50%;
  background: var(--fb-blue);
  color: var(--fb-white);
  font-size: 28px;
  line-height: 1;
  cursor: pointer;
  box-shadow: 0 8px 20px rgba(59, 110, 246, 0.35);
}

/* ---------- 断点：与 Flutter 版 isWide(1024) 对齐 ---------- */
@media (min-width: 1024px) {
  .layout__side {
    display: flex;
  }

  .layout__bottom {
    display: none;
  }

  .layout__content {
    padding: 24px;
    padding-bottom: 24px;
  }

  .layout__content > * {
    max-width: 1180px;
    margin-left: auto;
    margin-right: auto;
  }

  .layout__notice {
    bottom: 24px;
  }
}
</style>
