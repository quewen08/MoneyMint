import { createBeancountManager } from '../utils/beancountManager'
import { defineEventHandler, type H3Event } from 'h3'

// 创建Beancount管理器实例
const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  return {
    message: 'MoneyMint API is running',
    endpoints: {
      transactions: '/api/transactions',
      balance: '/api/balance'
    }
  }
})