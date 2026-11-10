/**
 * 金额隐私遮挡开关（F-1 / AC-11）
 *
 * 家庭场景防瞟屏：开启后所有 `MoneyText` 统一显示遮挡符。
 * 只持久化一个布尔量，不含任何凭据。
 */
import { ref } from 'vue'
import { defineStore } from 'pinia'

import { readPrivacy, writePrivacy } from '@/core/storage'

export const usePrivacyStore = defineStore('privacy', () => {
  const enabled = ref<boolean>(readPrivacy())

  function setEnabled(next: boolean): void {
    enabled.value = next
    writePrivacy(next)
  }

  function toggle(): void {
    setEnabled(!enabled.value)
  }

  return { enabled, setEnabled, toggle }
})
