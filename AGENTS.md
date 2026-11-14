# AGENTS.md — 家庭协作记账系统（类 Beancount 云记账）

> 项目给 AI 助手的一致上下文。会话开始前先读。
> 用户：全栈独做（老板），后端 Go / 前端与产品决策均本人拍板。沟通可直接给结论。
> 提醒：因独做全栈，"分配负责人/邀请角色关注"类提醒不适用，改为提醒其自写验收标准、留档、关键模块自评审。

**文档分工**：AI / 开发者技术上下文 = 本文；用户与部署快速上手 = [`README.md`](README.md)；Claude 系工具的工作约定 = [`CLAUDE.md`](CLAUDE.md)。**本文只记「已实现的现状」，待办见 §9，勿把计划当现状。**

---

## 1. 项目概述

跑在 NAS 个人云上的**复式记账应用**，对标 Beancount 数据模型：
- 后端：Go 标准库 + `modernc.org/sqlite`（纯 Go，免 CGO，便于 NAS arm64 交叉编译）。
- 数据库：SQLite（开发期）；后期平滑切 Postgres（DB 访问层已抽象）。
- 家庭成员**共享同一账本**；支持离线多端记账 + 增量同步。

**前端已完成 M7 全量重构并 cutover（批 6，2026-11-12；关键，勿再引用旧口径）**：

| | 唯一前端（Vue 3 SPA） | 已清退（Flutter Web） |
|---|---|---|
| 技术 | **Vue 3 + Vite 纯 SPA**（不引入 Nuxt / SSR / Nitro）+ Pinia + Vue Router + 离线同步内核 | Flutter Web（PWA），MVVM |
| 源码 | `frontend/index.html`、`frontend/src/`、`frontend/public/` | ~~`frontend/lib/`、`frontend/test/`、`pubspec.*`~~ 已 `git rm`（批 6），需要时从 git 历史找回 |
| 产物 | `frontend/dist/`（gitignore），由 `LEDGER_WEB_ROOT` 托管；Docker 镜像内 `/app/web` | `frontend/dist/legacy/` 为**冻结的回滚基准**（不入库、不再重建；`check-legacy.sh` 仍可校验） |

- **批 0~6 全部落地**（2026-11-12 交付）：36 端点、迁移 014 库世代、恢复/导入向导（P-19/P-20）、离线同步内核（`src/core/sync/`，9 份真实契约样本 + 14 引擎用例）、PWA（`public/sw.js`）、CI `web-verify` job、Dockerfile Node 构建、工具链钉版（`.nvmrc` 22.23.4 / `engines >=22.22 <23` / `engine-strict`）。
- Vitest **301/301 通过**（37 文件）；`go build` / `go vet` / `go test ./...` 全绿。
- `LEDGER_AUTH_EXPOSE_TOKEN` 默认已改 `false`（Vue 版只靠 httpOnly Cookie，D-7）；回滚 `/legacy/` 时显式设 `true`。
- 后端与账芯**不变**：复式借贷平衡、append-only 不可变交易、`bean-check` 导出校验、离线同步协议全部保留。

**核心不变约束**：导出的 `.beancount` 必须能被 `bean-check` 直接校验通过（退出码 0、无报错）。

**当前阶段**：M7 六批全部完成，仓库只剩一套前端；§7 端点表 36 条已与 `internal/server/router.go`、`frontend/src/api/routes.ts` 逐条核对一致。剩余边界见 §9。

---

## 2. 技术栈与环境基线

