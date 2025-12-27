import { ref, onMounted } from 'vue'
import { useApi } from './useApi'

export const useSystemConfig = () => {
  // 系统配置
  const config = ref({
    currency: 'CNY', // 默认货币
  })
  
  // 保存完整的账本信息
  const ledger = ref<any>({
    title: '',
    currency: '',
    entries_count: 0,
    errors_count: 0,
    errors: []
  })

  const { getLedger } = useApi()

  // 初始化系统配置
  const initConfig = async () => {
    try {
      const ledgerData = await getLedger()
      if (ledgerData) {
        // 更新账本信息
        ledger.value = ledgerData
        // 更新货币配置
        if (ledgerData.currency) {
          config.value.currency = ledgerData.currency
        }
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
    ledger,
    initConfig,
    updateConfig,
    getCurrency
  }
}