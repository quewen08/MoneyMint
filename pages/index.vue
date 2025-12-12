<template>
  <div class="container">
    <header>
      <nav>
        <div class="logo">MoneyMint</div>
        <div>
          <NuxtLink to="/transactions" class="btn btn-primary mr-2"
            >交易记录</NuxtLink
          >
          <button @click="openModal" class="btn btn-primary mr-2">
            记录交易
          </button>
          <NuxtLink to="/accounts" class="btn btn-secondary">账户管理</NuxtLink>
        </div>
      </nav>
    </header>

    <main class="mt-6">
      <h1 class="text-2xl font-bold mb-4">财务总览</h1>

      <!-- 本月收支概览 -->
      <section class="mb-8">
        <h2 class="text-xl font-bold mb-4">本月概览</h2>
        <div class="stats-grid">
          <div class="stat-card bg-green-50 border-green-200">
            <div class="stat-value text-green-600">
              {{ formatCurrency(monthStats.totalIncome) }}
            </div>
            <div class="stat-label">本月收入</div>
          </div>
          <div class="stat-card bg-red-50 border-red-200">
            <div class="stat-value text-red-600">
              {{ formatCurrency(monthStats.totalExpense) }}
            </div>
            <div class="stat-label">本月支出</div>
          </div>
          <div class="stat-card bg-blue-50 border-blue-200">
            <div class="stat-value text-blue-600">
              {{ formatCurrency(monthStats.netAmount) }}
            </div>
            <div class="stat-label">本月结余</div>
          </div>
        </div>
      </section>

      <!-- 账户统计 -->
      <section class="mb-8">
        <h2 class="text-xl font-bold mb-4">账户统计</h2>
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-value">{{ accounts.length }}</div>
            <div class="stat-label">账户数量</div>
          </div>
          <div class="stat-card bg-green-50 border-green-200">
            <div class="stat-value text-green-600">
              {{ formatCurrency(totalAssets) }}
            </div>
            <div class="stat-label">总资产</div>
          </div>
          <div class="stat-card bg-red-50 border-red-200">
            <div class="stat-value text-red-600">
              {{ formatCurrency(totalLiabilities) }}
            </div>
            <div class="stat-label">总负债</div>
          </div>
          <div class="stat-card bg-purple-50 border-purple-200">
            <div class="stat-value text-purple-600">
              {{ formatCurrency(netWorth) }}
            </div>
            <div class="stat-label">净资产</div>
          </div>
        </div>
      </section>

      <!-- 收支趋势图表 -->
      <section class="mb-8">
        <h2 class="text-xl font-bold mb-4">收支趋势</h2>
        <div class="stats-grid">
          <!-- 今日 -->
          <div class="stat-card bg-white border-gray-200 shadow-sm">
            <div class="text-sm text-gray-500 mb-1">
              {{ todayStats.dateText }}
            </div>
            <h3 class="text-lg font-semibold mb-2">今日</h3>
            <div class="flex justify-between">
              <div class="text-sm">
                <div class="flex items-center">
                  <span class="w-2 h-2 bg-green-500 rounded-full mr-1"></span>
                  <span class="text-gray-600">收入:</span>
                </div>
                <div class="text-green-600 mt-1">
                  {{ formatCurrency(todayStats.income) }}
                </div>
              </div>
              <div class="text-sm">
                <div class="flex items-center">
                  <span class="w-2 h-2 bg-red-500 rounded-full mr-1"></span>
                  <span class="text-gray-600">支出:</span>
                </div>
                <div class="text-red-600 mt-1">
                  {{ formatCurrency(todayStats.expense) }}
                </div>
              </div>
            </div>
          </div>

          <!-- 本周 -->
          <div class="stat-card bg-white border-gray-200 shadow-sm">
            <div class="text-sm text-gray-500 mb-1">
              {{ weekStats.dateText }}
            </div>
            <h3 class="text-lg font-semibold mb-2">本周</h3>
            <div class="flex justify-between">
              <div class="text-sm">
                <div class="flex items-center">
                  <span class="w-2 h-2 bg-green-500 rounded-full mr-1"></span>
                  <span class="text-gray-600">收入:</span>
                </div>
                <div class="text-green-600 mt-1">
                  {{ formatCurrency(weekStats.income) }}
                </div>
              </div>
              <div class="text-sm">
                <div class="flex items-center">
                  <span class="w-2 h-2 bg-red-500 rounded-full mr-1"></span>
                  <span class="text-gray-600">支出:</span>
                </div>
                <div class="text-red-600 mt-1">
                  {{ formatCurrency(weekStats.expense) }}
                </div>
              </div>
            </div>
          </div>

          <!-- 本月 -->
          <div class="stat-card bg-white border-gray-200 shadow-sm">
            <div class="text-sm text-gray-500 mb-1">
              {{ monthStats.dateText }}
            </div>
            <h3 class="text-lg font-semibold mb-2">本月</h3>
            <div class="flex justify-between">
              <div class="text-sm">
                <div class="flex items-center">
                  <span class="w-2 h-2 bg-green-500 rounded-full mr-1"></span>
                  <span class="text-gray-600">收入:</span>
                </div>
                <div class="text-green-600 mt-1">
                  {{ formatCurrency(monthStats.income) }}
                </div>
              </div>
              <div class="text-sm">
                <div class="flex items-center">
                  <span class="w-2 h-2 bg-red-500 rounded-full mr-1"></span>
                  <span class="text-gray-600">支出:</span>
                </div>
                <div class="text-red-600 mt-1">
                  {{ formatCurrency(monthStats.expense) }}
                </div>
              </div>
            </div>
          </div>

          <!-- 本年 -->
          <div class="stat-card bg-white border-gray-200 shadow-sm">
            <div class="text-sm text-gray-500 mb-1">
              {{ yearStats.dateText }}
            </div>
            <h3 class="text-lg font-semibold mb-2">本年</h3>
            <div class="flex justify-between">
              <div class="text-sm">
                <div class="flex items-center">
                  <span class="w-2 h-2 bg-green-500 rounded-full mr-1"></span>
                  <span class="text-gray-600">收入:</span>
                </div>
                <div class="text-green-600 mt-1">
                  {{ formatCurrency(yearStats.income) }}
                </div>
              </div>
              <div class="text-sm">
                <div class="flex items-center">
                  <span class="w-2 h-2 bg-red-500 rounded-full mr-1"></span>
                  <span class="text-gray-600">支出:</span>
                </div>
                <div class="text-red-600 mt-1">
                  {{ formatCurrency(yearStats.expense) }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 收支趋势图表 -->
        <div class="card mt-6">
          <div ref="chartContainer" class="chart-container"></div>
        </div>
      </section>

      <!-- 快速操作 -->
      <section class="mb-8">
        <h2 class="text-xl font-bold mb-4">快速操作</h2>
        <div class="quick-actions">
          <NuxtLink to="/transactions" class="quick-action-card">
            <div class="quick-action-icon">📋</div>
            <div class="quick-action-label">查看交易记录</div>
          </NuxtLink>
          <NuxtLink to="/transactions" class="quick-action-card">
            <div class="quick-action-icon">➕</div>
            <div class="quick-action-label">添加新交易</div>
          </NuxtLink>
          <NuxtLink to="/accounts" class="quick-action-card">
            <div class="quick-action-icon">🏦</div>
            <div class="quick-action-label">管理账户</div>
          </NuxtLink>
        </div>
      </section>
    </main>

    <!-- Transaction Modal -->
    <AddTransactionModal />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch, onUnmounted } from "vue";
