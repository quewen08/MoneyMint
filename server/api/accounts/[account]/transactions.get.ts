import { createBeancountManager } from '../../../utils/beancountManager'
import { defineEventHandler, getQuery, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  const account = event.context.params?.account
  const query = getQuery(event)
  const limit = parseInt(query.limit as string) || 0
  
  if (!account) {
    return {
      success: false,
      error: '缺少账户名称'
    }
  }
  
  try {
    const transactions = await beancountManager.getAccountTransactions(account, limit)
    return {
      success: true,
      account,
      transactions
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '获取账户交易记录失败'
    }
  }
})