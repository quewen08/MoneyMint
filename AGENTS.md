# AGENTS.md — 家庭协作记账系统（类 Beancount 云记账）

> 项目给 AI 助手的一致上下文。会话开始前先读。
> 用户：全栈独做（老板），后端 Go / 前端与产品决策均本人拍板。沟通可直接给结论。
> 提醒：因独做全栈，"分配负责人/邀请角色关注"类提醒不适用，改为提醒其自写验收标准、留档、关键模块自评审。

**文档分工**：AI / 开发者技术上下文 = 本文；用户与部署快速上手 = [`README.md`](README.md)；Claude 系工具的工作约定 = [`CLAUDE.md`](CLAUDE.md)；M7 重构的**方案、决策、验收与风险** = [`docs/next-phase-frontend-plan.md`](docs/next-phase-frontend-plan.md)（当前 v2.3，D-1~D-7 已锁定）。**本文只记「已实现的现状」，方案里的待办见 §9，勿把计划当现状。**

---

## 1. 项目概述

跑在 NAS 个人云上的**复式记账应用**，对标 Beancount 数据模型：
- 后端：Go 标准库 + `modernc.org/sqlite`（纯 Go，免 CGO，便于 NAS arm64 交叉编译）。
- 数据库：SQLite（开发期）；后期平滑切 Postgres（DB 访问层已抽象）。
- 家庭成员**共享同一账本**；支持离线多端记账 + 增量同步。

**前端正处于 M7 全量重构的双版并存期（关键，勿搞混）**：

| | 生产主前端 | 新版（开发中） |
|---|---|---|
| 技术 | Flutter Web（PWA） | **Vue 3 + Vite 纯 SPA**（不引入 Nuxt / SSR / Nitro） |
| 源码 | `frontend/lib/`、`frontend/test/`、`pubspec.*` | `frontend/index.html`、`frontend/src/`、`frontend/public/` |
| 产物 | `frontend/build/web/`（现在由 `LEDGER_WEB_ROOT` 托管） | `frontend/dist/`（gitignore） |
| 状态 | **当前仍是默认可用版本**，cutover 后清退 | **批 0/1 骨架阶段**，仅 10 个源文件（App / HomeView / Money / api / router / styles），页面与离线内核未复刻完成 |

- `frontend/dist/legacy/` 是 Flutter 旧版用 `--base-href /legacy/` **重建后复制**的回滚产物（`scripts/build-legacy.sh` + `check-legacy.sh` 校验），不是简单的目录搬运。
- **尚未 cutover**：根路径仍指向 Flutter 产物；Vue 版切根路径、清退 Dart、Dockerfile 改 Node 构建都在批 6。
- 后端与账芯**不变**：31 个 REST 端点、复式借贷平衡、append-only 不可变交易、`bean-check` 导出校验、离线同步协议全部保留。

**核心不变约束**：导出的 `.beancount` 必须能被 `bean-check` 直接校验通过（退出码 0、无报错）。

**当前阶段**：M0 / M1 / M3 / F-1~F-10 前端改版均已完成；M7 推进到**批 0~1**——Vue 脚手架、定点金额 `Money`、D-7 httpOnly Cookie 认证、迁移 013 幂等已落地并实测通过。M2（恢复 + 导入）、迁移 014（库世代）、离线内核重写、Dockerfile / CI 改造**均未开工**。

---

## 2. 技术栈与本机环境

| 工具 | 版本 / 路径 | 说明 |
|---|---|---|
| Go | 1.26.5，`/d/Environment/go/bin` | Git Bash 需 `export PATH="$PATH:/d/Environment/go/bin"` |
| Node.js | `>=20`（`frontend/package.json` `engines`）；方案钉版目标 **22.23.4** | 仅**构建阶段**需要，运行时镜像无 Node。`.nvmrc` / `.npmrc` 尚未入库；`package-lock.json` 目前由 Windows 生成，**镜像内 `npm ci` 前须在 Linux/WSL 重生成** |
| Flutter | 本地 **3.38.9**（Dart 3.10.8），`/d/Environment/flutter/bin` | **仅旧版**维护与 `dist/legacy/` 重建用；Docker 镜像内固定 3.35.1，版本差会让 `pubspec.lock` 在镜像内重解析（详见 `Dockerfile` 注释） |
| Python | 项目内 `.venv` | 仅用于 `bean-check`（`beancount` 包），不污染系统 |
| GOPROXY | `https://goproxy.cn,direct` | `proxy.golang.org` 国内不可达 |
| PUB_HOSTED_URL | `https://pub.flutter-io.cn` | 旧版 `flutter pub get` 国内镜像 |
| 前端依赖 | 见 `frontend/package.json` | 版本按方案 §14.7 钉版；**禁装** ECharts / Vuetify / Framework7 / moment / monaco / leaflet / swiper |

