<template>
  <div class="container mx-auto px-4 py-4 sm:py-6">
    <!-- 页面标题 -->
    <h1 class="text-xl sm:text-2xl font-bold mb-4 sm:mb-6 dark:text-white">
      首页
    </h1>

    <!-- 错误信息显示 - 移动端简化 -->
    <div
      v-if="ledger.errors && ledger.errors.length > 0"
      class="card bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 mb-4 sm:mb-6"
    >
      <h2
        class="text-lg sm:text-xl font-semibold mb-2 sm:mb-3 text-red-800 dark:text-red-300 px-3 sm:px-4 pt-3"
      >
        账本错误
      </h2>
      <div
        class="space-y-2 sm:space-y-3 max-h-48 sm:max-h-60 overflow-y-auto px-3 sm:px-4 pb-3"
      >
        <div
          v-for="(error, index) in ledger.errors"
          :key="index"
          class="bg-white dark:bg-gray-700 p-2 sm:p-3 rounded shadow-sm border-l-4 border-red-500"
        >
          <div class="flex justify-between items-start mb-1">
            <span class="font-medium text-red-700 dark:text-red-300 text-sm"
              >错误 {{ Number(index) + 1 }}</span
            >
            <span
              class="text-xs bg-red-100 dark:bg-red-800 text-red-800 dark:text-red-200 px-1.5 py-0.5 rounded"
              >{{ error.severity || "Error" }}</span
            >
          </div>
          <p class="text-xs sm:text-sm text-gray-700 dark:text-gray-300 mb-1">
            {{ error.message }}
          </p>
          <div
            v-if="error.source"
            class="text-xs text-gray-500 dark:text-gray-400"
          >
            <span>{{ error.source.filename }}:{{ error.source.lineno }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 第一行：收支统计卡片和账本信息卡片 -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 sm:gap-6 mb-4 sm:mb-6">
      <!-- 收支统计卡片 - 合并总收入、总支出、净收入 -->
      <div
        class="card bg-gradient-to-br from-blue-50 to-indigo-50 dark:from-blue-900/30 dark:to-indigo-900/30 p-4"
      >
        <h2 class="text-lg sm:text-xl font-semibold mb-3 dark:text-white">
          收支统计
        </h2>
        <div class="space-y-4">
          <!-- 本月/近一个月数据 -->
          <div>
            <div class="flex justify-between items-center mb-2">
              <span class="text-sm text-gray-600 dark:text-gray-400"
                >本期（{{
                  formatDateRange(dashboardStats.dateRange?.current)
                }}）</span
              >
              <span
                class="text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded"
              >
                较上月
              </span>
            </div>

            <div class="grid grid-cols-3 gap-3">
              <!-- 总收入 -->
              <div class="text-center">
                <p
                  class="text-xs text-blue-700 dark:text-blue-300 font-medium mb-1"
                >
                  总收入
                </p>
                <p class="text-lg font-bold text-blue-900 dark:text-blue-200">
                  {{ dashboardStats.totalIncome }} {{ getCurrency() }}
                </p>
                <div
                  v-if="dashboardStats.changes.income !== 0"
                  class="text-xs mt-1 flex items-center justify-center"
                  :class="
                    dashboardStats.changes.income > 0
                      ? 'text-green-600'
                      : 'text-red-600'
                  "
                >
                  <span>{{
                    dashboardStats.changes.income > 0 ? "↑" : "↓"
                  }}</span>
                  <span>{{ Math.abs(dashboardStats.changes.income) }}%</span>
                </div>
              </div>

              <!-- 总支出 -->
              <div class="text-center">
                <p
                  class="text-xs text-red-700 dark:text-red-300 font-medium mb-1"
                >
                  总支出
                </p>
                <p class="text-lg font-bold text-red-900 dark:text-red-200">
                  {{ dashboardStats.totalExpense }} {{ getCurrency() }}
                </p>
                <div
                  v-if="dashboardStats.changes.expense !== 0"
                  class="text-xs mt-1 flex items-center justify-center"
                  :class="
                    dashboardStats.changes.expense > 0
                      ? 'text-red-600'
                      : 'text-green-600'
                  "
                >
                  <span>{{
                    dashboardStats.changes.expense > 0 ? "↑" : "↓"
                  }}</span>
                  <span>{{ Math.abs(dashboardStats.changes.expense) }}%</span>
                </div>
              </div>

              <!-- 净收入 -->
              <div class="text-center">
                <p
                  class="text-xs text-green-700 dark:text-green-300 font-medium mb-1"
                >
                  净收入
                </p>
                <p class="text-lg font-bold text-green-900 dark:text-green-200">
                  {{ dashboardStats.netIncome }} {{ getCurrency() }}
                </p>
                <div
                  v-if="dashboardStats.changes.net !== 0"
                  class="text-xs mt-1 flex items-center justify-center"
                  :class="
                    dashboardStats.changes.net > 0
                      ? 'text-green-600'
                      : 'text-red-600'
                  "
                >
                  <span>{{ dashboardStats.changes.net > 0 ? "↑" : "↓" }}</span>
                  <span>{{ Math.abs(dashboardStats.changes.net) }}%</span>
                </div>
              </div>
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
            <span class="text-sm text-gray-600 dark:text-gray-400"
              >账本名称:</span
            >
            <span
              class="font-medium dark:text-gray-300 truncate max-w-[150px] sm:max-w-none"
              >{{ ledger.title }}</span
            >
          </div>
          <div class="flex justify-between items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400"
              >主要货币:</span
            >
            <span class="font-medium dark:text-gray-300">{{
              ledger.currency
            }}</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400"
              >记账条目:</span
            >
            <span class="font-medium dark:text-gray-300">{{
              ledger.entries_count
            }}</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400"
              >错误数量:</span
            >
            <span class="font-medium text-red-500">{{
              ledger.errors_count
            }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 第二行：分类支出 -->
    <div class="card mb-4 sm:mb-6">
      <h2
        class="text-lg sm:text-xl font-semibold mb-3 dark:text-white p-4 pt-4 pb-0"
      >
        分类支出
      </h2>
      <div class="space-y-3 p-4 pt-3">
        <div
          v-for="(category, index) in dashboardStats.expenseByCategory.slice(
            0,
            5
          )"
          :key="index"
          class="flex justify-between items-center"
        >
          <div class="flex items-center">
            <div
              class="w-2.5 h-2.5 rounded-full mr-2"
              :style="{ backgroundColor: getCategoryColor(Number(index)) }"
            ></div>
            <span
              class="text-sm sm:text-base text-gray-700 dark:text-gray-300 truncate max-w-[120px] sm:max-w-none"
              >{{ category.name }}</span
            >
          </div>
          <div class="text-right">
            <span class="font-medium dark:text-gray-200 text-sm sm:text-base"
              >{{ category.amount }} {{ getCurrency() }}</span
            >
            <span
              class="text-xs sm:text-sm text-gray-500 dark:text-gray-400 ml-2"
              >({{ category.percentage }}%)</span
            >
          </div>
        </div>
        <!-- 移动端只显示前5个分类 -->
        <div
          v-if="dashboardStats.expenseByCategory.length > 5"
          class="text-center text-xs sm:text-sm text-gray-500 dark:text-gray-400"
        >
          +{{ dashboardStats.expenseByCategory.length - 5 }} 个分类
        </div>
      </div>
    </div>

    <!-- 第三行：操作菜单 -->
    <div class="grid grid-cols-3 gap-3 sm:gap-4 mb-4 sm:mb-6">
      <button
        @click="showAddModal = true"
        class="btn btn-primary flex flex-col items-center p-4 sm:p-6 h-full"
      >
        <span class="text-2xl sm:text-3xl mb-1 sm:mb-2">📝</span>
        <span class="text-xs sm:text-sm">添加记录</span>
      </button>
      <button
        @click="$router.push('/entries')"
        class="btn btn-secondary flex flex-col items-center p-4 sm:p-6 h-full"
      >
        <span class="text-2xl sm:text-3xl mb-1 sm:mb-2">📋</span>
        <span class="text-xs sm:text-sm">查看记录</span>
      </button>
      <button
        @click="$router.push('/stats')"
        class="btn btn-secondary flex flex-col items-center p-4 sm:p-6 h-full"
      >
        <span class="text-2xl sm:text-3xl mb-1 sm:mb-2">📊</span>
        <span class="text-xs sm:text-sm">查看统计</span>
      </button>
    </div>

    <!-- 第四行：最近记录 -->
    <div class="card">
      <h2
        class="text-lg sm:text-xl font-semibold mb-3 dark:text-white p-4 pt-4 pb-0"
      >
        最近记录
      </h2>
      <div v-if="loading" class="text-center py-4">
        <div
          class="inline-block animate-spin rounded-full h-6 w-6 sm:h-8 sm:w-8 border-b-2 border-primary"
        ></div>
      </div>
      <div
        v-else-if="entries.length === 0"
        class="text-center py-6 sm:py-8 text-gray-500 dark:text-gray-400"
      >
        <p class="text-sm sm:text-base">暂无记账记录</p>
      </div>
      <div v-else class="space-y-3 p-4 pt-3">
        <div
          v-for="entry in entries
            .filter((e) => e.type === 'Transaction')
            .slice(0, 4)"
          :key="entry.meta.filename + entry.meta.lineno"
          class="border-b pb-3 last:border-0 hover:bg-gray-50 dark:hover:bg-gray-800/70 p-2 rounded transition-colors"
        >
          <div class="flex flex-col space-y-1">
            <!-- 日期和类型 -->
            <div class="flex justify-between items-center">
              <span class="font-medium dark:text-gray-300 text-sm">{{
                entry.date
              }}</span>
              <span
                class="text-xs px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
                >{{ entry.type }}</span
              >
            </div>

            <!-- 交易描述 -->
            <div
              v-if="entry.type === 'Transaction' && entry.narration"
              class="text-sm text-gray-700 dark:text-gray-300 ml-1.5 truncate"
            >
              {{ entry.narration }}
            </div>

            <!-- 标签 - 移动端简化 -->
            <div
              v-if="entry.tags && entry.tags.length > 0"
              class="flex flex-wrap gap-1 ml-1.5"
            >
              <span
                v-for="(tag, index) in entry.tags.slice(0, 2)"
                :key="index"
                class="inline-block bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs px-1.5 py-0.5 rounded"
              >
                #{{ tag }}
              </span>
              <span
                v-if="entry.tags.length > 2"
                class="text-xs text-gray-500 dark:text-gray-400"
                >+{{ entry.tags.length - 2 }}</span
              >
            </div>

            <!-- 收支信息 - 移动端简化 -->
            <div
              v-if="entry.type === 'Transaction' && entry.postings"
              class="ml-1.5"
            >
              <div
                v-for="(posting, index) in entry.postings.slice(0, 2)"
                :key="index"
                class="flex justify-between text-sm"
              >
                <span
                  class="text-gray-600 dark:text-gray-400 truncate max-w-[120px] sm:max-w-[200px]"
                  >{{ posting.account.split(":").pop() }}</span
                >
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
                </span>
              </div>
              <!-- 显示更多记账行 -->
              <div
                v-if="entry.postings.length > 2"
                class="text-xs text-gray-500 dark:text-gray-400"
              >
                +{{ entry.postings.length - 2 }} 行
              </div>
            </div>

            <!-- 操作按钮 - 移动端固定位置 -->
            <div class="ml-auto">
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
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Add Entry Drawer - 确保在移动端正常显示 -->
  <AddEntryModal
    v-if="showAddModal || showEditModal"
    @close="closeModal"
    @entry-added="handleEntryAdded"
    @entry-updated="handleEntryUpdated"
    @entry-deleted="handleEntryDeleted"
    :entry="editingEntry"
  />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { useNuxtApp } from "#app";
import dayjs from "dayjs";
import { useSystemConfig } from "~/composables/useSystemConfig";

const router = useRouter();
const { $api } = useNuxtApp();
const { getLedger, getEntries, user, accounts } = $api;

// 系统配置
const { config, initConfig, getCurrency } = useSystemConfig();

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

// 计算仪表盘统计数据
const calculateDashboardStats = async () => {
  try {
    const { current, previous } = getCurrentAndPreviousMonthDates();

    // 使用新的API获取本月收支统计
    const currentMonthStats = await $api.getMonthlyIncomeExpense({
      start_date: dayjs(current.start).format("YYYY-MM-DD"),
      end_date: dayjs(current.end).format("YYYY-MM-DD"),
    });

    // 使用新的API获取上月收支统计
    const previousMonthStats = await $api.getMonthlyIncomeExpense({
      start_date: dayjs(previous.start).format("YYYY-MM-DD"),
      end_date: dayjs(previous.end).format("YYYY-MM-DD"),
    });

    // 使用新的API获取本月分类支出统计
    const monthlyExpenses = await $api.getMonthlyExpenses({
      start_date: dayjs(current.start).format("YYYY-MM-DD"),
      end_date: dayjs(current.end).format("YYYY-MM-DD"),
    });

    // 计算分类支出百分比
    const totalExpense = parseFloat(currentMonthStats.expense) || 0;
    const expenseByCategory = monthlyExpenses.monthly_expenses
      .map((item: any) => {
        // 提取分类名称
        const categoryParts = item.account.split(":");
        const category =
          categoryParts.length >= 2 ? categoryParts[1] : item.account;
        const amount = parseFloat(item.total) || 0;

        return {
          name: accountConfig.value?.Expenses?.[category] || category, // 使用中文名称，如果没有则使用英文名称
          amount,
          percentage:
            totalExpense > 0 ? Math.round((amount / totalExpense) * 100) : 0,
        };
      })
      .sort((a: any, b: any) => b.amount - a.amount);

    // 计算与上月比较的变化率
    const incomeChange = calculatePercentageChange(
      parseFloat(currentMonthStats.income) || 0,
      parseFloat(previousMonthStats.income) || 0
    );
    const expenseChange = calculatePercentageChange(
      parseFloat(currentMonthStats.expense) || 0,
      parseFloat(previousMonthStats.expense) || 0
    );
    const netChange = calculatePercentageChange(
      (parseFloat(currentMonthStats.income) || 0) -
        (parseFloat(currentMonthStats.expense) || 0),
      (parseFloat(previousMonthStats.income) || 0) -
        (parseFloat(previousMonthStats.expense) || 0)
    );

    // 更新统计数据
    dashboardStats.value = {
      totalIncome: parseFloat(
        (parseFloat(currentMonthStats.income) || 0).toFixed(2)
      ),
      totalExpense: parseFloat(
        (parseFloat(currentMonthStats.expense) || 0).toFixed(2)
      ),
      netIncome: parseFloat(
        (
          (parseFloat(currentMonthStats.income) || 0) -
          (parseFloat(currentMonthStats.expense) || 0)
        ).toFixed(2)
      ),
      previous: {
        totalIncome: parseFloat(
          (parseFloat(previousMonthStats.income) || 0).toFixed(2)
        ),
        totalExpense: parseFloat(
          (parseFloat(previousMonthStats.expense) || 0).toFixed(2)
        ),
        netIncome: parseFloat(
          (
            (parseFloat(previousMonthStats.income) || 0) -
            (parseFloat(previousMonthStats.expense) || 0)
          ).toFixed(2)
        ),
      },
      changes: {
        income: incomeChange,
        expense: expenseChange,
        net: netChange,
      },
      expenseByCategory: expenseByCategory,
      dateRange: {
        current: {
          start: current.start,
          end: current.end,
        },
        previous: {
          start: previous.start,
          end: previous.end,
        },
      },
    };
  } catch (error) {
    console.error("获取仪表盘统计数据失败:", error);
    // 出错时使用本地计算作为回退
    calculateDashboardStatsLocally();
  }
};

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
  const { totalIncome: prevTotalIncome, totalExpense: prevTotalExpense } =
    previousMonthStats;

  // 计算分类支出百分比
  const expenseByCategory = Object.entries(currentCategoryExpenses)
    .map(([name, amount]) => ({
      name: accountConfig.value?.Expenses?.[name] || name, // 使用中文名称，如果没有则使用英文名称
      amount,
      percentage:
        totalExpense > 0 ? Math.round((amount / totalExpense) * 100) : 0,
    }))
    .sort((a, b) => b.amount - a.amount);

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
    totalIncome: totalIncome.toFixed(2),
    totalExpense: totalExpense.toFixed(2),
    netIncome: (totalIncome - totalExpense).toFixed(2),
    previous: {
      totalIncome: prevTotalIncome.toFixed(2),
      totalExpense: prevTotalExpense.toFixed(2),
      netIncome: (prevTotalIncome - prevTotalExpense).toFixed(2),
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
    ledger.value = await getLedger();
    // 因为仪表盘需要获取近30天的交易记录，所以这里限制获取最近30天的交易记录
    const result = await getEntries({
      start_date: dayjs().subtract(30, "day").toISOString(),
      end_date: dayjs().toISOString(),
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
onMounted(async () => {
  await initConfig();
  await refreshData();

  window.addEventListener("sse:data-updated", refreshData);
});

// 组件卸载时移除事件监听
onUnmounted(() => {
  window.removeEventListener("sse:data-updated", refreshData);
});
</script>