import * as echarts from "echarts";
import { useTransactionModal } from "~/composables/useTransactionModal";

const { openModal } = useTransactionModal();

// 获取账户列表
const { data: accounts } = await useAsyncData("accounts", async () => {
  const response = await $fetch("/api/accounts");
  return response.accounts || [];
});

// 获取所有交易记录
const { data: transactions } = await useAsyncData("transactions", async () => {
  const response = await $fetch("/api/transactions");
  return response.transactions || [];
});

// 获取月度收支数据
const { data: monthlyStats } = await useAsyncData("monthlyStats", async () => {
  const response = await $fetch("/api/monthly-stats");
  return response.monthlyStats || [];
});

// 计算账户统计数据
const totalAssets = computed(() => {
  return accounts.value
    .filter((account) => account.type === "asset")
    .reduce((sum, account) => sum + account.balance, 0);
});

const totalLiabilities = computed(() => {
  return accounts.value
    .filter((account) => account.type === "liability")
    .reduce((sum, account) => sum + Math.abs(account.balance), 0);
});

const netWorth = computed(() => {
  return totalAssets.value - totalLiabilities.value;
});

// 格式化日期为 YYYY-MM-DD
const formatDate = (date) => {
  return date.toISOString().split("T")[0];
};

// 日期工具函数
const getDateRange = (type) => {
  const now = new Date();
  const start = new Date(now);
  const end = new Date(now);

  switch (type) {
    case "today":
      start.setHours(0, 0, 0, 0);
      end.setHours(23, 59, 59, 999);
      return {
        start,
        end,
        dateText: formatDate(start),
      };
    case "week":
      const dayOfWeek = now.getDay() || 7; // 将周日视为第7天
      start.setDate(now.getDate() - dayOfWeek + 1);
      start.setHours(0, 0, 0, 0);
      end.setDate(now.getDate() + (7 - dayOfWeek));
      end.setHours(23, 59, 59, 999);
      return {
        start,
        end,
        dateText: `${formatDate(start)} - ${formatDate(end)}`,
      };
    case "month":
      start.setDate(1);
      start.setHours(0, 0, 0, 0);
      end.setMonth(now.getMonth() + 1);
      end.setDate(0);
      end.setHours(23, 59, 59, 999);
      return {
        start,
        end,
        dateText: `${start.getFullYear()}-${String(
          start.getMonth() + 1
        ).padStart(2, "0")}`,
      };
    case "year":
      start.setFullYear(now.getFullYear(), 0, 1);
      start.setHours(0, 0, 0, 0);
      end.setFullYear(now.getFullYear(), 11, 31);
      end.setHours(23, 59, 59, 999);
      return {
        start,
        end,
        dateText: `${start.getFullYear()}`,
      };
    default:
      return { start, end, dateText: "" };
  }
};

