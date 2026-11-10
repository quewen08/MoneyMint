/**
 * 记一笔面板的跨视图开关（C-01 单组件、单实例）
 *
 * 首页与布局都要打开同一个 RecordSheet，用 store 收敛状态，避免各自挂载
 * 多个实例重复加载账户；保存成功后用 savedEpoch 通知首页刷新余额与流水。
 */
import { ref } from 'vue'
import { defineStore } from 'pinia'

import type { TransactionCreateResult } from '@/api/client'

export const useRecordSheetStore = defineStore('recordSheet', () => {
  const isOpen = ref(false)
  const savedEpoch = ref(0)
  const lastSaved = ref<TransactionCreateResult | null>(null)

  function open(): void {
    isOpen.value = true
  }

  function close(): void {
    isOpen.value = false
  }

  function notifySaved(result: TransactionCreateResult): void {
    lastSaved.value = result
    // 纯计数器（非金额运算），供首页 watch 后重新加载数据。
    savedEpoch.value = savedEpoch.value + 1
  }

  return { isOpen, savedEpoch, lastSaved, open, close, notifySaved }
})
