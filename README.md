# FamilyLedger 家庭协作记账

跑在 NAS 个人云上的**家庭协作复式记账**应用，数据模型对标 [Beancount](https://beancount.github.io/)。
前端为 Flutter Web（PWA，浏览器访问），后端为 Go + SQLite（可平滑切换 Postgres）。

- **数据自持**：账本数据全部保存在你自己的 NAS 上，不上传第三方。
- **离线优先**：断网也能记账，联网后自动增量同步。
- **标准兼容**：可导出标准 `.beancount` 文本，通过 `bean-check` 校验。

## 特性

- **复式记账**：Beancount 五大账户类型（资产/负债/权益/收入/支出），按币种借贷平衡校验，金额定点十进制无浮点误差。
- **家庭共享**：一个家庭一个账本，`owner` / `editor` / `viewer` 三级角色权限，成员管理内置。
- **多账本**：一个账号可属于多个账本，数据完全隔离，随时切换。
- **离线多端同步**：append-only + 不可变交易 + 全局水位，增量 pull / 逐条幂等 push，天然几乎无冲突。
- **标准导出**：单文件或多文件目录（`main.bean` + `accounts/` + `date/`）两种导出，解压后可直接 `bean-check`。
- **PWA**：可安装、离线壳、IndexedDB 本地真相源。

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.26 + 标准库 + [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite)（纯 Go，免 CGO） |
| 前端 | Flutter Web 3.35+（MVVM 分层）+ sembast_web（IndexedDB） |
| 导出校验 | 项目内 Python venv + `beancount` |

## 快速开始

### 环境要求

- Go 1.26+
- Flutter 3.35+
- Python 3.x（仅用于导出校验，装进项目 `.venv`，不污染系统）

### 1. 启动后端

```bash
export PATH="$PATH:/go/bin"        # 若 shell 未识别 go
LEDGER_DB=data/ledger.db go run ./cmd/server   # 默认监听 :8080，LISTEN=:9000 可覆盖
```

首次启动自动建库并执行迁移。健康检查：

```bash
curl http://127.0.0.1:8080/api/health   # {"status":"ok"}
```

### 2. 启动前端

```bash
cd frontend
export PUB_HOSTED_URL="https://pub.flutter-io.cn"   # 国内 pub 镜像
flutter pub get
flutter run -d chrome        # 开发模式
# 或 flutter build web --release 后托管 build/web 产物（部署到 NAS）
```

### 3. 验证导出的 .beancount

```bash
# 一次性：创建验证环境
python3 -m venv .venv
.venv/Scripts/python.exe -m pip install beancount

# 每次导出后校验（退出码 0、无报错即通过）
.venv/Scripts/python.exe -m beancount.scripts.check data/family.beancount
```

> 注：Windows 下 `bean-check.exe` 直接运行可能静默失败，建议用上述 `python -m beancount.scripts.check` 方式。

## 使用文档

### 账号与权限

- **注册**：首个注册用户创建默认账本并成为 `owner`；后续用户仅建账号，需 owner 在成员管理中按用户名邀请。
- **角色**：`owner` 管理成员与账本设置 / `editor` 可写 / `viewer` 只读。
- **鉴权**：业务端点需 `Authorization: Bearer <token>`；多账本下还需 `X-Ledger-Id` 头指定账本（未带时回退默认账本）。

### 离线同步

- 交易/账户由客户端离线生成 `uuid`；改账 = 新增冲正交易，绝不覆盖；删除 = 软删 + `op=delete` 事件。
- 服务端 `sync_log` 全局水位；`pull` 增量拉取，`push` 逐条幂等提交（单条失败不阻塞其余）。

### API 端点

完整端点清单（认证 / 账户 / 交易 / 成员 / 账本 / 同步 / 导出）见 [`AGENTS.md`](AGENTS.md) §7。

### 导出

| 端点 | 说明 |
|---|---|
| `GET /api/export` | 导出单文件 `.beancount` 文本 |
| `GET /api/export/archive` | 导出多文件目录（`main.bean` + `accounts/` + `date/`），打包 zip 下载 |

快速验证示例：

```bash
B=http://127.0.0.1:8080
TOKEN=$(curl -s -X POST $B/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
AUTH="Authorization: Bearer $TOKEN"

# 建账户（记下返回的 id）
curl -s -X POST $B/api/accounts -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}'

# 记一笔（postings 用 account_id；借贷必须按币种平衡）
curl -s -X POST $B/api/transactions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"date":"2026-08-02","description":"早餐","postings":[
        {"account_id":1,"commodity":"CNY","amount":"-10.00"},
        {"account_id":2,"commodity":"CNY","amount":"10.00"}]}'

# 导出并校验
curl -s $B/api/export -H "$AUTH" -o data/family.beancount
.venv/Scripts/python.exe -m beancount.scripts.check data/family.beancount
```

> 字段提醒：在线 `POST /api/transactions` 的 posting 用 `account_id`（整数）；离线 `POST /api/sync/push` 的交易 posting 用 `account_uuid`（字符串），两者勿混。

## 目录结构

```
.
├── schema.sql            # 数据库 DDL（含 Beancount 映射注释）
├── cmd/server/           # Go 后端入口
├── internal/             # 后端分层
│   ├── config/           # 集中配置（环境变量）
│   ├── domain/           # 领域模型 + 哨兵错误
│   ├── db/               # 开库 + 增量迁移
│   ├── repository/       # 数据访问层（SQL 集中于此）
│   ├── service/          # 业务逻辑层（校验/事务/同步/权限）
│   ├── handler/          # HTTP 适配层
│   ├── server/           # 路由 + CORS/鉴权中间件
│   ├── auth/             # 认证纯函数（bcrypt/token/context）
│   └── export/           # DB → .beancount 导出器（单文件 + 多文件目录）
├── frontend/             # Flutter Web 应用（MVVM 分层）
├── export_demo/          # 验证脚本（建库导出 / 双端同步模拟）
├── data/                 # 运行时 SQLite 文件（git 忽略）
└── docs/                 # 设计规格 / 同步协议 / 低保真原型
```

前端 `frontend/lib/` 按 `app / core / viewmodels / widgets / screens` 分层（详见 [`AGENTS.md`](AGENTS.md) §3.1）。

## 部署到 NAS

纯 Go 驱动免 CGO，一条命令交叉编译：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o familyledger ./cmd/server   # ARM64 NAS
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o familyledger ./cmd/server   # x86_64 NAS

# NAS 上运行（docker / 后台进程均可）
LEDGER_DB=/path/to/data/ledger.db ./familyledger
```

建议用反向代理（Nginx/Caddy）加 HTTPS；前端将 `flutter build web --release` 产物托管，并把
`frontend/lib/core/api/ledger_api.dart` 中的 `kDefaultApiBase` 改为 NAS 地址。

## 相关文档

- [`AGENTS.md`](AGENTS.md) — 开发上下文：架构 / 约定 / 数据库 Schema / 验证记录（面向开发者与 AI 助手）
- [`docs/design-spec.md`](docs/design-spec.md) — 产品设计规格
- [`docs/sync-design.md`](docs/sync-design.md) — 离线同步协议设计
- [`CHANGELOG.md`](CHANGELOG.md) — 版本变更记录

## 许可证

License 暂未指定。
