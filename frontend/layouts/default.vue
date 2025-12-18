<template>
  <div class="min-h-screen bg-gray-50 flex flex-col">
    <!-- 顶部导航栏 -->
    <header class="bg-white shadow-sm sticky top-0 z-50">
      <div class="container mx-auto px-4 py-3 md:py-4 flex justify-between items-center">
        <h1 class="text-xl md:text-2xl font-bold text-primary">
          <NuxtLink to="/">MoneyMint 记账</NuxtLink>
        </h1>
        
        <!-- 桌面端导航 -->
        <div class="hidden md:flex items-center gap-3">
          <!-- 根据登录状态显示不同的导航项 -->
          <template v-if="user">
            <NuxtLink to="/accounts" class="btn btn-primary">账户管理</NuxtLink>
            <button @click="showAddModal = true" class="btn btn-secondary">+ 添加记录</button>
            <div class="relative">
              <button @click="showUserMenu = !showUserMenu" class="flex items-center gap-2 px-3 py-2 bg-gray-100 rounded-full hover:bg-gray-200 transition-colors">
                <span class="text-sm font-medium">{{ user.username }}</span>
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
                </svg>
              </button>
              <!-- 用户菜单 -->
              <div v-if="showUserMenu" class="absolute right-0 mt-2 w-48 bg-white rounded-lg shadow-xl py-1 z-50">
                <button @click="logout" class="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100">退出登录</button>
              </div>
            </div>
          </template>
          <NuxtLink v-else to="/login" class="btn btn-primary">登录</NuxtLink>
        </div>
        
        <!-- 移动端菜单按钮 -->
        <div class="md:hidden">
          <button @click="showMobileMenu = !showMobileMenu" class="p-2 rounded-lg hover:bg-gray-100">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path v-if="!showMobileMenu" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>
              <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
            </svg>
          </button>
        </div>
      </div>
      
      <!-- 移动端导航菜单 -->
      <div v-if="showMobileMenu" class="md:hidden bg-white border-t">
        <div class="container mx-auto px-4 py-3 space-y-3">
          <template v-if="user">
            <NuxtLink to="/accounts" class="block btn btn-primary" @click="showMobileMenu = false">账户管理</NuxtLink>
            <button @click="showAddModal = true; showMobileMenu = false" class="block w-full btn btn-secondary">+ 添加记录</button>
            <div class="border-t pt-3">
              <div class="flex items-center justify-between mb-2">
                <span class="text-sm font-medium">{{ user.username }}</span>
              </div>
              <button @click="logout; showMobileMenu = false" class="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded">退出登录</button>
            </div>
          </template>
          <NuxtLink v-else to="/login" class="block btn btn-primary" @click="showMobileMenu = false">登录</NuxtLink>
        </div>
      </div>
    </header>
    
    <!-- 主内容区 -->
    <main class="flex-1 container mx-auto px-4 py-4 md:py-6">
      <NuxtPage />
    </main>
    
    <!-- 移动端底部导航 -->
    <nav v-if="user" class="md:hidden fixed bottom-0 left-0 right-0 bg-white border-t shadow-lg z-40">
      <div class="flex justify-around items-center h-14">
        <NuxtLink to="/" class="flex flex-col items-center justify-center p-2 flex-1">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"></path>
          </svg>
          <span class="text-xs mt-1">首页</span>
        </NuxtLink>
        <NuxtLink to="/entries" class="flex flex-col items-center justify-center p-2 flex-1">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"></path>
          </svg>
          <span class="text-xs mt-1">记录</span>
        </NuxtLink>
        <button @click="showAddModal = true" class="flex flex-col items-center justify-center p-2 flex-1">
          <div class="w-10 h-10 rounded-full bg-primary text-white flex items-center justify-center -mt-4 shadow-lg">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path>
            </svg>
          </div>
          <span class="text-xs mt-1">添加</span>
        </button>
        <NuxtLink to="/accounts" class="flex flex-col items-center justify-center p-2 flex-1">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"></path>
          </svg>
          <span class="text-xs mt-1">账户</span>
        </NuxtLink>
        <button @click="showUserMenu = !showUserMenu" class="flex flex-col items-center justify-center p-2 flex-1">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"></path>
          </svg>
          <span class="text-xs mt-1">我的</span>
        </button>
      </div>
    </nav>
    
    <!-- 全局添加交易记录弹窗 -->
    <div v-if="showAddModal" class="fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center overflow-y-auto">
      <div class="bg-white rounded-lg shadow-xl w-full max-w-2xl my-8 mx-4">
        <AddEntryModal 
          @close="showAddModal = false" 
          @entry-added="handleEntryAdded"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useSse } from '../composables/useSse'
import AddEntryModal from '../components/AddEntryModal.vue'
import { useApi } from '../composables/useApi'

// 全局添加弹窗状态
const showAddModal = ref(false)
// 用户菜单状态
const showUserMenu = ref(false)
// 移动端菜单状态
const showMobileMenu = ref(false)
// 获取API钩子和用户状态
const { user, logout } = useApi()

// 全局SSE集成
useSse()

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