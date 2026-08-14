# AGENTS.md — 家庭协作记账系统（类 Beancount 云记账）

> 本文件是项目给 AI 助手/智能体的一致上下文。任何会话开始前先读它。
> 项目根：`C:\Users\FLASHBASE\WorkBuddy\2026-08-10-16-07-53`
> 用户：全栈独做（老板），后端 Go / 前端 Flutter / 设计稿均本人拍板。沟通可直接给结论。
> 提醒：因独做全栈，"分配负责人/邀请角色关注"类提醒不适用，改为提醒其自写验收标准、留档、关键模块自评审。

---

## 1. 项目概述

跑在 NAS 个人云上的**复式记账应用**，对标 Beancount 的数据模型：
- 前端：Flutter Web（PWA），经浏览器访问，无原生客户端计划。
- 后端：Go 标准库 + `modernc.org/sqlite`（纯 Go，免 CGO，便于 NAS arm64 交叉编译）。
- 数据库：SQLite（开发期）；后期平滑切 Postgres（DB 访问层已做抽象）。
- 数据自持；家庭成员**共享同一账本**；导出标准 `.beancount` 文本必须过 `bean-check`。
- 支持离线多端记账 + 增量同步。

**核心不变约束**：导出的 `.beancount` 必须能被 `bean-check` 直接校验通过（退出码 0、无报错）。

**当前阶段**：P0 垂直切片、P1-A（前端打磨/离线联调）、P1-B（后端增强：删除语义/逐条 push/认证增强/成员管理）、**P1-C（多账本隔离 + 角色鉴权 + 账本设置 + 多文件目录导出）** 均已完成并通过端到端验证（2026-08-14）。下一阶段进入上线就绪（HTTPS 部署、真机双端联调）。

---

## 2. 技术栈与本机环境

| 工具 | 版本 / 路径 | 说明 |
|---|---|---|
| Go | 1.26.5，路径 `/d/Environment/go/bin` | Git Bash 需 `export PATH="$PATH:/d/Environment/go/bin"` 才识别 `go` |
| Flutter | 3.35.1，路径 `/d/Environment/flutter/bin` | 同上需加 PATH |
| Python | 3.x，项目内 `.venv` | 仅用于 `bean-check` 验证，**不污染系统** |
| GOPROXY | `https://goproxy.cn,direct` | `proxy.golang.org` 国内不可达；`go env -w GOPROXY=...` 已设 |
| PUB_HOSTED_URL | `https://pub.flutter-io.cn` | Flutter pub 国内镜像（前端 `flutter pub get` 用） |

Python 环境（一次建好，已做过可跳过）：
```bash
python3 -m venv .venv
.venv/Scripts/python.exe -m pip install beancount
```

---

## 3. 目录结构

```
.
├── schema.sql            # 初始核心表 DDL（含 Beancount 映射注释）
├── go.mod / go.sum
├── cmd/
│   └── server/main.go    # Go 后端入口（加载配置 → 装配依赖 → 启动监听）
├── internal/             # 后端分层（config → domain → db → repository → service → handler → server）
│   ├── config/config.go  # 集中配置（环境变量 LEDGER_DB/LISTEN/LEDGER_SCHEMA → Config）
│   ├── domain/           # 共享领域模型（Account/Transaction/Posting/...）+ 哨兵错误 + DefaultLedgerID
│   ├── db/db.go          # 开库 + 初始 schema 迁移（显式传 schemaPath）
│   ├── db/migrate.go     # 增量迁移框架 + 001_sync + 002_delete_semantics（见 §4.2）
│   ├── repository/       # 数据访问层：全部 SQL 集中于此，按 account/transaction/member/sync/session/user 分文件
│   ├── service/          # 业务逻辑层：校验/事务编排/余额计算/自然键合并/同步协议/成员权限
│   ├── handler/          # HTTP 适配层（薄）：解析请求 → 调 service → 写响应（错误码统一映射）
│   ├── server/           # 组合根：路由注册 + CORS/auth 中间件 + 依赖装配（server.New）
│   ├── auth/auth.go      # 认证纯函数：bcrypt / token 生成 / context 用户态（不依赖 DB）
│   └── export/beancount.go  # DB → .beancount 导出器
├── frontend/             # Flutter Web 应用（MVVM 分层，见 §3.1）
│   ├── lib/
│   │   ├── main.dart          # 入口：AppScope 注入 + MaterialApp + AuthGate
│   │   ├── app/{theme,scope}.dart        # 设计系统 + AppScope(InheritedWidget) DI
│   │   ├── core/{models,api,store,sync}/ # 领域层：模型/REST/本地存储/同步编排（UI 无关）
│   │   ├── viewmodels/{auth_controller,ledger_controller}.dart  # ChangeNotifier 状态持有
│   │   ├── widgets/{common,sync_badge,balance_meter,pc}.dart    # 可复用组件（pc=PC 端卡/图表/按钮）
│   │   ├── utils/money.dart   # 金额格式化 / 借贷校验
│   │   └── screens/
│   │       ├── auth/ login_screen.dart
│   │       ├── shell/ app_shell(响应式路由) + mobile_shell(3 Tab 底栏) + desktop_shell(9 模块侧边栏)
│   │       ├── dashboard/ accounts/ transactions/ record/ settings/(sync,settings)
│   │       └── members/ commodities/ import_export/ reports/   # PC 专属模块（members 真实实现，commodities 占位）
│   └── web/                   # PWA：manifest.json + index.html(注册 SW) + icons
├── export_demo/          # Python/sh 验证参考（seed_and_export.py / sync_test.sh）
├── data/                 # 运行时 SQLite 文件（git 忽略）
├── docs/                 # design-spec.md（设计规格）、sync-design.md（同步协议）、prototype/index.html（低保真原型）
└── .venv/                # Python 虚拟环境
```

