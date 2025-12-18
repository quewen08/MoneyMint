<template>
  <div class="p-6">
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-xl font-bold">
        {{ isEditMode ? "编辑记账记录" : "添加记账记录" }}
      </h2>
      <div class="flex gap-2">
        <button
          v-if="isEditMode"
          @click="handleDelete"
          class="bg-red-100 text-red-600 px-3 py-1 rounded-lg hover:bg-red-200 flex items-center"
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
              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
            />
          </svg>
          删除
        </button>
        <button
          @click="emit('close')"
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
    </div>

    <div class="card">
      <form @submit.prevent="handleSubmit">
        <div class="space-y-4">
          <!-- Date -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1"
              >日期</label
            >
            <input
              type="date"
              v-model="formData.date"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
              required
            />
          </div>

          <!-- Narration -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1"
              >描述</label
            >
            <input
              type="text"
              v-model="formData.narration"
              placeholder="例如：午餐"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
              required
            />
          </div>

          <!-- Tags -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1"
              >标签</label
            >
            <input
              type="text"
              v-model="formData.tagsInput"
              placeholder="例如：food,restaurant"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
            />
          </div>

          <!-- Postings -->
          <div>
            <div class="flex justify-between items-center mb-2">
              <label class="block text-sm font-medium text-gray-700"
                >记账行</label
              >
              <button
                type="button"
                @click="addPosting"
                class="text-sm text-primary hover:text-primary/80"
              >
                + 添加行
              </button>
            </div>

            <div
              v-for="(posting, index) in formData.postings"
              :key="index"
              class="space-y-2 mb-3"
            >
              <!-- 账户搜索框 -->
              <input
                type="text"
                v-model="posting.accountSearch"
                placeholder="搜索账户..."
                class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
              />
              <div class="grid grid-cols-3 gap-2">
                <select
                  v-model="posting.account"
                  class="px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                  required
                >
                  <option value="">选择账户</option>
                  <option
                    v-for="account in filteredAccounts(posting.accountSearch)"
                    :key="account"
                    :value="account"
                  >
                    {{ account }}
                  </option>
                </select>
                <input
                  type="number"
                  v-model="posting.amount"
                  placeholder="金额"
                  step="0.01"
                  class="px-4 py-2 border border-gray-300 rounded-lg focus:ring-primary focus:border-primary"
                  required
                />
                <button
                  type="button"
                  @click="removePosting(index)"
                  class="bg-red-100 text-red-600 px-4 py-2 rounded-lg hover:bg-red-200"
                >
                  删除
                </button>
              </div>
            </div>
          </div>

          <!-- Submit Button -->
          <div class="pt-4">
            <button
              type="submit"
              class="btn btn-primary w-full"
              :disabled="submitting"
            >
              <span
                v-if="submitting"
                class="inline-block animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"
              ></span>
              {{ isEditMode ? "更新记录" : "保存记录" }}
            </button>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from "vue";
import { useApi } from "~/composables/useApi";
import { useAccountsStore } from "~/stores/accounts";

interface EntryWithId {
  id: string;
  date: string;
  narration: string;
  tags: string[];
  postings: { account: string; amount: string }[];
}

const { addEntry, updateEntry, deleteEntry, accounts } = useApi();
const accountsStore = useAccountsStore();

// 定义属性
const props = defineProps<{
  entry?: EntryWithId;
}>();

// 定义事件
const emit = defineEmits([
  "close",
  "entryAdded",
  "entryUpdated",
  "entryDeleted",
]);

const submitting = ref(false);
const isEditMode = ref(false);
const editingEntryId = ref<string>("");

const formData = reactive({
  date: new Date().toISOString().split("T")[0],
  narration: "",
  tagsInput: "",
  postings: [
    { account: "", amount: "", accountSearch: "" },
    { account: "", amount: "", accountSearch: "" },
  ],
});

