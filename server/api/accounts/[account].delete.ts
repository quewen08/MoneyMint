import { createBeancountManager } from '../../utils/beancountManager'
import { defineEventHandler, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  const account = event.context.params?.account
  
  if (!account) {
    return {
      success: false,
      error: '缺少账户名称'
    }
  }
  
  try {
    await beancountManager.deleteAccount(decodeURIComponent(account))
    
    return {
      success: true,
      message: '账户删除成功'
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '删除账户失败'
    }
  }
})