// 计算指定时间范围内的收支数据
const calculateStats = (dateRange) => {
  const { start, end } = dateRange;
  const filteredTransactions = transactions.value.filter((transaction) => {
    const transactionDate = new Date(transaction.date);
    return transactionDate >= start && transactionDate <= end;
  });

  let income = 0;
  let expense = 0;

  filteredTransactions.forEach((transaction) => {
    if (transaction.type === "income") {
      income += Math.abs(transaction.postings[0].amount);
    } else if (transaction.type === "expense") {
      expense += Math.abs(transaction.postings[0].amount);
    }
  });

  return { income, expense, dateText: dateRange.dateText };
};

// 不同时间段的收支数据
const todayStats = computed(() => calculateStats(getDateRange("today")));
const weekStats = computed(() => calculateStats(getDateRange("week")));
const monthStats = computed(() => {
  const baseStats = calculateStats(getDateRange("month"));
  return {
    ...baseStats,
    totalIncome: baseStats.income,
    totalExpense: baseStats.expense,
    netAmount: baseStats.income - baseStats.expense,
  };
});
const yearStats = computed(() => calculateStats(getDateRange("year")));

// 格式化货币
function formatCurrency(amount) {
  return new Intl.NumberFormat("zh-CN", {
    style: "currency",
    currency: "CNY",
  }).format(amount);
}

// 图表容器引用
const chartContainer = ref(null);
let chart = null;

