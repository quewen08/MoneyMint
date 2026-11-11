# FamBean 家庭协作记账

跑在 NAS 个人云上的**家庭协作复式记账**应用，数据模型对标 [Beancount](https://beancount.github.io/)。
后端为 Go + SQLite（可平滑切换 Postgres），前端与 API 由同一个 Go 进程在单端口托管。

- **数据自持**：账本数据全部保存在你自己的 NAS 上，不上传第三方。
- **离线优先**：断网也能记账，联网后自动增量同步。
- **标准兼容**：可导出标准 `.beancount` 文本，通过 `bean-check` 校验。
- **自动备份**：每日一致性快照（WAL 安全），保留策略自动清理，浏览器直接下载。

> **当前状态（2026-11-11）**：项目正在做 **M7 前端全量重构**，处于双前端并存期。
> **默认可用的生产前端仍是 Flutter Web**；新的 **Vue 3 + Vite 纯 SPA** 已落地批 0~2.5（57 个源文件 / 14,067 行 / 11 个页面，核心记账闭环打通，Vitest 202 用例全过），报表 / 设置 / 成员 / 离线内核 / 导入恢复向导待后续批次。
> **尚未切换**。完整方案、决策与验收见 [`docs/next-phase-frontend-plan.md`](docs/next-phase-frontend-plan.md)（v2.5）。
> 想直接跑起来用，按下面「快速开始」即可；想参与重构，看 [`AGENTS.md`](AGENTS.md) §1 与 §9。

## 特性

- **复式记账**：Beancount 五大账户类型（资产/负债/权益/收入/支出），按币种借贷平衡校验，金额定点十进制无浮点误差。
- **家庭共享**：一个家庭一个账本，`owner` / `editor` / `viewer` 三级角色权限，成员管理内置。
- **多账本**：一个账号可属于多个账本，数据完全隔离，随时切换。
- **自动备份**：每日 `VACUUM INTO` 一致性快照（含未 checkpoint 的 WAL 数据），附带 per-账本 Beancount 导出；保留最近 N 份每日 + M 个月月度，手动备份永不清理。
- **账号安全**：系统管理员 + 账本 owner 双层角色；注册开关（部署配置或界面切换）；CLI 密码重置兜底；**httpOnly Cookie 会话 + CSRF 自定义头 + CORS 来源白名单**，登录失败限流与强制改密。
- **在线记账幂等**：提交可带 `client_request_id`，弱网重复提交只产生一笔交易（后端部分唯一索引兜底）。
- **离线多端同步**：append-only + 不可变交易 + 全局水位，增量 pull / 逐条幂等 push，天然几乎无冲突。
- **标准导出**：单文件或多文件目录（`main.bean` + `accounts/` + `date/`）两种导出，解压后可直接 `bean-check`。
- **PWA**：可安装、离线壳、IndexedDB 本地真相源。

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.26 + 标准库 + [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite)（纯 Go，免 CGO） |
| 前端（当前生产） | Flutter Web 3.35+（MVVM 分层）+ sembast_web（IndexedDB） |
| 前端（M7 开发中） | **Vue 3 + Vite 纯 SPA** + Pinia + Vue Router + `decimal.js`；`es2015` + plugin-legacy，目标 iOS 14.8 / Safari 14.1 |
| 导出校验 | 项目内 Python venv + `beancount` |
| 部署 | Docker 多阶段构建（amd64 / arm64），单容器单端口 |

> 新版不引入 SSR / Nuxt / Node 运行时——Node 只在构建阶段出现，运行时镜像仍只有 Go 二进制 + 静态文件。

## 快速开始（Docker，推荐）

```bash
# 1. 构建（或使用 CI 发布的镜像）
docker build -t yourname/money-mint:0.5.0 .

# 2. 准备数据目录
mkdir -p ./data

# 3. 启动（数据 / 备份 / 前端 / API 全在这个容器里）
docker run -d --name famben -p 8080:8080 \
  -e TZ=Asia/Shanghai \
  -e LEDGER_OWNER_USER=awen \
  -e LEDGER_OWNER_PASSWORD=你的密码 \
  -v $(pwd)/data:/app/data \
  yourname/money-mint:0.5.0

# 4. 浏览器打开 http://<主机IP>:8080 ，用预设的 owner 账号登录
```

仓库内附带 [`docker-compose.yml`](docker-compose.yml)（含健康检查 / 备份保留 / 可选第二块盘异地备份），
配合 [`.env.example`](.env.example) 使用更方便。

> 空库 + 未设置 `LEDGER_OWNER_USER` 时，**首个注册的账号自动成为系统管理员**；
> 设置了则跳过注册直接用预设账号登录（见下文「账号与权限」）。
>
> **注意**：`Dockerfile` 目前仍以 Flutter SDK 构建前端产物（M7 批 6 才换成 Node 构建），
> 所以此命令现在产出的是 Flutter 版；Vue 版需用 `bash scripts/local.sh web-build` 本地构建后自行托管 `frontend/dist/`。

## 从源码运行

### 环境要求

- Go 1.26+
- Node.js 20+（**M7 新版前端**；方案钉版目标 22.23.4）
- Flutter 3.35+（**当前生产前端**与 `/legacy/` 回滚产物）
- Python 3.x（仅用于导出校验，装进项目 `.venv`，不污染系统）

```bash
# 后端（当前默认托管 Flutter 旧版产物）
(cd frontend && flutter build web --release)
LEDGER_DB=data/ledger.db LEDGER_WEB_ROOT=frontend/build/web go run ./cmd/server

# 新版 Vue 前端开发模式（另开终端先跑后端）
(cd frontend && npm ci && npm run dev)     # Vite 已把 /api 代理到 127.0.0.1:8080
# 等价：bash scripts/local.sh web-dev / web-build / web-test / web-typecheck / web-compat
```

健康检查：`curl http://127.0.0.1:8080/api/health` → `{"status":"ok"}`

常用命令统一走 `bash scripts/local.sh <command>`（Windows 无 make；Linux/CI 用 `Makefile`），
完整清单见 [`AGENTS.md`](AGENTS.md) §10.1。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `LEDGER_DB` | `data/ledger.db` | SQLite 数据库路径 |
| `LISTEN` | `:8080` | HTTP 监听地址 |
| `LEDGER_WEB_ROOT` | `./web` | 前端产物目录；不存在则只提供 API |
| `LEDGER_OWNER_USER` / `LEDGER_OWNER_PASSWORD` | 空 | 预设 owner 账号（空库时生效，自动成为系统管理员） |
| `LEDGER_REGISTRATION_ENABLED` | `true` | 注册开关初始值（之后可在界面修改并持久化） |
| `LEDGER_BACKUP_ENABLED` | `true` | 自动备份开关 |
| `LEDGER_BACKUP_DIR` | `{DB 目录}/backups` | 备份目录（可指向第二块盘） |
| `LEDGER_BACKUP_HOUR` | `3` | 每日备份触发小时 |
| `LEDGER_BACKUP_KEEP_DAILY` / `LEDGER_BACKUP_KEEP_MONTHLY` | `14` / `12` | 备份保留策略 |
| `LEDGER_LOGIN_MAX_ATTEMPTS` / `LEDGER_LOGIN_WINDOW_MINUTES` / `LEDGER_LOGIN_LOCKOUT_MINUTES` | `5` / `15` / `10` | 登录失败限流（次数 / 窗口分钟 / 锁定分钟） |
| `LEDGER_AUTH_COOKIE` | `true` | 会话用 httpOnly Cookie 承载；`false` 完全退回 Bearer（不校验 CSRF） |
| `LEDGER_COOKIE_SECURE` | `false` | `Set-Cookie` 的 `Secure`；HTTPS 部署置 `true` |
| `LEDGER_CORS_ORIGINS` | 空 | 跨域来源白名单（逗号分隔）；空=仅同源，`*`=通配但不带凭证 |
| `LEDGER_AUTH_EXPOSE_TOKEN` | `true` | 登录/刷新响应体是否仍返回 `token`；兼容期保留，cutover 后置 `false` |
| `TZ` | `Asia/Shanghai` | 时区（交易日与备份时间依赖它） |

> 全部配置在启动时由 `config.Validate()` 集中校验，非法值立即失败而非静默回退。

## 账号与权限

### 两层角色

| 角色 | 管什么 | 典型操作 |
|---|---|---|
| **系统管理员**（`users.is_admin`，全局唯一层级） | 全库级 | 数据备份 / 下载 / 还原、注册开关 |
| **账本 owner**（`ledger_members.role`，账本级） | 单个账本 | 成员邀请 / 改角色 / 移除 / 重置成员密码、账本设置 |

首个账号 / 预设的 `LEDGER_OWNER_USER` 自动成为系统管理员兼账本 owner。

### 会话与 CSRF

登录 / 刷新 / 改密会下发 `fambean_session` httpOnly Cookie（`SameSite=Lax`、`Path=/`、30 天）。
**Cookie 会话的非 GET 请求必须带 `X-CSRF` 自定义头**，缺失返回 403 `csrf_required`；
显式 `Authorization: Bearer` 的请求豁免。旧版客户端在兼容期仍可用 Bearer，
`LEDGER_AUTH_EXPOSE_TOKEN=false` 后响应体不再返回 token。

### 注册开关

- **部署配置**：`LEDGER_REGISTRATION_ENABLED=false`（初始值）
- **界面切换**：系统管理员随时修改（持久化，之后以界面值为准）
- 关闭后新用户注册返回 403；已注册成员不受影响

### 忘记密码

| 谁 | 怎么办 |
|---|---|
| 普通成员 | 请账本 owner 在成员管理中重置（自动注销其全部会话） |
| owner / 管理员 | 由另一位系统管理员重置（互救）；**单人家庭的兜底**：到服务器执行 CLI |

```bash
# 服务器上直接执行（不经 HTTP，无需登录）
./server -reset-password awen -new-password 新密码
# Docker：
docker exec famben /app/server -reset-password awen -new-password 新密码
```

重置后该用户全部会话立即失效，用新密码重新登录。

## 自动备份

- 每日 `LEDGER_BACKUP_HOUR` 点（启动时若超过 24h 未备份会补跑一次）。
- 快照用 SQLite `VACUUM INTO` 生成，**完整性校验 + 行数比对**后才写入清单；
  同目录附每账本一份 `.beancount` 文本。
- 清理策略：最近 14 份每日 ∪ 最近 12 个月每月一份 ∪ 全部手动备份。
- 下载前先看 SHA256 与行数清单：`GET /api/backup/list`（仅系统管理员）。
- ⚠️ 请勿直接复制 `ledger.db` 文件当备份——WAL 模式下会丢数据且不报错。

> 备份还原与 Beancount 导入（M2）方案已出但**尚未实现**，见 [`docs/m2-plan.md`](docs/m2-plan.md)。

## 使用文档

### 离线同步

- 交易/账户由客户端离线生成 `uuid`；改账 = 新增冲正交易，绝不覆盖；删除 = 软删 + `op=delete` 事件。
- 服务端 `sync_log` 全局水位；`pull` 增量拉取，`push` 逐条幂等提交（单条失败不阻塞其余）。

### API 端点

完整端点清单（认证 / 账户 / 交易 / 成员 / 账本 / 同步 / 导出 / 备份 / 设置）见 [`AGENTS.md`](AGENTS.md) §7。

快速验证示例（兼容期 Bearer 写法；生产推荐 Cookie + `X-CSRF`）：

```bash
B=http://127.0.0.1:8080
TOKEN=$(curl -s -X POST $B/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
AUTH="Authorization: Bearer $TOKEN"

# 建账户（记下返回的 id）
curl -s -X POST $B/api/accounts -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}'

# 记一笔（postings 用 account_id；借贷必须按币种平衡；client_request_id 可防重复提交）
curl -s -X POST $B/api/transactions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"date":"2026-08-02","description":"早餐","client_request_id":"demo-001","postings":[
        {"account_id":1,"commodity":"CNY","amount":"-10.00"},
        {"account_id":2,"commodity":"CNY","amount":"10.00"}]}'

# 导出并校验
curl -s $B/api/export -H "$AUTH" -o data/family.beancount
.venv/Scripts/python.exe -m beancount.scripts.check data/family.beancount
```

> 字段提醒：在线 `POST /api/transactions` 的 posting 用 `account_id`（整数）；离线 `POST /api/sync/push` 的交易 posting 用 `account_uuid`（字符串），两者勿混。
> 多账本场景业务请求须带 `X-Ledger-Id`，缺失会回退默认账本 id=1。

## 目录结构

```
.
├── schema.sql            # 数据库 DDL（含 Beancount 映射注释）
├── cmd/server/           # Go 后端入口（含 CLI：密码重置）
├── internal/             # 后端分层
│   ├── config/           # 集中配置（环境变量 + 启动校验）
│   ├── domain/           # 领域模型 + 哨兵错误
│   ├── db/               # 开库 + 增量迁移（当前至 013）
│   ├── repository/       # 数据访问层（SQL 集中于此）
│   ├── service/          # 业务逻辑层（校验/事务/同步/权限/备份调度）
│   ├── handler/          # HTTP 适配层
│   ├── server/           # 路由 + CORS/CSRF/鉴权中间件 + 前端静态托管
│   ├── auth/             # 认证纯函数（bcrypt/token/context）
│   ├── backup/           # 一致性快照 + 定时调度 + 保留清理
│   └── export/           # DB → .beancount 导出器（单文件 + 多文件目录）
├── frontend/             # 双版并存（M7 过渡期）
│   ├── lib/ test/ pubspec.* web/ build/   # 旧版 Flutter Web
│   ├── index.html / package.json / vite.config.ts / tsconfig*.json
│   ├── public/           # manifest.json + icons/（新版静态资源）
│   ├── src/              # 新版 Vue 源码（api/ core/ router/ styles/ views/）
│   └── dist/             # 新版产物（gitignore），含 legacy/ 回滚目录
├── scripts/              # local.sh / check-bean / verify-version / smoke / build-legacy / check-legacy
├── Dockerfile            # 当前三阶段（Flutter → Go → alpine），批 6 改 Node
├── docker-compose.yml    # 家用部署示例
├── data/                 # 运行时 SQLite 与 backups/（git 忽略）
└── docs/                 # 设计规格 / 同步协议 / 原型 / 任务书 / M7 方案
```

旧版 `frontend/lib/` 按 `app / core / viewmodels / widgets / screens` 分层，新版 `frontend/src/` 按
`core / api / stores / views / components` 分层，详见 [`AGENTS.md`](AGENTS.md) §3。

## 从源码交叉编译到 NAS

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o familyledger ./cmd/server   # ARM64 NAS
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o familyledger ./cmd/server   # x86_64 NAS
```

前端与 API 同端口部署（Go 托管静态产物），反向代理只需把 `/` 转到后端端口，
HTTPS / 换端口 / 域名均无需改代码：Vue 产物用相对 `/api` 与 `base: './'`，反代到子路径也能工作。
**不要**在生产代码里硬编码 `http://localhost:8080` 之类的绝对后端地址。

建议用反向代理（Nginx / Caddy / Cloudflare）加 HTTPS——账本数据不要明文跑公网；
启用 HTTPS 后记得把 `LEDGER_COOKIE_SECURE=true`。

## 相关文档

- [`AGENTS.md`](AGENTS.md) — 开发上下文：架构 / 约定 / Schema / 端点清单（面向开发者与 AI 助手）
- [`CLAUDE.md`](CLAUDE.md) — Claude 系工具的工作约定：命令 / 禁区 / 关键文件索引
- [`docs/next-phase-frontend-plan.md`](docs/next-phase-frontend-plan.md) — **M7 全量重构方案 v2.5**（决策 / 分批 / 验收 / 风险，权威来源）
- [`docs/frontend-framework-rewrite-assessment.md`](docs/frontend-framework-rewrite-assessment.md) — 换框架可行性评估
- [`docs/m2-plan.md`](docs/m2-plan.md) — 备份还原 + Beancount 导入方案（待开工）
- [`docs/design-spec.md`](docs/design-spec.md) — 产品设计规格
- [`docs/sync-design.md`](docs/sync-design.md) — 离线同步协议设计
- [`docs/frontend-revamp-tasks.md`](docs/frontend-revamp-tasks.md) — 旧版前端改版任务清单
- [`CHANGELOG.md`](CHANGELOG.md) — 版本变更记录

## 许可证

License 暂未指定。
