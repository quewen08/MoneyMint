<template>
  <div class="mx-auto">
    <div class="flex justify-between items-center mb-6">
      <h2 class="text-2xl font-bold">账户管理</h2>
      <div class="flex gap-3">
        <button
          @click="showAddAccountModal = true"
          class="btn btn-primary justify-items-center"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 mr-1"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 4v16m8-8H4"
            />
          </svg>
          新增账户
        </button>
        <button
          @click="refreshData"
          class="btn btn-secondary justify-items-center"
          :disabled="loading"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 mr-1"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
          刷新
        </button>
      </div>
    </div>

    <!-- 日期范围选择器 -->
    <div class="card p-4 mb-6">
      <div class="flex flex-col md:flex-row gap-4 items-center">
        <div class="flex-1">
          <label class="block text-sm font-medium text-gray-700 mb-1"
            >开始日期</label
          >
          <input
            v-model="dateRange.startDate"
            type="date"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
          />
        </div>
        <div class="flex-1">
          <label class="block text-sm font-medium text-gray-700 mb-1"
            >结束日期</label
          >
          <input
            v-model="dateRange.endDate"
            type="date"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
          />
        </div>
        <div class="flex gap-3 pt-4 md:pt-0">
          <button
            @click="applyDateRange"
            class="btn btn-primary"
            :disabled="loading"
          >
            应用筛选
          </button>
          <button @click="clearDateRange" class="btn btn-secondary">
            清除筛选
          </button>
        </div>
      </div>
    </div>

    <!-- 账户统计信息 -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
      <div class="card p-4 bg-blue-50">
        <div class="text-sm text-blue-700 mb-1">总资产</div>
        <div class="text-2xl font-bold text-blue-900">
          {{ formatCurrency(totalAssets) }}
        </div>
      </div>
      <div class="card p-4 bg-red-50">
        <div class="text-sm text-red-700 mb-1">总负债</div>
        <div class="text-2xl font-bold text-red-900">
          {{ formatCurrency(totalLiabilities) }}
        </div>
      </div>
      <div class="card p-4 bg-green-50">
        <div class="text-sm text-green-700 mb-1">净资产</div>
        <div class="text-2xl font-bold text-green-900">
          {{ formatCurrency(totalNetWorth) }}
        </div>
      </div>
    </div>

    <div class="card">
      <div v-if="loading" class="text-center py-8">
        <div
          class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
        ></div>
      </div>

      <div
        v-else-if="accountList.length === 0"
        class="text-center py-8 text-gray-500"
      >
        <p>暂无账户</p>
      </div>

      <div v-else class="space-y-6">
        <!-- 按账户类型分组显示 -->
        <div v-for="(accountGroup, type) in groupedAccounts" :key="type">
          <div class="flex justify-between items-center mb-3">
            <h3 class="text-lg font-semibold text-gray-700">
              {{ getAccountTypeName(type) }}
            </h3>
            <span class="text-sm text-gray-500">
              {{ formatCurrency(getGroupTotal(type)) }}
            </span>
          </div>
          <div class="space-y-2">
            <div
              v-for="account in accountGroup"
              :key="account.name"
              class="flex justify-between items-center p-3 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors cursor-pointer"
              @click="showAccountDetails(account)"
            >
              <div class="flex items-center">
                <div
                  class="w-8 h-8 rounded-full flex items-center justify-center mr-3"
                  :class="getAccountTypeColor(type)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-4 w-4 text-white"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z"
                    />
                  </svg>
                </div>
                <div>
                  <div class="font-medium">{{ account.name }}</div>
                  <div class="text-xs text-gray-500">
                    {{ account.note || account.type }}
                  </div>
                </div>
              </div>
              <div class="text-right">
                <div
                  class="font-medium"
                  :class="
                    account.balance >= 0 ? 'text-green-600' : 'text-red-600'
                  "
                >
                  {{ formatCurrency(account.balance) }}
                </div>
                <div class="text-xs text-gray-500">{{ account.currency }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 账户详情弹窗 -->
    <div
      v-if="selectedAccount"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
    >
      <div
        class="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto"
      >
        <div class="p-6">
          <div class="flex justify-between items-center mb-4">
            <h3 class="text-xl font-bold">账户详情</h3>
            <button
              @click="selectedAccount = null"
              class="text-gray-500 hover:text-gray-700"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <div class="space-y-4">
            <div class="grid grid-cols-2 gap-4">
              <div>
                <div class="text-sm text-gray-500 mb-1">账户名称</div>
                <div class="font-medium">{{ selectedAccount.name }}</div>
              </div>
              <div>
                <div class="text-sm text-gray-500 mb-1">账户类型</div>
                <div class="font-medium">
                  {{ getAccountTypeName(selectedAccount.type) }}
                </div>
              </div>
              <div>
                <div class="text-sm text-gray-500 mb-1">账户说明</div>
                <div class="font-medium">
                  {{ selectedAccount.note || "无" }}
                </div>
              </div>
              <div>
                <div class="text-sm text-gray-500 mb-1">货币单位</div>
                <div class="font-medium">{{ selectedAccount.currency }}</div>
              </div>
              <div class="col-span-2">
                <div class="text-sm text-gray-500 mb-1">当前余额</div>
                <div
                  class="text-2xl font-bold"
                  :class="
                    selectedAccount.balance >= 0
                      ? 'text-green-600'
                      : 'text-red-600'
                  "
                >
                  {{ formatCurrency(selectedAccount.balance) }}
                  {{ selectedAccount.currency }}
                </div>
              </div>
            </div>
            <div class="pt-4 border-t border-gray-200 space-y-2">
              <button
                @click="showSetBalanceModal = true"
                class="btn btn-primary w-full flex items-center justify-center"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 mr-1"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z"
                  />
                </svg>
                设置余额
              </button>
              <button
                @click="showCloseAccountModal = true"
                class="btn btn-danger w-full flex items-center justify-center"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 mr-1"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
                关闭账户
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 新增账户弹窗 -->
    <div
      v-if="showAddAccountModal"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
    >
      <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
        <div class="p-6">
          <div class="flex justify-between items-center mb-4">
            <h3 class="text-xl font-bold">新增账户</h3>
            <button
              @click="showAddAccountModal = false"
              class="text-gray-500 hover:text-gray-700"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <form @submit.prevent="handleAddAccount">
            <div class="space-y-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1"
                  >账户名称</label
                >
                <input
                  v-model="newAccount.account"
                  type="text"
                  class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                  placeholder="例如：Assets:Cash"
                  required
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1"
                  >账户说明</label
                >
                <input
                  v-model="newAccount.note"
                  type="text"
                  class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                  placeholder="例如：钱包现金"
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1"
                  >货币单位</label
                >
                <input
                  v-model="newAccount.currency"
                  type="text"
                  class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                  placeholder="例如：CNY"
                  required
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1"
                  >日期</label
                >
                <input
                  v-model="newAccount.date"
                  type="date"
                  class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                  required
                />
              </div>
              <div class="pt-4">
                <button
                  type="submit"
                  class="btn btn-primary w-full"
                  :disabled="submitting"
                >
                  <span
                    v-if="submitting"
                    class="inline-block animate-spin rounded-full h-4 w-4 mr-2 border-b-2 border-white"
                  ></span>
                  新增账户
                </button>
              </div>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- 关闭账户弹窗 -->
    <div
      v-if="showCloseAccountModal"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
    >
      <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
        <div class="p-6">
          <div class="flex justify-between items-center mb-4">
            <h3 class="text-xl font-bold">关闭账户</h3>
            <button
              @click="showCloseAccountModal = false"
              class="text-gray-500 hover:text-gray-700"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <div class="space-y-4">
            <div class="text-sm text-gray-600">
              确定要关闭账户
              <span class="font-medium">{{ selectedAccount?.name }}</span> 吗？
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1"
                >日期</label
              >
              <input
                v-model="closeAccountData.date"
                type="date"
                class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                required
              />
            </div>
            <div class="pt-4 flex gap-3">
              <button
                @click="showCloseAccountModal = false"
                class="btn btn-secondary flex-1"
              >
                取消
              </button>
              <button
                @click="handleCloseAccount"
                class="btn btn-danger flex-1"
                :disabled="submitting"
              >
                <span
                  v-if="submitting"
                  class="inline-block animate-spin rounded-full h-4 w-4 mr-2 border-b-2 border-white"
                ></span>
                确认关闭
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 设置账户余额弹窗 -->
    <div
      v-if="showSetBalanceModal"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
    >
      <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
        <div class="p-6">
          <div class="flex justify-between items-center mb-4">
            <h3 class="text-xl font-bold">设置账户余额</h3>
            <button
              @click="showSetBalanceModal = false"
              class="text-gray-500 hover:text-gray-700"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <form @submit.prevent="handleSetBalance">
            <div class="space-y-4">
              <div class="text-sm text-gray-600">
                为账户
                <span class="font-medium">{{ selectedAccount?.name }}</span>
                设置余额
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1"
                  >日期</label
                >
                <input
                  v-model="setBalanceData.date"
                  type="date"
                  class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                  required
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1"
                  >余额</label
                >
                <input
                  v-model="setBalanceData.amount"
                  type="number"
                  step="0.01"
                  class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                  placeholder="输入金额"
                  required
                />
              </div>
              <div class="pt-4 flex gap-3">
                <button
                  @click="showSetBalanceModal = false"
                  class="btn btn-secondary flex-1"
                >
                  取消
                </button>
                <button
                  type="submit"
                  class="btn btn-primary flex-1"
                  :disabled="submitting"
                >
                  <span
                    v-if="submitting"
                    class="inline-block animate-spin rounded-full h-4 w-4 mr-2 border-b-2 border-white"
                  ></span>
                  确认设置
                </button>
              </div>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { useApi } from "~/composables/useApi";

const { accounts } = useApi();

const loading = ref(true);
const submitting = ref(false);
const accountList = ref<any[]>([]);
const selectedAccount = ref<any>(null);
const showAddAccountModal = ref(false);
const showCloseAccountModal = ref(false);
const showSetBalanceModal = ref(false);
const dateRange = ref({ startDate: "", endDate: "" });

// 新增账户表单数据
const newAccount = ref({
  account: "",
  note: "",
  currency: "CNY",
  date: new Date().toISOString().split("T")[0],
});

// 关闭账户表单数据
const closeAccountData = ref({
  date: new Date().toISOString().split("T")[0],
});

// 设置余额表单数据
const setBalanceData = ref({
  date: new Date().toISOString().split("T")[0],
  amount: 0,
});

// 按账户类型分组
const groupedAccounts = computed(() => {
  const groups: Record<string, any[]> = {};

  accountList.value.forEach((account) => {
    const type = account.type;
    if (!groups[type]) {
      groups[type] = [];
    }
    groups[type].push(account);
  });

  return groups;
});

// 获取账户类型名称
const getAccountTypeName = (type: string): string => {
  switch (type) {
    case "Assets":
      return "资产账户";
    case "Liabilities":
      return "负债账户";
    case "Equity":
      return "权益账户";
    case "Income":
      return "收入账户";
    case "Expenses":
      return "支出账户";
    default:
      return type;
  }
};

// 获取账户类型颜色
const getAccountTypeColor = (type: string): string => {
  switch (type) {
    case "Assets":
      return "bg-blue-500";
    case "Liabilities":
      return "bg-red-500";
    case "Equity":
      return "bg-purple-500";
    case "Income":
      return "bg-green-500";
    case "Expenses":
      return "bg-orange-500";
    default:
      return "bg-gray-500";
  }
};

// 获取分组总额
const getGroupTotal = (type: string): number => {
  const group = groupedAccounts.value[type] || [];
  return group.reduce((total, account) => total + account.balance, 0);
};

// 计算统计数据
const totalAssets = computed(() => getGroupTotal("Assets"));
const totalLiabilities = computed(() => getGroupTotal("Liabilities"));
const totalNetWorth = computed(
  () => totalAssets.value - totalLiabilities.value
);

// 格式化货币
const formatCurrency = (amount: number): string => {
  return amount.toFixed(2);
};

// 显示账户详情
const showAccountDetails = (account: any) => {
  selectedAccount.value = account;
};

// 处理新增账户
const handleAddAccount = async () => {
  try {
    submitting.value = true;

    const accountData = {
      type: "Open",
      date: newAccount.value.date,
      account: newAccount.value.account,
      note: newAccount.value.note,
      currency: newAccount.value.currency,
    };

    await accounts.openAccount(accountData);

    // 重置表单
    newAccount.value = {
      account: "",
      note: "",
      currency: "CNY",
      date: new Date().toISOString().split("T")[0],
    };

    showAddAccountModal.value = false;
    await loadAccounts();
  } catch (error) {
    console.error("Error adding account:", error);
  } finally {
    submitting.value = false;
  }
};

// 处理关闭账户
const handleCloseAccount = async () => {
  try {
    submitting.value = true;

    const accountData = {
      type: "Close",
      date: closeAccountData.value.date,
      account: selectedAccount.value.name,
    };

    await accounts.closeAccount(accountData);

    showCloseAccountModal.value = false;
    selectedAccount.value = null;
    await loadAccounts();
  } catch (error) {
    console.error("Error closing account:", error);
  } finally {
    submitting.value = false;
  }
};

// 处理设置账户余额
const handleSetBalance = async () => {
  try {
    submitting.value = true;

    const balanceData = {
      type: "Balance",
      date: setBalanceData.value.date,
      account: selectedAccount.value.name,
      amount: `${setBalanceData.value.amount} ${selectedAccount.value.currency}`,
    };

    await accounts.setBalance(balanceData);

    showSetBalanceModal.value = false;
    selectedAccount.value = null;
    await loadAccounts();
  } catch (error) {
    console.error("Error setting balance:", error);
  } finally {
    submitting.value = false;
  }
};

// 加载账户数据
const loadAccounts = async (
  dateRangeParams: { start_date?: string; end_date?: string } = {}
) => {
  try {
    loading.value = true;
    accountList.value = await accounts.getAccountBalances(dateRangeParams);
  } catch (error) {
    console.error("Error loading accounts:", error);
  } finally {
    loading.value = false;
  }
};

// 应用日期范围筛选
const applyDateRange = async () => {
  await loadAccounts({
    start_date: dateRange.value.startDate,
    end_date: dateRange.value.endDate,
  });
};

// 清除日期范围筛选
const clearDateRange = async () => {
  dateRange.value = { startDate: "", endDate: "" };
  await loadAccounts();
};

// 刷新数据
const refreshData = async () => {
  await loadAccounts({
    start_date: dateRange.value.startDate,
    end_date: dateRange.value.endDate,
  });
};

onMounted(async () => {
  await loadAccounts();

  // 监听全局SSE事件
  window.addEventListener("sse:data-updated", handleSseUpdate);
  // 监听手动触发的刷新事件
  window.addEventListener("refreshData", handleRefreshData);
});

// 处理SSE更新
const handleSseUpdate = (event: Event) => {
  const customEvent = event as CustomEvent;
  const data = customEvent.detail;
  if (data.event === "update" || data.event === "entry_added") {
    console.log("Refreshing accounts due to global SSE event...");
    loadAccounts();
  }
};

// 处理手动触发的刷新事件
const handleRefreshData = () => {
  console.log("Refreshing accounts due to manual refresh event...");
  loadAccounts();
};

// 组件卸载时移除事件监听
onUnmounted(() => {
  window.removeEventListener("sse:data-updated", handleSseUpdate);
  window.removeEventListener("refreshData", handleRefreshData);
});
</script>