### 3.1 前端架构（MVVM 分层 + 响应式双布局，2026-08-13 重构）

前端已从「单文件 `main.dart` + 扁平 `models/api/store/sync.dart`」重构为 **MVVM 分层**，
并严格对齐 `docs/prototype/index.html` 的响应式设计（断点：宽度 ≥1024 走 PC 侧边栏，<1024 降级移动底栏）：
视觉语言（配色 / 圆角卡片 / 净值大字号 / 三态同步角标 / 借贷平衡指示）落地在 `app/theme.dart`。

- **响应式导航模型**（`app/navigation.dart`）：
  - `AppRoute` 枚举（9 目的地）；`isWide(ctx)` / `kDesktopBreakpoint=1024` 决定布局。
  - 移动端 3 个底栏 Tab（`mobileTabs`：首页 / 流水 / 我的）；PC 端侧边栏 9 模块分组（`pcSidebar`：概览 / 管理 / 工具 / 系统）。
  - 「记一笔」是**动作**而非导航项（移动 FAB / PC 顶栏按钮）。
- **响应式壳**（`screens/shell/`）：`app_shell.dart`（按 `isWide` 分流到 `mobile_shell` / `desktop_shell`，并持有 `_route`、初始化 `ledger.init()`、监听 `window.onOnline/onOffline`）；`mobile_shell.dart`（深色主题 AppBar + 底部 `NavigationBar` + FAB）；`desktop_shell.dart`（左侧深色 `Sidebar` 9 模块 + 右侧 `AppBar`+内容区，顶栏含同步徽标/记一笔/账户入口）。
- **Models**（`core/models/`）：纯数据（uuid 主键、toMap/fromMap、`Decimal` 字符串金额），不依赖 Flutter。
  - `account.dart` / `posting.dart` / `transaction.dart`（`LocalTxn.reversedOf` 标记冲正）/ `pending_change.dart` / `sync_models.dart`（PullResp/PushResp 响应模型）。
- **Core**（UI 无关，可单测）：
  - `api/ledger_api.dart`：认证 + 同步 + 导出 REST 客户端；token 静态存 `localStorage`；`kDefaultApiBase = 'http://localhost:8080'`。
  - `store/local_store.dart`：sembast_web / IndexedDB 本地真相源；`computeBalances` / `computeNetWorth`（资产−负债）/ `computeBalancesByType` 全部用 `Decimal` 精确计算。
  - `sync/sync_service.dart`：pull/push 编排 + `createAccount` / `createTransaction` / `reverseTxn` 命令。
- **ViewModels**（`ChangeNotifier`）：`auth_controller.dart`（登录态/busy/error）、`ledger_controller.dart`（账户/交易/余额/净值/`SyncState` 三态 + 全部业务命令 + `exportText`）。视图用 `ListenableBuilder`（`material.dart` 提供）订阅。
- **Views / Widgets**：`screens/*` 哑视图只渲染与转发事件；`widgets/{common,sync_badge,balance_meter,pc}.dart` 复用组件（`pc.dart` 提供 `PcSectionTitle`/`PcMetricCard`/`ComingSoon`/`MiniBars`/`PcButton`）。
- **AppScope**（`app/scope.dart`）：根部 `InheritedWidget`，注入 `LedgerApi` / `LocalStore` 单例与两个 controller。
  - 注：因 `LedgerApi` token 为静态共享、`LocalStore` 为工厂单例，构造函数中 auth/ledger 直接内联 `LedgerApi()` / `LocalStore()` 表达式，**不可**在初始化列表里用 `api`/`store` 字段（Dart 禁止 `implicit_this_reference_in_initializer`）。

**PC 专属模块落地状态**（按设计稿「占位+可计算」策略，2026-08-13）：
- `screens/reports/`：**真实**（试算平衡 / 支出分类 / 月度趋势，全部本地交易计算）。
- `screens/import_export/`：**真实导出**（Beancount 文本 + CSV，浏览器 Blob 下载）；导入（OFX/CSV）为占位拖放区。
- `screens/members/`：**真实实现**（P1-B3 成员管理：owner 列出/添加/改角色/移除成员、重置成员密码；editor/viewer 只读；API 见 §7）。
- `screens/commodities/`：**占位**（`ComingSoon`，后端 prices/commodities CRUD 端点尚未实现，见 §13）。

> 已删除的旧扁平文件：`lib/api.dart`、`lib/models.dart`、`lib/store.dart`、`lib/sync.dart`、`lib/report.dart`。

