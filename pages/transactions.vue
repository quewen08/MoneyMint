<template>
  <div class="container">
    <header>
      <nav>
        <div class="logo">MoneyMint</div>
        <div>
          <NuxtLink to="/" class="btn btn-secondary mr-2">返回首页</NuxtLink>
          <NuxtLink to="/accounts" class="btn btn-primary">账户管理</NuxtLink>
        </div>
      </nav>
    </header>

    <main class="mt-6">
      <div class="flex justify-between items-center mb-4">
        <h1 class="text-2xl font-bold">交易记录</h1>
        <button @click="openModal" class="btn btn-primary">
          添加新交易
        </button>
      </div>

      <!-- All Transactions -->
      <div class="card">
        <h2 class="text-xl font-bold mb-4">所有交易</h2>
        <table>
          <thead>
            <tr>
              <th>日期</th>
              <th>描述</th>
              <th>交易明细</th>
              <th>类型</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="transaction in transactions" :key="transaction.id">
              <td>{{ transaction.date }}</td>
              <td>{{ transaction.narration || transaction.description }}</td>
              <td>
                <div v-for="(posting, index) in transaction.postings" :key="index" class="text-sm">
                  {{ posting.account }}: {{ formatCurrency(posting.amount) }}
                </div>
              </td>
              <td>
                <span :class="getTransactionTypeClass(transaction)" 
                      class="px-2 py-1 rounded-full text-xs">
                  {{ getTransactionTypeLabel(transaction) }}
                </span>
              </td>
              <td>
                <button @click="deleteTransaction(transaction.id)" class="btn text-red-600 hover:text-red-800">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="transactions.length === 0" class="text-center text-gray-500 py-4">
          暂无交易记录
        </div>
      </div>
    </main>


  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useTransactionModal } from '../composables/useTransactionModal'

const { data: transactionsData, refresh: refreshTransactions } = await useAsyncData('transactions', async () => {
  const response = await $fetch('/api/transactions')
  return response.transactions || []
})

// 获取所有账户列表
const { data: accountsData } = await useAsyncData('allAccounts', async () => {
  const response = await $fetch('/api/accounts')
  return response.accounts || []
})

const transactions = computed(() => transactionsData.value || [])
const accounts = computed(() => accountsData.value || [])

// 使用交易弹窗的状态管理
const { openModal } = useTransactionModal()

// 获取交易类型类名
function getTransactionTypeClass(transaction) {
  if (transaction.type === 'income') return 'bg-green-100 text-green-800'
  if (transaction.type === 'expense') return 'bg-red-100 text-red-800'
  return 'bg-blue-100 text-blue-800'
}

// 获取交易类型标签
function getTransactionTypeLabel(transaction) {
  if (transaction.type === 'income') return '收入'
  if (transaction.type === 'expense') return '支出'
  return '转账'
}



async function deleteTransaction(id) {
  if (confirm('确定要删除这条交易记录吗？')) {
    try {
      await $fetch(`/api/transactions/${id}`, {
        method: 'DELETE'
      })
      
      // 刷新交易列表
      await refreshTransactions()
      
      // 显示成功提示
      alert('交易记录删除成功！')
    } catch (error) {
      console.error('删除交易失败:', error)
      alert('删除交易失败，请稍后重试。')
    }
  }
}

function formatCurrency(amount) {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY'
  }).format(amount)
}
</script>

<style scoped>
.mt-6 {
  margin-top: 1.5rem;
}

.text-2xl {
  font-size: 1.5rem;
}

.font-bold {
  font-weight: bold;
}

.mb-4 {
  margin-bottom: 1rem;
}

.text-xl {
  font-size: 1.25rem;
}

.grid {
  display: grid;
}

.md\:grid-cols-2 {
  grid-template-columns: repeat(2, 1fr);
}

.gap-4 {
  gap: 1rem;
}

.w-full {
  width: 100%;
}

.text-green-600 {
  color: #10b981;
}

.text-red-600 {
  color: #ef4444;
}

.bg-green-100 {
  background-color: #d1fae5;
}

.text-green-800 {
  color: #065f46;
}

.bg-red-100 {
  background-color: #fee2e2;
}

.text-red-800 {
  color: #991b1b;
}

.px-2 {
  padding-left: 0.5rem;
  padding-right: 0.5rem;
}

.py-1 {
  padding-top: 0.25rem;
  padding-bottom: 0.25rem;
}

.rounded-full {
  border-radius: 9999px;
}

.text-xs {
  font-size: 0.75rem;
}

.text-center {
  text-align: center;
}

.text-gray-500 {
  color: #6b7280;
}

.py-4 {
  padding-top: 1rem;
  padding-bottom: 1rem;
}

.hover\:text-red-800 {
  cursor: pointer;
}

/* 弹窗样式 */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
  padding: 1rem;
}

.modal-content {
  background-color: white;
  border-radius: 0.5rem;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
  width: 100%;
  max-width: 600px;
  max-height: 90vh;
  overflow-y: auto;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid #e5e7eb;
}

.modal-body {
  padding: 1rem;
}

.modal-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  color: #6b7280;
  cursor: pointer;
  padding: 0;
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.25rem;
}

.modal-close:hover {
  background-color: #f3f4f6;
  color: #374151;
}

/* 按钮样式增强 */
.btn-primary {
  background-color: #3b82f6;
  color: white;
  padding: 0.5rem 1rem;
  border-radius: 0.25rem;
  border: none;
  cursor: pointer;
  font-weight: 500;
}

.btn-primary:hover {
  background-color: #2563eb;
}

.btn-secondary {
  background-color: #f3f4f6;
  color: #374151;
  padding: 0.5rem 1rem;
  border-radius: 0.25rem;
  border: none;
  cursor: pointer;
  font-weight: 500;
}

.btn-secondary:hover {
  background-color: #e5e7eb;
}

.btn-danger {
  background-color: #ef4444;
  color: white;
  padding: 0.5rem 1rem;
  border-radius: 0.25rem;
  border: none;
  cursor: pointer;
  font-weight: 500;
}

.btn-danger:hover {
  background-color: #dc2626;
}

/* 表单样式增强 */
.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.25rem;
  font-weight: 500;
  color: #374151;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid #d1d5db;
  border-radius: 0.25rem;
  font-size: 1rem;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

/* 响应式设计 */
@media (max-width: 768px) {
  .grid md\:grid-cols-2 {
    grid-template-columns: 1fr;
  }
}
</style>