// 加载账户列表
const loadAccounts = async () => {
  try {
    // 添加调试信息，显示账户列表来源
    console.time("loadAccounts");
    await accountsStore.fetchAccounts();
    console.timeEnd("loadAccounts");
    console.log("账户列表加载完成，共", accountsStore.list.length, "个账户");
  } catch (error) {
    console.error("Error loading accounts:", error);
    // 添加简单的错误提示
    alert("加载账户列表失败，请刷新页面重试");
  }
};

// 组件挂载时加载账户列表并设置SSE事件监听
onMounted(async () => {
  // 监听SSE事件，当账户列表更新时自动刷新
  const handleSseUpdate = (event: Event) => {
    const customEvent = event as CustomEvent;
    accountsStore.handleSseUpdate(customEvent);
  };

  // 组件卸载时移除事件监听
  onUnmounted(() => {
    window.removeEventListener("sse:data-updated", handleSseUpdate);
  });

  window.addEventListener("sse:data-updated", handleSseUpdate);

  // 加载账户列表（这是async操作，必须放在生命周期钩子注册之后）
  await loadAccounts();
});

// 将entry数据加载到表单
const loadEntryToForm = (entry: EntryWithId) => {
  formData.date = entry.date;
  formData.narration = entry.narration;
  formData.tagsInput =
    entry.tags && entry.tags.length ? entry.tags.join(",") : "";
  // 解析金额数据，兼容不同的数据结构
  formData.postings = entry.postings.map((p: any) => {
    let amount = "";
    if (p.amount) {
      // 旧格式：金额包含货币单位
      amount = p.amount.replace(" CNY", "").trim();
    } else if (p.units) {
      // 新格式：金额和货币分开
      amount = p.units.number?.toString() || "";
    }
    return {
      account: p.account,
      amount: amount,
      accountSearch: "", // 初始化搜索字段
    };
  });
};

// 重置表单
const resetForm = () => {
  formData.date = new Date().toISOString().split("T")[0];
  formData.narration = "";
  formData.tagsInput = "";
  formData.postings = [
    { account: "", amount: "" },
    { account: "", amount: "" },
  ];
};

const handleSubmit = async () => {
  try {
    submitting.value = true;

    // 处理标签
    const tags = formData.tagsInput
      .split(",")
      .map((tag) => tag.trim())
      .filter((tag) => tag);

    // 准备提交数据
    const entry = {
      date: formData.date,
      narration: formData.narration,
      tags: tags,
      postings: formData.postings.map((p) => ({
        account: p.account,
        amount: `${p.amount} CNY`, // 使用默认货币
      })),
    };

    if (isEditMode.value && editingEntryId.value) {
      await updateEntry(editingEntryId.value, entry);
      emit("entryUpdated");
    } else {
      await addEntry(entry);
      emit("entryAdded");
    }

    // 关闭弹窗
    emit("close");

    // 重置表单
    resetForm();
  } catch (error) {
    console.error("Error saving entry:", error);
  } finally {
    submitting.value = false;
  }
};

// 添加记账行
const addPosting = () => {
  formData.postings.push({ account: "", amount: "", accountSearch: "" });
};

// 筛选账户列表
const filteredAccounts = (searchTerm: string) => {
  if (!searchTerm.trim()) {
    return accountsStore.list;
  }
  return accountsStore.list.filter(account => 
    account.toLowerCase().includes(searchTerm.toLowerCase())
  );
};

const removePosting = (index: number) => {
  if (formData.postings.length > 2) {
    formData.postings.splice(index, 1);
  }
};

const handleDelete = async () => {
  if (!isEditMode.value || !editingEntryId.value) return;

  if (confirm("确定要删除这条记账记录吗？此操作不可恢复。")) {
    try {
      await deleteEntry(editingEntryId.value);
      emit("entryDeleted");
      emit("close");
    } catch (error) {
      console.error("Error deleting entry:", error);
    }
  }
};

// 监听entry属性变化
watch(
  () => props.entry,
  (newValue) => {
    if (newValue) {
      isEditMode.value = true;
      editingEntryId.value = newValue.id;
      loadEntryToForm(newValue);
    } else {
      isEditMode.value = false;
      editingEntryId.value = "";
      resetForm();
    }
  },
  { immediate: true }
);
</script>