/**
 * Hash 路由与登录守卫（批 1）
 *
 * hash 模式绕开 `static.go` 把无扩展名路径回退到根首页的限制，后端零改动。
 * 守卫负责：Cookie 会话恢复（AC-32）→ 强制改密（428 同源）→ 待邀请 → 首页。
 * 同时把 401 / 428 的全局回调注入 API 客户端，避免 api 层反向依赖路由。
 */
import {
  createRouter,
  createWebHashHistory,
  type RouteRecordRaw,
} from 'vue-router'

import { setApiClientHooks } from '@/api/client'
import { useSessionStore } from '@/stores/session'
import AccountsView from '@/views/AccountsView.vue'
import AccountDetailView from '@/views/AccountDetailView.vue'
import AwaitInviteView from '@/views/AwaitInviteView.vue'
import CategoriesView from '@/views/CategoriesView.vue'
import ChangePasswordView from '@/views/ChangePasswordView.vue'
import CommoditiesView from '@/views/CommoditiesView.vue'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/LoginView.vue'
import TrashView from '@/views/TrashView.vue'
import TransactionDetailView from '@/views/TransactionDetailView.vue'
import TransactionsView from '@/views/TransactionsView.vue'

// 批 3 的 7 个管理与设置页（reports / members / ledgers / sessions /
// data-safety / sync / settings）改为路由懒加载以压首屏体积。动态
// import() 是 ES2020 语法，v2.6 起 es-check 门禁已抬到 es2020 放行；
// Safari 14.1 真机支持动态导入，真机冒烟待老板验证。其余核心页保持
// 静态导入（首屏即需渲染，懒加载无意义）。

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: LoginView,
    meta: { standalone: true, public: true },
  },
  {
    path: '/await-invite',
    name: 'await-invite',
    component: AwaitInviteView,
    meta: { standalone: true, requiresAuth: true },
  },
  {
    path: '/change-password',
    name: 'change-password',
    component: ChangePasswordView,
    meta: { standalone: true, requiresAuth: true },
  },
  {
    path: '/',
    name: 'home',
    component: HomeView,
    meta: { requiresAuth: true },
  },
  {
    path: '/transactions',
    name: 'transactions',
    component: TransactionsView,
    meta: { requiresAuth: true },
  },
  {
    path: '/transactions/detail',
    name: 'transaction-detail',
    component: TransactionDetailView,
    meta: { requiresAuth: true },
  },
  {
    path: '/trash',
    name: 'trash',
    component: TrashView,
    meta: { requiresAuth: true },
  },
  {
    path: '/accounts',
    name: 'accounts',
    component: AccountsView,
    meta: { requiresAuth: true },
  },
  {
    path: '/accounts/:id',
    name: 'account-detail',
    component: AccountDetailView,
    meta: { requiresAuth: true },
  },
  {
    path: '/categories',
    name: 'categories',
    component: CategoriesView,
    meta: { requiresAuth: true },
  },
  {
    path: '/commodities',
    name: 'commodities',
    component: CommoditiesView,
    meta: { requiresAuth: true },
  },
  // 批 3：管理与设置页（P-11~P-15 / P-17 / P-18；P-16 改密已存在）。
  {
    path: '/reports',
    name: 'reports',
    component: () => import('@/views/ReportsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/members',
    name: 'members',
    component: () => import('@/views/MembersView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/ledgers',
    name: 'ledgers',
    component: () => import('@/views/LedgersView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/sessions',
    name: 'sessions',
    component: () => import('@/views/SessionsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/data-safety',
    name: 'data-safety',
    component: () => import('@/views/DataSafetyView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/sync',
    name: 'sync',
    component: () => import('@/views/SyncStatusView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/SettingsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

// E-07：401 刷新失败统一登出、428 跳改密页。回调运行时 Pinia 已激活。
setApiClientHooks({
  onUnauthorized: () => {
    const session = useSessionStore()
    session.markGuest()
    if (router.currentRoute.value.name !== 'login') {
      void router.replace({ name: 'login' })
    }
  },
  onPasswordChangeRequired: () => {
    if (router.currentRoute.value.name !== 'change-password') {
      void router.replace({ name: 'change-password' })
    }
  },
})

router.beforeEach(async (to) => {
  const session = useSessionStore()

  if (to.meta.public === true) {
    if (to.name === 'login' && session.isAuthenticated) {
      if (session.mustChangePassword) {
        return { name: 'change-password' }
      }
      if (session.needsInvite) {
        return { name: 'await-invite' }
      }
      return { name: 'home' }
    }
    return true
  }

  if (to.meta.requiresAuth === true) {
    if (!session.isAuthenticated) {
      // Cookie 会话刷新后仍有效：先尝试 /api/auth/me 恢复（AC-32）。
      const ok = await session.bootstrap()
      if (!ok) {
        return { name: 'login', query: { redirect: to.fullPath } }
      }
    }
    if (session.mustChangePassword && to.name !== 'change-password') {
      return { name: 'change-password' }
    }
    if (session.needsInvite && to.name !== 'await-invite') {
      return { name: 'await-invite' }
    }
  }
  return true
})

export default router
