# MoneyMint 记账系统 中文|[ENGLISH](README.md) [![docker image size](https://img.shields.io/docker/image-size/quewen08/money-mint/latest?label=docker-image)](https://hub.docker.com/repository/docker/quewen08/money-mint/general) ![docker pulls](https://img.shields.io/docker/pulls/quewen08/money-mint)

一个基于 Beancount 和 Nuxt.js 构建的现代化的个人记账系统，提供强大的后端服务和移动端友好的前端界面，帮助用户轻松管理个人财务。

## 🚀 项目简介

MoneyMint 是一款开源的个人财务管理系统，旨在提供简洁直观的界面和强大的记账功能。系统采用前后端分离架构，后端基于 Python 和 Flask 构建，使用 Beancount 作为核心记账引擎，前端使用 Nuxt.js 和 Vue 3 开发，提供现代化的用户体验。

### 核心优势

- **基于 Beancount**: 利用专业的双记账系统确保财务数据的准确性
- **现代化界面**: 响应式设计，支持桌面和移动设备
- **实时同步**: SSE 技术实现数据实时更新
- **灵活部署**: 支持本地部署和 Docker 容器化部署
- **数据安全**: 采用 JWT 认证机制，数据存储在本地纯文本文件中

## 📋 功能特性

### 核心功能

- **账本管理**: 支持多账本切换，数据以纯文本格式存储，安全可靠
- **收支记录**: 完整的双记账系统，支持收入、支出、转账等多种交易类型
- **分类统计**: 按类别、时间、账户等维度统计财务数据
- **图表分析**: 直观的图表展示财务状况和趋势
- **账户管理**: 支持多账户管理，包括资产、负债、收入、支出等各类账户

### 增强功能

- **交易记录复制**: 快速创建相似交易，提高记账效率
- **智能账户筛选**: 添加交易时提供账户搜索筛选功能，方便多账户选择
- **账户交易详情**: 在账户详情页面查看该账户的所有交易记录，支持日期筛选和分页
- **实时数据更新**: 使用 SSE 技术实现账本数据的实时推送更新
- **用户认证**: 完整的用户登录、注册和权限管理系统

## 🛠️ 技术栈

### 后端

- **Python 3.11**: 主要开发语言
- **Flask**: Web 应用框架
- **Beancount**: 核心记账引擎
- **Fava**: Beancount 的 Web 界面（可选集成）
- **Flask-JWT-Extended**: JWT 认证
- **Flask-CORS**: 跨域资源共享
- **Flask-Bcrypt**: 密码加密

### 前端

- **Nuxt.js 3**: Vue.js 框架的全栈解决方案
- **Vue 3**: 渐进式 JavaScript 框架
- **Tailwind CSS**: 实用优先的 CSS 框架
- **TypeScript**: 类型安全的 JavaScript 超集
- **Pinia**: Vue 3 的状态管理库

### 数据存储

- **Beancount 纯文本账本**: 人类可读的财务数据格式
- **本地文件系统**: 数据存储在本地，确保隐私安全

## 📁 项目结构

```
MoneyMint/
├── backend/                    # Python 后端服务
│   ├── app/                   # 应用代码
│   │   ├── __init__.py        # 应用初始化
│   │   ├── auth/              # 认证模块
│   │   ├── ledger/            # 账本管理模块
│   │   ├── entries/           # 记账条目模块
│   │   ├── accounts/          # 账户管理模块
│   │   ├── events/            # SSE 事件模块
│   │   └── utils/             # 工具函数
│   ├── data/                  # 账本数据目录
│   │   └── main.bean          # 默认账本文件
│   ├── .env                   # 环境变量配置
│   ├── requirements.txt       # Python 依赖
│   ├── run.py                 # 应用入口
│   ├── setup.py               # 包安装配置
│   └── README.md              # 后端模块文档
├── frontend/                  # Nuxt 前端项目
│   ├── assets/                # 静态资源
│   ├── components/            # Vue 组件
│   ├── composables/           # 可组合函数
│   │   └── useApi.ts          # API 调用封装
│   ├── pages/                 # 页面组件
│   │   ├── index.vue          # 首页/仪表盘
│   │   ├── accounts.vue       # 账户管理
│   │   ├── entries.vue        # 交易记录
│   │   └── login.vue          # 登录页面
│   ├── plugins/               # Nuxt 插件
│   ├── public/                # 公共资源
│   ├── nuxt.config.ts         # Nuxt 配置
│   ├── package.json           # Node.js 依赖
│   └── tailwind.config.js     # Tailwind CSS 配置
├── Dockerfile                 # Docker 构建文件
├── docker-compose.yml         # Docker Compose 配置
├── README.md                  # 英文版文档
└── README-zh.md               # 中文版文档
```

