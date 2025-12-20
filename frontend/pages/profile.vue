<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900 py-8 px-4 sm:px-6 lg:px-8 transition-colors duration-200">
    <div class="max-w-3xl mx-auto">
      <div class="bg-white dark:bg-gray-800 shadow-md rounded-lg overflow-hidden transition-colors duration-200">
        <div class="bg-gradient-to-r from-blue-500 to-indigo-600 h-24"></div>

        <div class="px-6 py-8">
          <div class="text-center">
            <div
              class="w-20 h-20 bg-blue-100 dark:bg-blue-900 rounded-full mx-auto -mt-12 flex items-center justify-center transition-colors duration-200">
              <span class="text-blue-600 dark:text-blue-300 text-3xl font-bold">
                {{ user?.username?.charAt(0)?.toUpperCase() || 'U' }}</span>
            </div>
            <h2 class="text-2xl font-bold text-gray-800 dark:text-white mt-4 transition-colors duration-200">{{
              user?.username || '用户' }}</h2>
          </div>

          <div class="mt-10 space-y-6">
            <div class="border-t border-gray-200 dark:border-gray-700 pt-6">
              <h3 class="text-lg font-medium text-gray-700 dark:text-gray-300 mb-4 transition-colors duration-200">
                账户信息
              </h3>
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label
                    class="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1 transition-colors duration-200">用户名</label>
                  <p class="text-gray-800 dark:text-white">{{ user?.username }}</p>
                </div>
              </div>
            </div>

            <div class="border-t border-gray-200 dark:border-gray-700 pt-6">
              <h3 class="text-lg font-medium text-gray-700 dark:text-gray-300 mb-4 transition-colors duration-200">
                应用设置
              </h3>
              <div class="space-y-4">
                <div class="flex items-center justify-between" @click="toggleDarkMode">
                  <span class="text-gray-700 dark:text-gray-300 transition-colors duration-200">深色模式</span>
                  <label class="relative inline-block w-12 align-middle select-none cursor-pointer">
                    <input type="checkbox" v-model="darkMode" class="sr-only" @change="toggleDarkMode">
                    <div
                      class="block bg-gray-200 dark:bg-gray-600 w-12 h-6 rounded-full transition-colors duration-200">
                    </div>
                    <div
                      class="absolute left-1 top-1 bg-white dark:bg-gray-300 w-4 h-4 rounded-full transition-transform duration-200 transform"
                      :class="{ 'translate-x-6': darkMode }"></div>
                  </label>
                </div>
                <div class="flex items-center justify-between" @click="toggleAutoSync">
                  <div>
                    <span class="text-gray-700 dark:text-gray-300 transition-colors duration-200">自动同步 (SSE)</span>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1 transition-colors duration-200">
                      <span v-if="isConnected" class="text-green-500">已连接</span>
                      <span v-else class="text-red-500">已断开</span>
                    </div>
                  </div>
                  <label class="relative inline-block w-12 align-middle select-none cursor-pointer">
                    <input type="checkbox" v-model="autoSync" class="sr-only" @change="toggleAutoSync">
                    <div
                      class="block bg-gray-200 dark:bg-gray-600 w-12 h-6 rounded-full transition-colors duration-200">
                    </div>
                    <div
                      class="absolute left-1 top-1 bg-white dark:bg-gray-300 w-4 h-4 rounded-full transition-transform duration-200 transform"
                      :class="{ 'translate-x-6': autoSync }"></div>
                  </label>
                </div>
              </div>
            </div>

            <div class="border-t border-gray-200 dark:border-gray-700 pt-6">
              <button @click="logout"
                class="w-full py-3 px-4 bg-red-500 hover:bg-red-600 text-white rounded-md transition-colors duration-200 font-medium">
                退出登录
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="mt-8 bg-white dark:bg-gray-800 shadow-md rounded-lg overflow-hidden transition-colors duration-200">
        <div class="px-6 py-6">
          <h3 class="text-lg font-medium text-gray-700 dark:text-gray-300 mb-4 transition-colors duration-200">关于
            MoneyMint</h3>
          <p class="text-gray-600 dark:text-gray-400 mb-4 transition-colors duration-200">版本: {{ appVersion }}</p>
          <p class="text-gray-600 dark:text-gray-400 transition-colors duration-200">{{ APP_DESCRIPTION }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useNuxtApp } from '#app'
import { useRouter } from 'vue-router'
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useSse } from '~/composables/useSse'
// 应用信息
import { APP_NAME, APP_DESCRIPTION, fetchVersionInfo } from '~/config/version'

const { $api } = useNuxtApp()
const router = useRouter()
const user = computed(() => $api.user.value)

// 深色模式功能
const darkMode = ref(false)

// 自动同步功能
const autoSync = ref(true)
const { connect: connectSse, disconnect: disconnectSse, isConnected } = useSse()

// 应用信息
const appVersion = ref('加载中...')
const appDescription = ref(APP_DESCRIPTION)

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

// 应用深色模式
const applyDarkMode = () => {
  if (darkMode.value) {
    // 应用深色模式类名
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

// 深色模式切换
const toggleDarkMode = () => {
  console.log('切换深色模式:', darkMode.value, isLocalStorageAvailable())
  darkMode.value = !darkMode.value
  if (isLocalStorageAvailable()) {
    localStorage.setItem('darkMode', darkMode.value.toString())
  }
  applyDarkMode()
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

// 退出登录
const logout = async () => {
  await $api.logout()
  router.push('/login')
}

// 初始化
onMounted(async () => {
  // 获取版本信息
  const versionInfo = await fetchVersionInfo()
  appVersion.value = versionInfo.version

  // 从localStorage恢复深色模式
  if (process.client && isLocalStorageAvailable()) {
    const savedDarkMode = localStorage.getItem('darkMode')
    if (savedDarkMode) {
      darkMode.value = savedDarkMode === 'true'
      applyDarkMode()
    } else {
      // 默认检查系统深色模式偏好
      darkMode.value = window.matchMedia('(prefers-color-scheme: dark)').matches
      applyDarkMode()
    }

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

  // 监听系统深色模式变化
  const darkModeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  const handleDarkModeChange = (e: MediaQueryListEvent) => {
    if (process.client && isLocalStorageAvailable()) {
      // 只有在用户没有手动设置的情况下才响应系统变化
      if (!localStorage.getItem('darkMode')) {
        darkMode.value = e.matches
        toggleDarkMode()
      }
    }
  }
  darkModeMediaQuery.addEventListener('change', handleDarkModeChange)

  // 清理事件监听
  onUnmounted(() => {
    darkModeMediaQuery.removeEventListener('change', handleDarkModeChange)
  })
})
</script>