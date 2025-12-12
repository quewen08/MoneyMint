import { createBeancountManager } from '../utils/beancountManager'
import { defineEventHandler, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  try {
    const accounts = await beancountManager.getAccounts()
    return {
      success: true,
      accounts
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '获取账户列表失败'
    }
  }
})