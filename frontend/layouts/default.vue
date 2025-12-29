<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900 flex flex-col transition-colors duration-200">
    <!-- 用户已登录且非登录页面的布局 -->
    <template v-if="user && $route.path !== '/login'">
      <!-- PC端左侧菜单 (md及以上屏幕) -->
      <aside
        class="hidden md:block w-64 bg-white dark:bg-gray-800 shadow-lg fixed h-screen overflow-y-auto z-50 transition-colors duration-200">
        <div class="p-4 border-b dark:border-gray-700">
          <NuxtLink to="/" class="text-xl font-bold text-primary dark:text-white">MoneyMint</NuxtLink>
          <p class="text-xs text-gray-500 dark:text-gray-400">个人记账系统</p>
        </div>
        <nav class="p-4 space-y-2">
          <NuxtLink v-for="menuItem in menuItems" :key="menuItem.path" :to="menuItem.path"
            class="flex items-center gap-3 px-4 py-3 rounded-lg transition-colors duration-200 group" :class="{
              'bg-primary text-white': $route.path === menuItem.path,
              'bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300': $route.path !== menuItem.path
            }">
            <svg :class="['w-5 h-5', $route.path === menuItem.path ? '' : 'group-hover:text-primary']" fill="none"
              stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="menuItem.icon"></path>
            </svg>
            <span>{{ menuItem.name }}</span>
          </NuxtLink>
          <!-- 添加记录按钮 -->
          <button @click="showAddModal = true"
            class="w-full flex items-center gap-3 px-4 py-3 bg-primary/10 text-primary rounded-lg transition-colors duration-200 hover:bg-primary/20">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path>
            </svg>
            <span>添加记录</span>
          </button>
        </nav>
      </aside>

      <!-- 顶部导航栏 -->
      <header class="bg-white dark:bg-gray-800 shadow-sm sticky top-0 z-40 transition-colors duration-200 md:pl-64">
        <div class="container mx-auto px-4 py-3 flex justify-between items-center">
          <!-- 移动端标题 -->
          <h1 class="md:hidden text-xl font-bold text-primary">
            <NuxtLink to="/" class="dark:text-white">MoneyMint</NuxtLink>
          </h1>

          <!-- 移动端菜单按钮 -->
          <div class="md:hidden">
            <button @click="showMobileMenu = !showMobileMenu"
              class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
              <svg class="w-6 h-6 text-gray-700 dark:text-gray-300" fill="none" stroke="currentColor"
                viewBox="0 0 24 24">
                <path v-if="!showMobileMenu" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M4 6h16M4 12h16M4 18h16"></path>
                <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12">
                </path>
              </svg>
            </button>
          </div>

          <!-- 桌面端用户菜单 -->
          <div class="hidden md:flex items-center gap-3">
            <!-- 设置按钮 -->
            <div class="flex items-center gap-2">
              <button @click="toggleDarkMode"
                class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                <svg v-if="!darkMode" class="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none"
                  stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z">
                  </path>
                </svg>
                <svg v-else class="w-5 h-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path>
                </svg>
              </button>
              <button @click="toggleAutoSync"
                class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                <svg v-if="!autoSync" class="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none"
                  stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15">
                  </path>
                </svg>
                <svg v-else class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15">
                  </path>
                </svg>
              </button>
            </div>

            <!-- 用户菜单 -->
            <div class="relative">
              <button @click="showUserMenu = !showUserMenu"
                class="flex items-center gap-2 px-3 py-2 bg-gray-100 dark:bg-gray-700 rounded-full hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ user?.username }}</span>
                <svg class="w-4 h-4 text-gray-600 dark:text-gray-300" fill="none" stroke="currentColor"
                  viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
                </svg>
              </button>
              <!-- 用户菜单下拉 -->
              <div v-if="showUserMenu"
                class="absolute right-0 mt-2 w-48 bg-white dark:bg-gray-800 rounded-lg shadow-xl py-1 z-50 transition-colors duration-200">
                <div class="px-4 py-2 border-b dark:border-gray-700">
                  <span class="text-xs font-semibold text-gray-500 dark:text-gray-400">账户</span>
                </div>
                <div class="px-2 py-1">
                  <NuxtLink to="/profile"
                    class="block w-full text-left px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                    <span class="text-sm text-gray-700 dark:text-gray-300">个人资料</span>
                  </NuxtLink>
                </div>
                <div class="px-2 py-1 mt-1">
                  <button @click="logout"
                    class="block w-full text-left px-2 py-1 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors">退出登录</button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 移动端导航菜单 -->
        <div v-if="showMobileMenu"
          class="bg-white dark:bg-gray-800 border-t dark:border-gray-700 transition-colors duration-200 md:hidden">
          <div class="container mx-auto px-4 py-3 space-y-3">
            <NuxtLink v-for="menuItem in menuItems" :key="menuItem.path" :to="menuItem.path"
              class="block px-4 py-3 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors duration-200" 
              @click="showMobileMenu = false">
              {{ menuItem.name }}
            </NuxtLink>
            <button @click="showAddModal = true; showMobileMenu = false"
              class="block w-full px-4 py-3 bg-primary text-white rounded-lg hover:bg-primary/90 transition-colors duration-200">+ 添加记录</button>
            <div class="border-t dark:border-gray-700 pt-3" v-if="user">
              <div class="flex items-center justify-between mb-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ user.username }}</span>
              </div>
              <div class="flex flex-col space-y-2">
                <div class="flex items-center justify-between px-4 py-2 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                  <span class="text-sm text-gray-700 dark:text-gray-300">深色模式</span>
                  <button @click="toggleDarkMode"
                    class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                    <svg v-if="!darkMode" class="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none"
                      stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z">
                      </path>
                    </svg>
                    <svg v-else class="w-5 h-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path>
                    </svg>
                  </button>
                </div>
                <div class="flex items-center justify-between px-4 py-2 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                  <span class="text-sm text-gray-700 dark:text-gray-300">自动同步</span>
                  <button @click="toggleAutoSync"
                    class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                    <svg v-if="!autoSync" class="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none"
                      stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15">
                      </path>
                    </svg>
                    <svg v-else class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15">
                      </path>
                    </svg>
                  </button>
                </div>
              </div>
              <button @click="logout; showMobileMenu = false"
                class="block w-full mt-3 text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors">退出登录</button>
            </div>
          </div>
        </div>
      </header>

      <!-- 主内容区 -->
      <main class="container mx-auto px-4 md:px-6 py-4 md:py-8 md:pl-64 pb-16 min-h-[calc(100vh-4rem)]">
        <NuxtPage />
      </main>

      <!-- 移动端底部导航 -->
      <nav class="md:hidden fixed bottom-0 left-0 right-0 bg-white dark:bg-gray-800 border-t dark:border-gray-700 shadow-lg z-40 transition-colors duration-200">
        <div class="flex justify-around items-center h-14">
          <NuxtLink v-for="menuItem in menuItems" :key="menuItem.path" :to="menuItem.path"
            class="flex flex-col items-center justify-center p-2 flex-1 transition-colors duration-200 group" :class="{
              'text-primary bg-primary/10 rounded-lg': $route.path === menuItem.path,
              'text-gray-600 dark:text-gray-300 hover:text-primary': $route.path !== menuItem.path
            }">
            <svg class="w-6 h-6 transition-colors duration-200" fill="none" stroke="currentColor" viewBox="0 0 24 24" :class="{
              'text-primary': $route.path === menuItem.path,
              'group-hover:text-primary': $route.path !== menuItem.path
            }">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="menuItem.icon"></path>
            </svg>
            <span class="text-xs mt-1 transition-colors duration-200">{{ menuItem.name }}</span>
          </NuxtLink>
          <button @click="showAddModal = true" @touchstart="showAddModal = true"
            class="flex flex-col items-center justify-center p-2 flex-1 text-gray-600 dark:text-gray-300">
            <div class="w-10 h-10 rounded-full bg-primary text-white flex items-center justify-center -mt-4 shadow-lg">
              <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path>
              </svg>
            </div>
            <span class="text-xs mt-1">添加</span>
          </button>
        </div>
      </nav>
    </template>

    <!-- 登录页面布局 -->
    <template v-else>
      <main class="container mx-auto px-4 py-8">
        <NuxtPage />
      </main>
    </template>

    <!-- Add Entry Drawer -->
    <AddEntryModal v-if="showAddModal" @close="showAddModal = false" @entry-added="handleEntryAdded" />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useSse } from '../composables/useSse'
