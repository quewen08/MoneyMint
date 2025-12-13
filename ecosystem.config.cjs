module.exports = {
  apps: [
    {
      name: 'money-mint', 
      exec_mode: 'cluster',
      instances: 'max', 
      port: '3000',
      script: './.output/server/index.mjs',
      args: 'start',
      error_file: './log/err.log', // 错误日志存放地址
      out_file: './log/out.log', // 输出日志存放地址
    }
  ]
}