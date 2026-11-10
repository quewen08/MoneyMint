<script setup lang="ts">
/**
 * 金额 / 符号口径统一组件（C-01 配套，F-1 / AC-11 / AC-12）
 *
 * - 只调用 `formatMoney`，负号前置到币种符号之前，金额本体取绝对值；
 * - **绝不 abs() / 按账户类型乘系数**：后端返回的 Liabilities / Income
 *   余额本身为负，这里原样显示负号（§6.4 静默错账防线）；
 * - 净资产为负同样保留负号；隐私开关开启后所有金额位统一遮挡。
 */
import { computed } from 'vue'

import { formatMoney } from '@/core/Money'
import { usePrivacyStore } from '@/stores/privacy'

const props = withDefaults(
  defineProps<{
    /** 定点十进制金额字符串（后端原始带符号值）。 */
    amount: string
    commodity?: string
    /** 正数是否显式加 `+`（零不显示符号）。 */
    signed?: boolean
    /** 单独强制遮挡；未设置时跟随全局隐私开关。 */
    privacy?: boolean
    size?: 'sm' | 'md' | 'lg'
  }>(),
  {
    commodity: 'CNY',
    signed: false,
    privacy: false,
    size: 'md',
  },
)

const privacyStore = usePrivacyStore()
const masked = computed(() => props.privacy || privacyStore.enabled)
const text = computed(() =>
  masked.value
    ? '••••••'
    : formatMoney(props.amount, props.commodity, { signed: props.signed }),
)
const negative = computed(
  () => !masked.value && text.value.charAt(0) === '-',
)
</script>

<template>
  <span
    class="money-text"
    :class="[
      `money-text--${size}`,
      { 'money-text--negative': negative, 'money-text--masked': masked },
    ]"
    :aria-label="masked ? '金额已隐藏' : text"
  >{{ text }}</span>
</template>

<style scoped>
.money-text {
  display: inline-block;
  font-variant-numeric: var(--fb-font-numeric);
  letter-spacing: 0.02em;
  white-space: nowrap;
}

.money-text--sm {
  font-size: 13px;
  font-weight: 600;
}

.money-text--md {
  font-size: 16px;
  font-weight: 700;
}

.money-text--lg {
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.02em;
}

/* 负债、收入账户的负余额与负净资产：保留负号并用危险色强调。 */
.money-text--negative {
  color: var(--fb-danger);
}

.money-text--masked {
  color: var(--fb-sub);
}
</style>
