<template>
  <div class="container">
    <header>
      <nav>
        <div class="flex items-center">
          <NuxtLink to="/accounts" class="btn btn-secondary mr-2">返回账户列表</NuxtLink>
          <div class="logo">{{ accountInfo?.nickname || accountName }}</div>
        </div>
        <div>
          <NuxtLink to="/" class="btn btn-secondary mr-2">返回首页</NuxtLink>
          <NuxtLink to="/transactions" class="btn btn-primary">记录交易</NuxtLink>
        </div>
      </nav>
    </header>

    <main class="mt-6">
      <div class="card mb-6">
        <div class="account-info-header">
          <h2 class="text-xl font-bold mb-2">账户余额</h2>
          <div class="account-actual-name">{{ accountInfo?.actualName }}</div>
        </div>
        <div class="account-balance-display">{{ formatCurrency(accountBalance) }}</div>
      </div>
      <!-- Set Balance Form -->
      <div class="card mb-6">
        <h2 class="text-xl font-bold mb-4">设置账户余额</h2>
        <form @submit.prevent="setAccountBalance">
          <div class="grid md:grid-cols-2 gap-4">
            <div class="form-group">
              <label for="balanceDate">日期</label>
              <input type="date" id="balanceDate" v-model="balanceForm.date" required>
            </div>
            <div class="form-group">
              <label for="targetBalance">目标余额</label>
              <input type="number" id="targetBalance" v-model.number="balanceForm.balance" step="0.01" required placeholder="输入目标余额">
            </div>
            <div class="form-group md:col-span-2">
              <label for="padAccount">差额平衡账户</label>
              <select id="padAccount" v-model="balanceForm.padAccount" required>
                <option value="" disabled>选择平衡账户</option>
                <option v-for="account in accountsData" :key="account.actualName" :value="account.nickname">
                  {{ account.nickname }}
                </option>
              </select>
              <p class="text-sm text-gray-500 mt-1">当设置余额与计算余额不一致时，将通过此账户平衡差额</p>
            </div>
            <div class="form-group md:col-span-2">
              <button type="submit" class="btn btn-primary w-full">设置余额</button>
            </div>
          </div>
        </form>
      </div>

      <!-- Add Transaction Form -->
      <div class="card mb-6">
        <h2 class="text-xl font-bold mb-4">添加{{ accountName }}交易</h2>
        <form @submit.prevent="addAccountTransaction">
          <div class="grid md:grid-cols-2 gap-4">
            <div class="form-group">
              <label for="date">日期</label>
              <input type="date" id="date" v-model="newTransaction.date" required>
            </div>
            <div class="form-group">
              <label for="type">类型</label>
              <select id="type" v-model="newTransaction.type" required>
                <option value="income">收入</option>
                <option value="expense">支出</option>
                <option value="transfer">转账</option>
              </select>
            </div>
            <div class="form-group md:col-span-2">
              <label for="narration">描述</label>
              <input type="text" id="narration" v-model="newTransaction.narration" required>
            </div>
            
            <!-- 多账户交易行 -->
            <div class="form-group md:col-span-2">
              <label>交易明细</label>
              <div v-for="(posting, index) in newTransaction.postings" :key="index" class="flex gap-2 mb-2">
                <select v-model="posting.account" class="flex-1" required>
                  <option value="" disabled>选择账户</option>
                  <option v-for="account in accountsData" :key="account.actualName" :value="account.nickname">
                    {{ account.nickname }}
                  </option>
                </select>
                <input type="number" v-model.number="posting.amount" step="0.01" class="flex-1" required placeholder="金额">
                <button type="button" @click="removePosting(index)" class="btn btn-danger" v-if="newTransaction.postings.length > 2">删除</button>
              </div>
              <button type="button" @click="addPosting" class="btn btn-secondary" v-if="newTransaction.postings.length < 5">添加交易行</button>
            </div>
            
            <div class="form-group md:col-span-2">
              <button type="submit" class="btn btn-primary w-full">保存交易</button>
            </div>
          </div>
        </form>
      </div>

      <!-- Account Transactions -->
      <div class="card">
        <h2 class="text-xl font-bold mb-4">{{ accountName }}交易记录</h2>
        <table>
          <thead>
            <tr>
              <th>日期</th>
              <th>描述</th>
              <th>金额</th>
              <th>类型</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="transaction in accountTransactions" :key="transaction.id">
              <td>{{ transaction.date }}</td>
              <td>{{ transaction.narration }}</td>
              <td>
                <!-- 显示当前账户在该交易中的金额 -->
                <span :class="getAccountAmountClass(transaction)">
                  {{ formatCurrency(getAccountAmount(transaction)) }}
                </span>
              </td>
              <td>
                <span :class="getTransactionTypeClass(transaction)" 
                      class="px-2 py-1 rounded-full text-xs">
                  {{ getTransactionTypeLabel(transaction) }}
                </span>
              </td>
              <td>
                <button @click="deleteTransaction(transaction.id)" class="btn-delete">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="accountTransactions.length === 0" class="text-center text-gray-500 py-4">
          暂无交易记录
        </div>
      </div>
    </main>
  </div>