**响应式落地细节（2026-08-13 实测）**：
- 「记一笔」在移动端是底部 FAB、在 PC 端是侧边栏底部固定按钮 + 顶栏「记一笔」按钮；点击均 `Navigator.push` 打开 `RecordScreen`（独立全屏页，自带 AppBar）。
- **PC 端仅 `AppRoute.accounts` 路由有右下角 `FloatingActionButton`**（蓝色 `+`，打开 `AccountEditScreen`），其余 content 路由不设 FAB——由 `app_shell.dart` 按 `wide && route==accounts` 条件传入 `DesktopShell.floatingActionButton` 实现。
- **页面读取 AppScope 必须放在 `build` / `didChangeDependencies`，禁止放 `initState`**：`RecordScreen` 曾在 `initState` 里调 `AppScope.of(context).ledger`，触发 `dependOnInheritedWidgetOfExactType called before initState completed` 红屏，已改为 `didChangeDependencies` + `_initialized` 守卫。任何新页面若需初始化读取 controller，务必遵循此约定。

---

## 4. 数据库 Schema

金额一律用 `NUMERIC`；应用层以**定点十进制字符串**或最小货币单位整数写入，避免浮点误差。
同一 transaction 下，按 commodity 分组的 `postings.amount` 之和必须为 0（有符号数，可负）。

### 4.1 核心表（`schema.sql`，幂等 `IF NOT EXISTS`）
- `users`(id, username UNIQUE, password_hash, display_name, status, ...)
- `ledgers`(id, name, owner_id, default_commodity, ...)
- `ledger_members`(ledger_id, user_id, role[owner|editor|viewer], invited_by, ...) — 权限在此层控制
- `accounts`(id, ledger_id, uuid, name, type[5类], open_date, close_date, commodity_restriction, ...)
- `commodities`(id, ledger_id, uuid, symbol, name, precision)
- `prices`(id, ledger_id, uuid, commodity, currency, date, rate, source)
- `transactions`(id, ledger_id, uuid UNIQUE, date, flag[*|!], description, version, created_by, ...)
- `postings`(id, transaction_id, account_id, commodity, amount 有符号, position)
- `sync_checkpoints` —— **不在 schema.sql 定义**，由迁移 001 重建（见下），避免冲突。

### 4.2 迁移列表（`internal/db/migrate.go`）
- **001_sync**：
  - `accounts`/`commodities`/`prices` 补 `uuid` + `updated_at`（旧库 ALTER），回填 uuid（前缀 `acc-`/`cmd-`/`prc-`+id），建唯一索引 `(ledger_id, uuid)`。
  - 新建 `sync_log`(id AUTOINCREMENT 即全局水位, ledger_id, entity_type, entity_uuid, op[create|delete], created_at)。
  - 重建 `sync_checkpoints`(ledger_id, client_id, last_seq, last_synced_at, PRIMARY KEY(ledger_id,client_id))。
  - 新建 `sessions`(token PK, user_id, created_at, expires_at)。
- **002_delete_semantics**（P1-B1）：
  - `transactions` 加 `deleted_at`（交易软删，保留分录做审计）。
  - **重建 `accounts`**：加 `deleted_at`（账户软删）；表级唯一约束改为**部分唯一索引** `(ledger_id, name) WHERE deleted_at IS NULL`（软删后可重建同名账户）；保留 `(ledger_id, uuid)` 唯一索引。
  - 兼容旧库：seed `owner` 用户密码从 `'unset'` 置为 `owner123456`（P1-B3 起注册语义改变，旧库升级后可用该密码登录后自行修改）。
- 幂等：已应用项记录在 `schema_migrations` 表，重复运行安全。

> 修改 schema 必须走迁移脚本（`migrate.go` 的 `migrations` 列表追加），**不要**直接改 `schema.sql` 后手动 ALTER 线上库。

---

## 5. 认证与权限模型（P1-B3 后已实现并验证）

**设计语义（老板确认）**：家庭账本是**共享账本**，隔离边界 = 「未登录 401 一律不可见 / 账本成员可见共享数据 / 非成员 403」。

- **密码**：`bcrypt` 加盐哈希，库中不落明文。
- **会话**：登录签发 32 字节随机 token，存 `sessions`（有效期 30 天）；退出即删 token，旧 token 立即失效；`POST /api/auth/refresh` 用当前 token 换新 token（再续 30 天，旧 token 立即失效）。
- **注册语义（P1-B3 起）**：**首个注册用户**创建默认账本并成为 **owner**；**后续注册仅创建账号（不加入账本）**，等待 owner 在成员管理中按用户名添加。已注册用户可用 `me/logout/change-password/refresh`（自服务端点仅需登录态），但业务端点（账户/交易/同步/导出）返回 **403** 直至被邀请。
- **成员管理**：仅 owner 可 `GET/POST /api/ledger/members`（列表/添加）、`POST/DELETE /api/ledger/members/{userID}`（改角色/移除）、`POST /api/ledger/members/{userID}/reset-password`（重置密码并使该用户全部会话失效）。角色：`owner | editor | viewer`。
- **鉴权**：`authMiddleware` 包裹所有业务路由；放行 `/api/health` 与公开认证端点（register/login）；其余必须 Bearer token；自服务端点仅需登录态，业务端点额外校验为默认账本成员（非成员 403）；通过后用户 id 注入 context。
- 记账 `handleCreateTransaction` 写入 `created_by`；`handleChangePassword` 改密后使该用户全部会话失效并签发新 token。

