# MoneyMint Backend

MoneyMint 是一个基于 Beancount 的个人财务管理系统的后端模块。它提供了 RESTful API 接口，用于管理财务账本、记账条目和账户信息。

## 功能特性

- **用户认证**：支持用户登录、注册和权限管理
- **账本管理**：加载、查询和更新 Beancount 账本文件
- **记账条目**：支持增删改查记账条目，包括交易、转账、余额断言等
- **账户管理**：获取账户列表和余额信息
- **实时通知**：通过 SSE (Server-Sent Events) 实现实时账本更新通知
- **缓存机制**：账本文件缓存，提高性能

## 技术栈

- **Python 3.8+**：主要开发语言
- **Flask**：Web 框架
- **Flask-JWT-Extended**：JWT 认证支持
- **Flask-CORS**：跨域资源共享支持
- **Flask-Bcrypt**：密码加密
- **Beancount**：会计引擎
- **Dotenv**：环境变量管理

## 项目结构

```
app/
├── __init__.py          # 应用初始化
├── auth/                # 用户认证模块
│   ├── __init__.py
│   └── routes.py        # 认证路由
├── ledger/              # 账本管理模块
│   ├── __init__.py
│   └── routes.py        # 账本路由
├── entries/             # 记账条目模块
│   ├── __init__.py
│   └── routes.py        # 条目路由
├── accounts/            # 账户管理模块
│   ├── __init__.py
│   └── routes.py        # 账户路由
├── events/              # SSE 事件模块
│   ├── __init__.py
│   └── routes.py        # 事件路由
└── utils/               # 工具函数模块
    ├── __init__.py
    └── ledger_utils.py  # 账本相关工具函数
.env                     # 环境变量配置
requirements.txt         # 依赖列表
run.py                   # 应用入口
setup.py                 # 包安装配置
```

## 安装和运行

### 1. 安装依赖

```bash
pip install -r requirements.txt
```

### 2. 配置环境变量

复制 `.env.example` 文件为 `.env` 并修改配置：

```bash
cp .env.example .env
```

主要配置项：

- `LEDGER_FILE`：账本文件路径
- `JWT_SECRET_KEY`：JWT 密钥
- `ADMIN_USERNAME`：管理员用户名
- `ADMIN_PASSWORD`：管理员密码
- `FLASK_ENV`：Flask 运行环境

### 3. 运行应用

```bash
python run.py
```

应用将在 `http://localhost:5000` 启动。

## API 接口

### 认证接口

- `POST /api/auth/login`：用户登录
- `GET /api/auth/register/status`：检查注册状态
- `POST /api/auth/register`：用户注册

### 账本接口

- `GET /api/ledger`：获取账本基本信息
- `POST /api/ledger/query`：执行 Beancount 查询

### 记账条目接口

- `GET /api/entries`：获取记账条目列表
- `POST /api/entries`：添加记账条目
- `PUT /api/entries/<entry_id>`：更新记账条目
- `DELETE /api/entries/<entry_id>`：删除记账条目

### 账户接口

- `GET /api/accounts`：获取账户列表
- `GET /api/accounts/balances`：获取账户余额

### 事件接口

- `GET /api/events`：SSE 实时通知

## 开发

### 代码风格

遵循 PEP 8 代码风格规范。

### 测试

运行测试：

```bash
python -m pytest
```

### 构建包

```bash
python setup.py sdist bdist_wheel
```

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！