<template>
  <div class="mx-auto">
    <div class="flex justify-between items-center mb-6">
      <h2 class="text-2xl font-bold dark:text-white">记账记录</h2>
      <button @click="showAddModal = true" class="btn btn-primary">
        + 新增
      </button>
    </div>

      <div class="card">
      <!-- 筛选条件 -->
      <div class="mb-6 p-4 bg-gray-50 dark:bg-gray-800 rounded">
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-4">
          <!-- 开始日期 -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">开始日期</label>
            <input
              type="date"
              v-model="filters.start_date"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
            />
          </div>
          
          <!-- 结束日期 -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">结束日期</label>
            <input
              type="date"
              v-model="filters.end_date"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
            />
          </div>
          
          <!-- 排序方式 -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">排序方式</label>
            <select
              v-model="filters.sort"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
            >
              <option value="date">日期</option>
              <option value="narration">描述</option>
            </select>
          </div>
          
          <!-- 排序顺序 -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">排序顺序</label>
            <select
              v-model="filters.order"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
            >
              <option value="desc">降序</option>
              <option value="asc">升序</option>
            </select>
          </div>
        </div>
        
        <!-- 操作按钮 -->
        <div class="flex flex-wrap gap-2">
          <button @click="applyFilters" class="btn btn-primary">应用筛选</button>
          <button @click="resetFilters" class="btn btn-secondary">重置</button>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="text-center py-8">
        <div
          class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
        ></div>
      </div>

      <!-- 无数据状态 -->
      <div
        v-else-if="entries.length === 0"
        class="text-center py-8 text-gray-500 dark:text-gray-400"
      >
        <p>暂无记账记录</p>
      </div>

      <!-- 记账记录列表 -->
      <div v-else class="space-y-6">
        <div
          v-for="entry in entries"
          :key="entry.meta?.filename + ':' + entry.meta?.lineno"
          class="border-b pb-5 last:border-0"
        >
          <div class="flex flex-col md:flex-row md:justify-between md:items-start mb-3">
            <div>
              <span class="font-medium text-lg">{{ entry.date }}</span>
              <span class="ml-2 text-sm text-gray-500">{{ entry.type }}</span>
            </div>
            
            <!-- 标签显示 -->
            <div v-if="entry.tags && entry.tags.length > 0" class="mt-2 md:mt-0">
              <span 
                v-for="(tag, index) in entry.tags" 
                :key="index"
                class="inline-block bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs px-2 py-1 rounded mr-1 mb-1"
              >
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
            <div
              v-for="(posting, index) in entry.postings"
              :key="index"
              class="flex justify-between items-center p-2 rounded bg-gray-50 dark:bg-gray-800/70 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            >
              <span class="text-sm font-medium dark:text-gray-300">{{ posting.account }}</span>
              <span class="text-sm font-medium"
                :class="posting.units?.number > 0 ? 'text-success' : posting.units?.number < 0 ? 'text-danger' : ''"
              >
                {{ posting.units?.number || "" }}
                {{ posting.units?.currency || "" }}
              </span>
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="ml-4 mt-3 flex space-x-2">
            <button
              @click="openEditModal({ ...entry, id: `${entry.meta?.filename}:${entry.meta?.lineno}` })"
              class="px-3 py-1 text-sm bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 transition-colors"
            >
              编辑
            </button>
            <button
              @click="openCopyModal({ ...entry, id: `${entry.meta?.filename}:${entry.meta?.lineno}` })"
              class="px-3 py-1 text-sm bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 rounded hover:bg-green-200 dark:hover:bg-green-800 transition-colors"
            >
              复制
            </button>
          </div>

          <!-- 账户记录详情 -->
          <div v-if="entry.type === 'Open'" class="ml-4 p-3 rounded bg-gray-50 dark:bg-gray-800/70">
            <span class="text-sm font-medium dark:text-gray-300">打开账户: {{ entry.account }}</span>
          </div>
        </div>
      </div>

      <!-- 分页控件 -->
      <div v-if="!loading && pagination.total > 0" class="mt-6 flex flex-wrap justify-between items-center">
        <div class="text-sm text-gray-600 dark:text-gray-400">
          显示第 {{ ((filters.page || 1) - 1) * (filters.page_size || 20) + 1 }}-{{ Math.min((filters.page || 1) * (filters.page_size || 20), pagination.total) }} 条，共 {{ pagination.total }} 条记录
        </div>
        <div class="flex gap-2">
          <button 
            @click="goToPage(1)" 
            class="btn btn-secondary"
            :disabled="filters.page <= 1"
          >
            首页
          </button>
          <button 
            @click="goToPage(filters.page - 1)" 
            class="btn btn-secondary"
            :disabled="filters.page <= 1"
          >
            上一页
          </button>
          <button 
            @click="goToPage(filters.page + 1)" 
            class="btn btn-secondary"
            :disabled="filters.page >= pagination.pages"
          >
            下一页
          </button>
          <button 
            @click="goToPage(pagination.pages)" 
            class="btn btn-secondary"
            :disabled="filters.page >= pagination.pages"
          >
            末页
          </button>
        </div>
      </div>
    </div>
  </div>
    <!-- 新增/编辑记账记录的模态框 -->
    <div
      v-if="showAddModal || showEditModal"
      class="fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center overflow-y-auto"
    >
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-2xl my-8 transition-colors duration-200">
        <AddEntryModal
          @close="closeModal"
          @entry-added="handleEntryAdded"
          @entry-updated="handleEntryUpdated"
          @entry-deleted="handleEntryDeleted"
          :entry="editingEntry"
        />
      </div>
    </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import AddEntryModal from "~/components/AddEntryModal.vue";
import { useApi } from "~/composables/useApi";

const { getEntries } = useApi();

// 状态管理
const loading = ref(true);
const entries = ref([] as any[]);
const pagination = ref({
  total: 0,
  page: 1,
  page_size: 20,
  pages: 0
});
// 模态框状态
const showAddModal = ref(false);
const showEditModal = ref(false);
const editingEntry = ref<any>(null);

// 筛选条件
const filters = ref({
  start_date: '',
  end_date: '',
  page: 1,
  page_size: 20,
  sort: 'date',
  order: 'desc'
});

// 加载数据的函数
const loadEntries = async () => {
  try {
    loading.value = true;
    const response = await getEntries(filters.value);
    entries.value = response.entries || [];
    pagination.value = response.pagination || {
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    };
  } catch (error) {
    console.error("Error loading entries:", error);
  } finally {
    loading.value = false;
  }
};

// 应用筛选条件
const applyFilters = () => {
  filters.value.page = 1; // 重置到第一页
  loadEntries();
};

// 重置筛选条件
const resetFilters = () => {
  filters.value = {
    start_date: '',
    end_date: '',
    page: 1,
    page_size: 20,
    sort: 'date',
    order: 'desc'
  };
  loadEntries();
};

// 跳转到指定页
const goToPage = (page: number) => {
  if (page >= 1 && page <= pagination.value.pages) {
    filters.value.page = page;
    loadEntries();
  }
};

// 初始加载数据
onMounted(async () => {
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
    date: new Date().toISOString().split('T')[0] // 默认使用当前日期
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

// 组件卸载时移除事件监听
onUnmounted(() => {
  window.removeEventListener("sse:data-updated", handleSseUpdate);
  window.removeEventListener("refreshData", handleRefreshData);
});
</script>