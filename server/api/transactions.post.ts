import { createBeancountManager } from '../utils/beancountManager'
import { defineEventHandler, readBody, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  try {
    const body = await readBody(event)
    
    // Validate transaction data for multi-account structure
    if (!body.date || !body.type || !body.narration || !body.postings || body.postings.length < 2) {
      return {
        success: false,
        error: '缺少必要的交易信息: 日期、类型、描述和至少两个账户行是必需的'
      }
    }
    
    // Validate postings
    for (const posting of body.postings) {
      if (!posting.account || posting.amount === undefined || posting.amount === null) {
        return {
          success: false,
          error: '每个账户行都必须包含账户和金额'
        }
      }
    }
    
    const transaction = await beancountManager.addTransaction(body)
    
    return {
      success: true,
      transaction
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '添加交易失败'
    }
  }
})