import AddEntryModal from '../components/AddEntryModal.vue'
import { useNuxtApp } from '#app'

// 菜单配置
const menuItems = [
  {
    name: '首页',
    path: '/',
    icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6'
  },
  {
    name: '记录',
    path: '/entries',
    icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2'
  },
  {
    name: '账户',
    path: '/accounts',
    icon: 'M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
  },
  {
    name: '统计',
    path: '/stats',
    icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z'
  },
  {
    name: '我的',
    path: '/profile',
    icon: 'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z'
  }
]

// 全局添加弹窗状态
const showAddModal = ref(false)
// 用户菜单状态
const showUserMenu = ref(false)
// 移动端菜单状态
const showMobileMenu = ref(false)
// 获取API钩子和用户状态
const { $api } = useNuxtApp()
const { user, logout } = $api

// 深色模式状态
const darkMode = ref(false)
// 自动同步状态
const autoSync = ref(true)

// 全局SSE集成
const { connect: connectSse, disconnect: disconnectSse } = useSse()

// 检查localStorage可用性
const isLocalStorageAvailable = () => {
  try {
    const test = '__localStorage_test__'
    localStorage.setItem(test, test)
    localStorage.removeItem(test)
    return true
  } catch (e) {
    console.warn('localStorage not available:', e)
    return false
  }
}

