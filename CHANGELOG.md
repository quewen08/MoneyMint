# Changelog

本项目变更记录遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 风格，
版本号语义化（[SemVer](https://semver.org/lang/zh-CN/)）。

> 版本号单一事实源：git tag（`vX.Y.Z`）。发布前 `scripts/verify-version.sh` 会校验
> tag / 本文件顶部版本 / `frontend/package.json` 三处一致，不一致则构建失败。

## [0.5.1] - 2026-11-13

M7 前端全量重构收官后的首个补丁版本：Vue 3 SPA 取代 Flutter Web，旧功能不降级，账芯与后端协议不变。

### Added（M7 批 3~6 与 UI 补丁，2026-11-12~13）

- **管理与设置全量页面**（批 3，`ffd55fa`）：报表 / 成员 / 账本 / 设置 / 数据安全 / 会话 / 同步状态 / 导入 / 恢复共 9 页；注册开关、账本改名、成员角色、会话吊销全部接入。
- **M2 并入与库世代**（批 4，`bf17cdd` / `8ddb8bb`）：导入与恢复向导（只读预检 → 确认 → 执行）；迁移 014 `db_instance_id`，恢复成功后轮换世代，客户端据此清本地并全量重拉。
- **离线同步内核与 PWA**（批 5，`2516cfe` / `d7a2970`）：IndexedDB 原子写、死信分类、多标签页租约、指数退避；`public/sw.js` 只缓存应用壳，`/api/*` 与 `/legacy/*` 不拦截。
- **cutover 收口**（批 6，`d297318` / `c04588b` / `6b09b94`）：清退 Flutter 源码与构建链，Dockerfile 改 Node 构建，CI 增 `web-verify`，`LEDGER_AUTH_EXPOSE_TOKEN` 默认 `false`。
- **品牌图标与密码可见**（`30dd122`）：全站 PWA 图标 / favicon / 登录页换成豆子图标，`theme_color` 对齐 `#3459A5`；新增 `PasswordInput` 组件，登录、改密、成员重置共 5 处密码框支持眼睛切换。

### Added（M7 前端全量重构，批 0~2.5）

- **Vue 3 + Vite 纯 SPA 工程底座**（批 0，`c4ce399`）：`frontend/index.html` + `src/` + `public/` + `dist/`；`build.target: 'es2015'` + `@vitejs/plugin-legacy`（Safari >= 14）+ `es-check` 兼容门禁；`/api` dev proxy；31 端点手写 TS 类型与路由常量。
- **D-7 会话安全改造**：httpOnly Cookie 优先 / Bearer 兜底、`X-CSRF` 头、CORS 由 `*` 收敛为白名单回显 + `Allow-Credentials`；保留 `LEDGER_AUTH_COOKIE` / `LEDGER_AUTH_EXPOSE_TOKEN` 回退开关。
- **在线记账幂等 G-04**（批 1，`814627a` / `4feccaf`）：迁移 013 增 `transactions.client_request_id` + 部分唯一索引，冲突矩阵覆盖 201 / 200 / 两类 409；前端类型化 fetch 客户端含 401 single-flight、428 跳改密、`X-Ledger-Id` 强制断言。
- **账本主体与口径修正**（批 2 `e018cc0` / 批 2.5 `20fd5d8`）：流水 / 详情 / 回收站 / 账户 / 账户详情 / 分类 / 币种 / 改密 / 待邀请共 11 页；`core/Money` 定点金额、日历与筛选 query 化、CSV 导出、期初与关户守门；批 2.5 按 fava / beancount 双审计裁决对齐口径。
- **`dist/legacy/` 回滚产物**：Flutter 旧版以 `--base-href /legacy/` 重建后复制，配套 `build-legacy.sh` / `check-legacy.sh` 与 `web-*` 命令族。

### Changed（文档口径收口，2026-11-11）

- 方案升 **v2.5**：§1 标为立项基线快照，§9 / §11 / §14.1 / §14.3 / §14.6 / §14.7 对齐代码现状；`AGENTS.md` / `CLAUDE.md` / `README.md` 同步到批 2.5；方案 / 换框架评估 / 进度审查报告已在本地修订，**按老板 2026-11-11 指示 `docs/` 目录暂不提交，保持未跟踪**。
- **按老板决策暂缓且本次未改动**：CI `web-verify` 门禁、`.nvmrc` / `.npmrc` 钉版、`package-lock.json` 在 Linux/WSL 重生成、`verify-version.sh` 改读 `frontend/package.json`。以上均保持未完成状态，勿当现状。

> 实测（2026-11-11）：Vitest 22 文件 / 202 用例全过，`vue-tsc --noEmit` 零错误，`go vet` + `BEAN_CHECK_STRICT=1 go test ./...` 全绿；`frontend/src` 57 文件 / 14,067 行；迁移至 013。

批 4 / F-10：前端改版收口（PC 增量 + 回归）。改版四批中的最后一批，全部为前端改动，后端与账芯零改动。

### Added

- **账本切换全局接入**：`widgets/ledger_switcher.dart` 提供底部面板（当前账本 ✓、角色、管理账本入口），`LedgerPill` 首次真正接进页面——此前组件已写好但无任何调用点，四个页面的「当前账本 ▾」形同虚设。
  - 账单 / 资产在渐变头内用白字 pill；报表 / 更多 / 流水在 AppBar 用 `light: true` 浅色 pill。
  - 切换链路复用既有机制（`AuthController.selectLedger` → `LedgerController.switchToLedger`），不新造协议。
- **PC 仪表盘改版（F-10）**：删除 0.4-B 的独立宽屏仪表盘，两端统一走「账单首页」，避免同一份数据维护两套视觉。
  - ≥1200 三栏：日历 / 净资产 + 资产概览 + 近 12 月趋势 / 快捷记账 + 最近交易；1024–1199 两栏；更窄纵向堆叠。
  - 原 PC 页的净资产卡、四宫格指标、趋势图并入右栏，功能不丢；资产/负债/权益分组收敛到「资产」页（侧边栏直达）。
- **流水页 PC 表格视图**：五列（日期 / 说明 / 账户·分类 / 标签 / 金额），日期与金额列点表头排序（同值稳定），行内保留冲正 / 已冲正徽章与记账人。
  - 用 `ListView.separated` 而非 `DataTable`：家庭账本动辄数千笔，`DataTable` 一次性构建全部行、无虚拟化。
- **流水页独立路由自带 Scaffold + AppBar**：新增 `embedded` 参数区分「shell 内容」与「Navigator.push 的独立页」。此前从账单 📋 / 账户详情进来是无 AppBar 的裸 Column（没有返回键，也无法切账本）。

### Added（M3 交易回收站）

- **回收站（M3）**：删除的交易不再「人间蒸发」，可逐条找回。底层不新增表——复用 002 引入的 `deleted_at`，此前软删数据一直在库里，只是没有任何入口能看见。
  - 新增 `GET /api/transactions/trash`（列表，按删除时间倒序）与 `POST /api/transactions/{uuid}/restore`（恢复）。
  - 恢复 = 清空 `deleted_at` + 写 `sync_log op=create`。**故意复用 create 而不是新增 `op=restore`**：客户端只认 create/delete/update/close 四种 op，复用 create 让旧客户端也能把交易拉回来，不引入协议版本分裂。
  - 恢复前校验引用账户：交易若引用了已关闭账户（0.4-A 起禁止记账引用关闭账户），直接拒绝并说明是哪个账户（用展示名「现金」而非 `Assets:Cash`），避免恢复出账本内不可再创建的脏数据。
  - 只读成员（viewer）可查看回收站（GET 放行），恢复被中间件以 403 拦截，无需额外判断。
  - 前端 `screens/transactions/trash_screen.dart`：流水页筛选栏新增回收站入口；列表展示说明/标签/日期/分录摘要/删除时间/记账人/金额，逐条「恢复」；恢复后立即同步一次，流水与余额同步刷新。
  - 回收站数据只来自服务端（删除时本地库已移除该条），因此离线时明确提示「需要联网」，而不是显示成空回收站。
  - `domain.Posting` 新增 `AccountDisplay`（展示名），与 `AccountName`（Beancount 全名）分开：导出/校验依赖全名，界面展示用展示名。

### Added（M3 强制改密）

- **强制改默认密码（M3）**：密码是「别人替我设的」时必须先改，否则业务接口一律 428。
  - 迁移 `010_force_password_change`：`users` 加 `must_change_password INTEGER NOT NULL DEFAULT 0`（默认 0——老库升级不会把早改过密码的人锁在改密页）。
  - 三条「系统代设密码」的路径置位：预设 owner（`LEDGER_OWNER_PASSWORD`）、owner/管理员重置成员密码、CLI `-reset-password`。用户自己改密后清除。
  - `authMiddleware` 新增闸门：置位期间只放行 `/api/auth/{me,logout,change-password,refresh}`，其余 `/api/` 返回 **428 Precondition Required**，body 带 `code: "password_change_required"` 供前端判别（不是笼统 403，避免与权限错误混淆）。
  - `Service.Authenticate` 由「返回 uid」改为返回 `SessionUser`（含强制改密标记）——该标记必须每个请求从库里读，管理员重置密码要立刻生效。
  - 登录 / `me` / 注册响应新增 `must_change_password`。
  - 前端：`AuthGate` 在强制状态下只渲染改密页（无返回键、右上「退出登录」），改密成功后自动进入主框架；设置页的改密入口行为不变（`forced=false`）。
  - 修一个由此暴露的问题：强制期间 `GET /api/ledgers` 被 428 挡掉，`isMember` 为 false，改密后若不补拉账本列表会被误判成「未加入任何账本」而跳到等待邀请页——改密成功后显式 `_loadLedgers()`。

### Added（M3 会话管理 + 登录限流）

- **登录设备与吊销（M3）**：新增 `GET /api/auth/sessions`、`DELETE /api/auth/sessions/{sid}`、`POST /api/auth/sessions/revoke-others`。
  - 迁移 `011_session_meta`：`sessions` 加 `user_agent` / `ip` / `last_seen_at`（都带常量默认值，SQLite 的 ADD COLUMN 不接受非常量默认）。
  - **对外只下发 SID（token 的 sha256 前 16 位）**，绝不下发 token 本身——这个页面即使泄露也拿不到可冒充登录的凭据。
  - 设备名由 UA 压成「Chrome · Windows」这类标签（不引第三方库）；回环 IP 显示为「本机」。
  - 「最近活跃」在中间件里打点，同 token 10 分钟内只写一次库（SQLite 单写者，每请求 UPDATE 会让请求互相排队）。
  - 前端设置页新增「登录设备」入口：列表 + 逐台吊销 + 「退出其他设备」，吊销当前设备等于退出登录。
  - 修正时区口径：`created_at` / `expires_at` 原本用服务器本地时间格式化，而 `last_seen_at` 用 SQLite 的 `datetime('now')`（UTC），同一行两列时区不一致，界面会显示「登录时间晚于最近活跃」。现在统一 UTC 存储，前端用 `formatServerTime` 转本地。
- **登录失败限流（M3）**：新增 `internal/ratelimit` 固定窗口计数器，`LoginFrom` 在凭证校验前先过限流。
  - 双维度计数：**账号维度** 5 次/15 分钟，**IP 维度阈值放宽 4 倍**——家里多台设备通常共用一个出口 IP，同阈值会让一个人的手误把全家锁在门外；脚本撞库仍会被 IP 维度拦住。
  - 超限返回 **429 + `Retry-After` 头 + `code: too_many_attempts` + `retry_after_seconds`**，文案直接写「请约 N 分钟后再试」。
  - 成功登录清零；只统计凭证失败，数据库错误不占用户额度。
  - 进程内实现（重启清零）：家庭内网场景够用，也避免为登录前置路径加表 + 每次失败写盘。
  - 参数化：`LEDGER_LOGIN_MAX_ATTEMPTS` / `LEDGER_LOGIN_WINDOW_MINUTES` / `LEDGER_LOGIN_LOCKOUT_MINUTES`，启动时校验。

### Fixed

- **日历每日金额恒为 0.00**：原实现把当日所有 postings 金额直接相加，而复式记账下借贷必然平衡、相加恒为零。改为按科目类型计算「收入 − 支出」，资产/负债/权益之间的转移不计入。`CalendarView` 新增 `accountTypes` 参数；不传时保持旧行为。
- **月度列表「当日合计」符号相反**：收入分录是贷方（负数）、支出是借方（正数），直接相加会让支出日显示为正（绿）。改为收入取正、支出取负。
- **窄屏（320~360）头部三栏金额换行**：`SummaryRow` 金额改用 `FittedBox(scaleDown)`，保证单行完整显示而不是断成两行撑高头部。
- **日历格金额被省略成「33…」**：同样改用等比缩小，窄格也能读出完整数字。
- **隐私开关未覆盖日历格**：金额隐私此前只作用于 AmountText / 头部 / 列表，日历格仍显示真实金额。新增 `CalendarView.hidden`。
- `flutter analyze` 两处存量告警清除：`analysis_options.yaml` 里 `deprecated_member_use` 不是 lint 规则（analyzer.errors 已忽略），删掉这条无效项；`test/widget_test.dart` 未使用的 material import。

### Changed

- PC 侧边栏与顶栏的 dashboard 文案「仪表盘」→「账单」，与移动端 Tab 统一。
- 空态文案去掉「点右下角」这类只在特定布局成立的指引，改为「点「记一笔」」「点「+」」。

## [0.5.0] - 2026-11-03

「敢上线」版本：补齐工程护栏、容器化与数据备份，让产品从「自己玩」变成「可以给家人开账号、数据丢了能救」。
本版不新增记账功能，只补可靠性底座。

### Added

- **导出兼容性强制校验（M0-a）**：新增 `BEAN_CHECK_STRICT` 环境变量，严格模式下找不到 `bean-check` 直接让测试失败。此前校验器缺失时测试会静默 skip，CI 里形成「假绿灯」——测试全绿但账芯兼容性从未被验证。
  - 抽出 `runBeanCheck` 统一校验入口，候选顺序：项目 `.venv`（Windows/POSIX）→ `python3` → `python` → `bean-check`。
  - 严格区分「校验器不可用」与「账本有错」：前者继续尝试下一候选，后者立即失败。
  - 新增 `TestExportArchiveBeanCheck`：**多文件目录导出首次纳入 bean-check 校验**（此前只有单文件路径被覆盖）。
  - 新增 `scripts/check-bean.sh`：确认校验器可用并用最小账本自检，CI 与本地共用。
- **前端静态托管（M0-b）**：Go 服务可直接托管 Flutter Web 产物，实现「一个进程 + 一个端口」开箱可用。
  - 新增 `LEDGER_WEB_ROOT`（默认 `./web`）；目录不存在时不阻断服务启动，仅提示未托管前端。
  - SPA 回退：无扩展名的未命中路径回退 `index.html`；带扩展名的静态资源缺失返回 404（避免浏览器把 HTML 当 JS 解析）。
  - 分级缓存：`index.html` / `flutter_bootstrap.js` / `manifest.json` 用 `no-cache`，`/assets/`、`/canvaskit/` 用 immutable 长缓存。
  - 安全：关闭目录列表、路径穿越防护（归一化 + 根目录归属双重校验）、`X-Content-Type-Options: nosniff`。
- **容器化（M0-c）**：新增三阶段 `Dockerfile`（Flutter builder → Go builder → alpine runtime），单镜像内含前端产物与 Go 静态二进制，非 root 用户运行，带 `HEALTHCHECK`。
  - 构建信息经 `-ldflags -X` 注入 `buildDate` / `buildHash` / `gitVersion`，启动日志首行打印，便于排障定位版本。
- **配置集中校验（M0-b/M1）**：`config.Validate()` 在启动时校验备份小时与保留份数范围，非法即快速失败。
- **版本号治理（M0-d）**：新增 `scripts/verify-version.sh`，校验 git tag / CHANGELOG / pubspec 三处一致；`frontend/pubspec.yaml` 版本由 `1.0.0+1` 修正为 `0.5.0+1`（此前与 CHANGELOG 长期漂移）。
- **构建与发布链路（M0-e）**：新增 `.github/workflows/docker-image.yml`（tag 触发，先校验后构建，双架构 `linux/amd64` + `linux/arm64` 推送 Docker Hub）、`docker-compose.yml`、`Makefile`。

### Added（M1 数据备份底座）

- **系统管理员角色（M1-a）**：迁移 `008_user_admin` 为 `users` 增加 `is_admin`；首个注册用户在事务内自动置为管理员；旧库升级时若不存在管理员，自动补选 id 最小的可用用户（`status<>'disabled'`）。备份属全库级操作（含全部账本与成员），必须与账本级 owner 区分，否则会出现「A 家庭 owner 下载到 B 家庭账本」的越权。
  - `/api/auth/me`、注册、登录响应新增 `is_admin`。
- **一致性快照引擎（M1-b）**：新增 `internal/backup` 包。
  - 使用 `VACUUM INTO`（参数化）生成快照，**禁止复制数据库文件**——WAL 模式下未 checkpoint 的数据驻留在 `-wal` 中，实测主库 188 KB / WAL 4.1 MB 时直接复制只得到 236/300 条交易，且不报错（形成"看起来有备份、实际数据不全"的假备份）。
  - 快照后强制 `PRAGMA integrity_check` 与五张核心表行数比对（快照不得多于主库）。
  - 生成 `manifest.json`（原子写：临时文件 + rename），记录体积、SHA256、行数、耗时、触发方式。
  - 额外为每个账本导出一份 `.beancount`（从快照库导出，保证严格一致；失败仅告警不影响备份成立）。
  - 互斥锁 + 30 秒超时：数据库连接池为 1，快照期间独占连接，异常时必须能被中断，避免全站阻塞。
- **定时调度与保留（M1-c）**：`Scheduler` 提供启动补跑（NAS 常关机，错过定时点是常态）与每日定时；`ApplyRetention` 按「最近 N 份每日 ∪ 最近 M 个月每月一份 ∪ 全部手动备份」清理，删除前逐个过文件名白名单，绝不使用通配符。
- **备份 API（M1-d）**：`GET /api/backup/list`、`GET /api/backup/download?name=`、`POST /api/backup/run`，全部要求系统管理员。
  - 下载做四层路径穿越防护（非空 / `filepath.Base` 全等 / 拒绝 `..` 与分隔符 / 白名单正则）+ `Join` 后目录归属二次确认，测试覆盖 10 种攻击写法。
  - 下载与手动备份写审计日志（操作人、文件名、IP）。
- **前端备份卡片（M1-e）**：设置页新增「数据备份」卡片，**仅管理员渲染**（非管理员不渲染而非置灰）；展示最近备份时间/份数/占用，支持立即备份、下载最新、展开全量列表、下载任意一份；错误映射为可读中文（401/403/409/503 分别对应不同文案）。
- **优雅停机**：`main.go` 改用 `http.Server` + `Shutdown`，收到 SIGINT/SIGTERM 后先停止接收新请求，再等待备份任务收尾（上限 15 秒）。

### Changed

- **鉴权范围收窄**：`authMiddleware` 由「白名单外的所有路径都要 token」改为「只对 `/api/` 前缀校验」。原实现会导致访问 `/`（前端入口）直接 401，静态托管无法工作；改后新增前端路由无需回来改白名单。
- 备份端点不解析 `X-Ledger-Id`、不校验账本成员身份（备份与具体账本无关），仅需登录态，管理员校验在 handler 内完成。
- `/api/` 下未注册路径在**已登录**时返回 404 JSON（未登录仍先返回 401，不泄露接口是否存在）。
- `newRouter` 签名新增 `webRoot` 参数；`handler.New` 新增 `engine` 参数；`server.New` 增加配置校验前置与备份引擎装配。
- 备份初始化失败不阻断启动：降级为「无备份模式」并在日志告警，备份端点返回 503。记账是核心功能，不能因备份目录不可用而全家无法记账。
- Dockerfile 运行阶段不再声明 `USER app`，改由入口脚本降权——只有入口阶段是 root，服务进程始终是 10001。

### Fixed

- **Docker 构建卡在 `flutter pub get --enforce-lockfile`（exit 65）**：`pubspec.lock` 由开发机 Flutter 3.38.9 生成，而镜像内为 3.35.1；`meta` / `vm_service` / `test_api` / `crypto` 属于随 SDK 捆绑的包，版本由 SDK 决定而非 pub 解析，两个 SDK 版本不同时 pub 必然要求调整这些包，`--enforce-lockfile` 遂直接判失败（日志表现为 "Would change 40 dependencies"）。
  改为「正常解析 + lockfile 漂移告警」：不因 SDK 微差阻断整个镜像构建，但一旦 lockfile 真被重新解析，构建日志会明确打印原因、影响与对齐方式，不静默放过。
- **bind mount 数据目录权限导致容器启动即失败**（`set WAL: unable to open database file (14)`）：镜像以非 root 用户（uid 10001）运行，而宿主机挂载目录通常由 docker/root 创建、属主不是 10001。SQLite 在 WAL 模式下除主库文件外还要在同目录创建 `-wal` / `-shm`，目录不可写即失败。
  - 新增 `docker-entrypoint.sh`：启动瞬间把数据目录属主修正为 10001，随即用 `su-exec` 降权执行，最终服务进程（PID 1）仍是非 root。**部署方不再需要手工 `chown`**。
  - 需要「全程非 root」时，在 compose 指定 `user: "10001:10001"`，入口脚本检测到非 root 会跳过修正直接执行。
  - 属主已正确则跳过递归 chown，避免每次启动都全量遍历数据目录；修正失败（只读挂载 / ACL）只告警不阻断。
  - 注：预置一个空 db 文件并不能绕过该问题——文件存在只解决一半，目录可写才是根因。
- **数据库不可写时的报错无法指导操作**：原样只有一句 `set WAL: unable to open database file (14)`，使用者无从下手。现在会附带排查建议：目录可写性要求、WAL 需要同目录可写、只读挂载、NAS 的 ACL 权限，并直接给出 `chown 10001:10001 <目录>` 命令。
- **反向代理下 API 请求全部指向 localhost:8080**：`kDefaultApiBase` 硬编码 `http://localhost:8080` 并在构建时编译进 JS。通过反向代理 / 域名 / 其他设备访问时，页面 JS 发的 API 请求指向的是「访问者自己电脑的 8080」——表现为 NAS 本机一切正常，换设备或走域名后 API 全部失败（HTTPS 下还会被浏览器按混合内容直接拦截）。改为默认**同域相对路径**（`String.fromEnvironment('API_BASE')`，默认空串）：前端与 API 同端口部署（Go 托管前端）时反代、直连、换端口、HTTPS 均零配置；本地开发与前后端分离部署通过 `--dart-define=API_BASE=...` 注入。

### 已知限制

- `POST /api/transactions`（REST 在线记账）的请求结构**不支持 `tags`**，且 handler 使用 `DisallowUnknownFields`，携带 `tags` 会直接 400。标签目前仅支持离线 `POST /api/sync/push` 路径。两条写入路径能力不对等，待后续版本统一。

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