| 工具 | 版本 / 路径 | 说明 |
|---|---|---|
| Go | 工程基线 `go.mod` 1.25.0（CI `go-version: '1.25'` / 镜像 `golang:1.25-alpine`） | 纯 Go 免 CGO，可交叉编译到 NAS arm64 |
| Node.js | **22.x 区间**（`engines: ">=22.22 <23"` + `engine-strict=true`）；`.nvmrc` 钉 **22.23.4**（CI/镜像同版） | 仅**构建阶段**需要，运行时镜像无 Node。三处（.nvmrc / engines / Dockerfile ARG）升级须同步改 |
| Python | 项目内 `.venv` | 仅用于 `bean-check`（`beancount` 包），不污染系统 |
| Go 依赖代理 | 国内网络可用 `https://goproxy.cn,direct` | `proxy.golang.org` 国内不可达；仅影响构建机，按各自网络环境配置，不写入仓库 |
| 前端依赖 | 见 `frontend/package.json` | 版本按方案 §14.7 钉版；**禁装** ECharts / Vuetify / Framework7 / moment / monaco / leaflet / swiper |

~~Flutter~~：批 6 已清退，Flutter SDK 与 `PUB_HOSTED_URL` 镜像不再是本项目依赖；旧源码从 git 历史（批 ≤5 提交）找回。

Vue 侧关键钉版：`vue ^3.5`、`vue-router ^4.5`、`pinia ^2.3`、`decimal.js ^10.4`、`vite ^5.4`、`@vitejs/plugin-vue ^5.1`、`@vitejs/plugin-legacy ^5.4`、`typescript ~5.6`、`vitest ^2.1`、`vue-tsc ^2.1`、`es-check ^7`。**不要照抄参考项目 ezbookkeeping 的 Vite 8 / TS 6 / vue-router 5 / Pinia 4 / `target: safari16.4`**，那会在 iOS 14.8 上「全绿白屏」。

项目内 `.venv` 供 `bean-check` 使用，按需创建，不装到系统 Python：
```bash
python3 -m venv .venv
python -m pip install beancount    # 激活 .venv 后执行
```

---

## 3. 目录结构

```
.
├── schema.sql            # 初始核心表 DDL（含 Beancount 映射注释）
├── cmd/server/main.go    # 后端入口
├── internal/             # 后端分层：config → domain → db → repository → service → handler → server
│   ├── db/{db,migrate}.go   # 开库 + schema 迁移（当前至 014）；迁移框架见 §4
│   ├── repository/       # 全部 SQL 集中于此（account/transaction/member/sync/session/user/ledger/opening/import）
│   ├── service/          # 业务：校验/事务编排/余额/自然键合并/同步协议/成员权限/恢复/导入
│   ├── handler/          # 薄 HTTP 适配层
│   ├── server/           # 组合根：路由 + CORS/CSRF/auth 中间件 + 静态托管
│   ├── auth/auth.go      # bcrypt / token / context 用户态（纯函数）
│   ├── backup/           # 一致性快照（VACUUM INTO）+ 定时调度 + 保留清理
│   ├── restore/          # M2-a 恢复核心（预检/pre-restore 快照/逐表替换/迁移重跑/库世代轮换）
│   ├── beancount/        # M2-d Beancount 严格子集解析器（全量错误行号）
│   └── export/{beancount,files}.go  # 单文件 + 多文件目录导出
├── frontend/             # Vue 3 SPA（cutover 后唯一前端；Flutter 版已清退，git 历史可查）
│   ├── index.html        # Vite 入口
│   ├── package.json / vite.config.ts / tsconfig*.json / .nvmrc / .npmrc
│   ├── public/           # sw.js + manifest.json + icons/ + favicon.png（PWA 静态资源）
│   ├── src/              # 源码：api/ core/（含 sync/ 离线内核） router/ stores/ styles/ views/ components/ + main.ts / App.vue
│   └── dist/             # 构建产物（gitignore）；dist/legacy/ 为冻结的旧版回滚基准（不入库）
├── scripts/              # check-bean / verify-version / smoke / local.sh / check-legacy / check-test-floor / compat
├── export_demo/          # Python/sh 验证参考
└── data/                 # 运行时 SQLite（git 忽略）；backups/ 也在其下
```

