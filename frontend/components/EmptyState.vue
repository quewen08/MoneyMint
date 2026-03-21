<template>
  <div class="empty-state">
    <div class="empty-state-icon">
      <slot name="icon">
        <svg class="w-full h-full" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
        </svg>
      </slot>
    </div>
    <p class="empty-state-title">
      <slot name="title">暂无数据</slot>
    </p>
    <p class="empty-state-desc">
      <slot name="description">还没有任何记录，开始添加吧！</slot>
    </p>
    <div v-if="$slots.actions" class="mt-6">
      <slot name="actions"></slot>
    </div>
  </div>
</template>

<script setup lang="ts">
// 通用空状态组件
</script>