// 深色模式切换
const toggleDarkMode = () => {
  darkMode.value = !darkMode.value
  if (isLocalStorageAvailable()) {
    localStorage.setItem('darkMode', darkMode.value.toString())
  }
  applyDarkMode()
}

// 应用深色模式
const applyDarkMode = () => {
  if (darkMode.value) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

// 自动同步切换
const toggleAutoSync = () => {
  autoSync.value = !autoSync.value
  if (isLocalStorageAvailable()) {
    localStorage.setItem('autoSync', autoSync.value.toString())
  }
  if (autoSync.value) {
    connectSse()
  } else {
    disconnectSse()
  }
}

// 初始化
onMounted(() => {
  // 从localStorage恢复深色模式
  if (isLocalStorageAvailable()) {
    const savedDarkMode = localStorage.getItem('darkMode')
    if (savedDarkMode) {
      darkMode.value = savedDarkMode === 'true'
    } else {
      // 默认检查系统深色模式偏好（使用try-catch防止某些设备不支持）
      try {
        darkMode.value = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
      } catch (e) {
        darkMode.value = false
      }
    }
    applyDarkMode()

    // 从localStorage恢复自动同步设置
    const savedAutoSync = localStorage.getItem('autoSync')
    if (savedAutoSync) {
      autoSync.value = savedAutoSync === 'true'
    }
    // 根据设置连接/断开SSE
    if (autoSync.value) {
      connectSse()
    } else {
      disconnectSse()
    }
  }
})

// 处理交易记录添加完成事件
const handleEntryAdded = () => {
  // 触发一个自定义事件，通知所有监听者刷新数据
  window.dispatchEvent(new CustomEvent('refreshData'))
}

// 点击外部关闭用户菜单
const handleClickOutside = (e) => {
  const userMenu = document.querySelector('.absolute.right-0.mt-2.w-48.bg-white.rounded-lg.shadow-xl.py-1.z-50')
  const userButton = document.querySelector('.flex.items-center.gap-2.px-3.py-2.bg-gray-100.rounded-full')

  if (userMenu && !userMenu.contains(e.target) && userButton && !userButton.contains(e.target)) {
    showUserMenu.value = false
  }
}

// 在组件挂载时添加事件监听器
onMounted(() => {
  // 从localStorage恢复深色模式
  if (isLocalStorageAvailable()) {
    const savedDarkMode = localStorage.getItem('darkMode')
    if (savedDarkMode) {
      darkMode.value = savedDarkMode === 'true'
    } else {
      // 默认检查系统深色模式偏好（使用try-catch防止某些设备不支持）
      try {
        darkMode.value = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
      } catch (e) {
        darkMode.value = false
      }
    }
    applyDarkMode()

    // 从localStorage恢复自动同步设置
    const savedAutoSync = localStorage.getItem('autoSync')
    if (savedAutoSync) {
      autoSync.value = savedAutoSync === 'true'
    }
    // 根据设置连接/断开SSE
    if (autoSync.value) {
      connectSse()
    } else {
      disconnectSse()
    }
  }

  // 添加点击外部关闭用户菜单的事件监听器
  document.addEventListener('click', handleClickOutside)
})

// 在组件卸载时移除事件监听器
onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>