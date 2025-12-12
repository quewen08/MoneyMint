# MoneyMint

基于 Beancount 的简单易用的记账应用，使用 Nuxt4 + Vue3 开发，支持 Web 和 H5 访问。

## 技术栈

- **前端**: Nuxt4 + Vue3
- **后端**: Node.js (Nuxt Server API)
- **数据存储**: JSON 文件 (支持导出 Beancount 格式)
- **样式**: 自定义 CSS (响应式设计，支持移动端)

## 功能特性

- ✅ 财务概览仪表盘
- ✅ 收入/支出记录
- ✅ 多种账户支持（现金、银行卡、支付宝、微信）
- ✅ 交易记录管理
- ✅ Beancount 格式支持（可导出标准 Beancount 文件）
- ✅ 响应式设计，支持移动端 H5

## 安装步骤

1. **安装依赖**

```bash
npm install
```

2. **启动开发服务器**

```bash
npm run dev
```

3. **访问应用**

打开浏览器访问: `http://localhost:3000`

## 使用说明

### 记录交易

1. 点击导航栏中的「记录交易」按钮
2. 填写交易信息：
   - 日期
   - 交易类型（收入/支出）
   - 金额
   - 账户类型
   - 交易描述
3. 点击「保存交易」按钮

### 查看财务概览

- 在首页查看总收入、总支出和当前余额
- 查看最近的交易记录

### 导出 Beancount 文件

生成的 Beancount 文件位于 `data/main.bean`，可以直接使用 Beancount 工具查看和处理。

## 项目结构

```
MoneyMint/
├── assets/              # 静态资源
│   └── css/            # 样式文件
├── components/         # Vue 组件
├── composables/        # 可复用的组合式函数
├── data/              # 数据存储目录
│   ├── transactions.json  # 交易记录 JSON 文件
│   └── main.bean         # Beancount 文件
├── pages/             # 页面组件
│   ├── index.vue        # 首页/仪表盘
│   └── transactions.vue # 交易记录页面
├── server/            # 服务器端代码
│   ├── api/           # API 端点
│   └── utils/         # 工具函数
├── nuxt.config.ts     # Nuxt 配置
├── package.json       # 项目依赖
└── README.md          # 项目说明
```

## API 端点

- `GET /api/transactions` - 获取交易记录
- `POST /api/transactions` - 添加新交易
- `DELETE /api/transactions/:id` - 删除交易
- `GET /api/balance` - 获取余额信息

## 配置说明

### 货币设置

默认使用人民币 (CNY)，如果需要修改货币单位，请修改 `server/utils/beancountManager.ts` 中的相关配置。

### 账户设置

默认支持以下账户类型：
- 现金
- 银行卡
- 支付宝
- 微信

如需添加更多账户类型，请修改：
1. `pages/transactions.vue` 中的账户下拉菜单
2. `server/utils/beancountManager.ts` 中的账户映射函数

## 开发说明

### 添加新页面

在 `pages/` 目录下创建新的 Vue 文件即可，Nuxt 会自动生成路由。

### 添加新 API

在 `server/api/` 目录下创建新的 API 文件，使用 RESTful 命名约定。

## 生产部署

### 方法一：直接部署

1. **构建应用**

```bash
npm run build
```

2. **启动生产服务器**

```bash
npm run preview
```

### 方法二：Docker部署

#### 1. 使用Dockerfile部署

```bash
# 构建镜像
docker build -t moneymint .

# 运行容器
docker run -p 3000:3000 -v ./data:/app/data moneymint
```

#### 2. 使用Docker Compose部署

```bash
# 启动服务
docker-compose up -d

# 停止服务
docker-compose down

# 查看日志
docker-compose logs -f
```

### 数据持久化

通过挂载 `./data` 目录到容器内部的 `/app/data` 目录，实现数据持久化存储。

## 注意事项

- 本项目不依赖外部数据库，所有数据存储在本地 JSON 文件中
- 支持导出标准 Beancount 格式文件，可以与其他 Beancount 工具兼容
- 应用采用响应式设计，支持桌面端和移动端访问

## 许可证

MIT License