**前端架构（Vue 3 SPA，唯一前端）**：
- `core/Money.ts`：`decimal.js` 定点金额唯一入口（只收 string 构造、`ROUND_HALF_UP`、`toString()` 固定 `toFixed()`）；另有 `core/{accounts,calendar,csv,filters,opening,ordering,record,storage,m2}.ts` 核心领域逻辑（全部带 `.test.ts`）。
- `core/sync/`：离线同步内核——`models.ts`（本地实体/死信分类，`DEAD_CODES` 与后端稳定码同源）、`localStore.ts`（Memory 实现）、`idbStore.ts`（IndexedDB 真相源，单事务原子写）、`engine.ts`（SyncEngine：pull→push 重映射→pull、库世代隔离、指数退避、CAS 多标签页租约）+ 9 份真实契约样本（`fixtures/`，其中 6 份被 `engine.test.ts` 引用）。
- `api/{types,routes,errors,client}.ts`：36 端点手写 TS 类型 / 路由常量 / `ApiError` / 类型化 fetch 客户端（401 single-flight / 428 / `X-Ledger-Id` 断言 / FormData 上传；**不生成** OpenAPI）。
- `stores/`：`session / privacy / recordSheet / sync`；views 20 页 + components（AppLayout / RecordSheet / SyncBadge 等）。
- `vite.config.ts`：`base: './'`、`build.target: 'es2015'`、`@vitejs/plugin-legacy`（`targets: Safari >= 14`）、`outDir: 'dist'`、`server.proxy['/api'] → 127.0.0.1:8080`、alias `@`、`define.__APP_VERSION__` 读 package.json。
- PWA：`public/sw.js` 只缓存应用壳与静态资源（`/api/*`、`/legacy/*` 绝不拦截），activate 清理非 fambean 前缀旧缓存（含 Flutter 时代 SW 缓存）；`core/pwa.ts` 更新提示不自动打断录入（AC-35）。
- **兼容基线**：语法门禁 es2020 + `compat.mjs` API/CSS 特性扫描；禁用 Safari 14.1 缺失 API（`.at()` / `structuredClone` 等）。

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

**迁移（`internal/db/migrate.go`，追加式、幂等、记录 `schema_migrations`；当前至 **014**）**：
- 001_sync：补 `accounts/commodities/prices` 的 `uuid`+`updated_at`；建 `sync_log`(id 自增即全局水位)；重建 `sync_checkpoints`；建 `sessions`。
- 002_delete_semantics：交易/账户加 `deleted_at`（软删）；账户重建部分唯一索引 `(ledger_id,name) WHERE deleted_at IS NULL`；旧库 owner 密码置为初始默认值。
- 003~009：账户显示名 / 视觉与层级字段、交易 `tags`、关户语义、账户排序、`users.is_admin`、`app_settings`。
- 010~012：强制改密、会话设备信息、期初余额（`opening_key` 等）。
- **013_txn_idempotency（M7 批 1）**：`transactions.client_request_id TEXT` + 部分唯一索引 `UNIQUE(ledger_id, client_request_id) WHERE client_request_id IS NOT NULL`；配套 `migrate013_test.go` 与 `server/idempotency_test.go`。
- **014_db_instance_id（M7 批 4，已实现）**：`db_instance` 单例表存库世代 UUID；`GetInstanceID` / `RotateInstanceID`；`GET /api/db/instance` 供登录用户读取；`health` / `sync/pull` / `sync/push` 响应追加 `instance_id`（additive）。恢复成功后轮换世代，客户端据此清本地实体并全量重拉。

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
- **兼容期**：`LEDGER_AUTH_EXPOSE_TOKEN` 默认 `false`（Vue 版只靠 httpOnly Cookie）；仅回滚 `/legacy/` 旧版时显式置 `true`，届时登录 / 刷新 / 改密响应体返回 `token` 供旧版使用。
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

