// 应用基本信息
export const APP_NAME = 'MoneyMint 记账'
export const APP_DESCRIPTION = '基于 Beancount 的现代化个人记账系统'

// 版本信息接口
export interface VersionInfo {
  version: string
}

// 从后端API获取版本信息
export const fetchVersionInfo = async (): Promise<VersionInfo> => {
  try {
    const response = await fetch('/api/version')
    if (!response.ok) {
      throw new Error('Failed to fetch version info')
    }
    return await response.json()
  } catch (error) {
    console.error('Error fetching version info:', error)
    // 出错时返回默认版本
    return { version: '1.0.0' }
  }
}