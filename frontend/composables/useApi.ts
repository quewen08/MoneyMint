export const useApi = () => {
  const config = useRuntimeConfig()
  const apiBaseUrl = config.public.apiBaseUrl

const fetchApi = async (endpoint: string, options: RequestInit = {}) => {
    try {
      const response = await fetch(`${apiBaseUrl}${endpoint}`, {
        ...options,
        headers: {
          'Content-Type': 'application/json',
          ...options.headers
        }
      })

      if (!response.ok) {
        const error = await response.json().catch(() => ({
          message: 'Unknown error occurred'
        }))
        throw new Error(error.message || response.statusText)
      }

      return await response.json()
    } catch (error) {
      console.error('API Error:', error)
      throw error
    }
  }

  return {
    // Ledger endpoints
    getLedger: () => fetchApi('/ledger'),
    getEntries: (params: {start_date?: string, end_date?: string, page?: number, page_size?: number, sort?: string, order?: string} = {}) => {
      // 构建查询参数
      const queryParams = new URLSearchParams()
      if (params.start_date) queryParams.append('start_date', params.start_date)
      if (params.end_date) queryParams.append('end_date', params.end_date)
      if (params.page) queryParams.append('page', params.page.toString())
      if (params.page_size) queryParams.append('page_size', params.page_size.toString())
      if (params.sort) queryParams.append('sort', params.sort)
      if (params.order) queryParams.append('order', params.order)
      
      const endpoint = `/entries?${queryParams.toString()}`
      return fetchApi(endpoint)
    },
    runQuery: (query: string) => fetchApi('/query', {
      method: 'POST',
      body: JSON.stringify({ query })
    }),
    addEntry: (entry: any) => fetchApi('/entries', {
      method: 'POST',
      body: JSON.stringify(entry)
    }),
    /**
     * 更新记账条目
     */
    updateEntry: (entryId: string, entry: any) => fetchApi(`/entries/${entryId}`, {
      method: 'PUT',
      body: JSON.stringify(entry)
    }),
    /**
     * 删除记账条目
     */
    deleteEntry: (entryId: string) => fetchApi(`/entries/${entryId}`, {
      method: 'DELETE'
    }),
    // Accounts endpoint
    accounts: {
      getAccounts: () => fetchApi('/accounts'),
      getAccountBalances: (params: {start_date?: string, end_date?: string} = {}) => {
        const queryParams = new URLSearchParams()
        if (params.start_date) queryParams.append('start_date', params.start_date)
        if (params.end_date) queryParams.append('end_date', params.end_date)
        
        const endpoint = `/accounts/balances?${queryParams.toString()}`
        return fetchApi(endpoint)
      },
      openAccount: (entry: any) => fetchApi('/entries', {
        method: 'POST',
        body: JSON.stringify(entry)
      }),
      closeAccount: (entry: any) => fetchApi('/entries', {
        method: 'POST',
        body: JSON.stringify(entry)
      }),
      setBalance: (entry: any) => fetchApi('/entries', {
        method: 'POST',
        body: JSON.stringify(entry)
      }),
      padBalance: (entry: any) => fetchApi('/entries', {
        method: 'POST',
        body: JSON.stringify(entry)
      })
    }
  }
}