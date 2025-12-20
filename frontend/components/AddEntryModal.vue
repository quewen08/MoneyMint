<template>
  <div class="fixed inset-0 bg-black bg-opacity-50 z-50 flex">
    <div
      class="bg-white dark:bg-gray-800 w-full max-w-2xl h-full overflow-y-auto transition-transform duration-300 ease-in-out transform translate-x-0">
      <div class="p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-xl font-bold dark:text-white">
            {{ isEditMode ? "编辑记账记录" : "添加记账记录" }}
          </h2>
          <div class="flex gap-2">
            <button v-if="isEditMode" @click="handleDelete"
              class="bg-red-100 dark:bg-red-900/50 text-red-600 dark:text-red-300 px-3 py-1 rounded-lg hover:bg-red-200 dark:hover:bg-red-800 flex items-center">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 mr-1" fill="none" viewBox="0 0 24 24"
                stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
              删除
            </button>
            <button @click="emit('close')"
              class="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24"
                stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        <!-- 交易类型选择 -->
        <div v-if="!isEditMode" class="mb-4">
          <div class="border-b border-gray-200 dark:border-gray-700">
            <nav class="flex space-x-8">
              <button @click="transactionType = 'expense'" :class="[
                'py-3 px-1 border-b-2 font-medium text-sm',
                transactionType === 'expense' ? 'border-primary text-primary' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'
              ]">
                支出
              </button>
              <button @click="transactionType = 'income'" :class="[
                'py-3 px-1 border-b-2 font-medium text-sm',
                transactionType === 'income' ? 'border-primary text-primary' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'
              ]">
                收入
              </button>
              <button @click="transactionType = 'transfer'" :class="[
                'py-3 px-1 border-b-2 font-medium text-sm',
                transactionType === 'transfer' ? 'border-primary text-primary' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'
              ]">
                转账
              </button>
            </nav>
          </div>
        </div>

        <div class="card">
          <form @submit.prevent="handleSubmit">
            <div class="space-y-4">
              <!-- Date -->
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">日期</label>
                <input type="date" v-model="formData.date"
                  class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                  required />
              </div>

              <!-- Narration -->
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">描述</label>
                <input type="text" v-model="formData.narration" placeholder="例如：午餐"
                  class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                  required />
              </div>

              <!-- Tags -->
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  标签
                </label>
                <input type="text" v-model="formData.tagsInput" placeholder="例如：food,restaurant"
                  class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" />
              </div>

              <!-- 交易类型表单 -->
              <div v-if="!isEditMode">
                <!-- 交易类型说明 -->
                <div class="mb-4 p-3 bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-800 rounded-lg">
                  <p class="text-sm text-blue-800 dark:text-blue-200">
                    {{ transactionType === 'expense' ? '支出：从资产账户扣除金额到支出账户' : 
                       transactionType === 'income' ? '收入：从收入账户增加金额到资产账户' : 
                       '转账：从一个账户转移金额到另一个账户' }}
                  </p>
                </div>
                
                <!-- 记账行列表 -->
                <div class="space-y-4">
                  <div v-for="(posting, index) in formData.postings" :key="index" class="space-y-2">
                    <div class="relative">
                      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        {{ index === 0 ? 
                           (transactionType === 'expense' ? '支出账户' : 
                            transactionType === 'income' ? '收入账户' : '转出账户') : 
                           (index === 1 ? 
                            (transactionType === 'expense' ? '支出类别' : 
                             transactionType === 'income' ? '收入类别' : '转入账户') : 
                            `账户 ${index + 1}`) }}
                      </label>
                      <!-- 使用自定义账户选择组件 -->
                      <AccountSelect
                        v-model="posting.account"
                        :accounts="accountsStore.list"
                        :account-type="index === 1 ? (transactionType === 'expense' ? 'expenses' : transactionType === 'income' ? 'income' : '') : ''"
                        placeholder="选择或搜索账户..."
                      />
                    </div>
                    <div class="grid grid-cols-2 gap-2">
                      <input type="number" v-model="posting.amount" placeholder="金额" step="0.01"
                        class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        required />
                      <button type="button" @click="removePosting(index)" 
                        class="bg-red-100 dark:bg-red-900/50 text-red-600 dark:text-red-300 px-4 py-2 rounded-lg hover:bg-red-200 dark:hover:bg-red-800"
                        :disabled="formData.postings.length <= 2">
                        删除
                      </button>
                    </div>
                  </div>
                  
                  <!-- 添加记账行按钮 -->
                  <div class="flex justify-center mt-2">
                    <button type="button" @click="addPosting" 
                      class="text-sm text-primary hover:text-primary/80 flex items-center">
                      <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                      </svg>
                      添加账户行
                    </button>
                  </div>
                </div>
              </div>

              <!-- 通用记账行（编辑模式或自定义模式） -->
              <div v-if="isEditMode">
                <div class="flex justify-between items-center mb-2">
                  <label class="block text-sm font-medium text-gray-700">记账行</label>
                  <button type="button" @click="addPosting" class="text-sm text-primary hover:text-primary/80">
                    + 添加行
                  </button>
                </div>

                <div v-for="(posting, index) in formData.postings" :key="index" class="space-y-2 mb-3">
                  <!-- 使用自定义账户选择组件 -->
                  <AccountSelect
                    v-model="posting.account"
                    :accounts="accountsStore.list"
                    placeholder="选择或搜索账户..."
                  />
                  <div class="grid grid-cols-2 gap-2">
                    <input type="number" v-model="posting.amount" placeholder="金额" step="0.01"
                      class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                      required />
                    <button type="button" @click="removePosting(index)"
                      class="bg-red-100 dark:bg-red-900/50 text-red-600 dark:text-red-300 px-4 py-2 rounded-lg hover:bg-red-200 dark:hover:bg-red-800">
                      删除
                    </button>
                  </div>
                </div>
              </div>

              <!-- Submit Button -->
              <div class="pt-4">
                <button type="submit" class="btn btn-primary w-full" :disabled="submitting">
                  <span v-if="submitting"
                    class="inline-block animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></span>
                  {{ isEditMode ? "更新记录" : "保存记录" }}
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
import { ref, reactive, onMounted, onUnmounted, watch } from "vue";
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

