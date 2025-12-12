import { createBeancountManager } from '../utils/beancountManager'
import { defineEventHandler, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  try {
    const monthlyStats = await beancountManager.getMonthlyStats()
    return {
      success: true,
      monthlyStats
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '获取月度收支数据失败'
    }
  }
})