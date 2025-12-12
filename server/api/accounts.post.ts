import { createBeancountManager } from '../utils/beancountManager'
import { defineEventHandler, readBody, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  try {
    const body = await readBody(event)
    
    // Validate account data
    if (!body.nickname || !body.actualName || !body.type) {
      return {
        success: false,
        error: '缺少必要的账户信息'
      }
    }
    
    const account = await beancountManager.addAccount(body.nickname, body.actualName, body.type)
    
    return {
      success: true,
      account
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '添加账户失败'
    }
  }
})