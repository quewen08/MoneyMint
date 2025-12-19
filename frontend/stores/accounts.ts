import { defineStore } from 'pinia'
import { useNuxtApp } from '#app'

export const useAccountsStore = defineStore('accounts', {
  state: () => ({
    list: [] as string[],
    loading: false,
    error: null as string | null
  }),

  getters: {
    hasAccounts: (state) => state.list.length > 0
  },

  actions: {
    async fetchAccounts() {
      if (this.loading) return
      if (this.hasAccounts) return this.list
      
      try {
        this.loading = true
        this.error = null
        
        const { $api } = useNuxtApp()
        const accountList = await $api.accounts.getAccounts()
        
        this.list = accountList
        return accountList
      } catch (error) {
        this.error = error instanceof Error ? error.message : '加载账户列表失败'
        console.error('Failed to fetch accounts:', error)
        return []
      } finally {
        this.loading = false
      }
    },

    clearAccounts() {
      this.list = []
    },

    updateAccounts(newAccounts: string[]) {
      this.list = newAccounts
    },

    // 监听SSE事件更新账户列表
    handleSseUpdate(event: CustomEvent) {
      const data = event.detail
      
      if (data.event === 'accounts' && data.data) {
        this.list = data.data
        console.log('账户列表已从SSE更新')
      }
      
      // 当有条目添加/更新/删除时，清除本地缓存，下次请求时重新获取
      if (['entry_added', 'entry_updated', 'entry_deleted'].includes(data.event)) {
        this.list = []
      }
    }
  }
})