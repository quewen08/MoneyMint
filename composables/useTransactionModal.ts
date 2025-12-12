import { ref } from 'vue'

// 创建全局响应式状态
const showModal = ref(false)

function openModal() {
  showModal.value = true
}

function closeModal() {
  showModal.value = false
}

// 导出可在组件中使用的组合式函数
export function useTransactionModal() {
  return {
    showModal,
    openModal,
    closeModal
  }
}

// 导出可在插件或其他地方使用的函数
export { showModal, openModal, closeModal }