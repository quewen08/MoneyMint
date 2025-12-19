import { useApi } from '~/composables/useApi'

export default defineNuxtPlugin(() => {
  // 创建一个单例的useApi实例
  const api = useApi()

  // 将api实例挂载到全局上下文中
  return {
    provide: {
      api
    }
  }
})