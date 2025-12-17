<template>
  <div class="max-w-4xl mx-auto">
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- 左侧：账本信息和操作按钮 -->
      <div class="lg:col-span-1">
        <div class="card mb-6">
          <h2 class="text-xl font-semibold mb-4">账本信息</h2>
          <div v-if="loading" class="text-center py-4">
            <div
              class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
            ></div>
          </div>
          <div v-else class="space-y-3">
            <div class="flex justify-between items-center">
              <span class="text-gray-600">账本名称:</span>
              <span class="font-medium">{{ ledger.title }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-600">主要货币:</span>
              <span class="font-medium">{{ ledger.currency }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-600">记账条目:</span>
              <span class="font-medium">{{ ledger.entries_count }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-600">错误数量:</span>
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
        </div>
      </div>

      <!-- 右侧：最近记录 -->
      <div class="lg:col-span-2">
        <div class="card">
          <h2 class="text-xl font-semibold mb-4">最近记录</h2>
          <div v-if="loading" class="text-center py-4">
            <div
              class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
            ></div>
          </div>
          <div
            v-else-if="entries.length === 0"
            class="text-center py-8 text-gray-500"
          >
            <p>暂无记账记录</p>
          </div>
          <div v-else class="space-y-4">
            <div
              v-for="entry in entries.filter(e => e.type === 'Transaction').slice(0, 5)"
              :key="entry.meta.filename + entry.meta.lineno"
              class="border-b pb-3 last:border-0 hover:bg-gray-50 p-2 rounded transition-colors"
            >
              <div class="flex flex-col space-y-1">
                <!-- 日期和类型 -->
                <div class="flex justify-between items-center">
                  <span class="font-medium">{{ entry.date }}</span>
                  <span
                    class="text-sm px-2 py-0.5 rounded-full bg-gray-100 text-gray-700"
                    >{{ entry.type }}</span
                  >
                </div>

                <!-- 交易描述 -->
                <div
                  v-if="entry.type === 'Transaction' && entry.narration"
                  class="text-sm text-gray-700 ml-2"
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
                    class="inline-block bg-gray-100 text-gray-700 text-xs px-2 py-1 rounded"
                  >
                    #{{ tag }}
                  </span>
                </div>

                <!-- 操作按钮 -->
                <div class="ml-2 mt-1">
                  <button
                    @click="openEditModal({ ...entry, id: `${entry.meta?.filename}:${entry.meta?.lineno}` })"
                    class="text-xs px-2 py-0.5 bg-blue-100 text-blue-700 rounded hover:bg-blue-200 transition-colors"
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
                    <span class="text-gray-600 truncate max-w-[200px]">{{
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
                    class="text-xs text-gray-500"
                  >
                    +{{ entry.postings.length - 2 }} 更多记账行
                  </div>
                </div>

                <!-- 开户信息 -->
                <div
                  v-if="entry.type === 'Open'"
                  class="text-sm text-gray-600 ml-2"
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
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { useApi } from "~/composables/useApi";

const { getLedger, getEntries } = useApi();

const loading = ref(true);
const showAddModal = ref(false);
const showEditModal = ref(false);
const editingEntry = ref<any>(null);
const ledger = ref({
  title: "",
  currency: "",
  entries_count: 0,
  errors_count: 0,
});
const entries = ref([] as any[]);

// 刷新数据的函数
const refreshData = async () => {
  try {
    loading.value = true;
    ledger.value = await getLedger();
    const result = await getEntries();
    // 兼容新旧API格式
    entries.value = result.entries ? result.entries : result;
  } catch (error) {
    console.error("Error refreshing data:", error);
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
  await refreshData();

  window.addEventListener("sse:data-updated", refreshData);
});

// 组件卸载时移除事件监听
onUnmounted(() => {
  window.removeEventListener("sse:data-updated", refreshData);
});
</script>