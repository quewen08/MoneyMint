import { createBeancountManager } from '../utils/beancountManager'
import { defineEventHandler, getQuery, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  const query = getQuery(event)
  const limit = parseInt(query.limit as string) || 0
  
  try {
    const transactions = await beancountManager.getTransactions(undefined, undefined, limit)
    return {
      success: true,
      transactions
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '获取交易记录失败'
    }
  }
})