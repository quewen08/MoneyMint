import { ref, onMounted } from 'vue'
import { useApi } from './useApi'

export const useSystemConfig = () => {
  // 系统配置
  const config = ref({
    currency: 'CNY', // 默认货币
  })

  const { getLedger } = useApi()

  // 初始化系统配置
  const initConfig = async () => {
    try {
      const ledger = await getLedger()
      if (ledger && ledger.currency) {
        config.value.currency = ledger.currency
      }
    } catch (error) {
      console.error('Failed to initialize system config:', error)
      // 使用默认值
      config.value.currency = 'CNY'
    }
  }

  // 更新系统配置
  const updateConfig = (newConfig: Partial<typeof config.value>) => {
    config.value = { ...config.value, ...newConfig }
  }

  // 获取当前货币
  const getCurrency = () => {
    return config.value.currency
  }

  return {
    config,
    initConfig,
    updateConfig,
    getCurrency
  }
}