代码位置：`internal/auth`（纯函数）、`internal/repository`（会话/成员查询）、`internal/service`（注册/登录/改密/刷新业务 + `Authenticate`/`AuthorizeLedger`）、`internal/server`（鉴权中间件 + 路由装配）。

---

## 6. 离线同步协议（#4，P1-B1/B2 后已实现并验证）

设计核心：**append-only + 不可变 + 水位增量**（天然几乎无冲突）。

- **不可变交易 + uuid 身份键**：交易/账户由客户端离线生成 `uuid`；改账 = 新增冲正交易，绝不覆盖；删除 = 软删 + `op=delete` 事件。
- **全局水位 `sync_log`**：每接受一次创建/删除即追加一行（`op=create|delete`），`id` 单调自增即服务端水位。
- **增量 pull**：`GET /api/sync/pull?since=<水位>&client_id=<设备>` 取回 `id>since` 的全部变更（含 `op`；delete 事件仅携带 `{uuid}`，create 事件携带全量实体 JSON）。实体被连带删除时跳过该条。
- **逐条 push（P1-B2）**：`POST /api/sync/push` 逐条提交离线变更，**单条失败不阻塞其余**；响应 `{checkpoint, accepted, results:[{index, entity_type, uuid, status, accepted, server_uuid, error}]}`，`results` 按请求原始顺序返回。处理顺序：先 create（commodity → account → transaction，被引用实体先落库）再 delete（transaction → account）。
- **自然键合并（P1-B1）**：账户推送时若同账本存在**未删除同名**账户，保留先到者，`results[].server_uuid` 返回真实 uuid；前端据此重映射本地 `loc-` uuid（`core/sync/sync_service.dart` 的 `_pushLoop` 两轮推送）。部分唯一索引 `(ledger_id,name) WHERE deleted_at IS NULL` 保证软删后可重建同名账户。
- **删除语义（P1-B1）**：`op=delete` 软删交易/账户（删账户连带删其引用交易，为每个实际软删实体各写一条 delete 日志）；幂等（不存在/已删返回 accepted=false 且无错误）；前端删除入口见 §3.1。
- **前端离线优先**：`core/store/local_store.dart`（sembast+IndexedDB）为本地真相源，断网可记账；`core/sync/sync_service.dart` 联网时先 pull 合并再 push 本地待推送队列，失败保留队列重试。PWA 外壳已补（见 §13 / P1-A1）。

### 6.1 接口字段要点（实测确认，容易踩坑）

两条「创建交易」路径的 **posting 账户引用字段不同**，切勿混用：

| 路径 | posting 账户字段 | 取值 |
|---|---|---|
| 在线 `POST /api/transactions` | `account_id` | 服务端自增整数 id（如 `1`、`2`） |
| 离线 `POST /api/sync/push`（entity_type=transaction） | `account_uuid` | 字符串 uuid；**所有** posting 都必须带，不能用 `account` 名字 |

`sync/push` 请求信封（**不是** `{"accounts":[...]}`）：

```json
{
  "client_id": "devA",
  "since": 0,
  "changes": [
    {"entity_type": "account",      "op": "create", "entity": {"uuid":"...","name":"Assets:Bank:CNY","type":"Assets","open_date":"2026-08-01","commodity_restriction":"CNY"}},
    {"entity_type": "transaction",  "op": "create", "entity": {"uuid":"...","date":"2026-08-10","flag":"*","description":"离线买菜",
       "postings":[{"account_uuid":"<银行uuid>","commodity":"CNY","amount":"-30.00"},
                   {"account_uuid":"<食品uuid>","commodity":"CNY","amount":"30.00"}]}},
    {"entity_type": "transaction",  "op": "delete", "entity": {"uuid":"<要删除的交易uuid>"}},
    {"entity_type": "account",      "op": "delete", "entity": {"uuid":"<要删除的账户uuid>"}}
  ]
}
```

`push` 响应（P1-B2 逐条）：

```json
{
  "checkpoint": 6,
  "accepted": 1,
  "results": [
    {"index": 0, "entity_type": "account", "uuid": "loc-x", "status": "ok", "accepted": true},
    {"index": 1, "entity_type": "account", "uuid": "loc-x-dup", "status": "ok", "accepted": false, "server_uuid": "loc-x"},
    {"index": 2, "entity_type": "transaction", "uuid": "loc-txn-bad", "status": "error", "error": "推送交易引用的账户 nope 不存在"}
  ]
}
```

> `op` 缺省视为 `create`（兼容旧客户端）。`accepted` 仅统计真正写入的条数（自然键合并 / 幂等跳过 / 失败均不计）。`server_uuid` 仅在发生自然键合并时非空。

---

## 7. API 端点清单

