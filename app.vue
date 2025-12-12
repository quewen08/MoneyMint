<template>
  <div>
    <!-- 全局布局内容 -->
    <NuxtPage />
    
    <!-- 全局添加交易弹窗 -->
    <AddTransactionModal 
      :show-modal="showModal" 
      @close="closeModal"
      @transaction-added="handleTransactionAdded"
    />
  </div>
</template>

<script setup>
import { useTransactionModal } from './composables/useTransactionModal'
import AddTransactionModal from './components/AddTransactionModal.vue'

// 使用交易弹窗的状态管理
const { showModal, closeModal } = useTransactionModal()

// 处理交易添加成功事件
function handleTransactionAdded() {
  console.log('交易添加成功，可在此处执行刷新数据等操作')
}
</script>

<style>
/* 全局样式 */
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: 'Inter', system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  background-color: #f9fafb;
  color: #1f2937;
  line-height: 1.6;
}
</style>