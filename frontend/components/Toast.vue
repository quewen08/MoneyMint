<template>
  <div class="fixed top-4 right-4 z-[9999] space-y-3 pointer-events-none">
    <TransitionGroup name="toast">
      <div v-for="toast in toasts" :key="toast.id"
        class="pointer-events-auto flex items-center gap-3 px-4 py-3 rounded-xl shadow-lg animate-slide-in-right"
        :class="getToastClasses(toast.type)">
        <div class="flex-shrink-0">
          <svg v-if="toast.type === 'success'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
          </svg>
          <svg v-else-if="toast.type === 'error'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
          </svg>
          <svg v-else-if="toast.type === 'warning'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>
          </svg>
          <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
          </svg>
        </div>
        <p class="text-sm font-medium flex-1">{{ toast.message }}</p>
        <button @click="removeToast(toast.id)" class="p-1 rounded hover:bg-black/10 dark:hover:bg-white/10 transition-colors">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
          </svg>
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

interface Toast {
  id: string
  message: string
  type: 'success' | 'error' | 'warning' | 'info'
}

const toasts = ref<Toast[]>([])

const getToastClasses = (type: string) => {
  const base = 'bg-white dark:bg-gray-800 text-gray-900 dark:text-white border'
  const types = {
    success: `${base} border-green-200 dark:border-green-800 text-green-700 dark:text-green-300`,
    error: `${base} border-red-200 dark:border-red-800 text-red-700 dark:text-red-300`,
    warning: `${base} border-yellow-200 dark:border-yellow-800 text-yellow-700 dark:text-yellow-300`,
    info: `${base} border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300`,
  }
  return types[type as keyof typeof types] || types.info
}

const showToast = (message: string, type: Toast['type'] = 'info', duration = 4000) => {
  const id = Date.now().toString()
  toasts.value.push({ id, message, type })

  if (duration > 0) {
    setTimeout(() => {
      removeToast(id)
    }, duration)
  }
}

const removeToast = (id: string) => {
  const index = toasts.value.findIndex(t => t.id === id)
  if (index > -1) {
    toasts.value.splice(index, 1)
  }
}

defineExpose({
  showToast,
  removeToast
})

onMounted(() => {
  (window as any).$toast = {
    success: (msg: string) => showToast(msg, 'success'),
    error: (msg: string) => showToast(msg, 'error'),
    warning: (msg: string) => showToast(msg, 'warning'),
    info: (msg: string) => showToast(msg, 'info'),
  }
})

onUnmounted(() => {
  delete (window as any).$toast
})
</script>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease-out;
}
.toast-enter-from {
  transform: translateX(100%);
  opacity: 0;
}
.toast-leave-to {
  transform: translateX(100%);
  opacity: 0;
}
</style>