| 端点 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/api/health` | GET | 公开 | 健康检查 `{"status":"ok"}` |
| `/api/auth/register` | POST | 公开 | 注册；**首个用户建默认账本并成 owner，后续用户仅建账号待邀请**，返回 token |
| `/api/auth/login` | POST | 公开 | 用户名+密码登录，返回 token |
| `/api/auth/logout` | POST | 登录 | 注销当前会话（安全退出） |
| `/api/auth/me` | GET | 登录 | 当前用户信息（role 恒空串，账本角色见 `/api/ledgers`） |
| `/api/auth/refresh` | POST | 登录 | 换发新 token（再续 30 天），旧 token 立即失效 |
| `/api/auth/change-password` | POST | 登录 | 凭旧密码改密；成功后该用户全部会话失效并签发新 token |
| `/api/ledgers` | GET/POST | 登录 | 列出当前用户所属账本（含角色）/ 创建新账本（创建者成 owner） |
| `/api/ledgers/{id}` | PATCH | 登录+**owner** | 账本设置：改名 / 默认币种 |
| `/api/accounts` | GET/POST | 登录+成员 | 账户列表（含余额）/ 新建（生成 uuid + appendSyncLog） |
| `/api/accounts/{uuid}` | DELETE | 登录+成员 | 软删账户（连带其引用交易，写 sync_log delete）；幂等 404 |
| `/api/transactions` | GET/POST | 登录+成员 | 流水 / 记一笔（postings 用 `account_id`；按币种借贷平衡校验，提交后 appendSyncLog，写 created_by） |
| `/api/transactions/{uuid}` | DELETE | 登录+成员 | 软删交易（写 sync_log delete）；幂等 404 |
| `/api/ledger/members` | GET/POST | 登录+**owner** | 成员列表 / 按用户名添加已注册用户（角色 editor/viewer） |
| `/api/ledger/members/{userID}` | POST/DELETE | 登录+**owner** | 调整角色 / 移除成员（禁止移除自己或 owner） |
| `/api/ledger/members/{userID}/reset-password` | POST | 登录+**owner** | 重置成员密码，并使该用户全部会话失效 |
| `/api/export` | GET | 登录+成员 | 导出单文件 `.beancount` 文本 |
| `/api/export/archive` | GET | 登录+成员 | 导出多文件目录（main.bean + accounts/ + date/）打包 zip |
| `/api/sync/pull` | GET | 登录+成员 | 增量拉取（since + client_id，事件含 `op`，delete 仅 uuid） |
| `/api/sync/push` | POST | 登录+成员 | 逐条幂等提交离线变更（见 §6.1 信封与响应格式） |

> 多账本上下文（P1-C）：业务端点（accounts/transactions/sync/export/members）需带 `X-Ledger-Id` 头指定账本；缺失时回退默认账本（id=1，向后兼容旧客户端）。角色鉴权：`viewer` 只读（非 GET 一律 403）、`editor` 可写、`owner` 可管理（成员/账本设置）。

CORS：`Access-Control-Allow-Origin: *`，Headers 含 `Content-Type, Authorization, X-Ledger-Id`，Methods 含 `GET,POST,DELETE,PATCH,OPTIONS`。

---

## 8. 关键设计决策（老板已拍板）

原 `docs/sync-design.md` §7 三项待决项全部确认并实现：

1. **同步范围** → 全实体 synced + sync_log 统一水位（非「仅同步 transactions」备选）。
2. **认证前置** → 最小登录 + Bearer token 鉴权（非「信任内网直连」备选）。
3. **schema 变更** → 已同意并执行迁移 001。

其他已确认决策：
- 静态加密：不需要（内网 / 磁盘层覆盖）。
- 自动导入（OFX/CSV）：放 P1/下期。
- 跨币种 cost 注解：P0 限定单币种。
- 数据隔离语义：家庭共享账本（见 §5），非「每人独立账本」。

---

## 9. 验证记录

### 9.1 后端 auth 冒烟（2026-08-12，`:8097`，全新 data/smoke.db）
| 场景 | 期望 | 实际 |
|---|---|---|
| health | 200 | 200 ✅ |
| 未登录 /api/accounts | 401 | 401 ✅ |
| 注册 alice | 201 + token | 201 ✅ |
| 错误密码登录 | 401 | 401 ✅ |
| 正确登录 | 200 + token | 200 ✅ |
| 退出后旧 token | 401 | 401 ✅ |
| /api/auth/me | 200 | 200 ✅ |
| 未登录 /api/sync/pull | 401 | 401 ✅ |
| 未登录 /api/sync/push | 401 | 401 ✅ |

### 9.2 端到端业务流 + 同步（2026-08-13，全新 data/smoke_test.db，`:8099`）
| 场景 | 期望 | 实际 |
|---|---|---|
| `go build ./...` / `go vet ./...` | 通过 | 通过 ✅ |
| 注册 alice 并拿 token | 201 + token | ✅ |
| 未登录 /api/accounts | 401 | 401 ✅ |
| 建账户（生成 uuid） | 200 + uuid | ✅ |
| 在线记账（postings 用 `account_id`） | 201 + uuid | ✅ |
| 借贷不平衡交易 | 400，报错「交易不平衡：币种 CNY 借贷差为 -2，必须为 0」 | ✅ |
| `/api/auth/me` | 200 | ✅ |
| `sync/pull?since=0` | 返回 checkpoint + 全量 changes（含 seq） | ✅ |
| `sync/push` 账户首推 / 重复推 | accepted=1 / accepted=0 | ✅ |
| `sync/push` 交易（postings 用 `account_uuid`）首推 / 重复推 | accepted=1 / accepted=0 | ✅ |
| 导出 `.beancount` 后 `bean-check` | 退出码 0 | ✅ |
| `flutter analyze lib` | 无 error | 10 条 info 告警，0 error ✅ |

### 9.3 历史回归
- 同步双端模拟（`export_demo/sync_test.sh`）：A 在线记账 → B pull → B 离线建账+交易 push → A pull → 重复 push 幂等（accepted=0）；余额正确；bean-check PASS。

### 9.4 P1-B 冒烟验证（2026-08-13，全新 data/p1b_verify.db，`:8123`，`export_demo/smoke_p1b.sh`）
| 场景 | 期望 | 实际 |
|---|---|---|
| `go build ./...` / `go vet ./...` | 通过 | 通过 ✅ |
| 迁移 002 自动应用（accounts 重建 + 部分唯一索引 + deleted_at） | 启动即生效 | ✅ |
| 首个注册用户 | role=owner | ✅ |
| 后续注册用户访问 `/api/accounts` | 403（未受邀） | 403 ✅ |
| 未加入账本用户使用 `/api/auth/me` | 200（自服务端点） | 200 ✅ |
| 非 owner 访问 `/api/ledger/members` | 403 | 403 ✅ |
| owner 添加 bob（editor）/ 改角色 viewer | 成功 | ✅ |
| `DELETE /api/transactions/{uuid}` | 200 + sync_log delete | ✅ |
| 重复删除同一交易 | 404（幂等） | 404 ✅ |
| `sync/push` 逐条 results（含引用缺失条目） | 单条 error，其余 ok，accepted 只计真正写入 | ✅ |
| `sync/push` 同名账户自然键合并 | server_uuid=先到者，accepted=false | ✅ |
| `change-password` 后旧 token | 401（全部会话失效） | 401 ✅ |
| `refresh` 换 token | 新 token 可用 / 旧 token 401 | ✅ |
| owner `reset-password` 重置成员 | 200 | ✅ |
| 导出 `.beancount` 后 `bean-check` | 退出码 0 | ✅ |

### 9.5 P1-C 冒烟验证（2026-08-14，全新 data/p1c_verify.db，`:8080`）
| 场景 | 期望 | 实际 |
|---|---|---|
| `go build ./...` / `go vet ./...` / `go test ./...` | 通过 | 通过 ✅ |
| 首个注册用户 `GET /api/ledgers` | 返回 1 个账本 id=1 role=owner | ✅ |
| 创建账本 2（POST /api/ledgers） | 返回 id=2 role=owner | ✅ |
| 账本 1 建账户/记账，账本 2 建账户 | 各自列表互不串库（隔离） | ✅ |
| `PATCH /api/ledgers/2` 改名 | 200 + 名称更新 | ✅ |
| owner 添加 bob 为 viewer | 成功 | ✅ |
| viewer 读 `/api/accounts` | 200 | 200 ✅ |
| viewer 写 `POST /api/accounts` | 403「只读成员无写权限」 | 403 ✅ |
| viewer 读非成员账本 2 | 403（非成员） | 403 ✅ |
| viewer `POST /api/sync/push` | 403（只读） | 403 ✅ |
| `/api/export/archive` 导出 zip 解压后 `bean-check main.bean` | 退出码 0 | 0 ✅ |
| 单文件 `/api/export` 回归 bean-check | 退出码 0 | 0 ✅ |
| `flutter analyze lib` | 无 error | 0 error ✅ |

> 注：`bean-check.exe` 在本机直接运行静默返回 1（Windows 包装脚本问题），改用 `.venv/Scripts/python.exe -m beancount.scripts.check` 校验，退出码 0 即通过。

---

## 10. 约定与惯例（给智能体的硬约束）

- **金额**：永远用定点十进制字符串或整数最小单位；**禁止**浮点参与金额运算/存储。导出时按 commodity precision 格式化。
- **Beancount 兼容**：导出文本必须过 `bean-check`。改导出器（`internal/export/beancount.go`）后必须重新跑 bean-check。
- **迁移优先**：任何 schema 改动走 `migrate.go`，幂等、记录 `schema_migrations`。
- **借贷平衡**：后端 `handleCreateTransaction` 已做按币种平衡校验；前端 `main.dart` 记一笔页做实时借贷平衡指示。
- **UUID 引用**：同步相关实体一律用 `uuid` 互引，勿用服务端自增 `id` 跨端传递；注意在线记账用 `account_id`、离线 push 用 `account_uuid`（见 §6.1）。
- **不可变**：交易一旦创建不原地改；改账=新增冲正交易。
- **注释语言**：代码注释用中文；保持 `schema.sql` 中 Beancount 映射注释完整。
- **Git**：本机操作注意 rm 可能被安全软件拦截；删除用工具而非裸删。

---

## 11. 已知边界 / P1 清单

- 跨币种兑换：需 `postings` 加 `cost_commodity`/`cost_amount`；P0 单币种。
- 前端下载用 `dart:html`，WASM 目标告警（JS 目标正常）。
- 多账本隔离、账本级细粒度权限（P1-B3 已做成员角色管理，但**仅默认账本 id=1**）、Web 管理后台 = P1-C。
- 商品/价格（`commodities`/`prices`）尚无 CRUD 端点（仅表 + 导出引用），`screens/commodities/` 占位。
- ~~push 整包事务提交~~ → **已改逐条提交**（P1-B2，§6）。
- ~~sync_log 只有 create~~ → **已支持 delete 软删语义**（P1-B1，§6）。
- ~~认证无重置/刷新/邀请~~ → **已实现** change-password / refresh / owner 添加成员 + 重置成员密码（P1-B3，§5）。
- 真机双端联调（浏览器 A/B 同时在线、离线多设备）尚未在浏览器侧完成端到端确认（后端与模拟脚本已验证）。

---

## 12. 快速运行与测试

```bash
# 后端
export PATH="$PATH:/d/Environment/go/bin"
LEDGER_DB=data/ledger.db go run ./cmd/server        # 默认 :8080，LISTEN=:9000 可覆盖

