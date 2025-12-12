import fs from 'fs/promises'
import path from 'path'
import Database from 'better-sqlite3'

// Get project root directory path
const ROOT_DIR = process.cwd()
const DATA_DIR = path.join(ROOT_DIR, 'data')
const TRANSACTIONS_FILE = path.join(DATA_DIR, 'transactions.json')
const DB_FILE = path.join(DATA_DIR, 'transactions.db')

// SQLite数据库连接
let db: Database = null

// Beancount文件结构
const BEANCOUNT_FILES = {
  main: path.join(DATA_DIR, 'main.bean'),
  accounts: {
    assets: path.join(DATA_DIR, 'accounts/assets.bean'),
    liabilities: path.join(DATA_DIR, 'accounts/liabilities.bean'),
    expenses: path.join(DATA_DIR, 'accounts/expenses.bean'),
    income: path.join(DATA_DIR, 'accounts/income.bean'),
    equity: path.join(DATA_DIR, 'accounts/equity.bean')
  },
  date: {
    index: path.join(DATA_DIR, 'date/date.bean')
  }
}

// Posting 接口定义 - 单个账户行
export interface Posting {
  account: string  // 账户昵称或实际名称
  amount: number  // 金额，正数表示流入，负数表示流出
  currency?: string  // 货币类型，默认CNY
}

// Transaction 接口定义 - 符合Beancount标准结构，支持多账户交易
export interface Transaction {
  id: string
  date: string
  flag?: string  // Beancount标志，默认为"*"
  payee?: string  // 收款人/付款人
  narration: string  // 交易描述
  tags?: string[]  // 标签列表
  links?: string[]  // 链接列表
  type: 'income' | 'expense' | 'transfer'  // 支持转账类型
  postings: Posting[]  // 多账户交易行
  timestamp: number  // 时间戳
}

interface Balance {
  balance: number
  totalIncome: number
  totalExpense: number
}

