import { createBeancountManager } from '../../../utils/beancountManager'
import { defineEventHandler, readBody, type H3Event } from 'h3'

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
    const body = await readBody(event)
    const { date, balance, padAccount } = body
    
    if (!date || !balance || !padAccount) {
      return {
        success: false,
        error: '缺少必要参数'
      }
    }
    
    const result = await beancountManager.setAccountBalance(account, date, balance, padAccount)
    
    if (result) {
      return {
        success: true,
        message: '账户余额设置成功'
      }
    } else {
      return {
        success: false,
        error: '账户余额设置失败'
      }
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '设置账户余额失败'
    }
  }
})