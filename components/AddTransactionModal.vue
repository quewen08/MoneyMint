<template>
  <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
    <div class="modal-content">
      <div class="modal-header">
        <h2 class="text-xl font-bold">添加新交易</h2>
        <button @click="closeModal" class="modal-close">&times;</button>
      </div>
      <div class="modal-body">
        <form @submit.prevent="addTransaction">
          <div class="grid md:grid-cols-2 gap-4">
            <div class="form-group">
              <label for="date">日期</label>
              <input
                type="date"
                id="date"
                v-model="newTransaction.date"
                required
              />
            </div>
            <div class="form-group">
              <label for="type">类型</label>
              <select
                id="type"
                v-model="newTransaction.type"
                required
                @change="onTransactionTypeChange"
              >
                <option value="income">收入</option>
                <option value="expense">支出</option>
                <option value="transfer">转账</option>
              </select>
            </div>

            <!-- 多账户交易行 -->
            <div class="form-group md:col-span-2">
              <label>交易明细</label>
              <div
                v-for="(posting, index) in newTransaction.postings"
                :key="index"
                class="flex gap-2 mb-2"
              >
                <div class="flex-1 relative">
                  <!-- 自定义select组件 -->
                  <div class="custom-select-container">
                    <!-- 输入框 -->
                    <input
                      type="text"
                      v-model="postingSearch[index]"
                      placeholder="选择或搜索账户..."
                      class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm cursor-pointer"
                      @click="toggleSelect(index, true)"
                      @input="onSearchInput(index)"
                    />

                    <!-- 下拉列表 -->
                    <div
                      v-if="isSelectOpen[index]"
                      class="custom-select-dropdown absolute top-full left-0 right-0 mt-1 bg-white border border-gray-300 rounded-md shadow-lg z-50 max-h-60 overflow-y-auto"
                    >
                      <!-- 按账户类型分组 -->
                      <div
                        v-for="(groupAccounts, groupName) in groupedAccounts(
                          index
                        )"
                        :key="groupName"
                        class="group"
                      >
                        <div
                          class="group-label px-3 py-1 text-sm font-medium bg-gray-50 text-gray-600 border-b"
                        >
                          {{ groupName }}
                        </div>
                        <div
                          v-for="account in groupAccounts"
                          :key="account.actualName"
                          class="option px-3 py-2 text-sm cursor-pointer hover:bg-blue-50 hover:text-blue-600"
                          @click="selectAccount(index, account.nickname)"
                        >
                          {{ account.nickname }}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
                <input
                  type="number"
                  v-model.number="posting.amount"
                  step="0.01"
                  class="flex-1"
                  required
                  placeholder="金额"
                />
                <button
                  type="button"
                  @click="removePosting(index)"
                  class="btn btn-danger"
                  v-if="newTransaction.postings.length > 2"
                >
                  删除
                </button>
              </div>
              <button
                type="button"
                @click="addPosting"
                class="btn btn-secondary"
              >
                添加交易行
              </button>
            </div>

            <div class="form-group md:col-span-2">
              <label for="narration">描述</label>
              <input
                type="text"
                id="narration"
                v-model="newTransaction.narration"
                required
              />
            </div>

            <div class="form-group md:col-span-2">
              <button type="submit" class="btn btn-primary w-full">
                保存交易
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from "vue";
import { useAsyncData } from "#app";

// 定义props
const props = defineProps({
  showModal: {
    type: Boolean,
    default: false,
  },
});

// 定义事件
const emit = defineEmits(["close", "transaction-added"]);

// 获取所有账户列表
const { data: accountsData, refresh: refreshAccounts } = await useAsyncData(
  "allAccounts",
  async () => {
    const response = await $fetch("/api/accounts");
    return response.accounts || [];
  }
);

const accounts = computed(() => accountsData.value || []);

// 新交易表单
const newTransaction = ref({
  date: new Date().toISOString().split("T")[0],
  type: "expense",
  narration: "",
  postings: [
    { account: "", amount: 0 },
    { account: "", amount: 0 },
  ],
});

// 账户搜索
const postingSearch = ref(["", ""]);

// 跟踪select是否展开
const isSelectOpen = ref([false, false]);

// 关闭弹窗
function closeModal() {
  emit("close");
}

// 添加交易行
function addPosting() {
  newTransaction.value.postings.push({ account: "", amount: 0 });
  postingSearch.value.push("");
  isSelectOpen.value.push(false);
}

// 删除交易行
function removePosting(index) {
  if (newTransaction.value.postings.length > 2) {
    newTransaction.value.postings.splice(index, 1);
    postingSearch.value.splice(index, 1);
    isSelectOpen.value.splice(index, 1);
  }
}

// 过滤账户
function filteredAccounts(index) {
  const searchTerm = postingSearch.value[index].toLowerCase().trim();
  return accounts.value.filter(
    (account) =>
      account.nickname.toLowerCase().includes(searchTerm) ||
      account.actualName.toLowerCase().includes(searchTerm)
  );
}