**批 4/5 已落地的硬约束**（实现口径，勿回退；与 §4 / §9 同口径）：
- **库世代**：restore 覆盖 DB 后 `sync_log` 自增回退，客户端 `last_seq` 超前会「永远拉不到且不报错」。迁移 014 建独立 `db_instance` 单例表存 UUID（建库写入、restore 成功后 `RotateInstanceID` 轮换），pull/push/health 返回 `instance_id`，客户端按 `{ledger_id:{instance_id,last_seq}}` 比对，不匹配则清该账本实体 + 全量 pull，pending 转隔离区不静默删。
- **IndexedDB 原子写**：只允许 `recordEntry` / `applyPushResults` / `applyPullChanges` 三个跨 store 的单事务原子操作；`last_seq` 仅在变更成功应用后单调推进。
- **多标签页租约**：Safari 14.1 无 `BroadcastChannel` / `navigator.locks`，用 `storage` 事件选举 + IndexedDB 单记录 CAS 租约；401 刷新也只由 leader 执行。
- **失败分类 / 死信**：pending 增 `attempts / last_error_code / next_retry_at / state`；4xx 按码转 dead 并在同步页可见，网络与 5xx 指数退避封顶。

---

## 7. API 端点清单

> 共 **36 条 `/api/` 端点**（下表 36 行 + 末尾 1 行静态回退），与 `internal/server/router.go` 的 `mux.HandleFunc` 及 `frontend/src/api/routes.ts` 的常量一一对应；新增端点须三处同步。