// 准备月份标签
const months = Array.from({ length: 12 }, (_, i) => {
  const month = i + 1;
  return `${month}月`;
});

// 使用computed属性生成图表配置，这样当数据变化时配置会自动更新
const chartOption = computed(() => {
  const monthNames = [];
  const incomeAmounts = [];
  const expenseAmounts = [];
  let minAmount = 0;
  let maxAmount = 0;

  // 固定颜色配置，根据当前项目调整
  const expenseIncomeAmountColor = {
    incomeAmountColor: '#4CAF50', // 收入颜色
    expenseAmountColor: '#F44336'  // 支出颜色
  };

  if (monthlyStats.value) {
    for (const item of monthlyStats.value) {
      // 获取月份名称
      const monthIndex = parseInt(item.month);
      const monthShortName = `${monthIndex}月`;

      monthNames.push(monthShortName);
      incomeAmounts.push(item.income);
      expenseAmounts.push(-item.expense);

      if (item.income > maxAmount) {
        maxAmount = item.income;
      }

      if (-item.expense > maxAmount) {
        maxAmount = -item.expense;
      }

      if (item.income < minAmount) {
        minAmount = item.income;
      }

      if (-item.expense < minAmount) {
        minAmount = -item.expense;
      }
    }
  }

  const amountGap = maxAmount - minAmount;

  return {
    // 标题，位置底部
    title: {
      text: '月收支统计',
      textStyle: {
        color: '#333',
        fontSize: 16
      },
      position: 'bottom',
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow',
        shadowStyle: {
          color: 'rgba(120, 120, 120, 0.05)'
        }
      },
      backgroundColor: '#fff',
      borderColor: '#fff',
      textStyle: {
        color: '#333'
      },
      formatter: (params) => {
        let incomeAmount = null;
        let expenseAmount = null;

        for (const param of params) {
          const dataIndex = param.dataIndex;
          const data = monthlyStats.value[dataIndex];

          if (param.seriesId === 'seriesIncome') {
            incomeAmount = formatCurrency(data.income);
          } else if (param.seriesId === 'seriesExpense') {
            expenseAmount = formatCurrency(data.expense);
          }
        }

        return `<table>` +
            `<thead>` +
            `<tr>` +
            `<td colspan="2" class="text-start">${params[0]?.name}</td>` +
            `</tr>` +
            `</thead>` +
            `<tbody>` +
            (incomeAmount !== null ?
                `<tr>` +
                `<td><span style="display:inline-block;width:10px;height:10px;background-color:#4CAF50;border-radius:50%;margin-right:5px;"></span><span style="margin-right:16px;">收入</span></td>` +
                `<td><strong>${incomeAmount}</strong></td>` +
                `</tr>` : '') +
            (expenseAmount !== null ?
                `<tr>` +
                `<td><span style="display:inline-block;width:10px;height:10px;background-color:#F44336;border-radius:50%;margin-right:5px;"></span><span style="margin-right:16px;">支出</span></td>` +
                `<td><strong>${expenseAmount}</strong></td>` +
                `</tr>` : '') +
            `</tbody>` +
            `</table>`;
      }
    },
    legend: {
      bottom: 20,
      itemWidth: 14,
      itemHeight: 14,
      textStyle: {
        color: '#333'
      },
      icon: 'circle',
      data: ['收入', '支出']
    },
    grid: {
      left: '20px',
      right: '20px',
      top: '10px',
      bottom: '100px'
    },
    xAxis: [
      {
        type: 'category',
        data: monthNames,
        axisLine: {
          show: false
        },
        axisTick: {
          show: false
        },
        axisLabel: {
          padding: [20, 0, 0, 0]
        }
      }
    ],
    yAxis: [
      {
        type: 'value',
        min: minAmount - amountGap / 20,
        max: maxAmount,
        splitNumber: 10,
        axisLabel: {
          show: false
        },
        splitLine: {
          show: false
        }
      },
      {
        type: 'value',
        min: minAmount,
        max: maxAmount + amountGap / 20,
        splitNumber: 10,
        axisLabel: {
          show: false
        },
        splitLine: {
          show: false
        }
      }
    ],
    series: [
      {
        type: 'bar',
        id: 'seriesIncome',
        name: '收入',
        yAxisIndex: 0,
        stack: 'Total',
        itemStyle: {
          color: expenseIncomeAmountColor.incomeAmountColor,
          borderRadius: 16
        },
        emphasis: {
          focus: 'series',
          labelLine: {
            show: false
          }
        },
        barMaxWidth: 16,
        data: incomeAmounts
      },
      {
        type: 'bar',
        id: 'seriesExpense',
        name: '支出',
        yAxisIndex: 1,
        stack: 'Total',
        itemStyle: {
          color: expenseIncomeAmountColor.expenseAmountColor,
          borderRadius: 16
        },
        emphasis: {
          focus: 'series',
          labelLine: {
            show: false
          }
        },
        barMaxWidth: 16,
        data: expenseAmounts
      }
    ]
  };
});

