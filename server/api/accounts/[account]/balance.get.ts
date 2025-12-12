import { createBeancountManager } from '../../../utils/beancountManager'
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
    const balance = await beancountManager.getAccountBalance(account)
    return {
      success: true,
      account,
      balance
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '获取账户余额失败'
    }
  }
})