| 端点 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/api/health` | GET | 公开 | `{"status":"ok","instance_id":"..."}`（库世代；读取失败时降级为不带该字段，公开端点永不失败） |
| `/api/auth/register` | POST | 公开 | 首个用户建默认账本成 owner，后续仅建账号待邀请；下发 httpOnly Cookie（**默认响应体不含 token**，回滚 `/legacy/` 需 `LEDGER_AUTH_EXPOSE_TOKEN=true`） |
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
| `/api/sync/pull` | GET | 登录+成员 | 增量拉取（since + client_id）；响应含 `instance_id`（空增量返回 `checkpoint:0`，**客户端不得拿 0 回写水位**） |
| `/api/sync/push` | POST | 登录+成员 | 逐条幂等提交离线变更，响应含 `instance_id`；稳定错误码 `account_not_found`（重映射重试）/ `account_closed`（死信）等 |
| `/api/db/instance` | GET | 登录 | 读取库世代 `instance_id`（迁移 014）；恢复成功后轮换，客户端据此清本地 + 全量重拉 |
| `/api/backup/list` | GET | 登录+**系统管理员** | 备份清单 |
| `/api/backup/download?name=` | GET | 登录+**系统管理员** | 下载备份；文件名白名单 + 四层穿越防护 |
| `/api/backup/run` | POST | 登录+**系统管理员** | 手动触发一次备份（进行中 409） |
| `/api/backup/restore/preview` | POST | 登录+**系统管理员** | **只读预检**：multipart `file`；备份 vs 当前四行覆盖量对比 + schema 版本 + 拒绝原因（如 `future_version` / `not_sqlite`），绝不改数据 |
| `/api/backup/restore` | POST | 登录+**系统管理员** | 执行还原：multipart `file` + `confirm=RESTORE` 确认词；pre-restore 快照、事务内逐表替换、迁移重跑、成功后轮换 `instance_id` |
| `/api/import/beancount/preview` | POST | 登录 | **只读解析预检**：JSON `{"text"}` 或 multipart `file`；全量错误行号 / 账户·交易·价格·币种计数 / 疑似重复提示，不落库 |
| `/api/import/beancount` | POST | 登录 | 执行导入：单一大事务新建账本（含 `sync_log`），同名自动加序号；引用未声明账户整体拒绝 |
| `/api/settings/registration` | GET/POST | **GET 公开** / POST 登录+**系统管理员** | 读取 / 修改注册开关（GET 供登录页判断是否显示注册入口） |
| `/*`（非 `/api/`） | GET/HEAD | 公开 | 前端静态资源 + SPA 回退（`LEDGER_WEB_ROOT`） |

> **在线记账幂等冲突矩阵（迁移 013，已实现）**：新建 → `201 {uuid, idempotent:false}`；同 key 命中未删 → `200 {uuid, idempotent:true}`；命中已软删 → `409 idempotency_key_deleted`（换新 key 重提）；同 key 不同 payload → `409 idempotency_payload_mismatch`。旧客户端不传 `client_request_id` 时保持原行为。冲正沿 `opening_key`，**禁止复用** `client_request_id`。
> **M2 端点（批 4，已实现）**：`/api/backup/restore/preview`、`/api/backup/restore`、`/api/import/beancount/preview`、`/api/import/beancount`。防误操作靠 **只读预检 + `RESTORE` 确认词 + pre-restore 快照 + 单一大事务**（**未**采用方案早期设想的 `Idempotency-Key` 头 + 通用 `idempotency_keys` 表）；恢复仅系统管理员、导入登录即可，两者均豁免账本上下文。
> 多账本上下文：业务端点需带 `X-Ledger-Id` 头，服务端缺失回退默认账本 id=1；`/api/import/*`、`/api/db/*`、`/api/backup/*` 为 middleware 豁免路径。`viewer` 只读（非 GET 一律 403）、`editor` 可写、`owner` 可管理。**新前端 API 客户端必须强制注入并断言该头，缺失直接抛错，绝不静默回退**（否则交易会记进错误账本）。
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
- **迁移优先**：任何 schema 改动走 `migrate.go`，幂等、记录 `schema_migrations`；新增编号顺延（当前至 014，下一个是 015）。
- **借贷平衡**：后端 `handleCreateTransaction` 按币种平衡校验；前端记一笔页实时指示。
- **UUID 引用**：同步实体一律用 `uuid` 互引，勿用服务端自增 `id` 跨端传递；在线用 `account_id`、离线 push 用 `account_uuid`（§6）。
- **不可变**：交易一旦创建不原地改；改账 = 新增冲正交易。
- **会话安全**：token 不落 `localStorage` / URL；Cookie 路径非 GET 必须带 `X-CSRF`；`SameSite=None` 禁用；Bearer 兜底观察期结束前不删。
- **兼容基线（iOS 14.8 / Safari 14.1.2）**：`build.target: 'es2015'` + plugin-legacy；**禁 WASM / WebGL / CanvasKit / OffscreenCanvas**；禁用 `structuredClone` / `crypto.randomUUID` / `.at()` / `findLast` / `Object.hasOwn` / `showModal` / `BroadcastChannel` / `navigator.locks`；CSS 不用 `aspect-ratio` / `:has()` / `dvh` / `inset`。`es-check` 只查语法，须另补 API / CSS 扫描；**升级任何前端依赖都要过 AC-19 与真机冒烟**，不允许「本地能跑就升」。
- **`/legacy/` 回滚（冻结基准）**：`frontend/dist/legacy/` 是 cutover 前用 `--base-href /legacy/` 重建复制的产物，现已冻结（不再重建）。`npm run build` 的 `emptyOutDir` 会清空 `dist/`，**需要保留回滚能力时先备份该目录**，或从 git 历史（批 ≤5）找回；回滚旧版必须同时设 `LEDGER_AUTH_EXPOSE_TOKEN=true`。旧 SW 清理已由 Vue 版 `public/sw.js` activate 步骤覆盖（删除非 fambean 前缀缓存；旧 `flutter_service_worker.js` 404 后浏览器自动注销）。
- **注释语言**：代码注释用中文；保留 `schema.sql` 中 Beancount 映射注释。

---

## 9. 已知边界 / 当前待办（M7 六批全部完成）

**已落地（批 0~6，2026-11-12 交付）**：Vue 3 + Vite 脚手架与全部 20 页、`core/Money` 定点金额、`es2015` + plugin-legacy + es2020 语法门禁 + `compat.mjs` API/CSS 扫描、36 端点手写 TS 类型 + 类型化 fetch 客户端（401 single-flight / 428 / `X-Ledger-Id` 断言 / FormData）、D-7 httpOnly Cookie + CSRF + CORS 白名单、迁移 013/014、恢复/导入全链路（M2-a~g）、离线同步内核（`core/sync/`，9 份真实契约样本〔其中 6 份被引擎用例引用〕+ 14 引擎用例）、PWA（`public/sw.js` + 更新提示）、CI `web-verify` job（含用例数下限防假绿）、Dockerfile Node 构建、工具链钉版（`.nvmrc` 22.23.4 / `engines >=22.22 <23` / `engine-strict`）、`LEDGER_AUTH_EXPOSE_TOKEN` 默认 `false`。Vitest **301/301**（37 文件）；Go `build` / `vet` / `test ./...` 全绿。

**遗留 / 注意**：
1. `package-lock.json` 在 Linux（CI ubuntu）上的 `npm ci` 与 Dockerfile Node 构建，以 GitHub Actions `web-verify` / 镜像构建的运行记录为准，别把「配置写了」当成「已经跑过」；若失败按 lock 缺平台二进制处理（重新在 Linux 生成）。
2. `docs/` 目录已于 2026-11-14 清空（19 份旧阶段文档），内容仅存于 git 历史，需要时从历史提交取回。
3. `frontend/dist/legacy/` 为**冻结回滚基准**（不入库，需从 git 历史重建）；`web-build` 的 `emptyOutDir` 会清掉它——需要回滚能力时从 `d297318` 之前的 git 历史重建。回滚旧版须同时设 `LEDGER_AUTH_EXPOSE_TOKEN=true`。
4. `CHANGELOG.md` 顶部已为 `[0.5.1] - 2026-11-13`，M7 批 0~6 与图标/密码补丁已记入该版本；tag / `frontend/package.json` / 镜像示例已同步为 0.5.1，**`v0.5.1` tag 已推送至 origin**。
5. 真机双端联调（浏览器 A/B 同时在线、离线多设备端到端）与批 5 离线链路真机冒烟尚未完整确认。
6. 下期（M4+）：Beancount 通用语法导入、OFX/CSV 自动导入、规则引擎、商品/价格 CRUD、跨币种 cost 注解、Postgres 横向扩展。

> 旧版功能在新版**一律 Must，不得降级**：CSV 导出、分类拖拽排序、导出不平告警、注册开关、默认账户管理详见方案 §14.5。

---

## 10. 快速运行与测试

```bash
# 后端（托管 frontend/dist，Vue 版）
(cd frontend && npm run build)   # 先构建前端产物
LEDGER_DB=data/ledger.db LEDGER_WEB_ROOT=frontend/dist go run ./cmd/server   # 默认 :8080

# Vue 前端开发（Vite dev server，/api 已代理到 127.0.0.1:8080）
cd frontend && npm ci && npm run dev        # 另开一个终端跑后端
# 或仓库根：bash scripts/local.sh web-dev

# 注册 / 登录（默认配置下响应体不再返回 token，走 Cookie + X-CSRF；
# 需要脚本化 Bearer 时临时 LEDGER_AUTH_EXPOSE_TOKEN=true）
curl -c /tmp/fb.cookie -X POST http://127.0.0.1:8080/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123","display_name":"Alice"}'

# Cookie 写法（生产路径）
# curl -b /tmp/fb.cookie -X POST .../api/transactions -H 'X-CSRF: 1' -H 'Content-Type: application/json' -d '{...}'

# 建账户 / 记一笔（postings 用 account_id）/ 导出校验
curl -s -b /tmp/fb.cookie -X POST http://127.0.0.1:8080/api/accounts -H 'X-CSRF: 1' -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}'
curl -s -b /tmp/fb.cookie -X POST http://127.0.0.1:8080/api/transactions -H 'X-CSRF: 1' -H 'Content-Type: application/json' \
  -d '{"date":"2026-08-02","description":"早餐","flag":"*","client_request_id":"demo-001","postings":[{"account_id":1,"commodity":"CNY","amount":"-10.00"},{"account_id":2,"commodity":"CNY","amount":"10.00"}]}'
curl -s -b /tmp/fb.cookie http://127.0.0.1:8080/api/export -o data/family.beancount
python -m beancount.scripts.check data/family.beancount   # 激活 .venv 后；退出码 0 通过

# NAS 交叉编译（纯 Go 免 CGO）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o familyledger ./cmd/server
```

> `bean-check` 一律用项目 `.venv` 内 Python 以模块方式调用（`-m beancount.scripts.check`），不依赖系统可执行文件。
> 质量门禁：`scripts/local.sh check`（Go 全量 + 严格 bean-check）、`web-test` / `web-typecheck` / `web-build` / `web-compat`；CI `web-verify` job 会再跑一遍并校验用例数下限。

### 10.1 统一命令入口

统一入口：仓库根用 `bash scripts/local.sh <cmd>`；Linux/CI 可用等价的 `Makefile`（两者一一对应）。

```bash
# 后端
bash scripts/local.sh test          # go vet + 全量测试（严格 bean-check）
bash scripts/local.sh check         # test + 版本校验 + bean-check 自检（CI 同款）
bash scripts/local.sh smoke         # 端到端：注册→记账→导出→bean-check→备份→下载→穿越防护
bash scripts/local.sh dev           # 起后端并托管 frontend/dist（Vue 版）
bash scripts/local.sh rehearse      # 迁移副本库预演

# Vue 前端（唯一前端）
bash scripts/local.sh web-build     # Vite 构建 → frontend/dist
bash scripts/local.sh web-dev       # Vite dev server（/api → 127.0.0.1:8080）
bash scripts/local.sh web-test      # Vitest
bash scripts/local.sh web-typecheck # vue-tsc --noEmit
bash scripts/local.sh web-compat    # es-check es2020 语法门禁 + §14.7 API/CSS 扫描（scripts/compat.mjs，构建到 .compat-tmp 不碰 dist）
bash scripts/local.sh check-legacy  # 校验冻结的 dist/legacy 回滚基准基路径
```

> `web-build` 会清空 `frontend/dist`（含其中的 `dist/legacy/`），需要回滚基准时先备份该目录。
> 版本号以 `frontend/package.json` 为准，与 CHANGELOG / git tag 三处一致（`verify-version.sh` 校验）。
> 前端门禁已进 CI：`web-verify` job（npm ci → vitest + 用例数下限 → typecheck → build → compat）。

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
| `LEDGER_AUTH_EXPOSE_TOKEN` | `false` | 登录/刷新/改密响应体是否返回 `token`；cutover 后默认关闭（Vue 版只靠 Cookie）。仅回滚 `/legacy/` 旧版时设 `true` |
| `BEAN_CHECK_STRICT` | 空 | 测试用；`1` 时找不到 bean-check 直接失败（CI 必设） |

> `config.Validate()` 启动时校验备份小时、保留份数、登录限流与 CORS 来源格式，非法即刻失败（快速失败，不静默回退）。

### 10.3 容器化

```bash
docker build -t money-mint:dev .                              # 三阶段：Node 构建 Vue → Go → alpine
docker run -d -p 8080:8080 -v $(pwd)/data:/app/data money-mint:dev
docker compose up -d                                          # 家用部署（见 docker-compose.yml）
```

> **Dockerfile 已改为 Node 构建**（批 6 cutover）：Stage 1 `node:22-alpine`（版本与 `frontend/.nvmrc` / `engines` 三处一致）→ `COPY package*.json` → `npm ci` → `npm run build`，产物从 `frontend/dist` 拷到 `/app/web`；Stage 2（Go）、Stage 3（alpine 运行时）、`docker-entrypoint.sh`、`docker-compose.yml` 未动，单容器单端口模型不变。
> `package-lock.json` 已含全平台 optional 二进制（`@esbuild/linux-x64` 等），镜像内 `npm ci` 可直接命中；Linux 实机构建以 CI `web-verify` / 镜像构建为准。

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
