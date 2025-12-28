<template>
  <div class="container mx-auto px-4 py-4 sm:py-6">
    <!-- 错误信息显示 - 移动端简化 -->
    <div v-if="ledger.errors && ledger.errors.length > 0"
      class="card bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 mb-4 sm:mb-6">
      <h2 class="text-lg sm:text-xl font-semibold mb-2 sm:mb-3 text-red-800 dark:text-red-300 px-3 sm:px-4 pt-3">
        账本错误
      </h2>
      <div class="space-y-2 sm:space-y-3 max-h-48 sm:max-h-60 overflow-y-auto px-3 sm:px-4 pb-3">
        <div v-for="(error, index) in ledger.errors" :key="index"
          class="bg-white dark:bg-gray-700 p-2 sm:p-3 rounded shadow-sm border-l-4 border-red-500">
          <div class="flex justify-between items-start mb-1">
            <span class="font-medium text-red-700 dark:text-red-300 text-sm">错误 {{ Number(index) + 1 }}</span>
            <span class="text-xs bg-red-100 dark:bg-red-800 text-red-800 dark:text-red-200 px-1.5 py-0.5 rounded">
              {{ error.severity || "Error" }}
            </span>
          </div>
          <p class="text-xs sm:text-sm text-gray-700 dark:text-gray-300 mb-1">
            {{ error.message }}
          </p>
          <div v-if="error.source" class="text-xs text-gray-500 dark:text-gray-400">
            <span>{{ error.source.filename }}:{{ error.source.lineno }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 第一行：收支统计卡片和账本信息卡片 -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 sm:gap-6 mb-4 sm:mb-6">
      <!-- 收支统计卡片 - 合并总收入、总支出、结余 -->
      <div
        class="card bg-gradient-to-br from-blue-50 to-indigo-50 dark:from-blue-900/30 dark:to-indigo-900/30 p-4 sm:p-5 rounded-xl shadow-sm border border-blue-100 dark:border-blue-800/50">
        <div class="space-y-4 sm:space-y-5">
          <!-- 标题行 -->
          <div class="flex flex-col">
            <div class="flex justify-between items-center">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ formatDateRange(dashboardStats.dateRange?.current) }}
              </span>
              <span
                class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-800/50 text-blue-700 dark:text-blue-300 rounded-full">
                较上月
              </span>
            </div>
            <!-- 农历日期和节日 -->
            <span class="text-xs text-gray-500 dark:text-gray-500 mt-0.5">
              {{ getLunarAndFestival() }}
            </span>
          </div>

          <!-- 主要数据行 - 支出 -->
          <div class="flex flex-col">
            <span class="text-xs text-gray-600 dark:text-gray-400 mb-1">总支出</span>
            <div class="flex items-baseline gap-2">
              <span class="text-2xl sm:text-2.5xl font-bold text-red-900 dark:text-red-200">
                {{ dashboardStats.totalExpense }} {{ getCurrency() }}
              </span>
              <div v-if="dashboardStats.changes.expense !== 0" class="flex items-center text-sm font-medium" :class="dashboardStats.changes.expense > 0
                ? 'text-red-600 dark:text-red-400'
                : 'text-green-600 dark:text-green-400'
                ">
                <span>{{ dashboardStats.changes.expense > 0 ? "↑" : "↓" }}</span>
                <span>{{ Math.abs(dashboardStats.changes.expense) }}%</span>
              </div>
            </div>
          </div>

          <!-- 辅助数据行 - 收入和结余 -->
          <div class="grid grid-cols-2 gap-4 pt-2 border-t border-blue-100 dark:border-blue-800/50">
            <!-- 总收入 -->
            <div class="flex flex-col">
              <div class="flex justify-between items-baseline">
                <span class="text-sm text-gray-600 dark:text-gray-400">收入</span>
                <div v-if="dashboardStats.changes.income !== 0" class="flex items-center text-xs font-medium" :class="dashboardStats.changes.income > 0
                  ? 'text-green-600 dark:text-green-400'
                  : 'text-red-600 dark:text-red-400'
                  ">
                  <span>{{ dashboardStats.changes.income > 0 ? "↑" : "↓" }}</span>
                  <span>{{ Math.abs(dashboardStats.changes.income) }}%</span>
                </div>
              </div>
              <span class="text-lg font-semibold text-green-900 dark:text-green-200 mt-1">
                {{ dashboardStats.totalIncome }} {{ getCurrency() }}
              </span>
            </div>

            <!-- 结余 -->
            <div class="flex flex-col">
              <div class="flex justify-between items-baseline">
                <span class="text-sm text-gray-600 dark:text-gray-400">结余</span>
                <div v-if="dashboardStats.changes.net !== 0" class="flex items-center text-xs font-medium" :class="dashboardStats.changes.net > 0
                  ? 'text-green-600 dark:text-green-400'
                  : 'text-red-600 dark:text-red-400'
                  ">
                  <span>{{ dashboardStats.changes.net > 0 ? "↑" : "↓" }}</span>
                  <span>{{ Math.abs(dashboardStats.changes.net) }}%</span>
                </div>
              </div>
              <span class="text-lg font-semibold" :class="dashboardStats.netIncome >= 0
                ? 'text-blue-900 dark:text-blue-200'
                : 'text-red-900 dark:text-red-200'
                ">
                {{ dashboardStats.netIncome }} {{ getCurrency() }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 账本信息卡片 -->
      <div class="card p-4">
        <h2 class="text-lg sm:text-xl font-semibold mb-3 dark:text-white">
          账本信息
        </h2>
        <div class="space-y-3">
          <div class="flex justify-between items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">账本名称:</span>
            <span class="font-medium dark:text-gray-300 truncate max-w-[150px] sm:max-w-none">{{ ledger.title }}</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">主要货币:</span>
            <span class="font-medium dark:text-gray-300">{{ ledger.currency }}</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">记账条目:</span>
            <span class="font-medium dark:text-gray-300"> {{ ledger.entries_count }}</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">错误数量:</span>
            <span class="font-medium text-red-500"> {{ ledger.errors_count }} </span>
          </div>
        </div>
      </div>
    </div>

    <!-- 第二行：分类支出 -->
    <div class="card mb-4 sm:mb-6">
      <h2 class="text-lg sm:text-xl font-semibold mb-3 dark:text-white p-4 pt-4 pb-0">
        分类支出
      </h2>
      <div class="space-y-3 p-4 pt-3">
        <div v-for="(category, index) in dashboardStats.expenseByCategory" :key="index"
          class="flex justify-between items-center">
          <div class="flex items-center">
            <div class="w-2.5 h-2.5 rounded-full mr-2" :style="{ backgroundColor: getCategoryColor(Number(index)) }">
            </div>
            <span class="text-sm sm:text-base text-gray-700 dark:text-gray-300 truncate max-w-[120px] sm:max-w-none">
              {{ category.name }}
            </span>
          </div>
          <div class="text-right">
            <span class="font-medium dark:text-gray-200 text-sm sm:text-base">
              {{ category.amount }} {{ getCurrency() }}
            </span>
            <span class="text-xs sm:text-sm text-gray-500 dark:text-gray-400 ml-2">({{ category.percentage }}%)</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 第三行：最近记录 -->
    <div class="card">
      <h2 class="text-lg sm:text-xl font-semibold mb-3 dark:text-white p-4 pt-4 pb-0">
        最近7天记录
      </h2>
      <div v-if="loading" class="text-center py-4">
        <div class="inline-block animate-spin rounded-full h-6 w-6 sm:h-8 sm:w-8 border-b-2 border-primary"></div>
      </div>
      <div v-else-if="entries.length === 0" class="text-center py-6 sm:py-8 text-gray-500 dark:text-gray-400">
        <p class="text-sm sm:text-base">暂无记账记录</p>
      </div>
      <div v-else-if="entries
        .filter((e) => e.type === 'Transaction')
        .filter((e) => e.date >= getLast7Days()).length === 0"
        class="text-center py-6 sm:py-8 text-gray-500 dark:text-gray-400">
        <p class="text-sm sm:text-base">暂无最近7天记账记录</p>
      </div>
      <div v-else class="space-y-3 p-4 pt-3">
        <!-- 最近7天交易记录 -->
        <div v-for="entry in entries
          .filter((e) => e.type === 'Transaction')
          .filter((e) => e.date >= getLast7Days())" :key="entry.meta.filename + entry.meta.lineno"
          class="border-b pb-3 last:border-0 hover:bg-gray-50 dark:hover:bg-gray-800/70 p-2 rounded transition-colors">
          <div class="flex flex-col space-y-1">
            <!-- 日期和类型 -->
            <div class="flex justify-between items-center">
              <span class="font-medium dark:text-gray-300 text-sm">
                {{ entry.date }}
              </span>
              <span
                class="text-xs px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
                {{ entry.type }}
              </span>
            </div>

            <!-- 交易描述 -->
            <div v-if="entry.type === 'Transaction' && entry.narration"
              class="text-sm text-gray-700 dark:text-gray-300 ml-1.5 truncate">
              {{ entry.narration }}
            </div>

            <!-- 标签 - 移动端简化 -->
            <div v-if="entry.tags && entry.tags.length > 0" class="flex flex-wrap gap-1 ml-1.5">
              <span v-for="(tag, index) in entry.tags.slice(0, 2)" :key="index"
                class="inline-block bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs px-1.5 py-0.5 rounded">
                #{{ tag }}
              </span>
              <span v-if="entry.tags.length > 2" class="text-xs text-gray-500 dark:text-gray-400">
                +{{ entry.tags.length - 2 }}
              </span>
            </div>

            <!-- 收支信息 - 移动端简化 -->
            <div v-if="entry.type === 'Transaction' && entry.postings" class="ml-1.5">
              <div v-for="(posting, index) in entry.postings.slice(0, 2)" :key="index"
                class="flex justify-between text-sm">
                <span class="text-gray-600 dark:text-gray-400 truncate max-w-[120px] sm:max-w-[200px]">
                  {{ posting.account.split(":").pop() }}
                </span>
                <span class="font-medium" :class="posting.units?.number > 0
                  ? 'text-green-600'
                  : posting.units?.number < 0
                    ? 'text-red-600'
                    : ''
                  ">
                  {{ posting.units?.number || 0 }}
                </span>
              </div>
              <!-- 显示更多记账行 -->
              <div v-if="entry.postings.length > 2" class="text-xs text-gray-500 dark:text-gray-400">
                +{{ entry.postings.length - 2 }} 行
              </div>
            </div>

            <!-- 操作按钮 - 移动端固定位置 -->
            <div class="ml-auto">
              <button @click="
                openEditModal({
                  ...entry,
                  id: `${entry.meta?.filename}:${entry.meta?.lineno}`,
                })
                "
                class="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 transition-colors">
                编辑
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Add Entry Drawer - 确保在移动端正常显示 -->
  <AddEntryModal v-if="showAddModal || showEditModal" @close="closeModal" @entry-added="handleEntryAdded"
    @entry-updated="handleEntryUpdated" @entry-deleted="handleEntryDeleted" :entry="editingEntry" />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { useNuxtApp } from "#app";
import dayjs from "dayjs";
import { useSystemConfig } from "~/composables/useSystemConfig";
import { getLunarAndFestival } from "~/utils/lunarUtils";

const router = useRouter();
const { $api } = useNuxtApp();
const { getLedger, getEntries, user, accounts } = $api;

// 系统配置
const { config, ledger: systemLedger, initConfig, getCurrency } = useSystemConfig();

const loading = ref(true);
const showAddModal = ref(false);
const showEditModal = ref(false);
const editingEntry = ref<any>(null);
// 使用系统配置中的ledger信息，避免重复请求
const ledger = systemLedger;
const entries = ref([] as any[]);
const accountConfig = ref<any>({
  Expenses: {},
});

// 仪表盘统计数据
const dashboardStats: any = ref({
  totalIncome: 0,
  totalExpense: 0,
  netIncome: 0,
  previous: {
    totalIncome: 0,
    totalExpense: 0,
    netIncome: 0,
  },
  changes: {
    income: 0,
    expense: 0,
    net: 0,
  },
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

// 格式化日期范围
const formatDateRange = (dateRange: any) => {
  if (!dateRange || !dateRange.start) return "";

  const startDate = dayjs(dateRange.start);
  const endDate = dayjs(dateRange.end);

  // 如果是同一个月，只显示月份
  if (startDate.isSame(endDate, "year") && startDate.isSame(endDate, "month")) {
    return startDate.format("YYYY年MM月");
  }

  // 否则显示完整日期范围
  return `${startDate.format("YYYY年MM月DD日")} - ${endDate.format(
    "YYYY年MM月DD日"
  )}`;
};



// 计算日期范围（本月和上月）
const getCurrentAndPreviousMonthDates = () => {
  const now = dayjs();

  // 本月日期范围
  const firstDayOfMonth = now.startOf("month");
  const lastDayOfMonth = now.endOf("month");

  // 上月日期范围
  const firstDayOfLastMonth = now.subtract(1, "month").startOf("month");
  const lastDayOfLastMonth = now.subtract(1, "month").endOf("month");

  return {
    current: { start: firstDayOfMonth.toDate(), end: lastDayOfMonth.toDate() },
    previous: {
      start: firstDayOfLastMonth.toDate(),
      end: lastDayOfLastMonth.toDate(),
    },
  };
};

// 计算指定日期范围内的统计数据
const calculateStatsForDateRange = async (dateRange: any) => {
  // 直接使用前端计算，不再调用已删除的getCategoryStats API
  // 获取当前日期范围内的交易记录
  const transactions = entries.value.filter(
    (e: any) =>
      e.type === "Transaction" &&
      (dayjs(e.date).isAfter(dateRange.start) ||
        dayjs(e.date).isSame(dateRange.start, "day")) &&
      (dayjs(e.date).isBefore(dateRange.end) ||
        dayjs(e.date).isSame(dateRange.end, "day"))
  );

  let totalIncome = 0;
  let totalExpense = 0;
  // 按后端配置的分类统计支出，初始化所有配置的分类
  const categoryExpenses: Record<string, number> = {};
  // 初始化所有配置的支出分类为0
  if (accountConfig.value && accountConfig.value.Expenses) {
    Object.keys(accountConfig.value.Expenses).forEach((category) => {
      categoryExpenses[category] = 0;
    });
  }

  // 计算总收入和总支出
  transactions.forEach((entry: any) => {
    if (entry.postings) {
      entry.postings.forEach((posting: any) => {
        if (posting.units && posting.units.number) {
          const amount = parseFloat(posting.units.number);
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
              // 只统计在配置中存在的分类
              if (categoryExpenses.hasOwnProperty(category)) {
                categoryExpenses[category] =
                  (categoryExpenses[category] || 0) + Math.abs(amount);
              }
            }
          }
        }
      });
    }
  });

  return { totalIncome, totalExpense, categoryExpenses, transactions };
};

// 计算与上月比较的百分比变化
const calculatePercentageChange = (current: number, previous: number) => {
  if (previous === 0) return current > 0 ? 100 : 0;
  return Math.round(((current - previous) / previous) * 100);
};

// 计算仪表盘统计数据（统一使用本地计算）
const calculateDashboardStats = async () => {
  try {
    calculateDashboardStatsLocally();
  } catch (error) {
    console.error("计算仪表盘统计数据失败:", error);
  }
};

// 获取最近7天的日期范围
const getLast7Days = () => {
  const now = dayjs();
  const last7Days = now.subtract(7, 'day');
  return { start: last7Days.toDate(), end: now.toDate() };
}

// 本地计算仪表盘统计数据（作为API调用失败的回退）
const calculateDashboardStatsLocally = async () => {
  const { current, previous } = getCurrentAndPreviousMonthDates();

  // 计算本月统计数据
  const currentMonthStats = await calculateStatsForDateRange(current);
  const {
    totalIncome,
    totalExpense,
    categoryExpenses: currentCategoryExpenses,
  } = currentMonthStats;

  // 计算上月统计数据
  const previousMonthStats = await calculateStatsForDateRange(previous);
  const { totalIncome: prevTotalIncome, totalExpense: prevTotalExpense } = previousMonthStats;

  // 计算分类支出百分比
  const expenseByCategory = Object.entries(currentCategoryExpenses)
    .map(([name, amount]) => ({
      name: accountConfig.value?.Expenses?.[name] || name, // 使用中文名称，如果没有则使用英文名称
      amount,
      percentage:
        totalExpense > 0 ? Math.round((amount / totalExpense) * 100) : 0,
    }))
    .sort((a, b) => b.amount - a.amount)
    .filter((item) => item.percentage > 0); // 过滤出有支出的分类

  // 计算与上月比较的变化率
  const incomeChange = calculatePercentageChange(totalIncome, prevTotalIncome);
  const expenseChange = calculatePercentageChange(
    totalExpense,
    prevTotalExpense
  );
  const netChange = calculatePercentageChange(
    totalIncome - totalExpense,
    prevTotalIncome - prevTotalExpense
  );

  // 更新统计数据
  dashboardStats.value = {
    totalIncome: parseFloat(totalIncome.toFixed(2)),
    totalExpense: parseFloat(totalExpense.toFixed(2)),
    netIncome: parseFloat((totalIncome - totalExpense).toFixed(2)),
    previous: {
      totalIncome: parseFloat(prevTotalIncome.toFixed(2)),
      totalExpense: parseFloat(prevTotalExpense.toFixed(2)),
      netIncome: parseFloat((prevTotalIncome - prevTotalExpense).toFixed(2)),
    },
    changes: {
      income: incomeChange,
      expense: expenseChange,
      net: netChange,
    },
    expenseByCategory: expenseByCategory,
    dateRange: {
      current: `${dayjs(current.start).format("YYYY-MM")}`,
      previous: `${dayjs(previous.start).format("YYYY-MM")}`,
    },
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
    // 不再重复调用getLedger，直接使用useSystemConfig中的ledger信息
    // 注：如果需要强制刷新ledger信息，可以在这里再次调用initConfig()

    // 获取本月和上月的数据，以确保统计计算准确
    const now = dayjs();
    const firstDayOfLastMonth = now.subtract(1, "month").startOf("month");
    const lastDayOfCurrentMonth = now.endOf("month");

    const result = await getEntries({
      start_date: firstDayOfLastMonth.toISOString(),
      end_date: lastDayOfCurrentMonth.toISOString(),
    });
    // 兼容新旧API格式
    entries.value = result.entries ? result.entries : result;

    // 获取账户配置信息
    const configResult = await accounts.getAccountConfig();
    accountConfig.value = configResult;

    // 计算仪表盘统计数据
    await calculateDashboardStats();
  } catch (error) {
    console.error("Error refreshing data:", error);
    // 如果API调用失败（可能是token过期），跳转到登录页
    await router.push("/login");
  } finally {
    loading.value = false;
  }
};

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
let mounted = false;
onMounted(async () => {
  // 防止热重载导致的重复执行
  if (mounted) return;
  mounted = true;

  await initConfig();
  await refreshData();

  window.addEventListener("sse:data-updated", refreshData);
});

// 组件卸载时移除事件监听
onUnmounted(() => {
  window.removeEventListener("sse:data-updated", refreshData);
});
</script>