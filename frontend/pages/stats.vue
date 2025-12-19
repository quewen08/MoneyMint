<template>
  <div>
    <!-- 页面标题 -->
    <h1 class="text-2xl font-bold mb-6 dark:text-white">统计分析</h1>

    <!-- 时间筛选器 -->
    <div class="card mb-6">
      <div class="flex flex-wrap gap-4 items-center">
        <div class="flex items-center gap-2">
          <label for="startDate" class="text-gray-700 dark:text-gray-300">开始日期:</label>
          <input
            type="date"
            id="startDate"
            v-model="dateRange.startDate"
            class="input"
          />
        </div>
        <div class="flex items-center gap-2">
          <label for="endDate" class="text-gray-700 dark:text-gray-300">结束日期:</label>
          <input
            type="date"
            id="endDate"
            v-model="dateRange.endDate"
            class="input"
          />
        </div>
        <button @click="refreshStats" class="btn btn-primary">刷新数据</button>
        <button @click="resetDateRange" class="btn btn-secondary">
          重置日期
        </button>
      </div>
    </div>

    <!-- 统计概览 -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
      <div class="card bg-gradient-to-br from-blue-50 to-blue-100 dark:from-blue-900/40 dark:to-blue-800/40 p-6 transition-colors">
        <div class="flex justify-between items-center">
          <div>
            <p class="text-sm text-blue-700 dark:text-blue-300 font-medium">总收入</p>
            <p class="text-2xl font-bold text-blue-900 dark:text-blue-100 mt-1">
              {{ stats.totalIncome }} {{ ledger.currency }}
            </p>
          </div>
          <div class="bg-blue-200 dark:bg-blue-700/50 rounded-full p-3">
            <span class="text-xl">📈</span>
          </div>
        </div>
      </div>

      <div class="card bg-gradient-to-br from-red-50 to-red-100 dark:from-red-900/40 dark:to-red-800/40 p-6 transition-colors">
        <div class="flex justify-between items-center">
          <div>
            <p class="text-sm text-red-700 dark:text-red-300 font-medium">总支出</p>
            <p class="text-2xl font-bold text-red-900 dark:text-red-100 mt-1">
              {{ stats.totalExpense }} {{ ledger.currency }}
            </p>
          </div>
          <div class="bg-red-200 dark:bg-red-700/50 rounded-full p-3">
            <span class="text-xl">📉</span>
          </div>
        </div>
      </div>

      <div class="card bg-gradient-to-br from-green-50 to-green-100 dark:from-green-900/40 dark:to-green-800/40 p-6 transition-colors">
        <div class="flex justify-between items-center">
          <div>
            <p class="text-sm text-green-700 dark:text-green-300 font-medium">净收入</p>
            <p class="text-2xl font-bold text-green-900 dark:text-green-100 mt-1">
              {{ stats.netIncome }} {{ ledger.currency }}
            </p>
          </div>
          <div class="bg-green-200 dark:bg-green-700/50 rounded-full p-3">
            <span class="text-xl">💰</span>
          </div>
        </div>
      </div>

      <div class="card bg-gradient-to-br from-purple-50 to-purple-100 dark:from-purple-900/40 dark:to-purple-800/40 p-6 transition-colors">
        <div class="flex justify-between items-center">
          <div>
            <p class="text-sm text-purple-700 dark:text-purple-300 font-medium">交易笔数</p>
            <p class="text-2xl font-bold text-purple-900 dark:text-purple-100 mt-1">
              {{ stats.totalTransactions }}
            </p>
          </div>
          <div class="bg-purple-200 dark:bg-purple-700/50 rounded-full p-3">
            <span class="text-xl">📋</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 分类统计 -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
      <!-- 收入分类统计 -->
      <div class="card">
        <h2 class="text-xl font-semibold mb-4">收入分类统计</h2>
        <div v-if="loading" class="text-center py-8">
          <div
            class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
          ></div>
        </div>
        <div
          v-else-if="stats.incomeByCategory.length === 0"
          class="text-center py-8 text-gray-500 dark:text-gray-400"
        >
          <p>暂无收入数据</p>
        </div>
        <div v-else class="space-y-3">
          <div
            v-for="(category, index) in stats.incomeByCategory"
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

      <!-- 支出分类统计 -->
      <div class="card">
        <h2 class="text-xl font-semibold mb-4">支出分类统计</h2>
        <div v-if="loading" class="text-center py-8">
          <div
            class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
          ></div>
        </div>
        <div
          v-else-if="stats.expenseByCategory.length === 0"
          class="text-center py-8 text-gray-500 dark:text-gray-400"
        >
          <p>暂无支出数据</p>
        </div>
        <div v-else class="space-y-3">
          <div
            v-for="(category, index) in stats.expenseByCategory"
            :key="index"
            class="flex justify-between items-center"
          >
            <div class="flex items-center">
              <div
                class="w-3 h-3 rounded-full mr-2"
                :style="{ backgroundColor: getCategoryColor(index + 10) }"
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
    </div>

    <!-- 账户统计 -->
    <div class="card mb-8">
      <h2 class="text-xl font-semibold mb-4">账户统计</h2>
      <div v-if="loading" class="text-center py-8">
        <div
          class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
        ></div>
      </div>
      <div
          v-else-if="stats.accounts.length === 0"
          class="text-center py-8 text-gray-500 dark:text-gray-400"
        >
          <p>暂无账户数据</p>
        </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-800">
              <th class="px-4 py-2 text-left text-gray-700 dark:text-gray-300">账户名称</th>
              <th class="px-4 py-2 text-right text-gray-700 dark:text-gray-300">余额</th>
              <th class="px-4 py-2 text-right text-gray-700 dark:text-gray-300">类型</th>
            </tr>
          </thead>
          <tbody class="dark:text-gray-300">
            <tr
              v-for="(account, index) in stats.accounts"
              :key="index"
              class="border-t dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/70"
            >
              <td class="px-4 py-2">{{ account.name }}</td>
              <td class="px-4 py-2 text-right font-medium">
                {{ account.balance }} {{ ledger.currency }}
              </td>
              <td class="px-4 py-2 text-right">
                <span
                  class="px-2 py-1 rounded text-xs"
                  :class="getAccountTypeClass(account.type) + ' dark:opacity-80'"
                >
                  {{ account.type }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 月度趋势 -->
    <div class="card">
      <h2 class="text-xl font-semibold mb-4">月度收支趋势</h2>
      <div v-if="loading" class="text-center py-8">
        <div
          class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
        ></div>
      </div>
      <div
          v-else-if="stats.monthlyTrend.length === 0"
          class="text-center py-8 text-gray-500 dark:text-gray-400"
        >
          <p>暂无月度趋势数据</p>
        </div>
      <div v-else class="space-y-4">
          <div
            v-for="(month, index) in stats.monthlyTrend"
            :key="index"
            class="flex flex-col"
          >
            <div class="flex justify-between items-center mb-1">
              <span class="font-medium dark:text-gray-200">{{ month.month }}</span>
              <span class="text-sm text-gray-500 dark:text-gray-400">
                收入: {{ month.income }} | 支出: {{ month.expense }} | 结余:
                {{ month.balance }}
              </span>
            </div>
            <div class="flex gap-1 h-6">
              <div
                class="bg-green-500 dark:bg-green-600 rounded-l"
                :style="{ width: month.incomePercentage + '%' }"
                title="收入"
              ></div>
              <div
                class="bg-red-500 dark:bg-red-600 rounded-r"
                :style="{ width: month.expensePercentage + '%' }"
                title="支出"
              ></div>
            </div>
          </div>
        </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useApi } from "~/composables/useApi";

