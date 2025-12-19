<template>
  <div>
    <!-- 页面标题 -->
    <h1 class="text-2xl font-bold mb-6 dark:text-white">首页</h1>

    <!-- 错误信息显示 -->
    <div
      v-if="ledger.errors && ledger.errors.length > 0"
      class="card bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 mb-6"
    >
      <h2 class="text-xl font-semibold mb-3 text-red-800 dark:text-red-300">账本错误</h2>
      <div class="space-y-3 max-h-60 overflow-y-auto">
        <div
          v-for="(error, index) in ledger.errors"
          :key="index"
          class="bg-white dark:bg-gray-700 p-3 rounded shadow-sm border-l-4 border-red-500"
        >
          <div class="flex justify-between items-start mb-1">
            <span class="font-medium text-red-700 dark:text-red-300">错误 {{ index + 1 }}</span>
            <span class="text-xs bg-red-100 dark:bg-red-800 text-red-800 dark:text-red-200 px-2 py-1 rounded">{{
              error.severity || "Error"
            }}</span>
          </div>
          <p class="text-sm text-gray-700 dark:text-gray-300 mb-1">{{ error.message }}</p>
          <div v-if="error.source" class="text-xs text-gray-500 dark:text-gray-400">
            <span>{{ error.source.filename }}:{{ error.source.lineno }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 主内容区 -->
    <div class="mx-auto py-8">
      <!-- 仪表盘统计卡片 -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div class="card bg-gradient-to-br from-blue-50 to-blue-100 dark:from-blue-900/30 dark:to-blue-800/30 p-6">
          <div class="flex justify-between items-center">
            <div>
              <p class="text-sm text-blue-700 dark:text-blue-300 font-medium">总收入</p>
              <p class="text-2xl font-bold text-blue-900 dark:text-blue-200 mt-1">
                {{ dashboardStats.totalIncome }} {{ ledger.currency }}
              </p>
            </div>
            <div class="bg-blue-200 dark:bg-blue-700 rounded-full p-3">
              <span class="text-xl">📈</span>
            </div>
          </div>
        </div>

        <div class="card bg-gradient-to-br from-red-50 to-red-100 dark:from-red-900/30 dark:to-red-800/30 p-6">
          <div class="flex justify-between items-center">
            <div>
              <p class="text-sm text-red-700 dark:text-red-300 font-medium">总支出</p>
              <p class="text-2xl font-bold text-red-900 dark:text-red-200 mt-1">
                {{ dashboardStats.totalExpense }} {{ ledger.currency }}
              </p>
            </div>
            <div class="bg-red-200 dark:bg-red-700 rounded-full p-3">
              <span class="text-xl">📉</span>
            </div>
          </div>
        </div>

        <div class="card bg-gradient-to-br from-green-50 to-green-100 dark:from-green-900/30 dark:to-green-800/30 p-6">
          <div class="flex justify-between items-center">
            <div>
              <p class="text-sm text-green-700 dark:text-green-300 font-medium">净收入</p>
              <p class="text-2xl font-bold text-green-900 dark:text-green-200 mt-1">
                {{ dashboardStats.netIncome }} {{ ledger.currency }}
              </p>
            </div>
            <div class="bg-green-200 dark:bg-green-700 rounded-full p-3">
              <span class="text-xl">💰</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 分类统计图表（简单版） -->
      <div class="card mb-8">
        <h2 class="text-xl font-semibold mb-4 dark:text-white">分类支出</h2>
        <div class="space-y-3">
          <div
            v-for="(category, index) in dashboardStats.expenseByCategory"
            :key="index"
            class="flex justify-between items-center"
          >
            <div class="flex items-center">
              <div
                class="w-3 h-3 rounded-full mr-2"
                :style="{ backgroundColor: getCategoryColor(index) }"
              ></div>
              <span class="text-gray-700 dark:text-gray-300">{{ category.name }}</span>
            </div>
            <div class="text-right">
              <span class="font-medium dark:text-gray-200"
                >{{ category.amount }} {{ ledger.currency }}</span
              >
              <span class="text-sm text-gray-500 dark:text-gray-400 ml-2"
                >({{ category.percentage }}%)</span
              >
            </div>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- 左侧：账本信息和操作按钮 -->
        <div class="lg:col-span-1">
          <div class="card mb-6">
            <h2 class="text-xl font-semibold mb-4 dark:text-white">账本信息</h2>
            <div v-if="loading" class="text-center py-4">
              <div
                class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
              ></div>
            </div>
            <div v-else class="space-y-3">
              <div class="flex justify-between items-center">
                <span class="text-gray-600 dark:text-gray-400">账本名称:</span>
                <span class="font-medium dark:text-gray-300">{{ ledger.title }}</span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-gray-600 dark:text-gray-400">主要货币:</span>
                <span class="font-medium dark:text-gray-300">{{ ledger.currency }}</span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-gray-600 dark:text-gray-400">记账条目:</span>
                <span class="font-medium dark:text-gray-300">{{ ledger.entries_count }}</span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-gray-600 dark:text-gray-400">错误数量:</span>
                <span class="font-medium text-red-500">{{
                  ledger.errors_count
                }}</span>
              </div>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <button
              @click="showAddModal = true"
              class="btn btn-primary flex flex-col items-center p-6"
            >
              <span class="text-3xl mb-2">📝</span>
              <span>添加记录</span>
            </button>
            <button
              @click="$router.push('/entries')"
              class="btn btn-secondary flex flex-col items-center p-6"
            >
              <span class="text-3xl mb-2">📋</span>
              <span>查看记录</span>
            </button>
            <button
              @click="$router.push('/stats')"
              class="btn btn-info flex flex-col items-center p-6"
            >
              <span class="text-3xl mb-2">📊</span>
              <span>查看统计</span>
            </button>
          </div>
        </div>

        <!-- 右侧：最近记录 -->
        <div class="lg:col-span-2">
          <div class="card">
            <h2 class="text-xl font-semibold mb-4 dark:text-white">最近记录</h2>
            <div v-if="loading" class="text-center py-4">
              <div
                class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
              ></div>
            </div>
            <div
              v-else-if="entries.length === 0"
              class="text-center py-8 text-gray-500 dark:text-gray-400"
            >
              <p>暂无记账记录</p>
            </div>
            <div v-else class="space-y-4">
              <div
                v-for="entry in entries
                  .filter((e) => e.type === 'Transaction')
                  .slice(0, 5)"
                :key="entry.meta.filename + entry.meta.lineno"
                class="border-b pb-3 last:border-0 hover:bg-gray-50 dark:hover:bg-gray-800/70 p-2 rounded transition-colors"
              >
                <div class="flex flex-col space-y-1">
                  <!-- 日期和类型 -->
                  <div class="flex justify-between items-center">
                    <span class="font-medium dark:text-gray-300">{{ entry.date }}</span>
                    <span
                      class="text-sm px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
                      >{{ entry.type }}</span
                    >
                  </div>

                  <!-- 交易描述 -->
                  <div
                    v-if="entry.type === 'Transaction' && entry.narration"
                    class="text-sm text-gray-700 dark:text-gray-300 ml-2"
                  >
                    {{ entry.narration }}
                  </div>

                  <!-- 标签 -->
                  <div
                    v-if="entry.tags && entry.tags.length > 0"
                    class="flex flex-wrap gap-1 ml-2"
                  >
                    <span
                      v-for="(tag, index) in entry.tags"
                      :key="index"
                      class="inline-block bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs px-2 py-1 rounded"
                    >
                      #{{ tag }}
                    </span>
                  </div>

                  <!-- 操作按钮 -->
                  <div class="ml-2 mt-1">
                    <button
                      @click="
                        openEditModal({
                          ...entry,
                          id: `${entry.meta?.filename}:${entry.meta?.lineno}`,
                        })
                      "
                      class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 transition-colors"
                    >
                      编辑
                    </button>
                  </div>

                  <!-- 收支信息 -->
                  <div
                    v-if="entry.type === 'Transaction' && entry.postings"
                    class="ml-2 space-y-1"
                  >
                    <div
                      v-for="(posting, index) in entry.postings.slice(0, 2)"
                      :key="index"
                      class="flex justify-between text-sm"
                    >
                      <span class="text-gray-600 dark:text-gray-400 truncate max-w-[200px]">{{
                        posting.account
                      }}</span>
                      <span
                        class="font-medium"
                        :class="
                          posting.units?.number > 0
                            ? 'text-green-600'
                            : posting.units?.number < 0
                            ? 'text-red-600'
                            : ''
                        "
                      >
                        {{ posting.units?.number || 0 }}
                        {{ posting.units?.currency || "" }}
                      </span>
                    </div>
                    <!-- 显示更多记账行 -->
                    <div
                      v-if="entry.postings.length > 2"
                      class="text-xs text-gray-500 dark:text-gray-400"
                    >
                      +{{ entry.postings.length - 2 }} 更多记账行
                    </div>
                  </div>

                  <!-- 开户信息 -->
                  <div
                    v-if="entry.type === 'Open'"
                    class="text-sm text-gray-600 dark:text-gray-400 ml-2"
                  >
                    打开账户: {{ entry.account }}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="showAddModal || showEditModal"
        class="fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center overflow-y-auto"
      >
        <div class="bg-white rounded-lg shadow-xl w-full max-w-2xl my-8">
          <AddEntryModal
            @close="closeModal"
            @entry-added="handleEntryAdded"
            @entry-updated="handleEntryUpdated"
            @entry-deleted="handleEntryDeleted"
            :entry="editingEntry"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { useNuxtApp } from '#app';

const router = useRouter();
const { $api } = useNuxtApp();
const { getLedger, getEntries, user } = $api;

const loading = ref(true);
const showAddModal = ref(false);
const showEditModal = ref(false);
const editingEntry = ref<any>(null);
const ledger: any = ref({
  title: "",
  currency: "",
  entries_count: 0,
  errors_count: 0,
});
const entries = ref([] as any[]);

// 仪表盘统计数据
const dashboardStats: any = ref({
  totalIncome: 0,
  totalExpense: 0,
  netIncome: 0,
  expenseByCategory: [] as {
    name: string;
    amount: number;
    percentage: number;
  }[],
});

// 为分类生成不同颜色
const getCategoryColor = (index: number) => {
  const colors = [
    "#FF6B6B",
    "#4ECDC4",
    "#45B7D1",
    "#FFA07A",
    "#98D8C8",
    "#F7DC6F",
    "#BB8FCE",
    "#85C1E2",
    "#F8C471",
    "#82E0AA",
  ];
  return colors[index % colors.length];
};

// 计算仪表盘统计数据
const calculateDashboardStats = () => {
  // 筛选最近30天的交易记录
  const thirtyDaysAgo = new Date();
  thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);

  const recentTransactions = entries.value.filter(
    (e: any) => e.type === "Transaction" && new Date(e.date) >= thirtyDaysAgo
  );

  let totalIncome = 0;
  let totalExpense = 0;
  const categoryExpenses: Record<string, number> = {};

  // 计算总收入和总支出
  recentTransactions.forEach((entry: any) => {
    if (entry.postings) {
      entry.postings.forEach((posting: any) => {
        if (posting.units && posting.units.number) {
          const amount = posting.units.number;
          const account = posting.account;

          // 判断是收入还是支出
          if (account.startsWith("Income")) {
            totalIncome += Math.abs(amount);
          } else if (account.startsWith("Expenses")) {
            totalExpense += Math.abs(amount);

            // 按分类统计支出
            const categoryParts = account.split(":");
            if (categoryParts.length >= 2) {
              const category = categoryParts[1];
              categoryExpenses[category] =
                (categoryExpenses[category] || 0) + Math.abs(amount);
            }
          }
        }
      });
    }
  });

  // 计算分类支出百分比
  const expenseByCategory = Object.entries(categoryExpenses)
    .map(([name, amount]) => ({
      name,
      amount,
      percentage:
        totalExpense > 0 ? Math.round((amount / totalExpense) * 100) : 0,
    }))
    .sort((a, b) => b.amount - a.amount);

  // 更新统计数据
  dashboardStats.value = {
    totalIncome: totalIncome.toFixed(2),
    totalExpense: totalExpense.toFixed(2),
    netIncome: (totalIncome - totalExpense).toFixed(2),
    expenseByCategory,
  };
};