# 注册并拿 token
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123","display_name":"Alice"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
AUTH="Authorization: Bearer $TOKEN"

# 建账户（记住返回的 id）
curl -s -X POST http://127.0.0.1:8080/api/accounts -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}'
# 记一笔（postings 用 account_id，本例账户 id=1/2）
curl -s -X POST http://127.0.0.1:8080/api/transactions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"date":"2026-08-02","description":"早餐","flag":"*","postings":[{"account_id":1,"commodity":"CNY","amount":"-10.00"},{"account_id":2,"commodity":"CNY","amount":"10.00"}]}'
curl -s http://127.0.0.1:8080/api/accounts -H "$AUTH"
curl -s http://127.0.0.1:8080/api/export -H "$AUTH" -o data/family.beancount
.venv/Scripts/bean-check.exe data/family.beancount    # 退出码 0 即通过

# 前端
cd frontend && export PATH="$PATH:/d/Environment/flutter/bin"
export PUB_HOSTED_URL="https://pub.flutter-io.cn"
flutter pub get && flutter run -d chrome
```

部署到 NAS（纯 Go 免 CGO）：
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o familyledger ./cmd/server
```
建议用反向代理（Nginx/Caddy）加 HTTPS；前端部署把 `api.dart` 的 `http://localhost:8080` 改为 NAS 地址。

