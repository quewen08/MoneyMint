import { createBeancountManager } from '../../utils/beancountManager'
import { defineEventHandler, type H3Event } from 'h3'

const beancountManager = createBeancountManager()

export default defineEventHandler(async (event: H3Event) => {
  const id = event.context.params?.id
  
  if (!id) {
    return {
      success: false,
      error: '缺少交易ID'
    }
  }
  
  try {
    await beancountManager.deleteTransaction(id)
    return {
      success: true,
      message: '交易记录删除成功'
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '删除交易失败'
    }
  }
})