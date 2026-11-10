<script setup lang="ts">
/**
 * 应用壳（批 1）
 *
 * 业务页套 L-01 单布局（PC 侧栏 / 移动底栏）；登录、待邀请、改密三张
 * 独立全屏页不套布局，与 v2 原型一致。版本标识来自构建期注入。
 */
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import AppLayout from '@/components/AppLayout.vue'

const route = useRoute()
const version = __APP_VERSION__
const standalone = computed(() => route.meta.standalone === true)
</script>

<template>
  <AppLayout v-if="!standalone">
    <RouterView />
  </AppLayout>
  <div v-else class="app-standalone">
    <RouterView />
    <div class="app-standalone__version">FamBean v{{ version }}</div>
  </div>
</template>

<style scoped>
.app-standalone {
  min-height: 100vh;
  background: var(--fb-bg);
  color: var(--fb-ink);
}

.app-standalone__version {
  padding: 16px;
  text-align: center;
  font-size: 11px;
  color: var(--fb-sub);
  font-variant-numeric: var(--fb-font-numeric);
}
</style>