---

## 13. 下一阶段任务规划（P1 起，老板自评审验收）

P0 切片、P1-A（前端打磨/离线联调）、P1-B（后端增强）均已闭环并验证（§9）。下一阶段聚焦 **P1-C（多账本/权限）** 与 **上线就绪**（HTTPS 部署、真机双端联调），每项由老板自写验收标准、留档、关键模块自评审。

### P1-A 前端打磨与离线联调（最高优先，决定能否真用）

> 实现状态（2026-08-13）：P1-A1~A4 已全部实现，`flutter pub get` + `flutter analyze` + `flutter build web --release` 均通过（EXIT=0，已确认产出 `build/web/{main.dart.js,flutter_service_worker.js,manifest.json}`）。

- **P1-A1 PWA 外壳** ✅ 已实现：`web/manifest.json` 已定制（家庭记账 / teal 主题色 / 各尺寸图标），`web/index.html` 注册 Service Worker（标题、theme-color、可安装）。Service Worker 由 `flutter build web` **自动生成**（`build/web/flutter_service_worker.js`，仅缓存静态资源清单，天然不缓存 `/api/`）。
  - 验收：浏览器「可安装」提示出现；断网打开首屏不白屏（已构建验证；真机部署可再确认）。
- **P1-A2 离线记账端到端联调** ✅ 已实现：`core/store/local_store.dart` + `core/sync/sync_service.dart` 已接通；`app_shell.dart` 监听 `window.onOnline/onOffline`，联网自动触发 pull 合并 + push 队列；AppBar 显示待推送队列数（橙色角标）。
  - 验收：断网记账 → 联网自动同步 → 双端余额一致；bean-check 仍通过（账户自然键合并已随 P1-B1 落地，见 §6）。
- **P1-A3 改账/冲正 UI** ✅ 已实现：交易列表「冲正」按钮，新增一笔反向交易（分录取负），不改原交易；`LocalTxn` 加 `reversedOf` 字段（后端 push 忽略未知字段，安全）；UI 标记「冲正 / 已冲正」。
  - 验收：冲正后余额正确，导出含原交易 + 冲正交易，bean-check PASS。
- **P1-A4 余额 / 报表页** ✅ 已实现：新增「报表」Tab——净资产概览（按币种）、按账户类型汇总（Assets/Liabilities/Equity/Income/Expenses）、月度交易活跃；余额累加改用 `decimal` 包精确计算（遵守 §10 禁浮点约定）。

### P1-B 后端增强（2026-08-13 已全部实现并验证，见 §9.4）
- **P1-B1 sync_log delete 语义** ✅：`accounts`/`transactions` 软删（`deleted_at`）+ `op=delete` 事件；删账户连带删其引用交易（每个实体各写一条 delete 日志）；`sync/pull` 对端收到 delete 仅 `{uuid}`；导出跳过已删实体。迁移 002 落地（accounts 重建 + 部分唯一索引 `(ledger_id,name) WHERE deleted_at IS NULL`）。
  - 验收：删除类变更能 pull 到对端并应用；导出不出现已删实体 → 已在 `smoke_p1b.sh` 验证。