Vue 侧关键钉版：`vue ^3.5`、`vue-router ^4.5`、`pinia ^2.3`、`decimal.js ^10.4`、`vite ^5.4`、`@vitejs/plugin-vue ^5.1`、`@vitejs/plugin-legacy ^5.4`、`typescript ~5.6`、`vitest ^2.1`、`vue-tsc ^2.1`、`es-check ^7`。**不要照抄参考项目 ezbookkeeping 的 Vite 8 / TS 6 / vue-router 5 / Pinia 4 / `target: safari16.4`**，那会在 iOS 14.8 上「全绿白屏」。

验证用（已建好可跳过）：
```bash
python3 -m venv .venv
.venv/Scripts/python.exe -m pip install beancount
```

---

## 3. 目录结构

```
.
├── schema.sql            # 初始核心表 DDL（含 Beancount 映射注释）
├── cmd/server/main.go    # 后端入口
├── internal/             # 后端分层：config → domain → db → repository → service → handler → server
│   ├── db/{db,migrate}.go   # 开库 + schema 迁移（当前至 013）；迁移框架见 §4
│   ├── repository/       # 全部 SQL 集中于此（account/transaction/member/sync/session/user/ledger/opening）
│   ├── service/          # 业务：校验/事务编排/余额/自然键合并/同步协议/成员权限
│   ├── handler/          # 薄 HTTP 适配层
│   ├── server/           # 组合根：路由 + CORS/CSRF/auth 中间件 + 静态托管
│   ├── auth/auth.go      # bcrypt / token / context 用户态（纯函数）
│   ├── backup/           # 一致性快照（VACUUM INTO）+ 定时调度 + 保留清理
│   └── export/{beancount,files}.go  # 单文件 + 多文件目录导出
├── frontend/             # 双版并存（M7 过渡期）
│   ├── lib/ test/ pubspec.* web/ build/  # 旧版 Flutter Web 源码与产物
│   ├── index.html        # 新版 Vite 入口
│   ├── package.json / vite.config.ts / tsconfig*.json
│   ├── public/           # manifest.json + icons/ + favicon.png（新版静态资源）
│   ├── src/              # 新版源码：api/ core/ router/ styles/ views/ + main.ts / App.vue
│   └── dist/             # 新版构建产物（gitignore），含 legacy/ 回滚目录
├── scripts/              # check-bean / verify-version / smoke / local.sh / build-legacy / check-legacy / copy-legacy.mjs
├── export_demo/          # Python/sh 验证参考
├── data/                 # 运行时 SQLite（git 忽略）；backups/ 也在其下
└── docs/                 # 设计规格 / 同步协议 / 原型 / 任务书 / M7 方案（next-phase-frontend-plan.md v2.3）
```

**旧版前端架构（仍需维护到 cutover）**：MVVM 分层 + 响应式双布局（≥1024 走 PC 侧边栏，<1024 降级移动底栏）。
- 根 `AppScope`（`app/scope.dart`，`InheritedWidget`）注入 `LedgerApi`/`LocalStore` 与两个 controller。
- 本地真相源 `core/store/local_store.dart`（sembast+IndexedDB）；同步编排 `core/sync/sync_service.dart`。
- **坑（务必遵守）**：页面读取 `AppScope.of(context)` 必须放在 `build`/`didChangeDependencies`，**禁止**放 `initState`（会触发 `dependOnInheritedWidgetOfExactType called before initState completed` 红屏）。

**新版前端架构（批 0/1 已落地部分）**：
- `core/Money.ts`：`decimal.js` 定点金额唯一入口（只收 string 构造、`ROUND_HALF_UP`、`toString()` 固定 `toFixed()`），配套 `Money.test.ts` 8 个用例。
- `api/{types,routes,errors}.ts`：31 端点手写 TS 类型 / 路由常量 / `ApiError`（`router.go` 是命令式注册，无 OpenAPI 可反射，故**不生成**客户端）。
- `router/index.ts`、`styles/tokens.css`（取自 `docs/prototype/`）、`views/HomeView.vue`。
- `vite.config.ts`：`base: './'`、`build.target: 'es2015'`、`@vitejs/plugin-legacy`（`targets: Safari >= 14`）、`outDir: 'dist'`（相对 Vite root，即 `frontend/dist`）、`server.proxy['/api'] → 127.0.0.1:8080`、alias `@`。
- **尚未实现**：20 个页面、Pinia stores、类型化 fetch 客户端、`sync/` 离线内核、`sw.ts` / PWA、CI `web-verify` job。

