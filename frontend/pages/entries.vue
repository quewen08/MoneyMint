<template>
  <div class="mx-auto">
    <div class="hidden flex justify-between items-center mb-6">
      <h2 class="text-2xl font-bold dark:text-white">记账记录</h2>
      <button @click="showAddModal = true" class="btn btn-primary">
        + 新增
      </button>
    </div>

    <div class="card">
      <div class="mb-6 p-4 bg-gray-50 dark:bg-gray-800 rounded">
        
        <div class="flex flex-row items-center justify-between gap-4">         
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">记录类型</label>
            <select v-model="filters.type"
              class="w-full md:w-auto px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100">
              <option value="">全部类型</option>
              <option value="Transaction">交易</option>
              <option value="Open">开户</option>
              <option value="Close">销户</option>
              <option value="Balance">余额</option>
              <option value="Pad">补账</option>
              <option value="Note">备注</option>
            </select>
          </div>
          
          <div class="flex gap-2">
            <button @click="applyFilters" class="btn btn-primary">应用筛选</button>
            <button @click="resetFilters" class="btn btn-secondary">重置</button>
          </div>
        </div>
        
        <!-- 日历展示 -->
        <div class="mt-4">
          <div class="flex justify-between items-center mb-3">
            <button @click="changeMonth(-1)" class="btn btn-sm btn-secondary">
              &lt; 上一月
            </button>
            <h3 class="text-lg font-medium dark:text-white">{{ currentMonthYear }}</h3>
            <button @click="changeMonth(1)" class="btn btn-sm btn-secondary">
              下一月 &gt;
            </button>
          </div>
          
          <!-- 星期标题 -->
          <div class="grid grid-cols-7 gap-1 mb-1">
            <div v-for="day in weekDays" :key="day" class="text-center text-sm font-medium text-gray-500 dark:text-gray-400 py-2">
              {{ day }}
            </div>
          </div>
          
          <!-- 日历网格 -->
          <div class="grid grid-cols-7 gap-1">
            <div 
              v-for="day in calendarDays" 
              :key="day.date"
              class="aspect-square flex flex-col items-center justify-center rounded cursor-pointer transition-colors p-1"
              :class="{
                'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400': !day.isCurrentMonth,
                'hover:bg-primary/20 dark:hover:bg-primary/30': day.isCurrentMonth,
                'bg-primary text-white font-medium': day.date === selectedDate
              }"
              @click="selectDate(day.date)"
            >
              <div class="text-sm">{{ day.day }}</div>
              <!-- 农历日期 - 有收支记录时隐藏 -->
              <div v-if="!dailyStats[day.date]?.income && !dailyStats[day.date]?.expense" class="text-[8px] md:text-xs opacity-70">{{ getLunarDay(day.date) }}</div>
              <!-- 每日收支 - 有收支记录时显示 -->
              <div v-else class="flex gap-1 mt-1 text-[8px] md:text-xs">
                <div v-if="dailyStats[day.date]?.income > 0" class="text-green-600 dark:text-green-400">
                  +{{ dailyStats[day.date].income.toFixed(2) }}
                </div>
                <div v-if="dailyStats[day.date]?.expense > 0" class="text-red-600 dark:text-red-400">
                  -{{ dailyStats[day.date].expense.toFixed(2) }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 当月统计信息 -->
        <div class="mt-4 p-4 bg-white dark:bg-gray-700 rounded-lg shadow-sm">
          <div class="flex flex-wrap gap-4 justify-between">
            <div>
              <div class="text-sm text-gray-500 dark:text-gray-400">总收入</div>
              <div class="text-xl font-bold text-green-600 dark:text-green-400">¥{{ monthlyStats.totalIncome.toFixed(2)
                }}</div>
            </div>
            <div>
              <div class="text-sm text-gray-500 dark:text-gray-400">总支出</div>
              <div class="text-xl font-bold text-red-600 dark:text-red-400">¥{{ monthlyStats.totalExpense.toFixed(2) }}
              </div>
            </div>
            <div>
              <div class="text-sm text-gray-500 dark:text-gray-400">结余</div>
              <div class="text-xl font-bold"
                :class="monthlyStats.netIncome >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                ¥{{ monthlyStats.netIncome.toFixed(2) }}</div>
            </div>
            <div>
              <div class="text-sm text-gray-500 dark:text-gray-400">日均支出</div>
              <div class="text-xl font-bold text-red-600 dark:text-red-400">¥{{
                monthlyStats.averageDailyExpense.toFixed(2) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="text-center py-8">
        <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>

      <!-- 无数据状态 -->
      <div v-else-if="entries.length === 0" class="text-center py-8 text-gray-500 dark:text-gray-400">
        <p>暂无记账记录</p>
      </div>

      <!-- 记账记录列表 -->
      <div v-else class="space-y-6">
        <div v-for="entry in entries" :key="entry.meta?.filename + ':' + entry.meta?.lineno"
          class="border-b pb-5 last:border-0">
          <div class="flex flex-col md:flex-row md:justify-between md:items-start mb-3">
            <div>
              <span class="font-medium text-lg">{{ entry.date }}</span>
              <span class="ml-2 text-sm text-gray-500">{{ entry.type }}</span>
            </div>

            <!-- 标签显示 -->
            <div v-if="entry.tags && entry.tags.length > 0" class="mt-2 md:mt-0">
              <span v-for="(tag, index) in entry.tags" :key="index"
                class="inline-block bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs px-2 py-1 rounded mr-1 mb-1">
                #{{ tag }}
              </span>
            </div>
          </div>

          <!-- 交易描述 -->
          <div v-if="entry.narration" class="ml-4 mb-3 text-sm text-gray-700 dark:text-gray-300">
            <span>{{ entry.narration }}</span>
          </div>

          <!-- 交易记录详情 -->
          <div v-if="entry.type === 'Transaction'" class="ml-4 space-y-2">
            <div v-for="(posting, index) in entry.postings" :key="index"
              class="flex justify-between items-center p-2 rounded bg-gray-50 dark:bg-gray-800/70 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
              <span class="text-sm font-medium dark:text-gray-300">{{ posting.account }}</span>
              <span class="text-sm font-medium"
                :class="posting.units?.number > 0 ? 'text-success' : posting.units?.number < 0 ? 'text-danger' : ''">
                {{ posting.units?.number || "" }}
                {{ posting.units?.currency || "" }}
              </span>
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="ml-4 mt-3 flex space-x-2">
            <button @click="openEditModal({ ...entry, id: `${entry.meta?.filename}:${entry.meta?.lineno}` })"
              class="px-3 py-1 text-sm bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 transition-colors">
              编辑
            </button>
            <button @click="openCopyModal({ ...entry, id: `${entry.meta?.filename}:${entry.meta?.lineno}` })"
              class="px-3 py-1 text-sm bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 rounded hover:bg-green-200 dark:hover:bg-green-800 transition-colors">
              复制
            </button>
          </div>

          <!-- 账户记录详情 -->
          <div v-if="entry.type === 'Open'" class="ml-4 p-3 rounded bg-gray-50 dark:bg-gray-800/70">
            <span class="text-sm font-medium dark:text-gray-300">打开账户: {{ entry.account }}</span>
          </div>
        </div>
      </div>

      <!-- 记录统计 -->
      <div v-if="!loading && entries.length > 0" class="mt-6 text-sm text-gray-600 dark:text-gray-400">
        共 {{ entries.length }} 条记录
      </div>
    </div>
  </div>
  <!-- Add Entry Drawer -->
  <AddEntryModal v-if="showAddModal || showEditModal" @close="closeModal" @entry-added="handleEntryAdded"
    @entry-updated="handleEntryUpdated" @entry-deleted="handleEntryDeleted" :entry="editingEntry" />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from "vue";
import AddEntryModal from "~/components/AddEntryModal.vue";
import { useApi } from "~/composables/useApi";
import dayjs from "dayjs";
import { getLunarDay } from "~/utils/lunarUtils";

const { getEntries } = useApi();

// 状态管理
const loading = ref(true);
const entries = ref([] as any[]);
const monthlyStats = ref({
  totalIncome: 0,
  totalExpense: 0,
  netIncome: 0,
  averageDailyExpense: 0
});
// 模态框状态
const showAddModal = ref(false);
const showEditModal = ref(false);
const editingEntry = ref<any>(null);

// 每日收支数据
const dailyStats = ref({} as Record<string, { income: number; expense: number }>);

// 日期选择相关
const selectedDate = ref(dayjs().format('YYYY-MM-DD'));
const currentDate = ref(new Date());
const weekDays = ref(['日', '一', '二', '三', '四', '五', '六']);
const calendarDays = ref<any[]>([]);

// 筛选条件
const filters = ref({
  start_date: '',
  end_date: '',
  type: 'Transaction' // 默认显示Transaction类型
});

// 计算当前月份和年份的显示
const currentMonthYear = computed(() => {
  return dayjs(currentDate.value).format('YYYY年MM月');
});

// 生成日历天数
const generateCalendar = () => {
  const year = currentDate.value.getFullYear();
  const month = currentDate.value.getMonth();
  
  // 获取当月第一天
  const firstDay = new Date(year, month, 1);
  // 获取当月最后一天
  const lastDay = new Date(year, month + 1, 0);
  // 获取当月第一天是星期几
  const startDay = firstDay.getDay();
  // 获取当月的天数
  const daysInMonth = lastDay.getDate();
  
  const days = [];
  
  // 添加上个月的日期
  for (let i = startDay - 1; i >= 0; i--) {
    const date = new Date(year, month, -i);
    days.push({
      date: dayjs(date).format('YYYY-MM-DD'),
      day: date.getDate(),
      isCurrentMonth: false
    });
  }
  
  // 添加当月的日期
  for (let i = 1; i <= daysInMonth; i++) {
    const date = new Date(year, month, i);
    days.push({
      date: dayjs(date).format('YYYY-MM-DD'),
      day: i,
      isCurrentMonth: true
    });
  }
  
  // 添加下个月的日期，使日历完整
  const remainingDays = 42 - days.length; // 6行7列共42天
  for (let i = 1; i <= remainingDays; i++) {
    const date = new Date(year, month + 1, i);
    days.push({
      date: dayjs(date).format('YYYY-MM-DD'),
      day: i,
      isCurrentMonth: false
    });
  }
  
  calendarDays.value = days;
};

// 选择日期
const selectDate = (date: string) => {
  selectedDate.value = date;
  applyFilters();
};

// 切换月份
const changeMonth = (direction: number) => {
  currentDate.value.setMonth(currentDate.value.getMonth() + direction);
  generateCalendar();
};

// 计算每月统计数据
const calculateMonthlyStats = (entries: any[], targetMonth: string) => {
  // 使用传入的目标月份，而不是始终使用当前月份
  
  let totalIncome = 0;
  let totalExpense = 0;
  
  // 重置每日统计
  dailyStats.value = {};
  
  entries.forEach(entry => {
    const entryMonth = dayjs(entry.date).format('YYYY-MM');
    const entryDate = dayjs(entry.date).format('YYYY-MM-DD');
    
    // 只处理目标月份的交易记录
    if (entryMonth === targetMonth && entry.type === 'Transaction') {
      entry.postings.forEach((posting: any) => {
        const amount = parseFloat(posting.units?.number || '0');
        
        // 根据账户类型判断是收入还是支出
        if (posting.account.includes('Income')) {
          totalIncome += amount;
          // 更新每日收入
          if (!dailyStats.value[entryDate]) {
            dailyStats.value[entryDate] = { income: 0, expense: 0 };
          }
          dailyStats.value[entryDate].income += amount;
        } else if (posting.account.includes('Expenses')) {
          totalExpense += Math.abs(amount); // 支出通常是负数，取绝对值
          // 更新每日支出
          if (!dailyStats.value[entryDate]) {
            dailyStats.value[entryDate] = { income: 0, expense: 0 };
          }
          dailyStats.value[entryDate].expense += Math.abs(amount);
        }
      });
    }
  });
  
  const netIncome = totalIncome - totalExpense;
  const daysInMonth = dayjs(currentDate.value).daysInMonth();
  const averageDailyExpense = daysInMonth > 0 ? totalExpense / daysInMonth : 0;
  
  monthlyStats.value = {
    totalIncome,
    totalExpense,
    netIncome,
    averageDailyExpense
  };
};

// 加载数据的函数
const loadEntries = async () => {
  try {
    loading.value = true;
    
    // 加载当月所有数据
    const monthStart = dayjs(currentDate.value).startOf('month').format('YYYY-MM-DD');
    const monthEnd = dayjs(currentDate.value).endOf('month').format('YYYY-MM-DD');
    
    const monthlyResponse = await getEntries({
      start_date: monthStart,
      end_date: monthEnd,
      type: 'Transaction',
      sort: 'date',
      order: 'desc'
    });
    
    // 计算统计信息 - 传入当前月份参数
    calculateMonthlyStats(monthlyResponse.entries || [], dayjs(currentDate.value).format('YYYY-MM'));
    
    // 根据筛选条件过滤当日条目
    if (filters.value.start_date && filters.value.end_date) {
      // 确保日期格式一致
      const filterDate = dayjs(filters.value.start_date).format('YYYY-MM-DD');
      entries.value = monthlyResponse.entries?.filter((entry: any) => 
        dayjs(entry.date).format('YYYY-MM-DD') === filterDate
      ) || [];
    } else {
      entries.value = monthlyResponse.entries || [];
    }
  } catch (error) {
    console.error("Error loading entries:", error);
  } finally {
    loading.value = false;
  }
};

// 应用筛选条件
const applyFilters = () => {
  // 设置开始日期和结束日期为选中的日期
  filters.value.start_date = selectedDate.value;
  filters.value.end_date = selectedDate.value;
  loadEntries();
};

// 重置筛选条件
const resetFilters = () => {
  // 重置为当前日期
  selectedDate.value = dayjs().format('YYYY-MM-DD');
  currentDate.value = new Date();
  generateCalendar();
  
  filters.value = {
    start_date: selectedDate.value,
    end_date: selectedDate.value,
    type: 'Transaction' // 默认显示Transaction类型
  };
  loadEntries();
};



// 初始加载数据
onMounted(async () => {
  // 初始化日历
  generateCalendar();
  
  // 默认筛选当前日期
  filters.value.start_date = selectedDate.value;
  filters.value.end_date = selectedDate.value;
  await loadEntries();

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
    console.log("Refreshing entries due to global SSE event...");
    loadEntries();
  }
};

// 处理手动触发的刷新事件
const handleRefreshData = () => {
  console.log("Refreshing entries due to manual refresh event...");
  loadEntries();
};

// 监听选中日期变化
watch(selectedDate, (newDate) => {
  // 当选中日期不在当前显示的月份时，切换到对应月份
  const selectedMonth = dayjs(newDate).month();
  const currentMonth = currentDate.value.getMonth();
  if (selectedMonth !== currentMonth) {
    currentDate.value = dayjs(newDate).toDate();
    generateCalendar();
  }
});

// 打开编辑模态框
const openEditModal = (entry: any) => {
  editingEntry.value = entry;
  showEditModal.value = true;
};

// 打开复制模态框
const openCopyModal = (entry: any) => {
  // 创建一个新的条目对象，不包含原条目的ID和元数据
  const copiedEntry = {
    ...entry,
    id: '', // 清空ID，使其成为新条目
    date: dayjs().format('YYYY-MM-DD') // 默认使用当前日期
  };
  editingEntry.value = copiedEntry;
  showAddModal.value = true; // 使用添加模态框而不是编辑模态框
};

// 关闭模态框
const closeModal = () => {
  showAddModal.value = false;
  showEditModal.value = false;
  editingEntry.value = null;
};

// 处理条目添加
const handleEntryAdded = () => {
  closeModal();
  loadEntries();
};

// 处理条目更新
const handleEntryUpdated = () => {
  closeModal();
  loadEntries();
};

// 处理条目删除
const handleEntryDeleted = () => {
  closeModal();
  loadEntries();
};

// 监听月份变化，重新加载数据
watch(() => currentDate.value.getMonth(), () => {
  loadEntries();
});

// 组件卸载时移除事件监听
onUnmounted(() => {
  window.removeEventListener("sse:data-updated", handleSseUpdate);
  window.removeEventListener("refreshData", handleRefreshData);
});
</script>