// 刷新数据的函数
const refreshData = async () => {
  // 检查登录状态
  if (!user.value) {
    await router.push("/login");
    return;
  }

  try {
    loading.value = true;
    ledger.value = await getLedger();
    const result = await getEntries();
    // 兼容新旧API格式
    entries.value = result.entries ? result.entries : result;

    // 计算仪表盘统计数据
    calculateDashboardStats();
  } catch (error) {
    console.error("Error refreshing data:", error);
    // 如果API调用失败（可能是token过期），跳转到登录页
    await router.push("/login");
  } finally {
    loading.value = false;
  }
};

// 检查登录状态
if (!user.value) {
  router.push("/login");
}

// 处理条目添加
const handleEntryAdded = () => {
  closeModal();
  refreshData();
};

// 处理条目更新
const handleEntryUpdated = () => {
  closeModal();
  refreshData();
};

// 处理条目删除
const handleEntryDeleted = () => {
  closeModal();
  refreshData();
};

// 打开编辑模态框
const openEditModal = (entry: any) => {
  editingEntry.value = entry;
  showEditModal.value = true;
};

// 关闭模态框
const closeModal = () => {
  showAddModal.value = false;
  showEditModal.value = false;
  editingEntry.value = null;
};

// 监听全局SSE事件
onMounted(async () => {
  await refreshData();

  window.addEventListener("sse:data-updated", refreshData);
});

// 组件卸载时移除事件监听
onUnmounted(() => {
  window.removeEventListener("sse:data-updated", refreshData);
});
</script>