<template>
  <div class="relative account-select-component">
    <!-- 主输入框 -->
    <input
      type="text"
      v-model="displayValue"
      :placeholder="placeholder"
      @focus="showSelector = true"
      @input="handleInput"
      @keydown.down.prevent="navigateDown"
      @keydown.up.prevent="navigateUp"
      @keydown.enter.prevent="selectCurrent"
      @keydown.esc="showSelector = false"
      class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 pr-10"
      ref="inputRef"
    />
    <!-- 下拉箭头 -->
    <div class="absolute inset-y-0 right-0 flex items-center pr-3 pointer-events-none">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </div>

    <!-- 选择器弹窗 -->
    <div
      v-if="showSelector"
      class="absolute z-50 mt-1 w-full max-h-60 overflow-auto bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg"
      @mousedown="handleSelectorMousedown"
    >
      <!-- 搜索过滤框 -->
      <div class="sticky top-0 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 p-2">
        <input
          type="text"
          v-model="searchQuery"
          placeholder="搜索账户..."
          class="w-full px-3 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:ring-primary focus:border-primary bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
          ref="searchInputRef"
        />
      </div>

      <!-- 账户列表 -->
      <div class="py-1">
        <div
          v-if="filteredAccounts.length === 0"
          class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400"
        >
          没有找到匹配的账户
        </div>
        <div
          v-else
          v-for="(account, index) in filteredAccounts"
          :key="account"
          @mousedown.prevent="selectAccount(account)"
          class="px-4 py-2 text-sm cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700 focus:outline-none focus:bg-gray-100 dark:focus:bg-gray-700 transition-colors"
          :class="{ 'bg-primary/10 text-primary dark:bg-primary/20': index === selectedIndex }"
        >
          {{ account }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue';

// 定义属性
const props = defineProps<{
  modelValue: string;
  accounts: string[];
  placeholder?: string;
  accountType?: 'expenses' | 'income' | 'assets' | '';
}>();

// 定义事件
const emit = defineEmits<{
  'update:modelValue': [value: string];
}>();

// 输入框引用
const inputRef = ref<HTMLInputElement | null>(null);
const searchInputRef = ref<HTMLInputElement | null>(null);

// 内部状态
const showSelector = ref(false);
const searchQuery = ref('');
const selectedIndex = ref(-1);
const displayValue = ref(props.modelValue);

// 监听外部值变化
watch(() => props.modelValue, (newValue) => {
  displayValue.value = newValue;
});

// 按账户类型筛选后的账户列表
const typeFilteredAccounts = computed(() => {
  if (!props.accountType) {
    return props.accounts;
  }
  
  switch (props.accountType) {
    case 'expenses':
      return props.accounts.filter(account => account.startsWith('Expenses:'));
    case 'income':
      return props.accounts.filter(account => account.startsWith('Income:'));
    case 'assets':
      return props.accounts.filter(account => account.startsWith('Assets:'));
    default:
      return props.accounts;
  }
});

// 筛选后的账户列表
const filteredAccounts = computed(() => {
  if (!searchQuery.value.trim()) {
    return typeFilteredAccounts.value;
  }
  
  const term = searchQuery.value.toLowerCase();
  return typeFilteredAccounts.value.filter(account => 
    account.toLowerCase().includes(term)
  );
});

// 处理输入
const handleInput = () => {
  // 实时筛选
  searchQuery.value = displayValue.value;
  selectedIndex.value = -1;
};

// 导航向下
const navigateDown = () => {
  if (filteredAccounts.value.length === 0) return;
  selectedIndex.value = (selectedIndex.value + 1) % filteredAccounts.value.length;
  scrollToSelected();
};

// 导航向上
const navigateUp = () => {
  if (filteredAccounts.value.length === 0) return;
  selectedIndex.value = (selectedIndex.value - 1 + filteredAccounts.value.length) % filteredAccounts.value.length;
  scrollToSelected();
};

// 选择当前高亮项
const selectCurrent = () => {
  if (selectedIndex.value >= 0 && selectedIndex.value < filteredAccounts.value.length) {
    selectAccount(filteredAccounts.value[selectedIndex.value]);
  }
};

// 选择账户
const selectAccount = (account: string) => {
  displayValue.value = account;
  emit('update:modelValue', account);
  showSelector.value = false;
  searchQuery.value = '';
  selectedIndex.value = -1;
  inputRef.value?.focus();
};

// 处理选择器鼠标按下事件
const handleSelectorMousedown = (event: MouseEvent) => {
  event.preventDefault();
};

// 滚动到选中项
const scrollToSelected = () => {
  nextTick(() => {
    const selectedElement = document.querySelector('.account-select-component .bg-primary\/10, .account-select-component .bg-primary\/20');
    if (selectedElement) {
      const container = selectedElement.closest('.max-h-60');
      if (container) {
        const containerRect = container.getBoundingClientRect();
        const elementRect = selectedElement.getBoundingClientRect();
        
        if (elementRect.top < containerRect.top) {
          container.scrollTop -= containerRect.top - elementRect.top;
        } else if (elementRect.bottom > containerRect.bottom) {
          container.scrollTop += elementRect.bottom - containerRect.bottom;
        }
      }
    }
  });
};

// 点击外部关闭选择器
const handleClickOutside = (event: MouseEvent) => {
  if (inputRef.value && !inputRef.value.contains(event.target as Node) && 
      !inputRef.value.parentElement?.querySelector('.sticky')?.contains(event.target as Node)) {
    showSelector.value = false;
    searchQuery.value = '';
    selectedIndex.value = -1;
  }
};

// 添加点击外部事件监听
watch(showSelector, (newValue) => {
  if (newValue) {
    setTimeout(() => {
      document.addEventListener('mousedown', handleClickOutside);
    }, 0);
  } else {
    document.removeEventListener('mousedown', handleClickOutside);
  }
});

// 当显示选择器时自动聚焦搜索框
watch(showSelector, (newValue) => {
  if (newValue) {
    nextTick(() => {
      searchInputRef.value?.focus();
    });
  }
});
</script>

<style scoped>
/* 自定义滚动条样式 */
.account-select-component .max-h-60::-webkit-scrollbar {
  width: 6px;
}

.account-select-component .max-h-60::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 3px;
}

.account-select-component .max-h-60::-webkit-scrollbar-thumb {
  background: #888;
  border-radius: 3px;
}

.account-select-component .max-h-60::-webkit-scrollbar-thumb:hover {
  background: #555;
}

/* 深色模式滚动条 */
.dark .account-select-component .max-h-60::-webkit-scrollbar-track {
  background: #1f2937;
}

.dark .account-select-component .max-h-60::-webkit-scrollbar-thumb {
  background: #4b5563;
}

.dark .account-select-component .max-h-60::-webkit-scrollbar-thumb:hover {
  background: #6b7280;
}
</style>