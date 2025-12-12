import { createBeancountManager } from '../utils/beancountManager'

export default defineNitroPlugin(async () => {
  console.log('开始同步Beancount文件与JSON文件...')
  const beancountManager = createBeancountManager()
  
  try {
    await beancountManager.syncTransactions()
    console.log('Beancount文件与JSON文件同步完成！')
  } catch (error) {
    console.error('同步过程中发生错误:', error)
  }
})