---

## 4. 数据库 Schema

金额一律 `NUMERIC`；应用层以**定点十进制字符串**或最小货币单位整数写入，**禁止浮点**。
同一 transaction 下，按 commodity 分组的 `postings.amount` 之和必须为 0（有符号数，可负）。

核心表（`schema.sql`，幂等 `IF NOT EXISTS`）：
- `users`(id, username UNIQUE, password_hash, display_name, status)
- `ledgers`(id, name, owner_id, default_commodity)
- `ledger_members`(ledger_id, user_id, role[owner|editor|viewer]) — 权限在此层
- `accounts`(id, ledger_id, uuid, name, type[5类], open_date, close_date, commodity_restriction, deleted_at)
- `commodities`(id, ledger_id, uuid, symbol, name, precision)
- `prices`(id, ledger_id, uuid, commodity, currency, date, rate, source)
- `transactions`(id, ledger_id, uuid UNIQUE, date, flag[*|!], description, version, created_by, deleted_at, **client_request_id**)
- `postings`(id, transaction_id, account_id, commodity, amount 有符号, position)

**迁移（`internal/db/migrate.go`，追加式、幂等、记录 `schema_migrations`；当前至 013）**：
- 001_sync：补 `accounts/commodities/prices` 的 `uuid`+`updated_at`；建 `sync_log`(id 自增即全局水位)；重建 `sync_checkpoints`；建 `sessions`。
- 002_delete_semantics：交易/账户加 `deleted_at`（软删）；账户重建部分唯一索引 `(ledger_id,name) WHERE deleted_at IS NULL`；旧库 owner 密码置为初始默认值。
- 003~009：账户显示名 / 视觉与层级字段、交易 `tags`、关户语义、账户排序、`users.is_admin`、`app_settings`。
- 010~012：强制改密、会话设备信息、期初余额（`opening_key` 等）。
- **013_txn_idempotency（M7 批 1，已实现）**：`transactions.client_request_id TEXT` + 部分唯一索引 `UNIQUE(ledger_id, client_request_id) WHERE client_request_id IS NOT NULL`；配套 `migrate013_test.go` 与 `server/idempotency_test.go`。
- **014（库世代 `db_instance_id`）尚未开工**，属 M7 批 4，见 §9。

> **修改 schema 必须走迁移脚本**，不要直接改 `schema.sql` 后手动 ALTER 线上库。
> **幂等实现边界**：落库用 `INSERT ... ON CONFLICT(ledger_id, client_request_id) WHERE client_request_id IS NOT NULL DO NOTHING`（conflict target 必须重复部分索引谓词，否则 SQLite 报 no matching unique index）；业务行与 `sync_log` 必须同事务。当前并发安全还隐式依赖 `MaxOpenConns=1`，放开连接数或切 Postgres 前必须补齐。

---

## 5. 认证与权限模型

家庭账本是**共享账本**：未登录 401 不可见 / 账本成员可见共享数据 / 非成员 403。
- 密码 bcrypt 加盐；登录签发 32 字节 token（存 `sessions`，有效期 30 天），退出即删、旧 token 立即失效；`POST /api/auth/refresh` 换新 token（旧立即失效）。
- 注册语义：首个注册用户创建默认账本并成为 owner；后续注册仅建账号，等待 owner 按用户名添加为成员（业务端点对未加入者返回 403）。
- 成员管理仅 owner 可操作（列表/添加/改角色/移除/重置密码——重置使该用户全部会话失效）。角色：`owner | editor | viewer`。
- `authMiddleware` 包裹所有业务路由；放行 `/api/health` 与 register/login；自服务端点仅需登录态，业务端点额外校验为账本成员。
- **会话凭据（D-7 = A，已实现）**：默认优先读 `fambean_session` httpOnly Cookie（`Path=/`、`SameSite=Lax`、30 天；内网 HTTP 不设 `Secure`），`Authorization: Bearer` 兜底；空串视为未登录，旧版 `/legacy/` 继续用 Bearer 不受影响。**Bearer 分支在观察期结束前不得删除。**
- **CSRF 只约束 Cookie 路径**：非 GET/HEAD/OPTIONS 的 Cookie 会话必须带 `X-CSRF`，缺失 403 `csrf_required`；显式 Bearer 请求豁免。`LEDGER_AUTH_COOKIE=false` 时完全退回 Bearer 旧行为（不下发、不读取 Cookie，也不要求 CSRF）。
- **兼容期**：`LEDGER_AUTH_EXPOSE_TOKEN=true`（默认）时登录 / 刷新 / 改密响应体仍返回 `token` 供旧版使用；批 6 cutover 后置 `false`。
- 登录失败限流：`LEDGER_LOGIN_MAX_ATTEMPTS` / `WINDOW_MINUTES` / `LOCKOUT_MINUTES`；强制改密态对业务端点返回 **428**，新前端需跳改密页而非显示通用错误。