- **P1-B2 push 逐条提交** ✅：整包事务改为逐条提交，单条失败只记 `results[i].error` 不影响其余；响应 `{checkpoint, accepted, results}`，`results` 按请求原始顺序返回。前端 `_pushLoop` 按逐条结果移除 pending、保留失败项重试。
  - 验收：批量中一条引用缺失，其余成功；响应列出每条结果 → 已在 `smoke_p1b.sh` 验证。
- **P1-B3 认证增强** ✅：`change-password`（改密后全部会话失效 + 签发新 token）、`refresh`（旧 token 立即失效）、成员管理端点（owner 按用户名添加/改角色/移除/重置密码，重置后该用户全部会话失效）。注册语义改为「首个用户 owner，后续用户待邀请」。
  - 验收：见 §9.4。
- **账户自然键合并（原文档缺口）** ✅：迁移 002 建部分唯一索引 `(ledger_id,name) WHERE deleted_at IS NULL`；`applyPushEntity` 推送账户时先按 uuid 幂等跳过，再按未删同名账户合并（保留先到者，`server_uuid` 回传真实 uuid）；前端 `sync_service.dart` 两轮推送用 `server_uuid` 重映射本地 `loc-` uuid 及待推交易引用。

> 前端落点（2026-08-13 实现，无新页面）：`core/sync/sync_service.dart` + `viewmodels/ledger_controller.dart`（逐条 push、`loc-` uuid 校正、delete 回写）；`screens/members/` 已从占位升级为**真实成员管理页**（owner 专属，editor/viewer 只读）；`core/api/ledger_api.dart` 含全部成员/删除/认证端点。
> `screens/commodities/` 仍占位（后端 prices/commodities CRUD 端点尚未实现，见下）。

### P1-C 多账本与权限（产品分水岭）— 2026-08-14 已实现并验证

> 实现状态：P1-C1/C2/C3 + 多文件目录导出全部落地，端到端验证通过（多账本隔离、viewer 只读 403、账本改名、zip 多文件导出 bean-check 退出码 0）。见 §9.5。

- **P1-C1 多账本隔离** ✅：`ledgers` 真正隔离。新增 `/api/ledgers`（列表/创建）与 `/api/ledgers/{id}`（PATCH 设置）；业务端点按 `X-Ledger-Id` 头解析账本（缺失回退默认账本 id=1，兼容旧客户端）；`domain.DefaultLedgerID` 不再被 service 层硬编码（仅作首个账本 id 与回退值）。repository 新增 `ledger.go`（ListLedgersByUser/LedgerByID/CreateLedger/UpdateLedger），`CreateLedgerForOwner` 去掉硬编码 id、改为 AUTOINCREMENT 返回新 id。
- **P1-C2 角色鉴权落地** ✅：`middleware.go` 解析账本后按角色拦截——`viewer` 只读（非 GET 一律 403）、`editor` 可写、`owner` 可管理；`auth` 包新增 `WithLedger/LedgerID/Role` context 存取。成员管理仍由 service 层 `requireOwner` 校验。
- **P1-C3 Web 管理后台** ✅：`screens/settings/ledgers_screen.dart` 真实实现（账本列表/切换/创建/改名，owner 专属改名）；`settings_screen.dart` 加「账本管理」入口（显示当前账本名）。
- **多文件目录导出** ✅（老板新增需求）：`internal/export/files.go` 把账本序列化为 `main.bean`（title + commodity/price 声明 + include）+ `accounts/{assets,liabilities,income,expenses,equity}.bean`（open/close）+ `date/{year}/{year}-{MM}.bean`（按月交易）+ `date/{year}/{year}.bean`（年度入口，include 12 个月）。`/api/export/archive` 打包 zip 下载；解压后对 `main.bean` 跑 bean-check 通过。前端 `import_export` 页加「下载账本目录 (zip)」按钮。

> 前端落点（2026-08-14 实现）：`core/api/ledger_api.dart`（静态 `ledgerId` + `X-Ledger-Id` 头 + listLedgers/createLedger/updateLedger/exportArchive）；`viewmodels/auth_controller.dart`（账本列表 + 当前账本选择，`isMember`=有账本）；`viewmodels/ledger_controller.dart`（`switchToLedger` 切换账本：先推 pending → 清本地 → 重拉）；`core/store/local_store.dart`（`clearAll` 清本地账本数据保留 client_id）；`screens/settings/ledgers_screen.dart`（账本管理页）。

### P2 / 下期（数据能力）
- Beancount 导入（迁移现有 `.beancount`）。
- 自动导入（OFX/CSV）、规则引擎、小票 OCR。
- 跨币种 cost 注解（`postings` 加 `cost_commodity`/`cost_amount`）。
- 商品/价格（`commodities`/`prices`）CRUD 端点 + `screens/commodities/` 页面。
- Postgres 横向扩展（DB 访问层已抽象）。

> 执行顺序建议：P1-A ✅ → P1-B ✅ → **P1-C**（多账本隔离 → 角色鉴权 → 管理后台）→ 上线就绪（HTTPS 反代部署、真机双端联调、PWA 安装实测）。P2 视使用反馈再排。
