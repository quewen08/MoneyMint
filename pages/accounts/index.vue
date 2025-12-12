<template>
  <div class="container">
    <header>
      <nav>
        <div class="logo">MoneyMint</div>
        <div>
          <NuxtLink to="/" class="btn btn-secondary mr-2">返回首页</NuxtLink>
          <button @click="openModal" class="btn btn-primary mr-2">记录交易</button>
          <button @click="showAddModal = true" class="btn btn-success">新增账户</button>
        </div>
      </nav>
    </header>

    <main class="mt-6">
      <h1 class="text-2xl font-bold mb-4">账户管理</h1>
      
      <!-- Search and Filter Section -->
      <div class="filter-section mb-6">
        <div class="search-box">
          <input 
            type="text" 
            v-model="searchQuery" 
            placeholder="搜索账户名称或实际名称..." 
            class="search-input"
          >
        </div>
        
        <div class="filter-tabs">
          <button 
            v-for="tab in filterTabs" 
            :key="tab.value"
            @click="activeFilter = tab.value"
            :class="['filter-tab', { active: activeFilter === tab.value }]"
          >
            {{ tab.label }}
            <span v-if="getAccountCountByType(tab.value) > 0" class="filter-count">{{ getAccountCountByType(tab.value) }}</span>
          </button>
        </div>
      </div>
      
      <!-- Accounts List -->
      <div class="accounts-grid">
        <div
          v-for="account in filteredAccounts"
          :key="account.nickname"
          class="account-card"
          :class="`account-type-${account.type}`"
        >
          <div class="account-header">
            <NuxtLink
              :to="`/accounts/${encodeURIComponent(account.actualName)}`"
              class="account-name"
            >
              {{ account.nickname }}
            </NuxtLink>
            <button
              @click="deleteAccount(account.nickname)"
              class="btn-delete"
              title="删除账户"
            >
              ×
            </button>
          </div>
          <div class="account-actual-name">{{ account.actualName }}</div>
          <div class="account-balance">
            {{ formatCurrency(account.balance) }}
          </div>
          <div class="account-transactions">{{ account.transactionCount }} 笔交易</div>
          <div class="account-type-badge">{{ getAccountTypeLabel(account.type) }}</div>
        </div>
      </div>
      
      <!-- No Accounts Message -->
      <div v-if="filteredAccounts.length === 0" class="text-center text-gray-500 py-12">
        <div v-if="searchQuery || activeFilter !== 'all'">
          没有找到匹配的账户
        </div>
        <div v-else>
          暂无账户，请先添加账户。
        </div>
      </div>
    </main>

    <!-- Add Account Modal -->
    <div v-if="showAddModal" class="modal-overlay" @click="showAddModal = false">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3>新增账户</h3>
          <button @click="showAddModal = false" class="btn-close">&times;</button>
        </div>
        <div class="modal-body">
          <form @submit.prevent="addAccount">
            <div class="form-group">
              <label for="nickname">账户昵称</label>
              <input 
                type="text" 
                id="nickname" 
                v-model="newAccount.nickname" 
                required 
                placeholder="例如：现金、银行卡"
              >
            </div>
            <div class="form-group">
              <label for="actualName">实际账户名称</label>
              <input 
                type="text" 
                id="actualName" 
                v-model="newAccount.actualName" 
                required 
                placeholder="例如：Assets:Cash、Assets:BankCard"
              >
            </div>
            <div class="form-group">
              <label for="type">账户类型</label>
              <select id="type" v-model="newAccount.type" required>
                <option value="asset">资产账户</option>
                <option value="liability">负债账户</option>
                <option value="income">收入账户</option>
                <option value="expense">支出账户</option>
              </select>
            </div>
            <div class="form-actions">
              <button type="button" @click="showAddModal = false" class="btn btn-secondary mr-2">取消</button>
              <button type="submit" class="btn btn-primary">添加账户</button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
const { data: accountsData, refresh: refreshAccounts } = await useAsyncData('accounts', async () => {
  const response = await $fetch('/api/accounts')
  return response.accounts || []
})

const accounts = computed(() => accountsData.value || [])
const showAddModal = ref(false)
const { openModal } = useTransactionModal()

// 搜索和过滤相关
const searchQuery = ref('')
const activeFilter = ref('all')
const filterTabs = [
  { label: '全部', value: 'all' },
  { label: '资产', value: 'asset' },
  { label: '负债', value: 'liability' },
  { label: '收入', value: 'income' },
  { label: '支出', value: 'expense' }
]

// 过滤后的账户列表
const filteredAccounts = computed(() => {
  return accounts.value.filter(account => {
    // 搜索过滤
    const matchesSearch = !searchQuery.value || 
      account.nickname.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      account.actualName.toLowerCase().includes(searchQuery.value.toLowerCase())
    
    // 类型过滤
    const matchesFilter = activeFilter.value === 'all' || account.type === activeFilter.value
    
    return matchesSearch && matchesFilter
  }).sort((a, b) => {
    // 按类型排序，然后按余额排序
    if (a.type !== b.type) {
      const typeOrder = { asset: 0, liability: 1, income: 2, expense: 3 }
      return typeOrder[a.type] - typeOrder[b.type]
    }
    return b.balance - a.balance
  })
})

// 获取账户类型标签
function getAccountTypeLabel(type) {
  const typeLabels = {
    asset: '资产账户',
    liability: '负债账户',
    income: '收入账户',
    expense: '支出账户'
  }
  return typeLabels[type] || type
}