---

## 6. 离线同步协议

**append-only + 不可变 + 水位增量**（天然几乎无冲突）。
- 不可变交易 + uuid 身份键：改账 = 新增冲正交易，绝不覆盖；删除 = 软删 + `op=delete`。
- `sync_log` 全局水位：每次接受 create/delete 追加一行，`id` 单调即服务端水位。
- pull：`GET /api/sync/pull?since=<水位>&client_id=<设备>` 取回 `id>since` 的全部变更（delete 仅 `{uuid}`）。
- push：`POST /api/sync/push` 逐条提交，单条失败不阻塞其余；响应 `{checkpoint, accepted, results:[{index, entity_type, uuid, status, accepted, server_uuid, error}]}`（按请求顺序）。先 create（commodity→account→transaction）再 delete（transaction→account）。
- 自然键合并：推送账户时若同账本存在未删同名账户，保留先到者，`server_uuid` 回传真实 uuid，前端据此重映射本地 `loc-` uuid。
- 前端离线优先：`local_store` 为本地真相源，断网可记账；联网先 pull 合并再 push 本地队列，失败保留重试。

**关键陷阱（勿混用）**：
| 路径 | posting 账户字段 | 取值 |
|---|---|---|
| 在线 `POST /api/transactions` | `account_id` | 服务端自增整数 id |
| 离线 `POST /api/sync/push` | `account_uuid` | 字符串 uuid；所有 posting 必须带，不能用账户名 |

**M7 批 4/5 待落地的硬约束**（方案 §6.6.1 / §7，勿提前宣称已有）：
- **库世代**：restore 覆盖 DB 后 `sync_log` 自增回退，客户端 `last_seq` 超前会「永远拉不到且不报错」。迁移 014 增 `app_settings.db_instance_id`（建库写入、restore 成功后重写），pull/push/health 返回 `instance_id`，客户端按 `{ledger_id:{instance_id,last_seq}}` 比对，不匹配则清该账本实体 + 全量 pull，pending 转隔离区不静默删。
- **IndexedDB 原子写**：只允许 `recordEntry` / `applyPushResults` / `applyPullChanges` 三个跨 store 的单事务原子操作；`last_seq` 仅在变更成功应用后单调推进。
- **多标签页租约**：Safari 14.1 无 `BroadcastChannel` / `navigator.locks`，用 `storage` 事件选举 + IndexedDB 单记录 CAS 租约；401 刷新也只由 leader 执行。
- **失败分类 / 死信**：pending 增 `attempts / last_error_code / next_retry_at / state`；4xx 按码转 dead 并在同步页可见，网络与 5xx 指数退避封顶。

---

## 7. API 端点清单

