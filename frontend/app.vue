<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow-sm">
      <div class="container mx-auto px-4 py-4 flex justify-between items-center">
        <h1 class="text-2xl font-bold text-primary"><NuxtLink to="/">MoneyMint 记账</NuxtLink></h1>
        <div class="flex gap-3">
          <NuxtLink to="/accounts" class="btn btn-primary">账户管理</NuxtLink>
          <button @click="showAddModal = true" class="btn btn-secondary">+ 添加记录</button>
        </div>
      </div>
    </header>
    
    <main class="container mx-auto px-4 py-6">
      <NuxtPage />
    </main>

    <!-- 全局添加交易记录弹窗 -->
    <div v-if="showAddModal" class="fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center overflow-y-auto">
      <div class="bg-white rounded-lg shadow-xl w-full max-w-2xl my-8">
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
import { useSse } from './composables/useSse'
import AddEntryModal from './components/AddEntryModal.vue'

// 全局添加弹窗状态
const showAddModal = ref(false)

// 全局SSE集成
useSse()

// 处理交易记录添加完成事件
const handleEntryAdded = () => {
  // 触发一个自定义事件，通知所有监听者刷新数据
  window.dispatchEvent(new CustomEvent('refreshData'))
}
</script>