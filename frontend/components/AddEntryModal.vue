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
                <!-- 交易类型说明 -->
                <div
                  class="mb-4 p-3 bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-800 rounded-lg">
                  <p class="text-sm text-blue-800 dark:text-blue-200">
                    {{ transactionType === 'expense' ? '支出：记录一笔支出交易' :
                      transactionType === 'income' ? '收入：记录一笔收入交易' :
                        '转账：记录一笔转账交易' }}
                  </p>
                  <p class="text-xs text-blue-600 dark:text-blue-300 mt-1">
                    提示：系统会自动处理借贷平衡：
                    <br>
                    • 请至少填写两个账户
                    <br>
                    • 金额之和应为零
                    <br>
                    • 可留空一个记账行金额，系统会自动计算差额
                  </p>
                </div>
              </div>

                <!-- 记账行列表 -->
                <div class="space-y-4">
                  <div v-for="(posting, index) in formData.postings" :key="index" class="space-y-2">
                    <div class="relative">
                      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        {{ index === 0 ? '账户 1' : index === 1 ? '账户 2' : `账户 ${index + 1}` }}
                      </label>
                      <!-- 使用自定义账户选择组件 -->
                      <AccountSelect v-model="posting.account" :accounts="accountsStore.list"
                        placeholder="选择或搜索账户..." />
                    </div>
                    <div class="grid grid-cols-2 gap-2">
                      <input type="number" v-model="posting.amount" :placeholder="index === 0 ? '金额（可留空）' : '金额'" step="0.01"
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
                      <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24"
                        stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                      </svg>
                      添加账户行
                    </button>
                  </div>
                </div>

              <!-- 编辑模式下根据记录类型显示不同的表单 -->
              <div v-if="isEditMode">
                <!-- Transaction类型记录 -->
                <div v-if="editingEntry?.type === 'Transaction'">
                  <div class="flex justify-between items-center mb-2">
                    <label class="block text-sm font-medium text-gray-700">记账行</label>
                    <button type="button" @click="addPosting" class="text-sm text-primary hover:text-primary/80">
                      + 添加行
                    </button>
                  </div>

                  <div v-for="(posting, index) in formData.postings" :key="index" class="space-y-2 mb-3">
                    <!-- 使用自定义账户选择组件 -->
                    <AccountSelect v-model="posting.account" :accounts="accountsStore.list" placeholder="选择或搜索账户..." />
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

                <!-- Open/Close类型记录 -->
                <div v-else-if="editingEntry?.type === 'Open' || editingEntry?.type === 'Close'">
                  <div class="space-y-4">
                    <div>
                      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">账户</label>
                      <AccountSelect v-model="formData.postings[0].account" :accounts="accountsStore.list"
                        placeholder="选择或搜索账户..." />
                    </div>

                    <div v-if="editingEntry?.type === 'Open'">
                      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">初始余额</label>
                      <input type="number" v-model="formData.postings[0].amount" placeholder="金额" step="0.01"
                        class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" />
                    </div>
                  </div>
                </div>

                <!-- Balance类型记录 -->
                <div v-else-if="editingEntry?.type === 'Balance'">
                  <div class="space-y-4">
                    <div>
                      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">账户</label>
                      <AccountSelect v-model="formData.postings[0].account" :accounts="accountsStore.list"
                        placeholder="选择或搜索账户..." />
                    </div>

                    <div>
                      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">余额</label>
                      <input type="number" v-model="formData.postings[0].amount" placeholder="金额" step="0.01"
                        class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        required />
                    </div>
                  </div>
                </div>

                <!-- 其他类型记录 -->
                <div v-else>
                  <div
                    class="p-4 bg-yellow-50 dark:bg-yellow-900/30 border border-yellow-200 dark:border-yellow-800 rounded-lg">
                    <p class="text-sm text-yellow-800 dark:text-yellow-200">
                      当前不支持编辑 {{ editingEntry?.type }} 类型的记录
                    </p>
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
import dayjs from "dayjs";

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
const editingEntry = ref<any>(null);