| 端点 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/api/health` | GET | 公开 | `{"status":"ok"}` |
| `/api/auth/register` | POST | 公开 | 首个用户建默认账本成 owner，后续仅建账号待邀请；下发 Cookie（兼容期响应体仍含 token） |
| `/api/auth/login` | POST | 公开 | 登录，下发 Cookie |
| `/api/auth/logout` | POST | 登录 | 注销当前会话并清 Cookie |
| `/api/auth/me` | GET | 登录 | 当前用户（role 恒空串） |
| `/api/auth/refresh` | POST | 登录 | 换发新 token（旧立即失效）并重设 Cookie |
| `/api/auth/change-password` | POST | 登录 | 改密后该用户全部会话失效并重设 Cookie |
| `/api/auth/sessions` | GET | 登录 | 会话列表（M3） |
| `/api/auth/sessions/revoke-others` | POST | 登录 | 吊销其他设备 |
| `/api/auth/sessions/{sid}` | DELETE | 登录 | 吊销指定会话 |
| `/api/ledgers` | GET/POST | 登录 | 列出所属账本（含角色）/ 建新账本 |
| `/api/ledgers/{id}` | PATCH | 登录+**owner** | 账本改名 / 默认币种 |
| `/api/accounts` | GET/POST | 登录+成员 | 账户列表（含余额）/ 新建 |
| `/api/accounts/{uuid}/opening` | GET/POST | 登录+成员 | 读取 / 设置期初余额（含冲正，走 `opening_key`） |
| `/api/accounts/{uuid}` | DELETE | 登录+成员 | 软删账户（连带引用交易，写 sync_log delete）；幂等 404 |
| `/api/identity/check` | GET | 登录+成员 | 恒等式检查 |
| `/api/transactions` | GET/POST | 登录+成员 | 流水 / 记一笔（postings 用 `account_id`；按币种借贷平衡校验；**支持 `client_request_id` 幂等**） |
| `/api/transactions/trash` | GET | 登录+成员 | 回收站列表 |
| `/api/transactions/{uuid}/restore` | POST | 登录+成员 | 恢复软删交易 |
| `/api/transactions/{uuid}` | DELETE | 登录+成员 | 软删交易；幂等 404 |
| `/api/ledger/members` | GET/POST | 登录+**owner** | 成员列表 / 按用户名添加已注册用户 |
| `/api/ledger/members/{userID}` | POST/DELETE | 登录+**owner** | 改角色 / 移除 |
| `/api/ledger/members/{userID}/reset-password` | POST | 登录+**owner** | 重置密码并使该用户全部会话失效 |
| `/api/export` | GET | 登录+成员 | 导出单文件 `.beancount` |
| `/api/export/archive` | GET | 登录+成员 | 导出多文件目录 zip |
| `/api/sync/pull` | GET | 登录+成员 | 增量拉取（since + client_id） |
| `/api/sync/push` | POST | 登录+成员 | 逐条幂等提交离线变更 |
| `/api/backup/list` | GET | 登录+**系统管理员** | 备份清单 |
| `/api/backup/download?name=` | GET | 登录+**系统管理员** | 下载备份；文件名白名单 + 四层穿越防护 |
| `/api/backup/run` | POST | 登录+**系统管理员** | 手动触发一次备份（进行中 409） |
| `/api/settings/registration` | GET/POST | **GET 公开** / POST 登录+**系统管理员** | 读取 / 修改注册开关（GET 供登录页判断是否显示注册入口） |
| `/*`（非 `/api/`） | GET/HEAD | 公开 | 前端静态资源 + SPA 回退（`LEDGER_WEB_ROOT`） |

> **在线记账幂等冲突矩阵（迁移 013，已实现）**：新建 → `201 {uuid, idempotent:false}`；同 key 命中未删 → `200 {uuid, idempotent:true}`；命中已软删 → `409 idempotency_key_deleted`（换新 key 重提）；同 key 不同 payload → `409 idempotency_payload_mismatch`。旧客户端不传 `client_request_id` 时保持原行为。冲正沿 `opening_key`，**禁止复用** `client_request_id`。
> **M2 四个端点**（`/api/backup/restore/preview`、`/api/backup/restore`、`/api/import/beancount/preview`、`/api/import/beancount`）**尚未实现**，见 `docs/m2-plan.md`；届时用 `Idempotency-Key` 头 + 通用 `idempotency_keys` 表。
> 多账本上下文：业务端点需带 `X-Ledger-Id` 头；缺失回退默认账本 id=1。`viewer` 只读（非 GET 一律 403）、`editor` 可写、`owner` 可管理。**新前端 API 客户端必须强制注入并断言该头，缺失直接抛错，绝不静默回退**（否则交易会记进错误账本）。
> 备份端点例外：备份是全库级产物，**不解析 `X-Ledger-Id`、不校验账本成员**，只要求登录态 + `users.is_admin`。账本级 owner ≠ 系统管理员，勿混用。
> 鉴权范围按 `/api/` 前缀划分：非 `/api/` 一律放行交由静态托管，新增前端路由无需改白名单。
> CORS：`LEDGER_CORS_ORIGINS` 未设 = 仅同源（不输出 CORS 头）；设为逗号白名单 = 命中回显 `Origin` + `Allow-Credentials: true` + `Vary: Origin`；设为 `*` = 旧行为但不带凭证。Headers 含 `Content-Type, Authorization, X-Ledger-Id, X-CSRF`，Methods 含 `GET,POST,DELETE,PATCH,OPTIONS`，`OPTIONS` 预检 204 不鉴权。
> 错误结构统一 `{error, code?, ...details}`（`internal/handler/handler.go` 的 `writeError`）。前端边界：**4xx 不重试、5xx 最多 3 次、fetch 失败显示离线提示并保留输入**；错误码到用户文案的映射集中一处，不把 `HTTP 500` 原文甩给用户。

---

## 8. 约定与惯例（给 AI 的硬约束）

- **金额**：永远用定点十进制字符串或整数最小单位；**禁止浮点参与金额运算/存储**。后端按 commodity precision 格式化；旧版用 `package:decimal`，新版一律走 `frontend/src/core/Money.ts`（`decimal.js`），**禁止 JS 裸 `+/-`**。导出文件必须过 `bean-check`。
- **Beancount 兼容**：改 `internal/export/beancount.go` / `files.go` 后必须重新跑 bean-check。**测试中 `BEAN_CHECK_STRICT=1` 时找不到校验器即失败**，不要改回静默跳过（会形成「测试全绿但账芯从未被验证」的假绿灯）。
- **备份必须用 `VACUUM INTO`，禁止复制 db 文件**：WAL 模式下未 checkpoint 的数据在 `-wal` 里。实测主库 188 KB / WAL 4.1 MB 时直接复制只得到 236/300 条记录，且**不报错**。快照后必须过 `PRAGMA integrity_check` 与行数比对。
- **全库级操作要系统管理员**：备份 / 下载 / 恢复校验 `users.is_admin`，不能用账本级 `owner` 代替。
- **迁移优先**：任何 schema 改动走 `migrate.go`，幂等、记录 `schema_migrations`；新增编号顺延（当前 013，下一个是 014）。
- **借贷平衡**：后端 `handleCreateTransaction` 按币种平衡校验；前端记一笔页实时指示。
- **UUID 引用**：同步实体一律用 `uuid` 互引，勿用服务端自增 `id` 跨端传递；在线用 `account_id`、离线 push 用 `account_uuid`（§6）。
- **不可变**：交易一旦创建不原地改；改账 = 新增冲正交易。
- **会话安全**：token 不落 `localStorage` / URL；Cookie 路径非 GET 必须带 `X-CSRF`；`SameSite=None` 禁用；Bearer 兜底观察期结束前不删。
- **兼容基线（iOS 14.8 / Safari 14.1.2）**：`build.target: 'es2015'` + plugin-legacy；**禁 WASM / WebGL / CanvasKit / OffscreenCanvas**；禁用 `structuredClone` / `crypto.randomUUID` / `.at()` / `findLast` / `Object.hasOwn` / `showModal` / `BroadcastChannel` / `navigator.locks`；CSS 不用 `aspect-ratio` / `:has()` / `dvh` / `inset`。`es-check` 只查语法，须另补 API / CSS 扫描；**升级任何前端依赖都要过 AC-19 与真机冒烟**，不允许「本地能跑就升」。
- **`/legacy/` 回滚**：旧版产物必须 `flutter build web --release --base-href /legacy/` **重建后复制**到 `frontend/dist/legacy/`（不能只搬目录，否则 `<base href="/">` 会把资源打到根路径被 Vue 接管）；`npm run build` 的 `emptyOutDir` 会清空 `dist/`，**每次 `web-build` 后必须重跑 `web-legacy`**。cutover 前还需部署根 `sw-kill.js` 清旧 SW。
- **注释语言**：代码注释用中文；保留 `schema.sql` 中 Beancount 映射注释。

---

## 9. 已知边界 / 当前待办（M7，按方案 v2.3）

**已落地（批 0~1，实测通过）**：Vue 3 + Vite 脚手架、`core/Money` 定点金额（Vitest 8/8）、`es2015` + plugin-legacy + `es-check`、`/api` dev proxy、31 端点手写 TS 类型、D-7 httpOnly Cookie + CSRF + CORS 白名单、迁移 013 幂等、`dist/legacy/` 回滚产物与校验脚本、`web-*` 命令族。

**未开工 / 勿当现状**：
1. Vue 端 20 个页面与 Pinia stores、类型化 fetch 客户端（含 401 single-flight / 428 / `X-Ledger-Id` 断言）、旧版 33 用例迁移（目前仅 8 个 Money 用例）。
2. 离线同步内核 `sync/`（IndexedDB 原子写 / 租约 / 死信 / `uuid_map` 回放）与契约测试（批 5）。
3. 迁移 014 库世代 + `instance_id`（批 4，M2 恢复的前置阻塞）。
4. M2 四个端点与导入 / 恢复向导（批 4）。
5. PWA：`vite-plugin-pwa` injectManifest + `src/sw.ts`（只缓存应用壳与静态资源，**不缓存 API 响应**）；`sw-kill.js`（批 5/6）。
6. **CI `web-verify` job 尚未加入**；`verify-version.sh` 尚未改读 `frontend/package.json`；`.nvmrc` / `.npmrc` 未入库；`package-lock.json` 需在 Linux/WSL 重生成，否则镜像 `npm ci` 会缺 rollup/esbuild 平台二进制。
7. **`Dockerfile` 仍是 Flutter 三阶段**（Stage 1 未换 `node:22-alpine`），`docker-compose.yml` 未改；批 6 才动。
8. cutover、真机全主流程冒烟、观察期后删 Dart 与 Flutter 构建链（批 6）。
9. 下期（M4+）：Beancount 通用语法导入、OFX/CSV 自动导入、规则引擎、商品/价格 CRUD、跨币种 cost 注解、Postgres 横向扩展。
10. 真机双端联调（浏览器 A/B 同时在线、离线多设备端到端）尚未确认。

> 旧版功能在新版**一律 Must，不得降级**：CSV 导出、分类拖拽排序、导出不平告警、注册开关、默认账户管理详见方案 §14.5。

---

## 10. 快速运行与测试

```bash
# 后端（当前 LEDGER_WEB_ROOT 默认托管 Flutter 旧版产物）
export PATH="$PATH:/d/Environment/go/bin"
LEDGER_DB=data/ledger.db LEDGER_WEB_ROOT=frontend/build/web go run ./cmd/server   # 默认 :8080

# 新版 Vue 前端开发（Vite dev server，/api 已代理到 127.0.0.1:8080）
cd frontend && npm ci && npm run dev        # 另开一个终端跑后端
# 或仓库根：bash scripts/local.sh web-dev

# 注册 / 登录（兼容期 Bearer 仍可用；推荐用 Cookie + X-CSRF）
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123","display_name":"Alice"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
AUTH="Authorization: Bearer $TOKEN"

# Cookie 写法（生产路径）
# curl -c /tmp/fb.cookie -X POST .../api/auth/login -H 'Content-Type: application/json' -d '{...}'
# curl -b /tmp/fb.cookie -X POST .../api/transactions -H 'X-CSRF: 1' -H 'Content-Type: application/json' -d '{...}'

# 建账户 / 记一笔（postings 用 account_id）/ 导出校验
curl -s -X POST http://127.0.0.1:8080/api/accounts -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}'
curl -s -X POST http://127.0.0.1:8080/api/transactions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"date":"2026-08-02","description":"早餐","flag":"*","client_request_id":"demo-001","postings":[{"account_id":1,"commodity":"CNY","amount":"-10.00"},{"account_id":2,"commodity":"CNY","amount":"10.00"}]}'
curl -s http://127.0.0.1:8080/api/export -H "$AUTH" -o data/family.beancount
.venv/Scripts/python.exe -m beancount.scripts.check data/family.beancount   # 退出码 0 通过

# 旧版 Flutter（cutover 前仍需维护）
cd frontend && export PATH="$PATH:/d/Environment/flutter/bin"
flutter pub get && flutter run -d chrome

# NAS 交叉编译（纯 Go 免 CGO）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o familyledger ./cmd/server
```

> 本机 `bean-check.exe` 直接运行静默返回 1（Windows 包装脚本问题），改用 `.venv/Scripts/python.exe -m beancount.scripts.check` 校验。
> 实测（2026-11-10）：`bash scripts/local.sh test`（Go 全量 + 严格 bean-check）、`web-test`（Vitest 8/8）、`web-typecheck`、`web-compat` 均通过。

### 10.1 统一命令入口

本机 Windows 无 `make`，用 `scripts/local.sh`；Linux/CI 用 `Makefile`（两者一一对应）。

```bash
# 后端
bash scripts/local.sh test          # go vet + 全量测试（严格 bean-check）
bash scripts/local.sh check         # test + 版本校验 + bean-check 自检（CI 同款）
bash scripts/local.sh smoke         # 端到端：注册→记账→导出→bean-check→备份→下载→穿越防护
bash scripts/local.sh dev           # 起后端并托管 frontend/build/web（Flutter 旧版）
bash scripts/local.sh rehearse      # 迁移副本库预演

# 新版 Vue 前端
bash scripts/local.sh web-build     # Vite 构建 → frontend/dist
bash scripts/local.sh web-dev       # Vite dev server（/api → 127.0.0.1:8080）
bash scripts/local.sh web-test      # Vitest
bash scripts/local.sh web-typecheck # vue-tsc --noEmit
bash scripts/local.sh web-compat    # es-check es2015 扫描 dist/assets
bash scripts/local.sh web-legacy    # 重建 Flutter 旧版 → dist/legacy 并校验基路径

# 旧版 Flutter
bash scripts/local.sh web           # flutter build web --release（产物 frontend/build/web）
bash scripts/local.sh test-web      # flutter analyze lib + flutter test
```

> `web-build` 会清空 `frontend/dist`，**每次构建后需补跑 `web-legacy`**，否则 `/legacy/` 回滚目录缺失。
> `check` 目前**只覆盖 Go**，Vue 门禁尚未进 CI（见 §9）。

### 10.2 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `LEDGER_DB` | `data/ledger.db` | SQLite 路径 |
| `LISTEN` | `:8080` | 监听地址 |
| `LEDGER_SCHEMA` | `schema.sql` | 初始 DDL（仅首次建库用） |
| `LEDGER_WEB_ROOT` | `./web` | 前端产物目录；不存在则不托管前端，服务仍启动 |
| `LEDGER_OWNER_USER` / `LEDGER_OWNER_PASSWORD` | 空 | 预设 owner 账号（空库时生效） |
| `LEDGER_REGISTRATION_ENABLED` | `true` | 注册开关初始值（之后界面值优先） |
| `LEDGER_BACKUP_ENABLED` | `true` | 自动备份开关 |
| `LEDGER_BACKUP_DIR` | `{DBPath 目录}/backups` | 备份目录（可指向外部挂载盘防单盘故障） |
| `LEDGER_BACKUP_HOUR` | `3` | 每日触发小时（0-23，按进程时区） |
| `LEDGER_BACKUP_KEEP_DAILY` | `14` | 每日备份保留份数 |
| `LEDGER_BACKUP_KEEP_MONTHLY` | `12` | 月度备份保留份数 |
| `LEDGER_LOGIN_MAX_ATTEMPTS` | `5` | 登录失败次数上限（0 关闭限流） |
| `LEDGER_LOGIN_WINDOW_MINUTES` | `15` | 失败计数窗口（分钟） |
| `LEDGER_LOGIN_LOCKOUT_MINUTES` | `10` | 超限锁定时间（分钟） |
| `LEDGER_AUTH_COOKIE` | `true` | 会话是否用 httpOnly Cookie；`false` 完全退回 Bearer（不下发/不读 Cookie、不校验 CSRF） |
| `LEDGER_COOKIE_SECURE` | `false` | `Set-Cookie` 的 `Secure`；HTTPS 部署置 `true`（内网 HTTP 保持默认） |
| `LEDGER_CORS_ORIGINS` | 空 | 跨域来源白名单（逗号分隔）；空=仅同源不输出 CORS 头，`*`=通配但不带凭证 |
| `LEDGER_AUTH_EXPOSE_TOKEN` | `true` | 登录/刷新/改密响应体是否返回 `token`；兼容期须为 `true`，批 6 cutover 后置 `false` |
| `BEAN_CHECK_STRICT` | 空 | 测试用；`1` 时找不到 bean-check 直接失败（CI 必设） |

> `config.Validate()` 启动时校验备份小时、保留份数、登录限流与 CORS 来源格式，非法即刻失败（快速失败，不静默回退）。

### 10.3 容器化

```bash
docker build -t money-mint:dev .                              # 当前仍是三阶段：Flutter → Go → alpine
docker run -d -p 8080:8080 -v $(pwd)/data:/app/data money-mint:dev
docker compose up -d                                          # 家用部署（见 docker-compose.yml）
```

> **Dockerfile 尚未改为 Node 构建**（方案批 6）：Stage 1 仍是 Flutter SDK 3.35.1，产物从 `frontend/build/web` 拷到 `/app/web`。切到 Vue 产物时只需把 Stage 1 换 `node:22-alpine`（先 `COPY package*.json` 再 `npm ci` 最后 `npm run build`），产物目录改为 `frontend/dist`；**Stage 2（Go）、Stage 3（alpine 运行时）、`docker-entrypoint.sh`、`docker-compose.yml` 全不动**，单容器单端口模型不变。
> 镜像内 `npm ci` 前务必用 Linux/WSL 重生成 `package-lock.json`（见 §2 / §9）。

> **数据目录权限**：容器由 `docker-entrypoint.sh` 以 root 启动，先把数据目录属主修正为 10001，
> 随即用 `su-exec` 降权执行，PID 1 始终是非 root 的服务进程。因此 bind mount 到任意目录都能用，
> 部署方无需手工 `chown`。若要「全程非 root」，compose 里指定 `user: "10001:10001"`，
> 入口脚本检测到非 root 会跳过修正（此时需自行保证目录对 10001 可写）。
>
> **为什么必须修属主**：SQLite 在 WAL 模式下除主库文件外还要在同目录创建 `-wal` / `-shm`，
> 目录不可写会报 `unable to open database file (14)`。预置一个空 db 文件并不能绕过——
> 文件存在只解决一半，目录权限才是根因。

镜像只在打 tag 时由 GitHub Actions 构建并推送 Docker Hub（`linux/amd64` + `linux/arm64`），
workflow 先跑完校验（版本一致 + 严格 bean-check）才允许推镜像。
> 建议用 Nginx/Caddy 反代加 HTTPS；前端与 API 同端口同域部署，Vue 产物用相对 `/api` 与 `base: './'`，反代到子路径也无需注入后端地址。