const router = useRouter();
const { getLedger, getEntries, accounts } = useApi();

const loading = ref(true);
const ledger = ref({
  title: "",
  currency: "",
  entries_count: 0,
  errors_count: 0,
});
const entries = ref([] as any[]);
const accountBalances = ref([] as any[]);

// 日期范围
const dateRange = ref({
  startDate: new Date(new Date().getFullYear(), new Date().getMonth(), 1)
    .toISOString()
    .split("T")[0],
  endDate: new Date().toISOString().split("T")[0],
});

// 统计数据
const stats = ref({
  totalIncome: 0,
  totalExpense: 0,
  netIncome: 0,
  totalTransactions: 0,
  incomeByCategory: [] as {
    name: string;
    amount: number;
    percentage: number;
  }[],
  expenseByCategory: [] as {
    name: string;
    amount: number;
    percentage: number;
  }[],
  accounts: [] as { name: string; balance: number; type: string }[],
  monthlyTrend: [] as {
    month: string;
    income: number;
    expense: number;
    balance: number;
    incomePercentage: number;
    expensePercentage: number;
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
    "#E74C3C",
    "#3498DB",
    "#2ECC71",
    "#F39C12",
    "#9B59B6",
  ];
  return colors[index % colors.length];
};

// 获取账户类型样式
const getAccountTypeClass = (type: string) => {
  const typeMap: Record<string, string> = {
    Assets: "bg-blue-100 text-blue-800",
    Liabilities: "bg-red-100 text-red-800",
    Equity: "bg-green-100 text-green-800",
    Income: "bg-purple-100 text-purple-800",
    Expenses: "bg-yellow-100 text-yellow-800",
  };
  return typeMap[type] || "bg-gray-100 text-gray-800";
};

// 重置日期范围
const resetDateRange = () => {
  dateRange.value.startDate = new Date(
    new Date().getFullYear(),
    new Date().getMonth(),
    1
  )
    .toISOString()
    .split("T")[0];
  dateRange.value.endDate = new Date().toISOString().split("T")[0];
};

// 刷新统计数据
const refreshStats = async () => {
  try {
    loading.value = true;

    // 获取数据
    ledger.value = await getLedger();
    const entriesResult = await getEntries({
      start_date: dateRange.value.startDate,
      end_date: dateRange.value.endDate,
    });
    entries.value = entriesResult.entries
      ? entriesResult.entries
      : entriesResult;

    const balancesResult = await accounts.getAccountBalances({
      start_date: dateRange.value.startDate,
      end_date: dateRange.value.endDate,
    });
    accountBalances.value = balancesResult;

    // 计算统计数据
    calculateStats();
  } catch (error) {
    console.error("Error refreshing stats:", error);
  } finally {
    loading.value = false;
  }
};

// 计算统计数据
const calculateStats = () => {
  let totalIncome = 0;
  let totalExpense = 0;
  const incomeByCategory: Record<string, number> = {};
  const expenseByCategory: Record<string, number> = {};

  // 计算总收入和总支出
  entries.value
    .filter((e: any) => e.type === "Transaction")
    .forEach((entry: any) => {
      if (entry.postings) {
        entry.postings.forEach((posting: any) => {
          if (posting.units && posting.units.number) {
            const amount = posting.units.number;
            const account = posting.account;

            // 判断是收入还是支出
            if (account.startsWith("Income")) {
              totalIncome += Math.abs(amount);

              // 按分类统计收入
              const categoryParts = account.split(":");
              if (categoryParts.length >= 2) {
                const category = categoryParts[1];
                incomeByCategory[category] =
                  (incomeByCategory[category] || 0) + Math.abs(amount);
              }
            } else if (account.startsWith("Expenses")) {
              totalExpense += Math.abs(amount);

              // 按分类统计支出
              const categoryParts = account.split(":");
              if (categoryParts.length >= 2) {
                const category = categoryParts[1];
                expenseByCategory[category] =
                  (expenseByCategory[category] || 0) + Math.abs(amount);
              }
            }
          }
        });
      }
    });

  // 转换为数组并计算百分比
  const incomeCategories = Object.entries(incomeByCategory)
    .map(([name, amount]) => ({
      name,
      amount: Number(amount.toFixed(2)),
      percentage:
        totalIncome > 0 ? Math.round((amount / totalIncome) * 100) : 0,
    }))
    .sort((a, b) => b.amount - a.amount);

  const expenseCategories = Object.entries(expenseByCategory)
    .map(([name, amount]) => ({
      name,
      amount: Number(amount.toFixed(2)),
      percentage:
        totalExpense > 0 ? Math.round((amount / totalExpense) * 100) : 0,
    }))
    .sort((a, b) => b.amount - a.amount);

  // 处理账户统计
  const accountStats = accountBalances.value.map((account: any) => {
    let type = "Unknown";
    if (account.name.startsWith("Assets")) type = "Assets";
    else if (account.name.startsWith("Liabilities")) type = "Liabilities";
    else if (account.name.startsWith("Equity")) type = "Equity";
    else if (account.name.startsWith("Income")) type = "Income";
    else if (account.name.startsWith("Expenses")) type = "Expenses";

    return {
      name: account.name,
      balance: Number(account.balance.toFixed(2)),
      type,
    };
  });

  // 计算月度趋势
  const monthlyTrend = calculateMonthlyTrend();

  // 更新统计数据
  stats.value = {
    totalIncome: Number(totalIncome.toFixed(2)),
    totalExpense: Number(totalExpense.toFixed(2)),
    netIncome: Number((totalIncome - totalExpense).toFixed(2)),
    totalTransactions: entries.value.filter(
      (e: any) => e.type === "Transaction"
    ).length,
    incomeByCategory: incomeCategories,
    expenseByCategory: expenseCategories,
    accounts: accountStats,
    monthlyTrend,
  };
};

// 计算月度趋势
const calculateMonthlyTrend = () => {
  const monthlyData: Record<string, { income: number; expense: number }> = {};

  // 按月份分组计算
  entries.value
    .filter((e: any) => e.type === "Transaction")
    .forEach((entry: any) => {
      const month = entry.date.substring(0, 7); // YYYY-MM

      if (!monthlyData[month]) {
        monthlyData[month] = { income: 0, expense: 0 };
      }

      if (entry.postings) {
        entry.postings.forEach((posting: any) => {
          if (posting.units && posting.units.number) {
            const amount = posting.units.number;
            const account = posting.account;

            if (account.startsWith("Income")) {
              monthlyData[month].income += Math.abs(amount);
            } else if (account.startsWith("Expenses")) {
              monthlyData[month].expense += Math.abs(amount);
            }
          }
        });
      }
    });

  // 转换为数组并计算百分比
  return Object.entries(monthlyData)
    .map(([month, data]) => {
      const total = data.income + data.expense;
      const incomePercentage =
        total > 0 ? Math.round((data.income / total) * 100) : 0;
      const expensePercentage =
        total > 0 ? Math.round((data.expense / total) * 100) : 0;

      return {
        month,
        income: Number(data.income.toFixed(2)),
        expense: Number(data.expense.toFixed(2)),
        balance: Number((data.income - data.expense).toFixed(2)),
        incomePercentage,
        expensePercentage,
      };
    })
    .sort((a, b) => a.month.localeCompare(b.month));
};

// 页面加载时刷新数据
onMounted(() => {
  refreshStats();
});
</script>