</template>

<script setup>
const route = useRoute()
const accountParam = computed(() => route.params.account ? decodeURIComponent(route.params.account) : '')
const accountName = computed(() => accountParam.value || '未命名账户')

// 获取所有账户信息
const { data: accountsData } = await useAsyncData('allAccounts', async () => {
  const response = await $fetch('/api/accounts')
  return response.accounts || []
})

// 找到当前账户的详细信息
const accountInfo = computed(() => {
  if (!accountParam.value || !accountsData.value) return null
  return accountsData.value.find(acc => acc.nickname === accountParam.value) || null
})

// 获取账户交易记录
const { data: transactionsData, refresh: refreshTransactions } = await useAsyncData('accountTransactions', async () => {
  if (!accountParam.value) return []
  
  const response = await $fetch(`/api/accounts/${encodeURIComponent(accountParam.value)}/transactions`)
  return response.transactions || []
}, { watch: [accountParam] })

// 获取账户余额
const { data: balanceData, refresh: refreshBalance } = await useAsyncData('accountBalance', async () => {
  if (!accountParam.value) return 0
  
  const response = await $fetch(`/api/accounts/${encodeURIComponent(accountParam.value)}/balance`)
  return response.balance || 0
}, { watch: [accountParam] })

const accountTransactions = computed(() => transactionsData.value || [])
const accountBalance = computed(() => balanceData.value || 0)

// 新交易表单
const newTransaction = ref({
  date: new Date().toISOString().split('T')[0],
  type: 'expense',
  narration: '',
  postings: [
    { account: '', amount: 0 },
    { account: '', amount: 0 }
  ]
})

// 设置余额表单
const balanceForm = ref({
  date: new Date().toISOString().split('T')[0],
  balance: 0,
  padAccount: 'Equity:HistoryIncome'
})

// 监听账户名称变化，更新表单中的固定账户字段
watch(accountName, (newAccount) => {
  if (newTransaction.value.type === 'income') {
    // 收入时，当前账户作为资产账户（接收资金）
    newTransaction.value.postings[1].account = newAccount
  } else if (newTransaction.value.type === 'expense') {
    // 支出时，当前账户作为资产账户（支出资金）
    newTransaction.value.postings[0].account = newAccount
  } else {
    // 转账时，当前账户作为转出账户
    newTransaction.value.postings[0].account = newAccount
  }
}, { immediate: true })

// 监听交易类型变化，自动调整账户设置
watch(() => newTransaction.value.type, (newType) => {
  // 当前字段为actualName
  const currentAccount = accountName.value 
  // 提取currentAccount的nickname
  const currentAccountNickname = accountsData.value.find(acc => acc.actualName === currentAccount)?.nickname || ''
  if (!accountsData.value) return
  
  if (newType === 'income') {
    // 收入时，设置收入账户和当前资产账户
    const firstIncomeAccount = accountsData.value.filter(acc => acc.actualName.startsWith('Income:')).shift()
    newTransaction.value.postings[0].account = ''
    newTransaction.value.postings[1].account = currentAccountNickname
    
    // 设置金额
    const amount = Math.abs(newTransaction.value.postings[0].amount) || 100
    newTransaction.value.postings[0].amount = -amount
    newTransaction.value.postings[1].amount = amount
  } else if (newType === 'expense') {
    // 支出时，设置当前资产账户和支出账户
    const firstExpenseAccount = accountsData.value.filter(acc => acc.actualName.startsWith('Expenses:')).shift()

    newTransaction.value.postings[0].account = currentAccountNickname
    newTransaction.value.postings[1].account = ''
    
    // 设置金额
    const amount = Math.abs(newTransaction.value.postings[0].amount) || 100
    newTransaction.value.postings[0].amount = -amount
    newTransaction.value.postings[1].amount = amount
  } else if (newType === 'transfer') {
    // 转账时，设置当前账户和另一个资产账户
    newTransaction.value.postings[0].account = currentAccountNickname
    
    // 找到另一个资产账户
    const otherAssetAccount = accountsData.value
      .filter(acc => acc.actualName.startsWith('Assets:') && acc.nickname !== currentAccountNickname)
      .shift()
    
    newTransaction.value.postings[1].account = ''
    
    // 设置金额
    const amount = Math.abs(newTransaction.value.postings[0].amount) || 100
    newTransaction.value.postings[0].amount = -amount
    newTransaction.value.postings[1].amount = amount
  }
}, { immediate: true })

