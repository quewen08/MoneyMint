# Changelog

本项目变更记录遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 风格，
版本号语义化（[SemVer](https://semver.org/lang/zh-CN/)）。

## [0.4.0] - 2026-08-20

### Added

- **删除语义升级为 Beancount close（0.4-A）**：账户「删除」从软删（`deleted_at` + 连带软删引用交易）改为置 `close_date` 关闭，**保留全部交易引用与历史余额**，与 Beancount `close` 指令对齐。
  - 迁移 `006_account_close_semantics`：旧库 `deleted_at IS NOT NULL` 的账户转写为 `close_date`（取前 10 字符日期）并清空 `deleted_at`；重建账户名唯一索引为账本内全局唯一（`UNIQUE(ledger_id, name)`，含已关闭账户）——关闭后账户名仍占用，不可重建同名（Beancount 实测不允许 close 后再 open 同名，报 duplicate open）。
  - `sync_log` 新增 `op=close`：`CloseAccount`（`internal/repository/transaction.go`）置 `close_date=date('now')`（满足 Beancount 日期格式）+ 写 `op=close`；幂等（已关闭/不存在返回 false）。
  - 同步协议：`SyncPull` 的 `op=close` 走 `loadEntity` 返回账户全量（含 `close_date`）；`SyncPush` 分拣顺序新增 `{OpClose, EntityAccount}`；`applyCloseEntity` 处理 `op=close`；旧客户端 `op=delete account` 向前兼容按 close 处理。
  - 账户列表 `GET /api/accounts` 新增 `?include_closed=1` 参数，默认仅返回未关闭账户（`close_date IS NULL`）；`AccountView` 增 `close_date` 字段。
  - 记账引用校验：`AccountIDByUUID` 加 `close_date IS NULL`，已关闭账户不可再被记账引用（在线 `account_id` 与离线 `account_uuid` 均禁止）。
  - 自然键合并：离线推送同名账户时，仅合并到**未关闭**的同名账户；若同名账户已关闭，不合并（走新建触发同名唯一约束冲突，符合「关闭后不可重建同名」语义）。
  - 导出器（`internal/export/beancount.go`）：`open`/`close` 指令改为按账户合并输出（open 紧跟其 close），更清晰；已关闭账户仍计入净资产与历史（Beancount close 不抹除历史）；导出通过 `bean-check`（退出码 0）。

### Changed

- **前端账户关闭交互**（`screens/accounts/accounts_screen.dart`）：账户行菜单「删除账户」改为「关闭账户」；确认弹窗文案改为「历史交易全部保留，余额历史照常计入；默认列表隐藏已关闭账户，可在『显示已关闭』中查看」。
- **前端「显示已关闭」开关**：账户页顶栏新增 Switch（仅当存在已关闭账户时显示），默认隐藏已关闭账户；已关闭账户行视觉降权（灰字 + 「已关闭 · 日期」副标题）。
- **前端记账选账户排除已关闭**：`record_dialog.dart` / `record_screen.dart` 选账户时过滤 `isClosed`，已关闭账户不可选入记账。
- **前端本地模型与同步**：`LocalAccount` 增 `closeDate` 字段 + `isClosed` getter；`LocalStore.applyChange` 新增 `op=close` 分支（更新账户 `close_date`，不删除账户与交易）；`SyncService.deleteAccount` → `closeAccount`（本地置 `close_date` + 入队 `op=close`，不再删引用交易）；`LedgerController.deleteAccount` → `closeAccount` + `toggleShowClosedAccounts`；`accountsOfType` 加 `includeClosed` 参数（默认 false）；`LedgerApi.deleteAccount` → `closeAccount`（DELETE URL 不变，语义改为 close）。
- `AccountRow`/`LocalAccountRow`（`widgets/common.dart`）新增 `dimmed` 参数用于已关闭账户视觉降权。

### Fixed

- **导出器 open/close 分离导致 close 后重建同名 duplicate open**：原导出器把所有 `open` 与所有 `close` 分别堆叠输出，close 后重建同名账户会触发 Beancount `duplicate open directive`。改为按账户合并输出（open 紧跟其 close）。同时确认 Beancount 不允许 close 后再 open 同名，账户名唯一索引改为全局唯一（关闭后不可重建同名），从根本上避免该场景。

### Added（0.4-C 账户增强）

- **分类拖拽排序（迁移 007）**：`accounts` 新增 `sort_order INTEGER DEFAULT 0`（幂等，记录 `schema_migrations`）；`GET /api/accounts` 的 `AccountView` 返回 `sort_order`；后端 `UpdateAccountSortOrder`（`internal/repository/account.go`）更新序位并写 `sync_log op=update`；同步协议新增 `op=update`（仅账户排序字段更新，无自然键合并，多端一致）。
- **前端排序链路**：`LocalAccount` 增 `sortOrder` 字段；`LocalStore.applyChange` 新增 `op=update` 分支（按 uuid 全量 upsert）；`SyncService.reorderAccounts` 本地按拖拽顺序写 `sort_order` + 逐条入队 `op=update` 并同步；`LedgerController.reorderAccounts` 透传；`reload()` 全局按「类型 → sort_order（子分类跟随父）→ name」稳定排序。
- **分类页拖拽（`screens/categories/categories_screen.dart`）**：`ReorderableListView` 长按拖拽重排 `Expenses`/`Income` 根分类（仅 `parent_uuid IS NULL`），拖完持久化，刷新后顺序保持。
- **默认账户（纯前端记忆，零后端改动）**：`LocalStore` 按账本隔离存默认账户映射（kind ∈ `expense`/`income`/`transferOut`/`transferIn` → uuid）；`LedgerController` 暴露 `defaultAccounts`/`setDefaultAccount`/`clearDefaultAccounts`；记一笔弹窗按交易类型预选默认账户、提交回写「上次选择」；账户行菜单「设为默认支出/收入账户」；设置页新增「默认账户」查看与「清除全部默认」。
- **账户详情页（`screens/accounts/account_detail_screen.dart`，新增）**：账户余额卡 + 该账户月度收支趋势（`TrendLineChart`）+ 最近交易预览 + 「查看全部流水 / 设为默认 / 关闭账户」入口；账户行点击改为进详情页（替代原直跳流水列表）。

### Changed（0.4-C 账户增强）

- 账户列表全局排序纳入 `sort_order`（同类型内升序，未设置回退按 name；子分类跟随其父根排序），`AccountsScreen`/`CategoriesScreen` 均按此展示。
- 账户行点击 → 账户详情页（原「查看流水」改为详情页内「查看全部流水」入口）；行菜单新增「账户详情」「设为默认支出/收入账户」。

## [0.3.0] - 2026-08-18

全端体验优化（分类 / 标签 / 图标 / 简化记账）第一阶段：账芯与数据层完成，核心交互弹窗落地。

### Added

- **分类体系（复用账户模型）**：`Expenses` / `Income` 类型账户通过 `parent_uuid` 表达二级/三级分类，不引入独立分类实体；新建账本 seed 默认二级分类（餐饮/食品/交通/购物…与薪资/奖金…）并带 icon/color。
- **账户视觉字段（迁移 004）**：`accounts` 新增 `icon` / `color` / `parent_uuid` / `sub_type`（TEXT，幂等、记录 `schema_migrations`）；贯穿 domain / repository / service / handler / sync 实体与导出引用；seed 账户补全 emoji 图标 + hex 颜色。
- **交易标签（迁移 005）**：`transactions` 新增 `tags`（JSON 数组字符串，如 `["餐饮","出差"]`）；贯穿 repository / service / handler / sync / 前端模型与同步层。
- **简化记一笔（核心交互）**：新增 `screens/record/record_dialog.dart`（PC 弹窗），支出/收入/转账分段切换 + 金额 + 分类(account)/账户 + 日期 + 标签 + 描述，自动生成标准复式 posting 并提交；保留「高级」双录入口。`AppShell` 在宽屏用 `RecordDialog`、窄屏用 `RecordScreen`。
- **账户/分类编辑弹窗**：新增 `screens/accounts/account_dialog.dart`，支持 emoji 图标选择、颜色面板、类型、父分类、名称、开户余额（自动生成 `Equity:Opening-Balances` 冲抵交易）。
- **分类页**：新增 `screens/categories/categories_screen.dart`，展示 Expenses/Income 二级/三级分类层级，支持新增三级分类。
- **账户页双 Tab**：`screens/accounts/accounts_screen.dart` 改为 `TabController`，Tab1「账户」、Tab2「分类」（接入 `CategoriesScreen`）。
- **图标组件**：新增 `widgets/account_icon.dart`，emoji 渲染于彩色圆角方块，无 icon 时按类型/名称派生默认图标与颜色。
- **前端模型/同步适配**：`LocalAccount` 增 `icon/color/parentUuid/subType`，`LocalTxn` 增 `tags`；`SyncService` / `LedgerController` 的建账户、建交易透传新字段。

### Changed

- **Beancount 标签导出改为元数据形式**：原计划的交易头行 `#tag1 #tag2` 语法仅允许 ASCII，中文标签会导致 `bean-check` 报语法错误。改为在交易头行之下写 `tags: "tag1 tag2"` 元数据行——保留中文且通过 `bean-check`（已用 `TestExportTags` 验证 `bean-check` 退出码 0）。代价是该标签不被 Beancount 识别为 `#tag` 查询键，但作为记账留档与兼容导出满足需求。见 PRD §3.4 设计权衡。
- `widgets/common.dart` 的 `todayStr()` 支持可选日期参数。

### Fixed

- **标签导出 `bean-check` 失败**：中文 `#餐饮` 触发 `unexpected HASH` 语法错误。根因为 Beancount `#tag` 词法不支持非 ASCII，已通过改用元数据形式修复（见 Changed）。

### Pending（后续迭代，见 PRD §8）

- 移动端底部三 Tab 导航 + 资产页 + 账单首页（日历/列表）。
- PC 总览卡片化（净资产/总资产/总负债/本月收支）+ 近 12 月趋势图。
- PC 交易流水筛选 + 日历视图。
- 标签页（按标签筛选交易列表与统计）。
- 报表图表化（分类占比/收支趋势/资产趋势）。

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
