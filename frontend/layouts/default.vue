<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900 flex flex-col transition-colors duration-200">
    <!-- 顶部导航栏 -->
    <header class="bg-white dark:bg-gray-800 shadow-sm sticky top-0 z-50 transition-colors duration-200">
      <div class="container mx-auto px-4 py-3 md:py-4 flex justify-between items-center">
        <h1 class="text-xl md:text-2xl font-bold text-primary">
          <NuxtLink to="/" class="dark:text-white">MoneyMint 记账</NuxtLink>
        </h1>
        
        <!-- 桌面端导航 -->
        <div class="hidden md:flex items-center gap-3">
          <!-- 根据登录状态显示不同的导航项 -->
          <template v-if="user">
            <NuxtLink to="/accounts" class="btn btn-primary dark:text-white">账户管理</NuxtLink>
            <button @click="showAddModal = true" class="btn btn-secondary dark:text-white">+ 添加记录</button>
            <div class="relative">
              <button @click="showUserMenu = !showUserMenu" class="flex items-center gap-2 px-3 py-2 bg-gray-100 dark:bg-gray-700 rounded-full hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ user.username }}</span>
                <svg class="w-4 h-4 text-gray-600 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
                </svg>
              </button>
              <!-- 用户菜单 -->
              <div v-if="showUserMenu" class="absolute right-0 mt-2 w-48 bg-white dark:bg-gray-800 rounded-lg shadow-xl py-1 z-50 transition-colors duration-200">
                <div class="px-4 py-2 border-b dark:border-gray-700">
                  <span class="text-xs font-semibold text-gray-500 dark:text-gray-400">设置</span>
                </div>
                <div class="px-2 py-1">
                  <div class="flex items-center justify-between px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700">
                    <span class="text-sm text-gray-700 dark:text-gray-300">深色模式</span>
                    <button @click="toggleDarkMode">
                      <svg v-if="!darkMode" class="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"></path>
                      </svg>
                      <svg v-else class="w-5 h-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path>
                      </svg>
                    </button>
                  </div>
                  <div class="flex items-center justify-between px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700">
                    <span class="text-sm text-gray-700 dark:text-gray-300">自动同步</span>
                    <button @click="toggleAutoSync">
                      <svg v-if="!autoSync" class="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                      </svg>
                      <svg v-else class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                      </svg>
                    </button>
                  </div>
                </div>
                <div class="px-2 py-1 mt-1">
                  <button @click="logout" class="block w-full text-left px-2 py-1 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors">退出登录</button>
                </div>
              </div>
            </div>
          </template>
          <NuxtLink v-else to="/login" class="btn btn-primary dark:text-white">登录</NuxtLink>
        </div>
        
        <!-- 移动端菜单按钮 -->
        <div class="md:hidden">
          <button @click="showMobileMenu = !showMobileMenu" @touchstart="showMobileMenu = !showMobileMenu" class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
            <svg class="w-6 h-6 text-gray-700 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path v-if="!showMobileMenu" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>
              <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
            </svg>
          </button>
        </div>
      </div>
      
      <!-- 移动端导航菜单 -->
      <div v-if="showMobileMenu" class="md:hidden bg-white dark:bg-gray-800 border-t dark:border-gray-700 transition-colors duration-200">
        <div class="container mx-auto px-4 py-3 space-y-3">
          <template v-if="user">
            <NuxtLink to="/accounts" class="block btn btn-primary dark:text-white" @click="showMobileMenu = false">账户管理</NuxtLink>
            <button @click="showAddModal = true; showMobileMenu = false" class="block w-full btn btn-secondary dark:text-white">+ 添加记录</button>
            <div class="border-t dark:border-gray-700 pt-3">
              <div class="flex items-center justify-between mb-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ user.username }}</span>
              </div>
              <button @click="logout; showMobileMenu = false" class="block w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors">退出登录</button>
            </div>
          </template>
          <NuxtLink v-else to="/login" class="block btn btn-primary dark:text-white" @click="showMobileMenu = false">登录</NuxtLink>
        </div>
      </div>
    </header>
    
    <!-- 主内容区 -->
    <main class="flex-1 container mx-auto px-4 py-4 md:py-6 pb-16 md:pb-6">
      <NuxtPage />
    </main>
    
    <!-- 移动端底部导航 -->
    <nav v-if="user" class="md:hidden fixed bottom-0 left-0 right-0 bg-white dark:bg-gray-800 border-t dark:border-gray-700 shadow-lg z-40 transition-colors duration-200">
      <div class="flex justify-around items-center h-14">
        <NuxtLink to="/" class="flex flex-col items-center justify-center p-2 flex-1 text-gray-600 dark:text-gray-300 active:text-primary">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"></path>
          </svg>
          <span class="text-xs mt-1">首页</span>
        </NuxtLink>
        <NuxtLink to="/entries" class="flex flex-col items-center justify-center p-2 flex-1 text-gray-600 dark:text-gray-300 active:text-primary">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"></path>
          </svg>
          <span class="text-xs mt-1">记录</span>
        </NuxtLink>
        <button @click="showAddModal = true" @touchstart="showAddModal = true" class="flex flex-col items-center justify-center p-2 flex-1 text-gray-600 dark:text-gray-300">
          <div class="w-10 h-10 rounded-full bg-primary text-white flex items-center justify-center -mt-4 shadow-lg">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path>
            </svg>
          </div>
          <span class="text-xs mt-1">添加</span>
        </button>
        <NuxtLink to="/accounts" class="flex flex-col items-center justify-center p-2 flex-1 text-gray-600 dark:text-gray-300 active:text-primary">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
          </svg>
          <span class="text-xs mt-1">账户</span>
        </NuxtLink>
        <NuxtLink to="/profile" class="flex flex-col items-center justify-center p-2 flex-1 text-gray-600 dark:text-gray-300 active:text-primary">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"></path>
          </svg>
          <span class="text-xs mt-1">我的</span>
        </NuxtLink>
      </div>
    </nav>
    
    <!-- Add Entry Drawer -->
    <AddEntryModal
      v-if="showAddModal"
      @close="showAddModal = false"
      @entry-added="handleEntryAdded"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useSse } from '../composables/useSse'
import AddEntryModal from '../components/AddEntryModal.vue'
import { useNuxtApp } from '#app'

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
      // 默认检查系统深色模式偏好
      darkMode.value = window.matchMedia('(prefers-color-scheme: dark)').matches
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
document.addEventListener('click', (e) => {
  const userMenu = document.querySelector('.absolute.right-0.mt-2.w-48.bg-white.rounded-lg.shadow-xl.py-1.z-50')
  const userButton = document.querySelector('.flex.items-center.gap-2.px-3.py-2.bg-gray-100.rounded-full')
  
  if (userMenu && !userMenu.contains(e.target) && userButton && !userButton.contains(e.target)) {
    showUserMenu.value = false
  }
})
</script>