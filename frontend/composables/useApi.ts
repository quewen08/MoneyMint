export const useApi = () => {
  const config = useRuntimeConfig()
  const apiBaseUrl = config.public.apiBaseUrl
  const authToken = useState<string | null>('authToken', () => null)
  const user = useState<{ username: string; role: string } | null>('user', () => null)

  // 从localStorage恢复登录状态
  if (process.client) {
    const savedToken = localStorage.getItem('authToken')
    const savedUser = localStorage.getItem('user')
    if (savedToken && savedUser) {
      authToken.value = savedToken
      user.value = JSON.parse(savedUser)
    }
  }

  const fetchApi = async (endpoint: string, options: RequestInit = {}) => {
    try {
      // 添加认证头
      const headers: any = {
        'Content-Type': 'application/json',
        ...options.headers
      }

      if (authToken.value) {
        headers['Authorization'] = `Bearer ${authToken.value}`
      }

      const response = await fetch(`${apiBaseUrl}${endpoint}`, {
        ...options,
        headers
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

  // 认证相关函数
  const login = async (username: string, password: string) => {
    try {
      const data = await fetchApi('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password })
      })
      
      // 保存认证信息
      authToken.value = data.access_token
      user.value = { username: data.username, role: data.role }
      
      // 持久化到localStorage
      if (process.client) {
        localStorage.setItem('authToken', data.access_token)
        localStorage.setItem('user', JSON.stringify({ username: data.username, role: data.role }))
      }
      
      return data
    } catch (error) {
      console.error('Login failed:', error)
      throw error
    }
  }

  const register = async (username: string, password: string) => {
    try {
      const data = await fetchApi('/auth/register', {
        method: 'POST',
        body: JSON.stringify({ username, password })
      })
      return data
    } catch (error) {
      console.error('Registration failed:', error)
      throw error
    }
  }

  const checkRegistrationStatus = async () => {
    try {
      const data = await fetchApi('/auth/register/status')
      return data
    } catch (error) {
      console.error('检查注册状态失败:', error)
      return { enabled: false }
    }
  }

  const logout = () => {
    // 清除认证信息
    authToken.value = null
    user.value = null
    
    // 从localStorage删除
    if (process.client) {
      localStorage.removeItem('authToken')
      localStorage.removeItem('user')
      // 重定向到登录页面
      window.location.href = '/login'
    }
  }

  return {
    // 认证状态
    authToken,
    user,
    
    // 认证函数
    login,
    register,
    checkRegistrationStatus,
    logout,
    
    // Ledger endpoints
    getLedger: () => fetchApi('/ledger'),
    getEntries: (params: {start_date?: string, end_date?: string, account?: string, page?: number, page_size?: number, sort?: string, order?: string} = {}) => {
      // 构建查询参数
      const queryParams = new URLSearchParams()
      if (params.start_date) queryParams.append('start_date', params.start_date)
      if (params.end_date) queryParams.append('end_date', params.end_date)
      if (params.account) queryParams.append('account', params.account)
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