// 切换select展开状态
function toggleSelect(index, isOpen) {
  // 切换当前select的状态
  isSelectOpen.value[index] = isOpen;
}

// 选择账户
function selectAccount(index, accountName) {
  // 设置选中的账户
  newTransaction.value.postings[index].account = accountName;
  // 更新搜索框显示
  postingSearch.value[index] = accountName;
  // 关闭下拉列表
  isSelectOpen.value[index] = false;
  // 更新金额
  updateAmounts();
}

// 搜索输入处理
function onSearchInput(index) {
  // 确保下拉列表在输入时保持打开
  isSelectOpen.value[index] = true;
}

// 按账户类型分组
function groupedAccounts(index) {
  const filtered = filteredAccounts(index);
  const grouped = {};

  // 定义账户类型映射
  const accountTypes = {
    "Assets:": "资产",
    "Liabilities:": "负债",
    "Income:": "收入",
    "Expenses:": "支出",
    "Equity:": "权益",
  };

  // 按类型分组账户
  filtered.forEach((account) => {
    let type = "其他";

    // 确定账户类型
    for (const [prefix, name] of Object.entries(accountTypes)) {
      if (account.actualName.startsWith(prefix)) {
        type = name;
        break;
      }
    }

    if (!grouped[type]) {
      grouped[type] = [];
    }
    grouped[type].push(account);
  });

  return grouped;
}

// 交易类型变化处理
function onTransactionTypeChange() {
  setDefaultAccounts();
}

// 账户变化处理
function onAccountChange(index) {
  // 确保金额总和为0
  updateAmounts();
}

// 设置默认账户
function setDefaultAccounts() {
  const { type } = newTransaction.value;

  if (type === "income") {
    // 收入时，默认收入账户和资产账户
    const firstIncomeAccount = accounts.value.find((acc) =>
      acc.actualName.startsWith("Income:")
    );
    const firstAssetAccount = accounts.value.find((acc) =>
      acc.actualName.startsWith("Assets:")
    );
    if (firstIncomeAccount)
      newTransaction.value.postings[0].account = firstIncomeAccount.nickname;
    if (firstAssetAccount)
      newTransaction.value.postings[1].account = firstAssetAccount.nickname;
    // 收入时，收入账户金额为负数，资产账户金额为正数
    newTransaction.value.postings[0].amount = 0;
    newTransaction.value.postings[1].amount = 0;
  } else if (type === "expense") {
    // 支出时，默认资产账户和支出账户
    const firstAssetAccount = accounts.value.find((acc) =>
      acc.actualName.startsWith("Assets:")
    );
    const firstExpenseAccount = accounts.value.find((acc) =>
      acc.actualName.startsWith("Expenses:")
    );
    if (firstAssetAccount)
      newTransaction.value.postings[0].account = firstAssetAccount.nickname;
    if (firstExpenseAccount)
      newTransaction.value.postings[1].account = firstExpenseAccount.nickname;
    // 支出时，资产账户金额为负数，支出账户金额为正数
    newTransaction.value.postings[0].amount = 0;
    newTransaction.value.postings[1].amount = 0;
  } else if (type === "transfer") {
    // 转账时，默认两个资产账户
    const assetAccounts = accounts.value.filter((acc) =>
      acc.actualName.startsWith("Assets:")
    );
    if (assetAccounts.length > 0)
      newTransaction.value.postings[0].account = assetAccounts[0].nickname;
    if (assetAccounts.length > 1)
      newTransaction.value.postings[1].account = assetAccounts[1].nickname;
    // 转账时，转出账户金额为负数，转入账户金额为正数
    newTransaction.value.postings[0].amount = 0;
    newTransaction.value.postings[1].amount = 0;
  }
}

// 更新金额
function updateAmounts() {
  // 确保金额总和为0
  const sum = newTransaction.value.postings.reduce(
    (acc, curr) => acc + (curr.amount || 0),
    0
  );
  if (sum !== 0 && newTransaction.value.postings.length >= 2) {
    // 如果最后一个金额为空或为0，则设置为-sum
    const lastPosting =
      newTransaction.value.postings[newTransaction.value.postings.length - 1];
    if (!lastPosting.amount) {
      lastPosting.amount = -sum;
    }
  }
}

async function addTransaction() {
  try {
    const response = await $fetch("/api/transactions", {
      method: "POST",
      body: newTransaction.value,
    });

    if (response.success) {
      // 关闭弹窗
      closeModal();

      // 重置表单
      resetForm();

      // 触发交易添加事件
      emit("transaction-added");

      // 显示成功提示
      alert("交易记录添加成功！");
    } else {
      // 显示错误信息
      alert(`添加交易失败: ${response.error || "未知错误"}`);
    }
  } catch (error) {
    console.error("添加交易失败:", error);
    alert("添加交易失败，请稍后重试。");
  }
}

