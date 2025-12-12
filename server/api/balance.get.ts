import { createBeancountManager } from '../utils/beancountManager'
import { defineEventHandler, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  try {
    const balance = await beancountManager.getBalance()
    return {
      success: true,
      ...balance
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '获取余额失败'
    }
  }
})