export function createBeancountManager() {
  // 确保数据目录存在
  async function ensureDataDir() {
    try {
      await fs.mkdir(DATA_DIR, { recursive: true })
    } catch (error) {
      console.error('创建数据目录失败:', error)
      throw error
    }
  }

  // 初始化数据库连接
  function initializeDatabase() {
    if (!db) {
      db = new Database(DB_FILE)
      
      // 检查并处理旧的表结构
      try {
        // 检查transactions表是否有postings字段
        const hasPostingsColumn = db.prepare(`
          SELECT COUNT(*) AS count FROM pragma_table_info('transactions') WHERE name = 'postings'
        `).get().count > 0
        
        if (hasPostingsColumn) {
          // 如果有postings字段，需要迁移数据
          console.log('检测到旧数据库结构，正在迁移数据...')
          
          // 确保posts表存在
          db.exec(`
            CREATE TABLE IF NOT EXISTS posts (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              transaction_id TEXT NOT NULL,
              account TEXT NOT NULL,
              amount REAL NOT NULL,
              currency TEXT DEFAULT 'CNY',
              FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE
            )
          `)
          
          // 从旧表中读取所有交易
          const oldTransactions = db.prepare('SELECT * FROM transactions').all()
          
          // 将旧表中的postings数据迁移到posts表
          for (const oldTrans of oldTransactions) {
            if (oldTrans.postings) {
              try {
                const postings = JSON.parse(oldTrans.postings)
                for (const posting of postings) {
                  db.prepare(`
                    INSERT INTO posts (transaction_id, account, amount, currency)
                    VALUES (?, ?, ?, ?)
                  `).run(oldTrans.id, posting.account, posting.amount, posting.currency || 'CNY')
                }
              } catch (e) {
                console.error(`迁移交易 ${oldTrans.id} 的分账单失败:`, e)
              }
            }
          }
          
          // 创建新的transactions表（不包含postings字段）
          db.exec('ALTER TABLE transactions RENAME TO transactions_old')
          db.exec(`
            CREATE TABLE IF NOT EXISTS transactions (
              id TEXT PRIMARY KEY,
              date TEXT NOT NULL,
              flag TEXT,
              payee TEXT,
              narration TEXT NOT NULL,
              tags TEXT,
              links TEXT,
              type TEXT NOT NULL,
              timestamp INTEGER NOT NULL
            )
          `)
          
          // 将旧表数据迁移到新表
          db.exec(`
            INSERT INTO transactions (id, date, flag, payee, narration, tags, links, type, timestamp)
            SELECT id, date, flag, payee, narration, tags, links, type, timestamp
            FROM transactions_old
          `)
          
          // 删除旧表
          db.exec('DROP TABLE transactions_old')
          
          console.log('数据库结构迁移完成！')
        } else {
          // 如果没有postings字段，直接创建表
          db.exec(`
            CREATE TABLE IF NOT EXISTS transactions (
              id TEXT PRIMARY KEY,
              date TEXT NOT NULL,
              flag TEXT,
              payee TEXT,
              narration TEXT NOT NULL,
              tags TEXT,
              links TEXT,
              type TEXT NOT NULL,
              timestamp INTEGER NOT NULL
            )
          `)
        }
      } catch (e) {
        console.error('检查或迁移数据库结构时出错:', e)
        // 如果出错，尝试重新创建表
        db.exec('DROP TABLE IF EXISTS transactions')
        db.exec('DROP TABLE IF EXISTS posts')
        
        // 创建新表
        db.exec(`
          CREATE TABLE IF NOT EXISTS transactions (
            id TEXT PRIMARY KEY,
            date TEXT NOT NULL,
            flag TEXT,
            payee TEXT,
            narration TEXT NOT NULL,
            tags TEXT,
            links TEXT,
            type TEXT NOT NULL,
            timestamp INTEGER NOT NULL
          )
        `)
      }
      
      // 确保posts表存在
      db.exec(`
        CREATE TABLE IF NOT EXISTS posts (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          transaction_id TEXT NOT NULL,
          account TEXT NOT NULL,
          amount REAL NOT NULL,
          currency TEXT DEFAULT 'CNY',
          FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE
        )
      `)
      
      // 为account字段创建索引，加速账户余额查询
      db.exec(`CREATE INDEX IF NOT EXISTS idx_posts_account ON posts(account)`)
    }
  }

  // 初始化Beancount文件
  async function initializeBeancountFile() {
    await ensureDataDir()
    // 检查主要文件是否存在
    try {
      await fs.access(BEANCOUNT_FILES.main)
      // 检查账户目录是否存在
      await fs.access(BEANCOUNT_FILES.accounts.assets)
      // 检查日期目录是否存在
      await fs.access(BEANCOUNT_FILES.date.index)
    } catch (error) {
      console.error('Beancount文件结构不完整:', error)
      throw new Error('Beancount文件结构不完整，请确保所有必要文件都存在')
    }
    // 初始化数据库
    initializeDatabase()
  }

  // 读取所有交易记录
  async function readTransactions(): Promise<Transaction[]> {
    await ensureDataDir()
    initializeDatabase()

    try {
      // 首先尝试从数据库读取
      const stmt = db.prepare('SELECT * FROM transactions ORDER BY timestamp')
      const rows = stmt.all()
      
      // 读取所有分账单记录
      const postsStmt = db.prepare('SELECT * FROM posts')
      const posts = postsStmt.all()
      
      // 将分账单按交易ID分组
      const postsByTransactionId = new Map<string, Posting[]>()
      for (const post of posts) {
        if (!postsByTransactionId.has(post.transaction_id)) {
          postsByTransactionId.set(post.transaction_id, [])
        }
        postsByTransactionId.get(post.transaction_id)?.push({
          account: post.account,
          amount: post.amount
        })
      }
      
      // 组合交易记录和分账单
      return rows.map((row: any) => ({
        ...row,
        tags: row.tags ? JSON.parse(row.tags) : [],
        links: row.links ? JSON.parse(row.links) : [],
        postings: postsByTransactionId.get(row.id) || []
      }))
    } catch (error) {
      console.error('从数据库读取交易记录失败:', error)
      // 如果数据库读取失败，尝试从JSON文件读取作为备选
      try {
        const data = await fs.readFile(TRANSACTIONS_FILE, 'utf-8')
        const transactions = JSON.parse(data)
        // 将JSON数据导入数据库
        await writeTransactions(transactions)
        return transactions
      } catch (jsonError) {
        if (jsonError instanceof Error && 'code' in jsonError && jsonError.code === 'ENOENT') {
          // 文件也不存在，返回空数组
          return []
        }
        throw jsonError
      }
    }
  }

  // 写入交易记录
  async function writeTransactions(transactions: Transaction[]): Promise<void> {
    await ensureDataDir()
    initializeDatabase()

    try {
      // 开始事务
      db.exec('BEGIN TRANSACTION')
      
      // 清空现有数据
      db.exec('DELETE FROM transactions')
      db.exec('DELETE FROM posts')
      
      // 批量插入新交易和分账单
      const transactionStmt = db.prepare(`
        INSERT INTO transactions (id, date, flag, payee, narration, tags, links, type, timestamp) 
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
      `)
      
      const postStmt = db.prepare(`
        INSERT INTO posts (transaction_id, account, amount) 
        VALUES (?, ?, ?)
      `)
      
      for (const transaction of transactions) {
        // 插入交易记录
        transactionStmt.run(
          transaction.id,
          transaction.date,
          transaction.flag,
          transaction.payee,
          transaction.narration,
          JSON.stringify(transaction.tags || []),
          JSON.stringify(transaction.links || []),
          transaction.type,
          transaction.timestamp
        )
        
        // 插入分账单记录
        for (const posting of transaction.postings) {
          postStmt.run(
            transaction.id,
            posting.account,
            posting.amount
          )
        }
      }
      
      // 提交事务
      db.exec('COMMIT')
      
      // 同时更新JSON文件作为备份
      await fs.writeFile(TRANSACTIONS_FILE, JSON.stringify(transactions, null, 2), 'utf-8')
      console.log('交易记录写入数据库成功！')
    } catch (error) {
      // 回滚事务
      db.exec('ROLLBACK')
      console.error('写入交易记录失败:', error)
      // 回退到JSON文件存储
      await fs.writeFile(TRANSACTIONS_FILE, JSON.stringify(transactions, null, 2), 'utf-8')
      throw error
    } finally {
      // 无论如何都更新Beancount文件
      await updateBeancountFile(transactions)
    }
  }

  // 更新Beancount文件
  async function updateBeancountFile(transactions: Transaction[]): Promise<void> {
    // 按日期分组交易记录
    const transactionsByMonth = transactions.reduce((acc, transaction) => {
      const [year, month] = transaction.date.split('-')
      const key = `${year}-${month}`
      if (!acc[key]) {
        acc[key] = []
      }
      acc[key].push(transaction)
      return acc
    }, {} as Record<string, Transaction[]>)

    // 处理每个月的交易记录
    for (const [yearMonth, monthTransactions] of Object.entries(transactionsByMonth)) {
      const [year, month] = yearMonth.split('-')
      const monthDir = path.join(DATA_DIR, `date/${year}`)
      const monthFile = path.join(monthDir, `${yearMonth}.bean`)

      // 确保目录存在
      await fs.mkdir(monthDir, { recursive: true })

      // 排序交易记录
      monthTransactions.sort((a, b) => a.timestamp - b.timestamp)

      // 生成交易记录内容 - 使用async/await正确处理
      const transactionContents = []
      for (const transaction of monthTransactions) {
        const date = transaction.date
        
        // 检查是否是balance指令
        if (transaction.tags && transaction.tags.includes('balance')) {
          const posting = transaction.postings[0]
          const currency = posting.currency || 'CNY'
          // balance指令格式: YYYY-MM-DD balance Account.Name 123.45 CNY
          const balanceLine = `${date} balance ${posting.account} ${posting.amount} ${currency}`
          transactionContents.push(`${balanceLine}\n`)
        }
        // 检查是否是pad指令
        else if (transaction.tags && transaction.tags.includes('pad')) {
          const targetAccount = transaction.postings[0].account
          const padAccount = transaction.postings[1].account
          // pad指令格式: YYYY-MM-DD pad Account.To.Balance Account.From.Pad
          const padLine = `${date} pad ${targetAccount} ${padAccount}`
          transactionContents.push(`${padLine}\n`)
        }
        // 普通交易
        else {
          const payee = transaction.payee ? `"${transaction.payee}"` : ''
          const narration = transaction.narration ? `"${transaction.narration}"` : ''

          // 生成标签和链接字符串
          const tags = transaction.tags ? transaction.tags.map(tag => `#${tag}`).join(' ') : ''
          const links = transaction.links ? transaction.links.map(link => `^${link}`).join(' ') : ''

          // 构建交易头
          const transactionHeader = `${date} ${transaction.flag || '*'} ${payee} ${narration} ${tags} ${links}`.trim()

          // 生成每个账户行（posting）
          const postings = transaction.postings
            .map(posting => {
              const amount = posting.amount
              const currency = posting.currency || 'CNY'

              // 格式化金额，正数不加符号，负数保留符号
              const formattedAmount = amount >= 0 ? `${amount} ${currency}` : `${amount} ${currency}`

              return `  ${posting.account}                  ${formattedAmount}`
            })
            .join('\n')

          // 组合完整交易记录
          transactionContents.push(`${transactionHeader}\n${postings}\n`)
        }
      }

      const transactionsContent = transactionContents.join('\n')

      // 写入月份文件
      await fs.writeFile(monthFile, transactionsContent, 'utf-8')
    }

    // 更新date.bean文件，确保所有月份文件都被包含
    const dateIndexContent = await fs.readFile(BEANCOUNT_FILES.date.index, 'utf-8')
    const yearDirs = await fs.readdir(path.join(DATA_DIR, 'date'))

    // 生成include语句
    const includeStatements = yearDirs
      .filter(dir => !isNaN(Number(dir)))
      .flatMap(async (year) => {
        const yearDir = path.join(DATA_DIR, `date/${year}`)
        const monthFiles = await fs.readdir(yearDir)
        return monthFiles
          .filter(file => file.endsWith('.bean'))
          .map(file => `include "${year}/${file}"`)
      })

    const includes = await Promise.all(includeStatements)
    const newDateIndexContent = `;【交易记录】\n\n${includes.flat().join('\n')}`

    // 写入date.bean文件
    await fs.writeFile(BEANCOUNT_FILES.date.index, newDateIndexContent, 'utf-8')
  }

  // 账户类型定义
  interface AccountInfo {
    nickname: string;
    actualName: string;
    type: 'asset' | 'liability' | 'income' | 'expense';
  }

  // 获取交易记录 - 支持按日期范围筛选和分页
  async function getTransactions(startDate?: number, endDate?: number, limit = 0): Promise<Transaction[]> {
    await initializeBeancountFile()
    initializeDatabase()

    try {
      // 使用SQLite查询交易记录
      let query = `
        SELECT * FROM transactions
        WHERE 1=1
      `

      const params: any[] = []

      // 添加日期范围筛选条件
      if (startDate) {
        query += ` AND timestamp >= ?`
        params.push(startDate)
      }
      if (endDate) {
        query += ` AND timestamp <= ?`
        params.push(endDate)
      }

      // 添加排序和限制条件
      query += ` ORDER BY timestamp DESC`
      if (limit > 0) {
        query += ` LIMIT ?`
        params.push(limit)
      }

      const stmt = db.prepare(query)
      const rows = stmt.all(...params)

      // 获取这些交易对应的所有分账单
      if (rows.length > 0) {
        const transactionIds = rows.map(row => row.id)
        const placeholders = transactionIds.map(() => '?').join(',')
        const postsStmt = db.prepare(`SELECT * FROM posts WHERE transaction_id IN (${placeholders})`)
        const posts = postsStmt.all(...transactionIds)

        // 将分账单按交易ID分组
        const postsByTransactionId = new Map<string, Posting[]>()
        for (const post of posts) {
          if (!postsByTransactionId.has(post.transaction_id)) {
            postsByTransactionId.set(post.transaction_id, [])
          }
          postsByTransactionId.get(post.transaction_id)?.push({
            account: post.account,
            amount: post.amount
          })
        }

        // 组合交易记录和分账单
        return rows.map(row => ({
          ...row,
          tags: row.tags ? JSON.parse(row.tags) : [],
          links: row.links ? JSON.parse(row.links) : [],
          postings: postsByTransactionId.get(row.id) || []
        }))
      } else {
        // 没有交易记录时返回空数组
        return []
      }
    } catch (error) {
      console.error('从数据库查询交易记录失败:', error)
      // 如果数据库查询失败，回退到原始方法
      const transactions = await readTransactions()
      let filteredTransactions = transactions

      if (startDate) {
        filteredTransactions = filteredTransactions.filter(t => t.timestamp >= startDate)
      }
      if (endDate) {
        filteredTransactions = filteredTransactions.filter(t => t.timestamp <= endDate)
      }

      const sortedTransactions = filteredTransactions.sort((a, b) => b.timestamp - a.timestamp)
      return limit > 0 ? sortedTransactions.slice(0, limit) : sortedTransactions
    }
  }

  // 新增账户
  async function addAccount(nickname: string, actualName: string, type: 'asset' | 'liability' | 'income' | 'expense'): Promise<AccountInfo> {
    await initializeBeancountFile()

    // 确定账户应该添加到哪个文件
    let accountFile: string
    switch (type) {
      case 'asset':
        accountFile = BEANCOUNT_FILES.accounts.assets
        break
      case 'liability':
        accountFile = BEANCOUNT_FILES.accounts.liabilities
        break
      case 'income':
        accountFile = BEANCOUNT_FILES.accounts.income
        break
      case 'expense':
        accountFile = BEANCOUNT_FILES.accounts.expenses
        break
    }

    // 读取账户文件内容
    const accountContent = await fs.readFile(accountFile, 'utf-8')
    const today = new Date().toISOString().split('T')[0]

    // 确定账户类型的部分
    let accountSection: string
    switch (type) {
      case 'asset':
        accountSection = '; 资产账户'
        break
      case 'liability':
        accountSection = '; 负债账户'
        break
      case 'income':
        accountSection = '; 收入账户'
        break
      case 'expense':
        accountSection = '; 支出账户'
        break
    }

    // 添加新账户到文件末尾
    const newAccountLine = `\n${today} open ${actualName} CNY\n${today} note ${actualName} "${nickname}"`
    const newContent = accountContent + newAccountLine

    await fs.writeFile(accountFile, newContent, 'utf-8')

    return { nickname, actualName, type }
  }

  // 关闭账户（Beancount的close语法）
  async function deleteAccount(nickname: string): Promise<void> {
    await initializeBeancountFile()

    // 获取所有账户信息
    const accounts = await getAccounts()
    const accountInfo = accounts.find(acc => acc.nickname === nickname)

    if (!accountInfo) {
      throw new Error('账户不存在')
    }

    const { actualName, type } = accountInfo

    // 检查该账户是否有交易记录 - 检查postings数组中是否包含该账户的actualName
    const transactions = await readTransactions()
    const hasTransactions = transactions.some(t =>
      t.postings.some(posting => posting.account === actualName)
    )

    if (hasTransactions) {
      throw new Error('该账户有交易记录，无法删除')
    }

    // 确定账户所在文件
    let accountFile: string
    switch (type) {
      case 'asset':
        accountFile = BEANCOUNT_FILES.accounts.assets
        break
      case 'liability':
        accountFile = BEANCOUNT_FILES.accounts.liabilities
        break
      case 'income':
        accountFile = BEANCOUNT_FILES.accounts.income
        break
      case 'expense':
        accountFile = BEANCOUNT_FILES.accounts.expenses
        break
    }

    // 读取账户文件内容
    const accountContent = await fs.readFile(accountFile, 'utf-8')

    // 添加close记录到文件末尾
    const today = new Date().toISOString().split('T')[0]
    const closeLine = `\n${today} close ${actualName}`
    const newContent = accountContent + closeLine

    await fs.writeFile(accountFile, newContent, 'utf-8')
  }

  // 添加交易记录
  async function addTransaction(transactionData: Omit<Transaction, 'id' | 'timestamp'>): Promise<Transaction> {
    await initializeBeancountFile()
    initializeDatabase()

    // 多账户交易验证：至少需要两个账户行
    if (transactionData.postings.length < 2) {
      throw new Error('交易至少需要两个账户行')
    }

    // 验证总金额是否平衡（收入和支出必须相等）
    const totalAmount = transactionData.postings.reduce((sum, posting) => sum + posting.amount, 0)
    if (Math.abs(totalAmount) > 0.001) {  // 允许微小的浮点数误差
      throw new Error(`交易金额不平衡，总差额为: ${totalAmount.toFixed(2)}`)
    }

    // 验证是否有重复账户
    const accountSet = new Set(transactionData.postings.map(p => p.account))
    if (accountSet.size !== transactionData.postings.length) {
      throw new Error('交易中不能有重复账户')
    }

    const newTransaction: Transaction = {
      ...transactionData,
      id: Date.now().toString() + Math.random().toString(36).substr(2, 5),
      timestamp: Date.now(),
      flag: transactionData.flag || '*',  // 默认标志为"*"
      narration: transactionData.narration || '',
      tags: transactionData.tags || [],
      links: transactionData.links || []
    }

    try {
      // 使用SQLite插入新交易
      const transactionStmt = db.prepare(`
        INSERT INTO transactions (id, timestamp, flag, date, payee, narration, tags, links, type)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
      `)
      
      // 插入交易记录
      transactionStmt.run(
        newTransaction.id,
        newTransaction.timestamp,
        newTransaction.flag,
        newTransaction.date,
        newTransaction.payee,
        newTransaction.narration,
        newTransaction.tags ? JSON.stringify(newTransaction.tags) : null,
        newTransaction.links ? JSON.stringify(newTransaction.links) : null,
        newTransaction.type
      )
      
      // 插入分账单记录
      const postStmt = db.prepare(`
        INSERT INTO posts (transaction_id, account, amount)
        VALUES (?, ?, ?)
      `)
      
      for (const posting of newTransaction.postings) {
        postStmt.run(
          newTransaction.id,
          posting.account,
          posting.amount
        )
      }
      
      // 更新JSON文件作为备份
      const transactions = await readTransactions()
      transactions.push(newTransaction)
      await fs.writeFile(TRANSACTIONS_FILE, JSON.stringify(transactions, null, 2), 'utf-8')
      
      return newTransaction
    } catch (error) {
      console.error('向数据库添加交易记录失败:', error)
      // 如果数据库操作失败，回退到原始方法
      const transactions = await readTransactions()
      transactions.push(newTransaction)
      await writeTransactions(transactions)
      return newTransaction
    }
  }

  // 删除交易记录
  async function deleteTransaction(id: string): Promise<void> {
    await initializeBeancountFile()
    initializeDatabase()

    try {
      // 使用SQLite删除特定id的交易
      const stmt = db.prepare('DELETE FROM transactions WHERE id = ?')
      stmt.run(id)

      // 更新JSON文件作为备份
      const updatedTransactions = await getTransactions()
      await fs.writeFile(TRANSACTIONS_FILE, JSON.stringify(updatedTransactions, null, 2), 'utf-8')
    } catch (error) {
      console.error('从数据库删除交易记录失败:', error)
      // 如果数据库操作失败，回退到原始方法
      const transactions = await readTransactions()
      const updatedTransactions = transactions.filter(transaction => transaction.id !== id)
      await writeTransactions(updatedTransactions)
    }
  }

  // 获取余额信息
  async function getBalance(): Promise<Balance> {
    await initializeBeancountFile()
    const accounts = await getAccounts()

    // 分别计算总资产和总负债
    let totalAssets = 0
    let totalLiabilities = 0
    let totalIncome = 0
    let totalExpense = 0

    for (const account of accounts) {
      // 根据账户类型进行分类统计
      if (account.type === 'asset') {
        // 资产账户余额直接计入总资产
        totalAssets += account.balance
      } else if (account.type === 'liability') {
        // 负债账户余额直接计入总负债（负债为负数）
        totalLiabilities += account.balance
      } else if (account.type === 'income') {
        // 收入账户余额计入总收入
        totalIncome += Math.abs(account.balance)
      } else if (account.type === 'expense') {
        // 支出账户余额计入总支出
        totalExpense += Math.abs(account.balance)
      }
    }

    // 计算总余额：总资产 + 总负债 + 总收入 - 总支出
    // 注意：负债账户余额本身是负数，所以是直接相加
    const balance = totalAssets + totalLiabilities

    return {
      balance,
      totalIncome,
      totalExpense
    }
  }

  // 获取月度收支数据
  async function getMonthlyStats(): Promise<Array<{ month: string; income: number; expense: number }>> {
    await initializeBeancountFile()
    const transactions = await readTransactions()

    // 按月份分组统计收支数据
    const monthlyStats = new Map<string, { income: number; expense: number }>()

    // 初始化12个月的数据
    for (let month = 1; month <= 12; month++) {
      const monthKey = month.toString().padStart(2, '0')
      monthlyStats.set(monthKey, { income: 0, expense: 0 })
    }

    // 统计每个交易的收支
    for (const transaction of transactions) {
      const [year, month] = transaction.date.split('-')
      
      // 只处理当前年份的数据
      const currentYear = new Date().getFullYear().toString()
      if (year !== currentYear) continue

      let stats = monthlyStats.get(month)
      if (!stats) {
        stats = { income: 0, expense: 0 }
        monthlyStats.set(month, stats)
      }

      if (transaction.type === 'income') {
        stats.income += Math.abs(transaction.postings[0].amount)
      } else if (transaction.type === 'expense') {
        stats.expense += Math.abs(transaction.postings[0].amount)
      }
    }

    // 转换为数组格式并排序
    return Array.from(monthlyStats.entries())
      .sort((a, b) => a[0].localeCompare(b[0]))
      .map(([month, data]) => ({
        month,
        income: data.income,
        expense: data.expense
      }))
  }

  // 获取特定账户的交易记录 - 支持通过昵称查询
  async function getAccountTransactions(account: string, limit = 0): Promise<Transaction[]> {
    await initializeBeancountFile()
    const accounts = await getAccounts()
    initializeDatabase()

    // 解码账户名（处理浏览器编码的中文参数）
    const decodedAccount = decodeURIComponent(account)

    // 创建账户映射，支持通过昵称或实际账户名称查询
    const nicknameToActual = new Map<string, string>()
    for (const acc of accounts) {
      nicknameToActual.set(acc.nickname, acc.actualName)
    }

    // 获取实际要匹配的账户名称（actualName）
    const targetActualName = nicknameToActual.get(decodedAccount) || decodedAccount

    try {
      // 首先通过posts表查询包含该账户的交易ID
      const stmt = db.prepare(`
        SELECT transaction_id FROM posts WHERE account = ?
        ORDER BY (SELECT timestamp FROM transactions WHERE id = transaction_id) DESC
        ${limit > 0 ? 'LIMIT ?' : ''}
      `)
      const rows = limit > 0 ? stmt.all(targetActualName, limit) : stmt.all(targetActualName)
      
      if (rows.length === 0) {
        return []
      }
      
      // 提取所有交易ID
      const transactionIds = rows.map(row => row.transaction_id)
      const placeholders = transactionIds.map(() => '?').join(',')
      
      // 查询完整的交易记录
      const transactionsStmt = db.prepare(`
        SELECT * FROM transactions WHERE id IN (${placeholders}) ORDER BY timestamp DESC
      `)
      const transactions = transactionsStmt.all(...transactionIds)
      
      // 获取这些交易对应的所有分账单
      const postsStmt = db.prepare(`
        SELECT * FROM posts WHERE transaction_id IN (${placeholders})
      `)
      const posts = postsStmt.all(...transactionIds)
      
      // 将分账单按交易ID分组
      const postsByTransactionId = new Map<string, Posting[]>()
      for (const post of posts) {
        if (!postsByTransactionId.has(post.transaction_id)) {
          postsByTransactionId.set(post.transaction_id, [])
        }
        postsByTransactionId.get(post.transaction_id)?.push({
          account: post.account,
          amount: post.amount
        })
      }
      
      // 组合交易记录和分账单
      return transactions.map(row => ({
        ...row,
        tags: row.tags ? JSON.parse(row.tags) : [],
        links: row.links ? JSON.parse(row.links) : [],
        postings: postsByTransactionId.get(row.id) || []
      }))
    } catch (error) {
      console.error('从数据库查询账户交易记录失败:', error)
      // 如果数据库查询失败，回退到原始方法
      const transactions = await readTransactions()
      
      // 筛选包含特定账户的交易记录
      return transactions.filter(transaction =>
        transaction.postings.some(posting => posting.account === targetActualName)
      ).sort((a, b) => b.timestamp - a.timestamp)
      .slice(0, limit || undefined)
    }
  }

  // 获取特定账户的余额 - 支持多账户交易
  async function getAccountBalance(account: string): Promise<number> {
    await initializeBeancountFile()
    const accounts = await getAccounts()
    initializeDatabase()

    // 解码账户名（处理浏览器编码的中文参数）
    const decodedAccount = decodeURIComponent(account)

    // 创建账户映射，支持通过昵称或实际账户名称查询
    const nicknameToActual = new Map<string, string>()
    for (const acc of accounts) {
      nicknameToActual.set(acc.nickname, acc.actualName)
    }

    // 获取实际要匹配的账户名称（actualName）
    const targetActualName = nicknameToActual.get(decodedAccount) || decodedAccount

    try {
      // 使用SQLite直接查询posts表计算账户余额
      const stmt = db.prepare('SELECT SUM(amount) as balance FROM posts WHERE account = ?')
      const result = stmt.get(targetActualName)

      // 如果没有记录，返回0
      return result.balance || 0
    } catch (error) {
      console.error('从数据库计算账户余额失败:', error)
      // 如果数据库查询失败，回退到原始方法
      const transactions = await readTransactions()
      const balance = transactions
        .reduce((sum, transaction) => {
          const accountPosting = transaction.postings.find(posting =>
            posting.account === targetActualName
          )

          if (accountPosting) {
            return sum + accountPosting.amount
          }

          return sum
        }, 0)

      return balance
    }
  }

  // 从Beancount文件解析交易记录
  // 尝试使用lezer-beancount来解析Beancount文件
  async function parseBeancountTransactions(): Promise<Transaction[]> {
    await initializeBeancountFile()
    const accounts = await getAccounts()
    const transactions: Transaction[] = []

    // 创建账户映射（实际账户名 -> 昵称）
    const accountMap = new Map(accounts.map(acc => [acc.actualName, acc.nickname]))
    // 创建昵称映射（昵称 -> 实际账户名）用于反向查找
    const nicknameToActual = new Map(accounts.map(acc => [acc.nickname, acc.actualName]))

    // 读取所有日期目录
    const dateDir = path.join(DATA_DIR, 'date')
    const yearDirs = await fs.readdir(dateDir)

    // 遍历所有年份目录
    for (const year of yearDirs.filter(dir => !isNaN(Number(dir)))) {
      const yearPath = path.join(dateDir, year)
      const monthFiles = await fs.readdir(yearPath)

      // 遍历所有月份文件
      for (const monthFile of monthFiles.filter(file => file.endsWith('.bean'))) {
        const monthFilePath = path.join(yearPath, monthFile)
        const fileContent = await fs.readFile(monthFilePath, 'utf-8')

        try {
          // 使用正则表达式匹配Beancount的各种指令（交易、pad、balance）
          // 首先匹配普通交易记录
          const transactionRegex = /(\d{4}-\d{2}-\d{2})\s+(\*|!)\s+("[^"]*"|\S*)\s+("[^"]*"|\S*)\s*([#^][^\n]*|)\n((?:\s+\S+\s+[-\d.]+\s+\S+\n)+)/g
          let match

          while ((match = transactionRegex.exec(fileContent)) !== null) {
            const [fullMatch, date, flag, payeeRaw, narrationRaw, tagsLinks, postingsRaw] = match

            // 解析收款人/付款人
            const payee = payeeRaw.match(/"([^"]*)"/)?.[1] || (payeeRaw !== '-' ? payeeRaw : undefined)

            // 解析交易描述
            const narration = narrationRaw.match(/"([^"]*)"/)?.[1] || (narrationRaw !== '-' ? narrationRaw : '')

            // 解析标签和链接
            const tags: string[] = []
            const links: string[] = []
            if (tagsLinks) {
              const tagMatches = tagsLinks.match(/#([\w\u4e00-\u9fa5]+)/g) || []
              const linkMatches = tagsLinks.match(/\^([\w\u4e00-\u9fa5]+)/g) || []

              tags.push(...tagMatches.map(tag => tag.slice(1)))
              links.push(...linkMatches.map(link => link.slice(1)))
            }

            // 解析账户行（postings）
            const postings: Posting[] = []
            const postingRegex = /\s+(\S+)\s+([-\d.]+)\s+(\S+)/g
            let postingMatch

            while ((postingMatch = postingRegex.exec(postingsRaw)) !== null) {
              const [, actualAccount, amountStr, currency] = postingMatch
              
              postings.push({
                account: actualAccount, // 存储actualName而不是nickname，确保唯一性
                amount: parseFloat(amountStr),
                currency
              })
            }

            // 确定交易类型
            let type: 'income' | 'expense' | 'transfer' = 'expense'
            const assetLiabilityAccounts = new Set(accounts.filter(a => a.type === 'asset' || a.type === 'liability').map(a => a.actualName))
            
            // 检查每个posting对应的实际账户
            const hasAssetLiability = postings.some(p => {
              return assetLiabilityAccounts.has(p.account)
            })
            
            const hasIncome = postings.some(p => {
              return p.amount > 0 && p.account.startsWith('Income')
            })
            
            const hasExpense = postings.some(p => {
              return p.amount < 0 && p.account.startsWith('Expenses')
            })

            if (hasIncome && hasExpense) {
              type = 'transfer'
            } else if (hasIncome) {
              type = 'income'
            } else if (hasExpense) {
              type = 'expense'
            }

            // 创建交易对象 - 使用稳定的ID生成方式（基于交易内容，不包含随机数）
            // 生成一个基于交易内容的稳定ID
            const generateStableId = (content: string) => {
              // 使用简单的哈希算法生成稳定的ID
              let hash = 0
              for (let i = 0; i < content.length; i++) {
                const char = content.charCodeAt(i)
                hash = ((hash << 5) - hash) + char
                hash = hash & hash // Convert to 32bit integer
              }
              return Math.abs(hash).toString(36)
            }
            
            // 构建用于生成ID的内容字符串
            const idContent = `${date}${flag}${payee || ''}${narration}${tags.join(',')}${links.join(',')}${postingsRaw}`
            
            const transaction: Transaction = {
              id: `bean_${date}_${generateStableId(idContent)}`,
              date,
              flag,
              payee,
              narration,
              tags,
              links,
              type,
              postings,
              timestamp: new Date(date).getTime()
            }

            transactions.push(transaction)
          }

          // 匹配并解析pad指令
          const padRegex = /(\d{4}-\d{2}-\d{2})\s+pad\s+(\S+)\s+(\S+)/g
          let padMatch

          while ((padMatch = padRegex.exec(fileContent)) !== null) {
            const [, date, targetAccount, sourceAccount] = padMatch

            // Pad指令本身不直接生成交易，但会在后续处理中由beancount自动生成
            // 我们创建一个特殊的交易记录来表示pad指令
            const padTransaction: Transaction = {
              id: `pad_${date}_${targetAccount.replace(/:/g, '_')}`,
              date,
              flag: '*',
              payee: undefined,
              narration: `Pad transaction for ${targetAccount}`,
              tags: ['pad'],
              links: [],
              type: 'transfer',
              postings: [
                {
                  account: targetAccount, // 存储actualName而不是nickname，确保唯一性
                  amount: 0, // 金额将在后续处理中计算
                  currency: 'CNY' // 假设默认货币为CNY
                },
                {
                  account: sourceAccount, // 存储actualName而不是nickname，确保唯一性
                  amount: 0, // 金额将在后续处理中计算（与targetAccount相反）
                  currency: 'CNY'
                }
              ],
              timestamp: new Date(date).getTime()
            }

            transactions.push(padTransaction)
          }

          // 匹配并解析balance指令
          const balanceRegex = /(\d{4}-\d{2}-\d{2})\s+balance\s+(\S+)\s+([-\d.]+)\s+(\S+)/g
          let balanceMatch

          while ((balanceMatch = balanceRegex.exec(fileContent)) !== null) {
            const [, date, targetAccount, amountStr, currency] = balanceMatch
            const amount = parseFloat(amountStr)

            // Balance指令表示账户在特定日期应有的余额
            // 我们创建一个特殊的交易记录来表示balance指令
            const balanceTransaction: Transaction = {
              id: `balance_${date}_${targetAccount.replace(/:/g, '_')}_${amount}_${currency}`,
              date,
              flag: '*',
              payee: undefined,
              narration: `Balance check for ${targetAccount}`,
              tags: ['balance'],
              links: [],
              type: 'transfer',
              postings: [
                {
                  account: targetAccount, // 存储actualName而不是nickname，确保唯一性
                  amount: amount,
                  currency: currency
                }
              ],
              timestamp: new Date(date).getTime()
            }

            transactions.push(balanceTransaction)
          }
        } catch (error) {
          console.error(`解析Beancount文件 ${monthFilePath} 失败:`, error)
          // 如果解析失败，回退到原有逻辑
          continue
        }
      }
    }

    return transactions
  }

  // 同步Beancount文件与数据库的交易记录
  async function syncTransactions(): Promise<void> {
    await initializeBeancountFile()
    initializeDatabase()

    // 读取数据库中的交易
    const dbTransactions = await getTransactions()

    // 解析Beancount文件中的交易
    const beanTransactions = await parseBeancountTransactions()

    // 创建交易ID映射（用于比较）
    const dbTransactionMap = new Map(dbTransactions.map(t => [t.id, t]))
    const beanTransactionMap = new Map(beanTransactions.map(t => [t.id, t]))

    // 需要添加到数据库的交易（Beancount有但数据库没有）
    const transactionsToAdd = beanTransactions.filter(t => !dbTransactionMap.has(t.id))

    // 需要从数据库删除的交易（数据库有但Beancount没有）
    const transactionsToDelete = dbTransactions.filter(t => !beanTransactionMap.has(t.id))

    // 如果有变更，更新数据库和JSON文件
    if (transactionsToAdd.length > 0 || transactionsToDelete.length > 0) {
      console.log(`同步交易记录: 添加 ${transactionsToAdd.length} 条, 删除 ${transactionsToDelete.length} 条`)

      try {
        // 开始数据库事务
        db.exec('BEGIN TRANSACTION')

        // 删除不需要的交易
        if (transactionsToDelete.length > 0) {
          // 准备删除交易的语句
          const deleteTransactionStmt = db.prepare('DELETE FROM transactions WHERE id = ?')
          // 准备删除对应分账单的语句
          const deletePostsStmt = db.prepare('DELETE FROM posts WHERE transaction_id = ?')
          
          for (const transaction of transactionsToDelete) {
            // 先删除对应的分账单
            deletePostsStmt.run(transaction.id)
            // 再删除交易记录
            deleteTransactionStmt.run(transaction.id)
          }
        }

        // 添加新的交易
        if (transactionsToAdd.length > 0) {
          // 准备交易插入语句
          const transactionStmt = db.prepare(`
            INSERT INTO transactions (id, timestamp, flag, date, narration, tags, links, type, payee)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
          `)
          
          // 准备分账单插入语句
          const postStmt = db.prepare(`
            INSERT INTO posts (transaction_id, account, amount, currency)
            VALUES (?, ?, ?, ?)
          `)
          
          for (const transaction of transactionsToAdd) {
            // 插入交易记录到transactions表
            transactionStmt.run(
              transaction.id,
              transaction.timestamp,
              transaction.flag,
              transaction.date,
              transaction.narration,
              transaction.tags ? JSON.stringify(transaction.tags) : null,
              transaction.links ? JSON.stringify(transaction.links) : null,
              transaction.type,
              transaction.payee
            )
            
            // 插入分账单到posts表
            for (const posting of transaction.postings) {
              postStmt.run(
                transaction.id,
                posting.account,
                posting.amount,
                posting.currency
              )
            }
          }
        }

        // 提交事务
        db.exec('COMMIT')

        // 更新JSON文件作为备份
        const updatedTransactions = await getTransactions()
        await fs.writeFile(TRANSACTIONS_FILE, JSON.stringify(updatedTransactions, null, 2), 'utf-8')

        console.log('交易记录同步完成！')
      } catch (error) {
        // 回滚事务
        db.exec('ROLLBACK')
        console.error('同步过程中数据库操作失败:', error)

        // 回退到原始JSON操作方法
        const jsonTransactions = await readTransactions()
        const updatedTransactions = [
          ...jsonTransactions.filter(t => !transactionsToDelete.includes(t)),
          ...transactionsToAdd
        ]
        await writeTransactions(updatedTransactions)
      }
    } else {
      console.log('交易记录已经同步，无需变更')
    }
  }

  // 获取所有账户
  async function getAccounts(): Promise<Array<{ nickname: string; actualName: string; type: 'asset' | 'liability' | 'income' | 'expense'; balance: number; transactionCount: number }>> {
    await initializeBeancountFile()
    const transactions = await readTransactions()

    const accounts: Array<{ nickname: string; actualName: string; type: 'asset' | 'liability' | 'income' | 'expense'; balance: number; transactionCount: number }> = []

    // 读取所有账户文件
    const accountFiles = [
      { path: BEANCOUNT_FILES.accounts.assets, type: 'asset' as const },
      { path: BEANCOUNT_FILES.accounts.liabilities, type: 'liability' as const },
      { path: BEANCOUNT_FILES.accounts.income, type: 'income' as const },
      { path: BEANCOUNT_FILES.accounts.expenses, type: 'expense' as const }
    ]

    for (const file of accountFiles) {
      try {
        const fileContent = await fs.readFile(file.path, 'utf-8')

        // 解析文件中的账户定义 - 使用更精确的正则表达式，确保账户名称不会匹配到空字符串
        const accountRegex = /(\d{4}-\d{2}-\d{2}) open (\S+) ?(.*?)?$/gm
        let match

        while ((match = accountRegex.exec(fileContent)) !== null) {
          const [, openDate, actualName, currency = 'CNY'] = match

          // 检查账户是否被关闭 - 查找包含"close"和账户名称的行
          const closeRegex = new RegExp(`\\d{4}-\\d{2}-\\d{2} close ${actualName}`)
          if (closeRegex.test(fileContent)) {
            // 账户已关闭，跳过
            continue
          }

          // 从文件中的note读取账户昵称
          let nickname: string
          // 使用更简单的方式来匹配note语句 - 先按行分割文件内容，然后查找包含"note"和账户名称的行
          const lines = fileContent.split('\n')
          // 查找包含"note"和账户名称的行
          const noteLine = lines.find(line => line.includes('note') && line.includes(actualName))

          if (noteLine) {
            // 从noteLine中提取昵称，格式为: YYYY-MM-DD note Account.Name "昵称"
            const noteMatch = noteLine.match(/"(.*?)"/)
            if (noteMatch) {
              nickname = noteMatch[1]
            } else {
              // 如果无法提取昵称，使用账户名称的最后一部分作为默认昵称
              nickname = actualName.split(':').pop() || actualName
            }
          } else {
            // 如果没有找到note，使用账户名称的最后一部分作为默认昵称
            nickname = actualName.split(':').pop() || actualName
          }

          // 计算账户余额和交易笔数 - 支持多账户交易结构
          const accountTransactions = transactions
            .filter(t => t.postings.some(posting => posting.account === actualName))
            .sort((a, b) => a.timestamp - b.timestamp) // 按时间顺序排序

          // 计算账户余额：根据Beancount规则，balance指令应该作为余额检查点
          let balance = 0
          let balanceCheckpoint = false
          let checkpointAmount = 0

          for (const t of accountTransactions) {
            const accountPosting = t.postings.find(p => p.account === actualName)
            if (accountPosting) {
              // 检查是否是balance指令
              if (t.tags && t.tags.includes('balance')) {
                // 遇到balance指令，设置检查点
                balanceCheckpoint = true
                checkpointAmount = accountPosting.amount
                // 根据Beancount规则，balance指令应该覆盖当前余额
                balance = checkpointAmount
              } else if (balanceCheckpoint) {
                // 在balance检查点之后，继续累加交易金额
                balance += accountPosting.amount
              } else {
                // 在第一个balance检查点之前，正常累加交易金额
                balance += accountPosting.amount
              }
            }
          }

          accounts.push({
            nickname,
            actualName,
            type: file.type,
            balance,
            transactionCount: accountTransactions.length
          })
        }
      } catch (error) {
        console.error(`Error reading account file ${file.path}:`, error)
        // 继续处理其他文件
      }
    }

    return accounts
  }


  // 设置账户余额
  async function setAccountBalance(account: string, date: string, balance: number, padAccount: string): Promise<boolean> {
    await initializeBeancountFile()
    const accounts = await getAccounts()
    initializeDatabase()

    // 解码账户名
    const decodedAccount = decodeURIComponent(account)
    const decodedPadAccount = decodeURIComponent(padAccount)

    // 创建账户映射
    const accountMap = new Map<string, { nickname: string; actualName: string }>()
    for (const acc of accounts) {
      accountMap.set(acc.nickname, { nickname: acc.nickname, actualName: acc.actualName })
      accountMap.set(acc.actualName, { nickname: acc.nickname, actualName: acc.actualName })
    }

    // 获取目标账户和pad账户的实际名称
    const targetAccount = accountMap.get(decodedAccount)
    const targetPadAccount = accountMap.get(decodedPadAccount)

    if (!targetAccount || !targetPadAccount) {
      console.error('未找到指定的账户')
      return false
    }

    try {
      // 计算当前账户的实际余额
      const actualBalance = await getAccountBalance(targetAccount.actualName)
      const difference = balance - actualBalance

      // 生成唯一ID
      const balanceId = `balance_${date}_${targetAccount.actualName.replace(/:/g, '_')}_${Math.random().toString(36).substr(2, 5)}`
      const padId = `pad_${date}_${targetAccount.actualName.replace(/:/g, '_')}_${Math.random().toString(36).substr(2, 5)}`

      // 准备交易插入语句
      const transactionStmt = db.prepare(`
        INSERT INTO transactions (id, timestamp, flag, date, narration, tags, links, type, payee)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
      `)
      
      // 准备分账单插入语句
      const postStmt = db.prepare(`
        INSERT INTO posts (transaction_id, account, amount, currency)
        VALUES (?, ?, ?, ?)
      `)

      // 开始事务
      db.exec('BEGIN TRANSACTION')

      // 创建balance指令交易
      transactionStmt.run(
        balanceId,
        new Date(date).getTime(),
        '*',
        date,
        `Balance check for ${targetAccount.nickname}`,
        JSON.stringify(['balance']),
        null,
        'transfer',
        null
      )

      postStmt.run(
        balanceId,
        targetAccount.actualName,
        balance,
        'CNY'
      )

      // 如果有差额，创建pad指令交易
      if (difference !== 0) {
        transactionStmt.run(
          padId,
          new Date(date).getTime(),
          '*',
          date,
          `Pad transaction for ${targetAccount.nickname}`,
          JSON.stringify(['pad']),
          null,
          'transfer',
          null
        )

        // Pad交易的两个分账单：目标账户和pad账户
        postStmt.run(
          padId,
          targetAccount.actualName,
          difference,
          'CNY'
        )

        postStmt.run(
          padId,
          targetPadAccount.actualName,
          -difference,
          'CNY'
        )
      }

      // 提交事务
      db.exec('COMMIT')

      // 更新Beancount文件
      const updatedTransactions = await getTransactions()
      await updateBeancountFile(updatedTransactions)

      return true
    } catch (error) {
      db.exec('ROLLBACK')
      console.error('设置账户余额失败:', error)
      return false
    }
  }

  return {
    getTransactions,
    addTransaction,
    deleteTransaction,
    getBalance,
    getAccountTransactions,
    getAccountBalance,
    getAccounts,
    addAccount,
    deleteAccount,
    syncTransactions,
    setAccountBalance,
    getMonthlyStats
  }
}