// 交易类型
const transactionType = ref('expense');

const formData = reactive({
  date: dayjs().format("YYYY-MM-DD"),
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
const loadEntryToForm = (entry: any) => {
  formData.date = entry.date;

  // 根据记录类型加载不同的字段
  if (entry.type === 'Transaction') {
    formData.narration = entry.narration;
    formData.tagsInput = entry.tags && entry.tags.length ? entry.tags.join(",") : "";
    // 解析金额数据，兼容不同的数据结构
    formData.postings = entry.postings.map((p: any) => {
      let amount = "";
      if (p.amount) {
        // 旧格式：金额包含货币单位
        amount = p.amount.replace(" CNY", "").trim();
      } else if (p.units) {
        // 新格式：金额和货币分开
        amount = p.units.number?.toString() || "";
      } else if (typeof p.amount === 'number') {
        // 直接是数字类型
        amount = p.amount.toString();
      }
      
      // 处理负数金额的显示
      if (amount.startsWith('-')) {
        amount = amount.substring(1); // 去掉负号，前端显示正数
      }
      
      return {
        account: p.account,
        amount: amount,
      };
    });
  } else {
    // 非Transaction类型，设置默认值
    formData.narration = entry.narration || "";
    formData.tagsInput = entry.tags && entry.tags.length ? entry.tags.join(",") : "";
    formData.postings = [];

    // 确保至少有一个记账行
    formData.postings.push({
      account: entry.account || "",
      amount: ""
    });
  }
};

// 重置表单
const resetForm = () => {
  formData.date = dayjs().format("YYYY-MM-DD");
  formData.narration = "";
  formData.tagsInput = "";
  formData.postings = [
    { account: "", amount: "" },
    { account: "", amount: "" }
  ];
  transactionType.value = 'expense';
  // 确保重置所有与编辑模式相关的变量
  isEditMode.value = false;
  editingEntryId.value = "";
  editingEntry.value = null;
};

const handleSubmit = async () => {
  try {
    submitting.value = true;

    // 处理标签
    const tags = formData.tagsInput
      .split(",")
      .map((tag) => tag.trim())
      .filter((tag) => tag);

    let entry: any;

    // 验证记账行数据
    const validPostings = formData.postings.filter(p => p.account && p.amount);
    if (validPostings.length < 2) {
      alert('根据Beancount记账规则，每笔交易至少需要两个记账行');
      submitting.value = false;
      return;
    }

    // 检查账户是否重复
    const accounts = validPostings.map(p => p.account);
    const uniqueAccounts = new Set(accounts);
    if (accounts.length !== uniqueAccounts.size) {
      alert('同一交易中不能使用相同的账户');
      submitting.value = false;
      return;
    }

    // 新增模式
    if (!isEditMode.value) {
      // 新增模式目前只支持Transaction类型
      entry = {
        date: formData.date,
        narration: formData.narration,
        tags: tags,
        postings: [] as any[],
      };

      // 计算所有记账行的总金额
      let total = 0;
      const postings = validPostings.map(p => {
        const amount = parseFloat(p.amount) || 0;
        total += amount;
        return {
          account: p.account,
          amount: `${amount} CNY`
        };
      });

      // 如果总金额不为零，自动平衡差额
      if (Math.abs(total) > 0.01) {
        // 查找第一个为0或空的记账行来调整差额
        let adjustmentIndex = -1;
        for (let i = 0; i < validPostings.length; i++) {
          const amount = parseFloat(validPostings[i].amount) || 0;
          if (Math.abs(amount) < 0.01) { // 金额为0或接近0
            adjustmentIndex = i;
            break;
          }
        }
        
        // 如果存在为0或空的记账行，自动调整差额
        if (adjustmentIndex !== -1) {
          // 调整差额到选定的记账行
          const adjustedAmount = -total; // 差额直接设为总金额的相反数
          postings[adjustmentIndex].amount = `${adjustedAmount} CNY`;
          
          console.log(`自动平衡借贷差额：在记账行${adjustmentIndex + 1}设置差额${adjustedAmount}`);
        } else {
          // 没有为0或空的记账行，提示用户
          alert('所有记账行都已填写金额，请确保金额之和为零（借贷平衡）');
          submitting.value = false;
          return;
        }
      }

      entry.postings = postings;
    }
    // 编辑模式
    else {
      // 根据记录类型构建不同的提交数据
      if (editingEntry.value?.type === 'Transaction') {
        // 验证借贷平衡
        const total = validPostings.reduce((sum, p) => sum + parseFloat(p.amount), 0);
        if (Math.abs(total) > 0.01) { // 允许0.01的误差
          alert('记账行金额之和必须为零（借贷平衡）');
          submitting.value = false;
          return;
        }

        entry = {
          date: formData.date,
          narration: formData.narration,
          tags: tags,
          postings: validPostings.map((p) => ({
            account: p.account,
            amount: `${p.amount} CNY`, // 使用默认货币
          })),
        };
      } else if (editingEntry.value?.type === 'Open') {
        entry = {
          date: formData.date,
          account: formData.postings[0]?.account,
          tags: tags,
        };
        // 添加初始余额（如果有）
        const amount = parseFloat(formData.postings[0]?.amount);
        if (!isNaN(amount)) {
          entry.postings = [{
            account: formData.postings[0]?.account,
            amount: `${amount} CNY`
          }];
        }
      } else if (editingEntry.value?.type === 'Close') {
        entry = {
          date: formData.date,
          account: formData.postings[0]?.account,
          tags: tags,
        };
      } else if (editingEntry.value?.type === 'Balance') {
        entry = {
          date: formData.date,
          account: formData.postings[0]?.account,
          balance: `${parseFloat(formData.postings[0]?.amount)} CNY`,
          tags: tags,
        };
      } else {
        // 其他类型暂时不支持编辑
        console.error("Unsupported entry type for editing:", editingEntry.value?.type);
        return;
      }
    }

    // 确保只有在明确的编辑模式下才调用updateEntry
    if (isEditMode.value && editingEntryId.value && props.entry) {
      console.log('编辑模式：更新记录', editingEntryId.value);
      await updateEntry(editingEntryId.value, entry);
      emit("entryUpdated");
    } else {
      console.log('添加模式：创建新记录');
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

// 根据记账行数据自动判断交易类型（简化版，主要用于显示分类）
const determineTransactionType = (entry: any) => {
  if (!entry.postings || entry.postings.length < 2) {
    transactionType.value = 'expense';
    return;
  }

  // 简化判断逻辑，仅根据金额正负进行基本分类
  const firstAmount = parseFloat(entry.postings[0]?.amount?.replace(' CNY', '') || '0');
  const secondAmount = parseFloat(entry.postings[1]?.amount?.replace(' CNY', '') || '0');

  if (firstAmount < 0 && secondAmount > 0) {
    // 第一行负，第二行正：支出
    transactionType.value = 'expense';
  } else if (firstAmount > 0 && secondAmount < 0) {
    // 第一行正，第二行负：收入
    transactionType.value = 'income';
  } else {
    // 其他情况：转账
    transactionType.value = 'transfer';
  }
};

// 监听entry属性变化
watch(
  () => props.entry,
  (newValue) => {
    if (newValue && newValue.id && newValue.id.trim() !== '') {
      // 只有当entry包含有效的id时才进入编辑模式
      isEditMode.value = true;
      editingEntryId.value = newValue.id;
      editingEntry.value = newValue;
      loadEntryToForm(newValue);
    } else if (newValue && Object.keys(newValue).length > 0) {
      // 有数据但无ID，这是复制交易的情况
      isEditMode.value = false;
      editingEntryId.value = "";
      editingEntry.value = null;
      // 加载数据到表单但不重置
      loadEntryToForm(newValue);
      // 复制交易时，根据记账行自动判断交易类型
      determineTransactionType(newValue);
    } else {
      // 否则进入添加模式
      isEditMode.value = false;
      editingEntryId.value = "";
      editingEntry.value = null;
      resetForm();
    }
  },
  { immediate: true }
);
</script>