## 🚀 快速开始

### 本地部署

#### 1. 后端设置

```bash
# 进入后端目录
cd backend

# 创建虚拟环境
python -m venv venv

# 激活虚拟环境
# Windows
env\Scripts\activate
# Linux/macOS
source venv/bin/activate

# 安装依赖
pip install -r requirements.txt

# 运行后端服务
python app/main.py
```

后端服务将在 `http://localhost:5000` 启动。

#### 2. 前端设置

```bash
# 进入前端目录
cd frontend

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

前端开发服务器将在 `http://localhost:3000` 启动。

### Docker 部署

使用 Docker Compose 可以快速部署整个应用：

```bash
# 拉取最新镜像
docker pull quewen08/money-mint:latest

# 复制 docker-compose.yml 文件
cp docker-compose.yml .

# 启动容器
docker-compose up -d

# 查看容器状态
docker-compose ps
```

应用将在 `http://localhost:3000` 可用。

## 🔧 配置选项

### 后端配置

可以通过环境变量或 `.env` 文件配置后端：

| 配置项 | 描述 | 默认值 |
|--------|------|--------|
| `LEDGER_FILE` | 账本文件路径 | `data/main.bean` |
| `JWT_SECRET_KEY` | JWT 密钥 | `your-secret-key-change-this-in-production` |
| `ADMIN_USERNAME` | 管理员用户名 | `admin` |
| `ADMIN_PASSWORD` | 管理员密码 | `admin123` |
| `ENABLE_REGISTRATION` | 是否启用注册 | `true` |
| `PAD_EQUITY_ACCOUNT` | 权益账户 | `Equity:Opening-Balances` |

### 前端配置

前端配置文件位于 `frontend/nuxt.config.ts`，主要配置项：

| 配置项 | 描述 | 默认值 |
|--------|------|--------|
| `apiBaseUrl` | 后端 API 地址 | `http://localhost:5000/api` |
| `app.port` | 前端服务端口 | `3000` |

## 📖 使用指南

### 1. 用户认证

- 首次访问系统时，使用默认管理员账号登录：
  - 用户名：`admin`
  - 密码：`admin123`
- 登录后可以修改密码或创建新用户

### 2. 账本管理

- 系统会自动创建默认账本文件
- 可以通过后端 API 或直接编辑账本文件进行管理
- 支持多账本切换（需要手动配置）

### 3. 账户管理

- 查看所有账户及其余额
- 点击账户名称查看该账户的交易记录
- 支持按日期范围筛选和分页查看交易记录

### 4. 添加交易

- 选择交易类型（收入、支出、转账等）
- 填写交易日期、金额、描述等信息
- 选择相关账户和分类
- 使用复制功能快速创建相似交易

### 5. 数据统计

- 仪表盘显示总收入、总支出和净收入
- 分类统计展示各类支出占比
- 图表分析直观展示财务趋势

## 🤝 贡献指南

欢迎各种形式的贡献！无论是报告 bug、提交新功能建议还是直接提交代码，都非常感谢您的支持。

### 贡献流程

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

### 开发规范

- 遵循现有的代码风格
- 提交前确保代码通过所有测试
- 为新功能添加文档
- 提交清晰的 commit 信息

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](https://github.com/quewen08/MoneyMint/blob/master/LICENSE) 文件了解详情。

## 📞 联系方式

- 项目地址：[https://github.com/quewen08/MoneyMint](https://github.com/quewen08/MoneyMint)
- 问题反馈：[https://github.com/quewen08/MoneyMint/issues](https://github.com/quewen08/MoneyMint/issues)

## 📦 技术文档

- [Beancount 官方文档](https://beancount.github.io/docs/)
- [Flask 官方文档](https://flask.palletsprojects.com/)
- [Nuxt.js 官方文档](https://nuxt.com/docs/)

## 🙏 致谢

感谢所有为 MoneyMint 项目做出贡献的开发者和用户！

---

如果您觉得 MoneyMint 对您有帮助，请给我们一个 ⭐ Star 支持一下！