// 添加交易行
function addPosting() {
  if (newTransaction.value.postings.length < 5) {
    newTransaction.value.postings.push({ account: '', amount: 0 })
  }
}

// 删除交易行
function removePosting(index) {
  if (newTransaction.value.postings.length > 2) {
    newTransaction.value.postings.splice(index, 1)
  }
}

// 添加账户交易
async function addAccountTransaction() {
  try {
    const response = await $fetch('/api/transactions', {
      method: 'POST',
      body: newTransaction.value
    })
    
    if (response.success) {
      // 重置表单
      newTransaction.value = {
        date: new Date().toISOString().split('T')[0],
        type: 'expense',
        narration: '',
        postings: [
          { account: accountName.value, amount: 0 },
          { account: '', amount: 0 }
        ]
      }
      
      // 刷新交易记录和余额
      await refreshTransactions()
      await refreshBalance()
      
      // 显示成功提示
      alert('交易记录添加成功！')
    } else {
      // 显示错误信息
      alert(`添加交易失败: ${response.error || '未知错误'}`)
    }
  } catch (error) {
    console.error('添加交易失败:', error)
    alert('添加交易失败，请稍后重试。')
  }
}

// 删除交易
async function deleteTransaction(id) {
  if (confirm('确定要删除这条交易记录吗？')) {
    try {
      await $fetch(`/api/transactions/${id}`, {
        method: 'DELETE'
      })
      
      // 刷新交易记录和余额
      await refreshTransactions()
      await refreshBalance()
      
      // 显示成功提示
      alert('交易记录删除成功！')
    } catch (error) {
      console.error('删除交易失败:', error)
      alert('删除交易失败，请稍后重试。')
    }
  }
}

// 设置账户余额
async function setAccountBalance() {
  try {
    const response = await $fetch(`/api/accounts/${encodeURIComponent(accountName.value)}/balance`, {
      method: 'POST',
      body: balanceForm.value
    })
    
    if (response.success) {
      // 刷新交易记录和余额
      await refreshTransactions()
      await refreshBalance()
      
      // 显示成功提示
      alert('账户余额设置成功！')
      
      // 重置表单
      balanceForm.value = {
        date: new Date().toISOString().split('T')[0],
        balance: 0,
        padAccount: ''
      }
    } else {
      // 显示错误信息
      alert(`设置账户余额失败: ${response.error || '未知错误'}`)
    }
  } catch (error) {
    console.error('设置账户余额失败:', error)
    alert('设置账户余额失败，请稍后重试。')
  }
}

// 辅助函数：获取当前账户在交易中的金额
function getAccountAmount(transaction) {
  const currentAccount = accountName.value
  const posting = transaction.postings.find(p => p.account === currentAccount)
  return posting ? posting.amount : 0
}

// 辅助函数：根据金额正负返回样式类
function getAccountAmountClass(transaction) {
  const amount = getAccountAmount(transaction)
  return amount >= 0 ? 'text-green-600' : 'text-red-600'
}

// 辅助函数：根据交易类型返回样式类
function getTransactionTypeClass(transaction) {
  if (transaction.type === 'income') {
    return 'bg-green-100 text-green-800'
  } else if (transaction.type === 'expense') {
    return 'bg-red-100 text-red-800'
  } else {
    return 'bg-blue-100 text-blue-800'
  }
}

// 辅助函数：根据交易类型返回中文标签
function getTransactionTypeLabel(transaction) {
  if (transaction.type === 'income') {
    return '收入'
  } else if (transaction.type === 'expense') {
    return '支出'
  } else {
    return '转账'
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

.mb-2 {
  margin-bottom: 0.5rem;
}

.text-xl {
  font-size: 1.25rem;
}

.flex {
  display: flex;
}

.items-center {
  align-items: center;
}

.mr-2 {
  margin-right: 0.5rem;
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

.account-info-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 1rem;
}

.account-actual-name {
  font-size: 0.875rem;
  color: #6b7280;
  font-family: monospace;
}

.btn-delete {
  background: none;
  border: none;
  color: #ef4444;
  cursor: pointer;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.875rem;
  transition: background-color 0.2s;
}

.btn-delete:hover {
  background-color: #fee2e2;
}

.account-balance-display {
  font-size: 2rem;
  font-weight: bold;
  color: #16a34a;
}

.mb-6 {
  margin-bottom: 1.5rem;
}

.text-red-600 {
  color: #ef4444;
}

.text-green-600 {
  color: #16a34a;
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

.bg-green-100 {
  background-color: #dcfce7;
}

.text-green-800 {
  color: #166534;
}

.bg-red-100 {
  background-color: #fee2e2;
}

.text-red-800 {
  color: #991b1b;
}
</style>