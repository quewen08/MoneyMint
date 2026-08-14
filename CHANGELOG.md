# Changelog

本项目变更记录遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 风格，
版本号语义化（[SemVer](https://semver.org/lang/zh-CN/)）。

## [0.2.1] - 2026-08-14

### Fixed

- **成员管理页面 build 抛「BoxConstraints forces an infinite width」级联崩溃**：根因是 `ListView` 收到无限宽度约束（`relevant error-causing widget` 指向 `members_screen.dart` 的 `ListView`）。两处叠加：① `BoxConstraints(maxWidth: wide ? 900 : double.infinity)` 在移动端/窄窗口下传 `double.infinity`；② `AppCard` 的 `Container(width: double.infinity)` 转成 `BoxConstraints(minWidth: inf)`。修复：`wide==true` 才包 `Center > ConstrainedBox(maxWidth: 900)`，否则直接 `ListView`；抽出 `_buildList()`；`AppCard` 去掉 `width: double.infinity`；并删除 `_isOwner` getter（次要嫌疑，一并清除）。
- **CORS 中间件顺序错误导致前端「无响应 / 账号已注册」**（根因）：`newRouter` 原返回 `authMiddleware(svc, cors(mux))`，auth 在**外层**、cors 在**内层**。浏览器对带 `Authorization` / `X-Ledger-Id` 自定义头的请求（`me`/`listLedgers` 等非简单请求）会先发 `OPTIONS` 预检，预检请求无 token，被 authMiddleware 拦截返回 `401 未登录`，到不了 cors 的 OPTIONS 处理，浏览器因此拒绝发真实请求，前端表现为「无响应」。修复：改为 `cors(authMiddleware(svc, mux))`（cors 最外层，先吞掉 OPTIONS 预检返回 204），并在 `Access-Control-Allow-Headers` 补充 `X-Ledger-Id`。删除 `data/ledger.db*` 后重启服务、注册新用户即可正常进入应用。
- **前端 401 残留 token 导致 AwaitInviteScreen 死循环**（防御性兜底）：删除后端 `data/ledger.db*` 重启服务后，浏览器 localStorage 里的旧 token 触发 `me()` 抛 401，原 `init()` 仅 catch 异常未清 token → `loggedIn=true && _ledgers=[]` → 跳到「账号已注册」页面且无法回到登录页。修复 `auth_controller.init()`/`_run()` 在 `ApiException(401)` 时统一调 `_resetLocalAuth()` 清本地登录态；同时优化 `AwaitInviteScreen` 文案，对 username 为空场景提示「如果是刚清空了后端数据库，请返回登录后重新注册」。

## [0.2.0] - 2026-08-14

### Added

- **多账本隔离（P1-C1）**：`ledgers` 真正隔离，业务端点按 `X-Ledger-Id` 头解析账本上下文（缺失时回退默认账本 id=1，兼容旧客户端）。
  - 新增 `GET/POST /api/ledgers`（列本人账本 / 创建新账本，创建者成 owner）、`PATCH /api/ledgers/{id}`（账本改名 / 默认币种）。
  - 后端 service 层全部方法按 ledgerID 参数化，`domain.DefaultLedgerID` 不再被业务层硬编码。
  - 前端：登录后拉取账本列表并选定当前账本（localStorage 记住上次选择）；新增 `screens/settings/ledgers_screen.dart` 账本管理页（列表 / 切换 / 创建 / 改名）。
- **角色鉴权落地（P1-C2）**：业务端点区分 `viewer` 只读（非 GET 一律 403）、`editor` 可写、`owner` 可管理（成员 / 账本设置）。
- **多文件目录导出（新需求）**：新增 `GET /api/export/archive`，导出 zip 目录：
  - `main.bean`（title + commodity/price 声明 + include）
  - `accounts/{assets,liabilities,income,expenses,equity}.bean`（账户 open/close）
  - `date/{year}/{year}-{MM}.bean`（按月交易）+ `date/{year}/{year}.bean`（年度入口）
  - 解压后对 `main.bean` 跑 `bean-check` 校验通过。

### Changed

- 登录 / `me` 不再返回「默认账本角色」（role 恒空串），账本角色改由 `GET /api/ledgers` 按账本返回。
- `CreateLedgerForOwner` 去掉硬编码账本 id，改为 AUTOINCREMENT 分配并返回新 id。
- CORS 增加 `PATCH` 方法与 `X-Ledger-Id` 头。

### Fixed

- 「前端启动后总显示离线」：根因为后端进程未运行（非代码缺陷），启动后端即恢复。

## [0.1.0] - 2026-08-13

### Added

- **P0 垂直切片**：
  - 数据库 Schema（`schema.sql`）：users / ledgers / ledger_members / accounts / commodities / prices / transactions / postings，金额一律定点十进制。
  - Beancount 导出器（`internal/export`）：commodity / open / close / price / transaction 指令序列化，通过 `bean-check`。
  - Go REST 后端 + SQLite：账户、交易、余额、删除、导出。
- **P1-A 前端打磨与离线联调**：
  - 前端 MVVM 分层重构 + 响应式双布局（≥1024 PC 侧边栏 / <1024 移动底栏）。
  - PWA 外壳（manifest.json + Service Worker 注册）。
  - 离线记账端到端联调（IndexedDB 本地真相源 + 联网自动 pull/push）。
  - 冲正 UI（`LocalTxn.reversedOf`，新增反向交易不原地改）。
  - 余额 / 报表页（Decimal 精确计算，净资产按资产−负债）。
- **P1-B 后端增强**：
  - 删除语义（P1-B1）：账户/交易软删（`deleted_at`）+ `sync_log` 写 `op=delete`；删账户连带删引用交易；迁移 002（部分唯一索引 `(ledger_id,name) WHERE deleted_at IS NULL`）。
  - push 逐条提交（P1-B2）：单条失败不阻塞其余，响应 `{checkpoint, accepted, results[]}`。
  - 认证增强 + 成员管理（P1-B3）：`change-password` / `refresh` / owner 添加、改角色、移除、重置成员密码；注册语义改为「首个用户 owner，后续用户待邀请」。
  - 账户自然键合并：push 同名账户保留先到者并回传 `server_uuid`，前端两轮推送重映射本地 `loc-` uuid。
- 迁移框架（`internal/db/migrate.go`）：`schema_migrations` 记录已应用项，幂等可重入。

### Changed

- 后端项目化分层（`internal/{config,domain,db,repository,service,handler,server,auth,export}`），API 路径与 JSON 结构保持兼容。

### Fixed

- 修复成员管理返回结构兼容（`AddMember` 补回 `user_id` / `role`）。
- 修复旧库 seed owner 密码 `unset` 无法登录（迁移 002 置为初始密码）。

---