// 按类型获取账户数量
function getAccountCountByType(type) {
  if (type === 'all') {
    return accounts.value.length
  }
  return accounts.value.filter(account => account.type === type).length
}

const newAccount = ref({
  nickname: '',
  actualName: '',
  type: 'asset'
})

// 添加账户
async function addAccount() {
  try {
    await $fetch('/api/accounts', {
      method: 'POST',
      body: newAccount.value
    })
    
    // 重置表单
    newAccount.value = {
      nickname: '',
      actualName: '',
      type: 'asset'
    }
    
    // 关闭模态框
    showAddModal.value = false
    
    // 刷新账户列表
    await refreshAccounts()
    
    // 显示成功提示
    alert('账户添加成功！')
  } catch (error) {
    console.error('添加账户失败:', error)
    alert('添加账户失败，请稍后重试。')
  }
}

// 删除账户
async function deleteAccount(accountName) {
  if (!confirm(`确定要删除账户 "${accountName}" 吗？`)) {
    return
  }
  
  try {
    await $fetch(`/api/accounts/${encodeURIComponent(accountName)}`, {
      method: 'DELETE'
    })
    
    // 刷新账户列表
    await refreshAccounts()
    
    // 显示成功提示
    alert('账户删除成功！')
  } catch (error) {
    console.error('删除账户失败:', error)
    const errorMessage = error.data?.error || '删除账户失败，请稍后重试。'
    alert(errorMessage)
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

/* Filter Section */
.filter-section {
  background-color: #f8fafc;
  padding: 1.5rem;
  border-radius: 8px;
  margin-bottom: 1.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.search-box {
  margin-bottom: 1rem;
}

.search-input {
  width: 100%;
  padding: 0.75rem 1rem;
  border: 2px solid #e2e8f0;
  border-radius: 6px;
  font-size: 1rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.search-input:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.filter-tabs {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.filter-tab {
  padding: 0.5rem 1rem;
  border: 2px solid #e2e8f0;
  border-radius: 20px;
  background-color: white;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.filter-tab:hover {
  border-color: #cbd5e1;
  background-color: #f1f5f9;
}

.filter-tab.active {
  border-color: #3b82f6;
  background-color: #3b82f6;
  color: white;
}

.filter-count {
  background-color: rgba(255, 255, 255, 0.2);
  padding: 0.125rem 0.5rem;
  border-radius: 10px;
  font-size: 0.75rem;
  font-weight: 600;
}

.filter-tab:not(.active) .filter-count {
  background-color: #e2e8f0;
  color: #475569;
}

/* Accounts Grid */
.accounts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.5rem;
}

.account-card {
  background-color: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  transition: transform 0.2s, box-shadow 0.2s, border-color 0.2s;
  border-left: 4px solid #e2e8f0;
  position: relative;
}

.account-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.1);
}

/* Account Type Colors */
.account-type-asset {
  border-left-color: #10b981;
}

.account-type-liability {
  border-left-color: #ef4444;
}

.account-type-income {
  border-left-color: #3b82f6;
}

.account-type-expense {
  border-left-color: #f59e0b;
}

.account-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.account-name {
  font-size: 1.25rem;
  font-weight: 600;
  color: #334155;
  text-decoration: none;
  max-width: 70%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.account-name:hover {
  text-decoration: underline;
}

.btn-delete {
  background: none;
  border: none;
  font-size: 1.5rem;
  color: #ef4444;
  cursor: pointer;
  padding: 0;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: background-color 0.2s;
  flex-shrink: 0;
}

.btn-delete:hover {
  background-color: #fee2e2;
}

.account-actual-name {
  font-size: 0.875rem;
  color: #64748b;
  margin-bottom: 1rem;
  word-break: break-all;
}

.account-balance {
  font-size: 1.75rem;
  font-weight: 700;
  margin-bottom: 0.5rem;
  color: #16a34a;
}

.account-type-liability .account-balance {
  color: #ef4444;
}

.account-type-income .account-balance {
  color: #3b82f6;
}

.account-type-expense .account-balance {
  color: #f59e0b;
}

.account-transactions {
  font-size: 0.875rem;
  color: #64748b;
  margin-bottom: 1rem;
}

.account-type-badge {
  position: absolute;
  top: 1rem;
  right: 1rem;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  color: white;
  text-transform: uppercase;
}

.account-type-asset .account-type-badge {
  background-color: #10b981;
}

.account-type-liability .account-type-badge {
  background-color: #ef4444;
}

.account-type-income .account-type-badge {
  background-color: #3b82f6;
}

.account-type-expense .account-type-badge {
  background-color: #f59e0b;
}

/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background-color: white;
  border-radius: 8px;
  width: 90%;
  max-width: 500px;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  background-color: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.btn-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #64748b;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: background-color 0.2s;
}

.btn-close:hover {
  background-color: #e2e8f0;
}

.modal-body {
  padding: 1.5rem;
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: #334155;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 0.75rem;
  border: 2px solid #e2e8f0;
  border-radius: 6px;
  font-size: 1rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  margin-top: 2rem;
}

/* Responsive Design */
@media (max-width: 640px) {
  .accounts-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }
  
  .filter-tabs {
    justify-content: center;
  }
  
  .filter-tab {
    padding: 0.4rem 0.8rem;
    font-size: 0.8rem;
  }
  
  .account-card {
    padding: 1.25rem;
  }
  
  .account-name {
    font-size: 1.125rem;
  }
  
  .account-balance {
    font-size: 1.5rem;
  }
}
</style>