// 监听图表配置变化，更新图表
watch(
  chartOption,
  (newOption) => {
    if (chart) {
      chart.setOption(newOption, true);
    }
  },
  { deep: true }
);

// 初始化图表
onMounted(() => {
  if (chartContainer.value) {
    chart = echarts.init(chartContainer.value);
    chart.setOption(chartOption.value);

    // 响应窗口大小变化
    window.addEventListener("resize", () => {
      chart.resize();
    });
  }
});

// 组件卸载时清理资源
onUnmounted(() => {
  if (chart) {
    chart.dispose();
  }
  window.removeEventListener("resize", () => {
    chart && chart.resize();
  });
});
</script>

<style scoped>
/* 通用样式 */
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

.text-green-600 {
  color: #10b981;
}

.text-red-600 {
  color: #ef4444;
}

.text-blue-600 {
  color: #3b82f6;
}

.text-purple-600 {
  color: #8b5cf6;
}

.bg-green-50 {
  background-color: #f0fdf4;
}

.bg-red-50 {
  background-color: #fef2f2;
}

.bg-blue-50 {
  background-color: #eff6ff;
}

.bg-purple-50 {
  background-color: #faf5ff;
}

.bg-gray-50 {
  background-color: #f9fafb;
}

.border-green-200 {
  border-color: #bbf7d0;
}

.border-red-200 {
  border-color: #fecaca;
}

.border-blue-200 {
  border-color: #bfdbfe;
}

.border-purple-200 {
  border-color: #e9d5ff;
}

.border-gray-200 {
  border-color: #e5e7eb;
}

.shadow-sm {
  box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
}

/* 统计卡片网格 */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
  margin-bottom: 2rem;
}

/* 统计卡片样式 */
.stat-card {
  background-color: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  padding: 1.5rem;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1),
    0 2px 4px -1px rgba(0, 0, 0, 0.06);
}

.stat-value {
  font-size: 1.75rem;
  font-weight: bold;
  margin-bottom: 0.5rem;
}

.stat-label {
  font-size: 0.875rem;
  color: #6b7280;
}

/* 图表容器 */
.chart-container {
  width: 100%;
  height: 400px;
  padding: 1rem 0;
}

/* 快速操作 */
.quick-actions {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 1rem;
}

.quick-action-card {
  background-color: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  padding: 2rem 1.5rem;
  text-align: center;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  text-decoration: none;
  color: #111827;
}

.quick-action-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1),
    0 2px 4px -1px rgba(0, 0, 0, 0.06);
}

.quick-action-icon {
  font-size: 2rem;
  margin-bottom: 0.5rem;
}

.quick-action-label {
  font-weight: 500;
  font-size: 0.875rem;
}

/* 部分样式 */
.mb-8 {
  margin-bottom: 2rem;
}

.mt-6 {
  margin-top: 1.5rem;
}

.text-center {
  text-align: center;
}

.text-gray-500 {
  color: #6b7280;
}
</style>