// 交易类型
const transactionType = ref('expense');

const formData = reactive({
  date: new Date().toISOString().split("T")[0],
  narration: "",
  tagsInput: "",
  postings: [
    { account: "", amount: "" },
    { account: "", amount: "" },
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
    { account: "", amount: "" }
  ];
  transactionType.value = 'expense';
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
      postings: [] as any[],
    };

    // 根据交易类型处理金额符号
    if (!isEditMode.value) {
      const amount = parseFloat(formData.postings[0].amount) || 0;

      if (transactionType.value === 'expense') {
        // 支出：资产账户减少，支出账户增加
        entry.postings = [
          { account: formData.postings[0].account, amount: `${-amount} CNY` },
          { account: formData.postings[1].account, amount: `${amount} CNY` }
        ];
      } else if (transactionType.value === 'income') {
        // 收入：资产账户增加，收入账户增加
        entry.postings = [
          { account: formData.postings[0].account, amount: `${amount} CNY` },
          { account: formData.postings[1].account, amount: `${-amount} CNY` }
        ];
      } else if (transactionType.value === 'transfer') {
        // 转账：转出账户减少，转入账户增加
        entry.postings = [
          { account: formData.postings[0].account, amount: `${-amount} CNY` },
          { account: formData.postings[1].account, amount: `${amount} CNY` }
        ];
      }
    } else {
      // 编辑模式：保持原有的金额结构
      entry.postings = formData.postings.map((p) => ({
        account: p.account,
        amount: `${p.amount} CNY`, // 使用默认货币
      }));
    }

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
  formData.postings.push({ account: "", amount: "" });
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