// 重置表单
function resetForm() {
  newTransaction.value = {
    date: new Date().toISOString().split("T")[0],
    type: "expense",
    narration: "",
    postings: [
      { account: "", amount: 0 },
      { account: "", amount: 0 },
    ],
  };
  postingSearch.value = ["", ""];
  isSelectOpen.value = [false, false];
  setDefaultAccounts();
}

// 监听弹窗显示状态
watch(
  () => props.showModal,
  (newVal) => {
    if (newVal) {
      // 弹窗打开时设置默认账户
      setDefaultAccounts();
    }
  }
);

// 点击外部区域关闭下拉列表
onMounted(() => {
  if (props.showModal) {
    setDefaultAccounts();
  }

  // 点击外部区域关闭下拉列表
  const handleClickOutside = (event) => {
    const customSelects = document.querySelectorAll(".custom-select-container");
    let clickedInside = false;

    customSelects.forEach((container, index) => {
      if (container.contains(event.target)) {
        clickedInside = true;
      } else {
        isSelectOpen.value[index] = false;
      }
    });
  };

  document.addEventListener("click", handleClickOutside);

  // 清理事件监听器
  onBeforeUnmount(() => {
    document.removeEventListener("click", handleClickOutside);
  });
});
</script>

<style scoped>
/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background-color: white;
  border-radius: 8px;
  width: 90%;
  max-width: 800px;
  max-height: 90vh;
  overflow-y: auto;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  background-color: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
}

.modal-header h2 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.modal-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #64748b;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: background-color 0.2s;
}

.modal-close:hover {
  background-color: #e2e8f0;
}

.modal-body {
  padding: 1.5rem;
}

/* Form Styles */
.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: #334155;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 0.75rem;
  border: 2px solid #e2e8f0;
  border-radius: 6px;
  font-size: 1rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-group input:focus,
.form-group select:focus,
.custom-select-container input:hover {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

/* Core Layout Styles */
.grid {
  display: grid;
  gap: 1rem;
}

.flex {
  display: flex;
}

.gap-2 {
  gap: 0.5rem;
}

.gap-4 {
  gap: 1rem;
}

.mb-2 {
  margin-bottom: 0.5rem;
}

.flex-1 {
  flex: 1;
}

.relative {
  position: relative;
}

.w-full {
  width: 100%;
}

.px-3 {
  padding-left: 0.75rem;
  padding-right: 0.75rem;
}

.py-2 {
  padding-top: 0.5rem;
  padding-bottom: 0.5rem;
}

.border {
  border: 1px solid #e5e7eb;
}

.border-gray-300 {
  border-color: #d1d5db;
}

.rounded-md {
  border-radius: 0.375rem;
}

.text-sm {
  font-size: 0.875rem;
}

.cursor-pointer {
  cursor: pointer;
}

.absolute {
  position: absolute;
}

.top-full {
  top: 100%;
}

.left-0 {
  left: 0;
}

.right-0 {
  right: 0;
}

.mt-1 {
  margin-top: 0.25rem;
}

.bg-white {
  background-color: white;
}

.shadow-lg {
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1),
    0 4px 6px -2px rgba(0, 0, 0, 0.05);
}

.z-50 {
  z-index: 50;
}

.max-h-60 {
  max-height: 15rem;
}

.overflow-y-auto {
  overflow-y: auto;
}

/* Button Styles */
.btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: background-color 0.2s;
}

.btn-primary {
  background-color: #3b82f6;
  color: white;
}

.btn-primary:hover {
  background-color: #2563eb;
}

.btn-secondary {
  background-color: #64748b;
  color: white;
}

.btn-secondary:hover {
  background-color: #475569;
}

.btn-danger {
  background-color: #ef4444;
  color: white;
}

.btn-danger:hover {
  background-color: #dc2626;
}

/* Custom Select Styles */
.custom-select-container {
  position: relative;
}

.custom-select-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  margin-top: 1px;
  background-color: white;
  border: 2px solid #e2e8f0;
  border-radius: 6px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  z-index: 50;
  max-height: 300px;
  overflow-y: auto;
}

.group {
  margin-bottom: 0.5rem;
}

.group-label {
  padding: 0.25rem 0.75rem;
  font-size: 0.875rem;
  font-weight: 500;
  background-color: #f8fafc;
  color: #64748b;
  border-bottom: 1px solid #e2e8f0;
}

.option {
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;
}

.option:hover {
  background-color: #ebf8ff;
  color: #2563eb;
}

/* Additional Styles for Template */
.text-xl {
  font-size: 1.25rem;
}

.font-bold {
  font-weight: bold;
}

/* Hover Effects */
.option:hover {
  background-color: #ebf8ff;
  color: #2563eb;
}

/* Responsive Design */
@media (max-width: 640px) {
  .modal-content {
    width: 95%;
  }

  .grid {
    grid-template-columns: 1fr;
  }
}

/* Medium Screen Styles */
@media (min-width: 768px) {
  .md\:grid-cols-2 {
    grid-template-columns: repeat(2, 1fr);
  }

  .md\:col-span-2 {
    grid-column: span 2 / span 2;
  }
}
</style>
