# FamBean M0 + M1 需求拆解

> 版本：v1.0 ｜ 日期：2026-11-03 ｜ 上游：`docs/product-review-2026-11.md` v2.0
> 范围：M0 容器化与质量护栏 + M1 备份底座
> 性质：可执行需求拆解（含功能规则、验收标准、技术约束），研发可直接据此排子任务

---

## 〇、开工前必须先说的三件事

### 0.1 ⚠️ `origin/master` 不是 FamBean 的代码

老板说「主分支应该有对应实现」，我查了，实际情况是：

| 项 | `origin/master`（远程主分支） | 本地 `develop` + 工作区 |
|---|---|---|
| 项目 | **MoneyMint**（另一套 Beancount Web 工具） | **FamBean** |
| 后端 | Python + Flask 类 | Go + SQLite |
| 前端 | Nuxt（Node） | Flutter Web |
| 结构 | `backend/` + `frontend/` + `Dockerfile` + `compose` | `cmd/` + `internal/` + `frontend/lib/` |
| 历史 | 30+ 提交，有 PR 合并 | 仅 2 个提交（`init` + `0.4版本全量迭代`） |

**两者是两条独立历史**（`git merge-base --is-ancestor` 判定：HEAD 不是 master 祖先）。

**结论**：master 上的 `Dockerfile` / `.github/workflows/docker-image.yml` 是**另一套技术栈**的，**不能直接用于 FamBean**，但**结构可以复用**（见 §1.4 对照表）。

**必须由老板确认的风险点**：如果把 develop 直接推到 master，会**覆盖掉 MoneyMint 的全部历史**。三种处理方式：
- ① 保留 MoneyMint → 新建仓库给 FamBean（推荐，最干净）
- ② 覆盖 master（MoneyMint 若已不用则可行，但历史不可恢复）
- ③ 保留 MoneyMint → FamBean 放到新分支，master 不动

> 本文档按「FamBean 有自己的仓库/分支，master 归属已确认」为假设推进，不预设推送动作。

### 0.2 ⚠️ 版本号三处不一致

| 位置 | 当前值 |
|---|---|
| `CHANGELOG.md` 最新条目 | `0.4.0` |
| `frontend/pubspec.yaml` | `1.0.0+1` |
| git tag | **无任何 tag** |

CI 打镜像必须有一个可信版本源。见 M0-1。

### 0.3 ⚠️ 现有 `bean-check` 测试会静默跳过 → 护栏是空的

`internal/export/beancount_test.go:132` 的逻辑：**找不到 `bean-check` 就 `t.Logf` 跳过校验**。

这意味着：CI 环境没装 `beancount` 时，测试照样绿，但**账芯根本没被校验过**。这是「假绿灯」，比没有护栏更危险——因为它给人安全感。

M0-6 必须把它改成「找不到即 fail」。

---

## 一、M0：容器化与质量护栏

### 1.1 目标

一条命令构建出可在 NAS 上直接跑的镜像（amd64 + arm64），镜像推送到 Docker Hub，且每次构建都强制验证账芯未损坏。

### 1.2 方案对比与选型

#### 决策 1：容器形态 —— 单镜像 vs 双容器

| 方案 | 实现成本 | 复杂度 | 用户体验 | 风险 | 推荐 |
|---|---|---|---|---|---|
| **A. 单镜像**（Go 服务同时托管前端静态文件） | 中（需新增静态托管能力） | 低 | 好（一个容器、一个端口、一条命令） | 低 | ✅ **推荐** |
| B. 双容器（Go + nginx 反代） | 低 | 中（compose 双服务 + 反代配置） | 中（多一层，排障要跨容器） | 中（家庭场景多一个故障点） | 备选 |

**推荐 A 的理由**：家庭 NAS 部署，组件越少越好。多一个 nginx 就多一个要升级、要排障、会挂的东西。而且当前 Go 服务**根本不托管前端产物**（无 `go:embed`，前端靠 `flutter run` 或单独托管），单镜像方案顺手把这个缺口一起补了。

#### 决策 2：版本号来源

| 方案 | 可靠性 | 成本 | 问题 |
|---|---|---|---|
| A. 解析 `pubspec.yaml` | 低 | 低 | 当前值 `1.0.0` 与 CHANGELOG `0.4.0` 不符，会打出错误 tag |
| B. 解析 `CHANGELOG.md` 首个 `## [x.y.z]` | 中 | 低 | 格式脆弱，改标题就崩 |
| **C. git tag 为准 + CI 校验 CHANGELOG 一致** | **高** | 低 | 需要养成打 tag 习惯 | ✅ **推荐** |

**推荐 C**：`git tag v0.5.0` → CI 打镜像 tag `0.5.0` + `latest`；同时校验 `CHANGELOG.md` 顶部版本 == tag，不一致直接 fail。**顺带解决 G-24「文档与代码漂移」**。

#### 决策 3：Flutter 在 Docker 里构建

| 方案 | 镜像体积 | CI 耗时（估算） | 说明 |
|---|---|---|---|
| A. CI 不构建前端，用户本地 `flutter build web` 后挂载 | 小（~30MB） | 快 | 用户多一步，违背「一条命令起服务」 |
| **B. 多阶段构建**（flutter builder → go builder → alpine 运行时） | 中（~60–90MB） | 首建 8–15 分钟，缓存后 2–4 分钟 | ✅ **推荐** |
| C. 前端产物预构建后提交进仓库 | 中 | 最快 | 二进制产物进 git，不推荐 |

**推荐 B**：最终镜像只含 `server` 二进制 + `build/web` 静态产物 + alpine 基础，体积可控。Flutter SDK 只存在于 builder 阶段。

### 1.3 需求清单

#### M0-1 版本号单一事实源与一致性校验

| 项 | 内容 |
|---|---|
| 需求 | 系统应以 **git tag** 为唯一版本源，并在 CI 中校验 `CHANGELOG.md` 顶部版本号与之一致 |
| 规则 | ① tag 格式 `v{major}.{minor}.{patch}`；② CI 提取 tag 去掉 `v` 作为镜像版本；③ CHANGELOG 顶部 `## [x.y.z]` 与 tag 不一致则构建失败并输出明确差异；④ 顺带校验 `pubspec.yaml` 的 `version:` 与 tag 一致（防前端版本长期漂移） |
| 验收 | **Given** 打了 `v0.5.0` 且 CHANGELOG 顶部为 `## [0.5.0]`，**When** 触发构建，**Then** 镜像 tag 为 `0.5.0` 与 `latest`；**Given** CHANGELOG 顶部为 `0.4.0` 而 tag 为 `v0.5.0`，**When** 触发构建，**Then** 构建失败并明确指出两处版本值 |

#### M0-2 前端静态产物由 Go 服务托管（新增能力）

| 项 | 内容 |
|---|---|
| 需求 | Go 服务应能从指定目录托管 Flutter Web 构建产物，实现「一个进程 + 一个端口」开箱可用 |
| 规则 | ① 新增环境变量 `LEDGER_WEB_ROOT`（默认 `./web`），指向 `build/web` 产物目录；② 非 `/api/` 前缀请求走静态文件服务；③ **SPA fallback**：路径不存在时回退 `index.html`（Flutter Web 路由需要）；④ 目录不存在时**不报错退出**，仅在日志提示「未托管前端」，保证后端可独立运行（本地开发场景）；⑤ 静态响应加 `Cache-Control`：`index.html`/`flutter_bootstrap.js` 用 `no-cache`，`assets/` 与 `canvaskit/` 用长缓存（带 hash 文件名）；⑥ 静态托管**不需要鉴权**（前端本身会做登录校验），但 `/api/` 前缀的鉴权逻辑保持不变 |
| 验收 | **Given** `LEDGER_WEB_ROOT` 指向有效 `build/web`，**When** 浏览器访问 `/`，**Then** 返回 Flutter 应用首页；**When** 访问 `/transactions`（前端路由，磁盘无此文件），**Then** 返回 `index.html` 且应用正常渲染该页；**Given** 目录不存在，**When** 启动服务，**Then** 服务正常启动，`/api/health` 可用，日志有提示 |

> 技术约束：不能用 `http.FileServer` 裸挂（会暴露目录列表且无 SPA fallback），需自己包一层 handler，关闭 directory listing。

#### M0-3 多阶段 Dockerfile

| 项 | 内容 |
|---|---|
| 需求 | 应提供多阶段 `Dockerfile`，产出含前端产物与 Go 二进制的单镜像 |
| 规则 | ① 阶段一 `flutter-builder`：基于 Flutter 3.35 镜像，`flutter pub get` + `flutter build web --release`；② 阶段二 `go-builder`：基于 `golang:1.25-alpine`，`CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server`；③ 运行阶段：`alpine` + 非 root 用户运行 + 时区数据（`tzdata`，交易日依赖本地日期）+ `/app/data` 卷目录；④ 暴露 8080；⑤ 镜像内设 `LEDGER_DB=/app/data/ledger.db`、`LEDGER_WEB_ROOT=/app/web`；⑥ 支持 `BUILD_DATE` / `BUILD_HASH` / `GIT_VERSION` 构建参数并可经 `/api/health` 或启动日志输出（排障用） |
| 验收 | **Given** 构建完成的镜像，**When** `docker run -p 8080:8080 -v ./data:/app/data <image>`，**Then** 浏览器访问 `:8080` 可见应用、可注册登录、可记账；**When** 重启容器，**Then** 数据仍在（卷持久化生效）；**When** 查看进程用户，**Then** 非 root |

> 技术约束 1：Flutter 的 `--release` 构建在 CI 上耗时较长，需启用 buildx 缓存（`cache-from/to: type=gha`）。
> 技术约束 2：`go.mod` 声明 `go 1.25.0`，builder 镜像必须与之匹配，不可用更低版本。
> 技术约束 3：Flutter 构建不需要 `PUB_HOSTED_URL` 国内镜像（CI 在境外，用默认源更快）；该变量只在本地需要。

#### M0-4 docker-compose

| 项 | 内容 |
|---|---|
| 需求 | 应提供 `docker-compose.yml`，供 NAS 用户直接 `docker compose up -d` 起服务 |
| 规则 | ① 单服务，镜像 `{DOCKER_USERNAME}/fambel?:latest`（**命名待定，见 §5 待确认**）；② 数据卷映射到宿主目录（默认 `./data`）；③ 环境变量示例：`LEDGER_DB`、时区 `TZ=Asia/Shanghai`；④ `restart: unless-stopped`；⑤ 端口映射 `8080:8080`；⑥ 附 `docker-compose.yml` 旁的部署说明注释（含 arm64 NAS 支持说明） |
| 验收 | **Given** 一台装了 Docker 的机器，**When** 执行 `docker compose up -d`，**Then** 服务起来且可访问；**When** `docker compose down && up -d`，**Then** 数据不丢 |

#### M0-5 GitHub Actions：构建 + 推 Docker Hub

| 项 | 内容 |
|---|---|
| 需求 | 应在 tag 推送时自动构建多架构镜像并推送至 Docker Hub |
| 规则 | ① 触发：`push: tags: ['v*']`（**不用 master 分支触发**——本项目按 tag 发布更符合「不急上线」节奏）；② 可选触发 `workflow_dispatch`（手动）；③ `docker/setup-qemu-action` + `docker/setup-buildx-action`；④ 平台 `linux/amd64` + `linux/arm64`（NAS 需要）；⑤ 登录用 secrets `DOCKER_USERNAME` / `DOCKERHUB_TOKEN`（沿用 MoneyMint 命名，已配则零改动）；⑥ 镜像 tag：`{version}` + `latest`；⑦ 构建前先跑 `go vet` / `go test` / `bean-check` 校验（见 M0-6），**校验失败不推镜像** |
| 验收 | **Given** 仓库配好上述两个 secret，**When** 推送 `v0.5.0` tag，**Then** DockerHub 出现 `0.5.0` 与 `latest` 两个 tag，且两个架构都可用（arm64 在 NAS 上 `docker pull` 成功）；**Given** 校验步骤失败，**When** 触发构建，**Then** 镜像不推送 |

#### M0-6 bean-check 护栏强化（关键）

| 项 | 内容 |
|---|---|
| 需求 | 导出兼容性校验必须是**强制项**，不允许静默跳过 |
| 规则 | ① 新增环境变量/构建标记控制严格模式（如 `BEAN_CHECK_STRICT=1`）；② 严格模式下找不到 `bean-check` / `python -m beancount.scripts.check` → **测试 fail 并给出安装指引**；③ 非严格模式（本地无 venv 时）保持现有 skip 行为，但要打印醒目警告；④ CI 中显式 `pip install beancount` 并设 `BEAN_CHECK_STRICT=1`；⑤ 校验范围不止 `TestExportTags`：**单文件导出**与**多文件目录导出（zip 解压后跑 main.bean）** 两条路径都要覆盖 |
| 验收 | **Given** CI 环境已装 beancount 且 `BEAN_CHECK_STRICT=1`，**When** 跑测试，**Then** 单文件与多文件两种导出均实际执行 bean-check 且退出码 0；**Given** 故意移除 beancount，**When** 严格模式跑测试，**Then** 测试失败并提示安装方式（不得 skip） |

> 这一条是整个 M0 里**最重要的护栏**。没有它，后面所有账芯改动都是盲改。

#### M0-7 本地一键脚本

| 项 | 内容 |
|---|---|
| 需求 | 应提供统一的本地命令入口，避免每次手敲长命令 |
| 规则 | 提供 `Makefile`（或 `justfile`），至少含：`make dev`（起后端）、`make web`（构建前端）、`make test`（go test + 前端 analyze）、`make check`（test + bean-check 严格校验）、`make image`（本地构建镜像）、`make smoke`（起服务 → 健康检查 → 注册 → 建账户 → 记账 → 导出 → bean-check，全自动） |
| 验收 | **Given** 干净环境，**When** 执行 `make check`，**Then** 全部校验跑完并给出明确通过/失败结论；**When** 执行 `make smoke`，**Then** 自动完成一条完整记账链路并校验导出 |

> `make smoke` 是上线前唯一需要人记的「最终确认」，把它脚本化能省掉每次的重复劳动。

### 1.4 与 MoneyMint 现有实现的对照（复用 vs 重写）

| 项 | MoneyMint master 实现 | FamBean 处理 |
|---|---|---|
| workflow 骨架（QEMU + buildx + login + 多平台） | ✅ 有 | **直接复用**，改触发条件与镜像名 |
| secrets 命名（`DOCKER_USERNAME` / `DOCKERHUB_TOKEN`） | ✅ 有 | **直接复用**（若已在仓库配好，零改动） |
| 版本号提取 | 从 `frontend/package.json` 读 | **重写**：改为 git tag + CHANGELOG 校验 |
| 构建参数（BUILD_DATE/HASH/BRANCH） | ✅ 有 | **复用思路**，FamBean 补 `GIT_VERSION` |
| Dockerfile 前端阶段 | `node:20-alpine` + npm | **重写**：Flutter builder |
| Dockerfile 后端阶段 | `python:3.11-alpine` + venv/pip | **重写**：`golang:1.25-alpine` + 静态编译 |
| 运行时基础镜像 | `python:3.11-alpine` | **改为** `alpine` + 非 root |
| `.dockerignore` | ✅ 有 | **复用并补充**（加 `frontend/build`、`data/`、`.venv`、`*.db*`） |
| 触发分支 | `push: branches: ["master"]` | **改为** `tags: ['v*']` |

> 一句话：**骨架能抄，内容全要重写**。这不影响 M0 的价值判断，但不要把「master 有现成的」当成「M0 已经完成一半」。

### 1.5 M0 工作量修正

| 原估 | 修正后 | 差异原因 |
|---|---|---|
| 0.5~1 天 | **2~2.5 天**（估算） | ① M0-2 前端静态托管是**全新功能**，原评估漏了；② Flutter in Docker 需要调通构建缓存与镜像体积；③ 版本号三处漂移需要治理；④ bean-check 严格化要改测试 + 补多文件导出路径 |

**拆成可中断的原子任务**（每项可独立完成并验证）：

| 序号 | 任务 | 估时 | 可独立验收 |
|---|---|---|---|
| M0-a | bean-check 严格化 + 多文件导出校验 | 0.5 天 | ✅ 严格模式下缺 beancount 会 fail |
| M0-b | Go 静态托管 + SPA fallback + 缓存头 | 0.5~1 天 | ✅ 本地 `build/web` 可被服务 |
| M0-c | Dockerfile 三阶段 + 体积/非 root 验证 | 0.5 天 | ✅ 本地 `docker run` 可用 |
| M0-d | 版本号治理（tag + CHANGELOG + pubspec 三处校验脚本） | 0.3 天 | ✅ 故意错版能 fail |
| M0-e | workflow + compose + Makefile | 0.5 天 | ✅ 手动 dispatch 跑通一次 |

---

## 二、M1：备份底座

### 2.1 目标

账本数据定时自动快照、可一键下载完整备份包。**恢复不做**（归 M2），但快照必须保证「将来一定能还原成功」。

### 2.2 关键前提：备份的权限归属（必须先决策）

备份是**全库级**操作（包含所有账本、所有用户、所有交易），不是账本级。但当前权限模型里**只有账本级 owner，没有系统管理员**。

| 方案 | 说明 | 风险 | 推荐 |
|---|---|---|---|
| A. 任意账本 owner 都能下载全库备份 | 实现最简单 | **严重越权**：A 家庭账本的 owner 能拿到 B 家庭的账本数据 | ❌ |
| B. 引入系统管理员（`users.is_admin`，首个注册用户自动为 true） | 加迁移 008 + 中间件判断 | 低，符合直觉 | ✅ **推荐** |
| C. 只能通过 NAS 文件系统直接取 db 文件 | 零代码 | 不满足「一键下载」，且绕不过 WAL 一致性 | ❌ |

**推荐 B**。附带收益：后续「公开阈值」里的资源配额、注册控制都需要一个全局管理员角色。

### 2.3 方案对比：快照怎么做（技术约束，必须写清）

| 方案 | 一致性 | 说明 | 推荐 |
|---|---|---|---|
| A. 直接 `cp ledger.db` | ❌ **会丢数据** | WAL 模式下未 checkpoint 的事务在 `-wal` 文件里，`cp` 主文件会得到残缺快照 | ❌ |
| **B. `VACUUM INTO 'snapshot.db'`** | ✅ 强一致 | SQLite 原生一致性快照，自动合并 WAL，产出单文件、已整理 | ✅ **推荐** |
| C. SQLite Online Backup API | ✅ 强一致 | 更灵活（可增量、可进度回调），但代码量更大 | 备选 |

**这是铁证不是推测**：项目 `data/` 目录实测 —— `verify_tags.db` **32 KB**，而 `verify_tags.db-wal` **1.5 MB**。直接 copy 主文件会丢掉约 **98%** 的数据。

> 需求里必须写死：**禁止直接复制 db 文件，必须用一致性快照**。这一条如果漏了，M1 会做出一个「看起来有备份、实际还原时数据不全」的假备份——比没备份更糟。

### 2.4 备份内容与存放

| 项 | 决策 |
|---|---|
| 快照格式 | 单文件 SQLite 快照 `.db`（M2 可直接用） |
| 附加产物 | 同时生成一份 `.beancount` 导出文本（人可读、可手工抢救、双保险） |
| 存放位置 | ① 默认 `{LEDGER_DB 所在目录}/backups/`；② 支持 `LEDGER_BACKUP_DIR` 指向**外部挂载路径**（防单盘故障） |
| 命名 | `ledger-{YYYYMMDD-HHmmss}.db` + 同名 `.beancount` |
| 元信息 | 同目录写 `manifest.json`：每份备份的时间、大小、SHA256、schema 版本、记录数（账户/交易/账本/用户）、触发方式（auto/manual） |

### 2.5 需求清单

#### M1-1 系统管理员角色

| 项 | 内容 |
|---|---|
| 需求 | 系统应有全局管理员概念，用于执行全库级操作（备份、下载、后续的恢复） |
| 规则 | ① 迁移 008：`users` 增 `is_admin INTEGER DEFAULT 0`；② 首个注册用户（id 最小且未删除）自动置 1；③ 若升级旧库时无 admin 存在，将 id 最小的用户置为 admin；④ `/api/auth/me` 返回 `is_admin`；⑤ 新增中间件 `requireAdmin` |
| 验收 | **Given** 全新库首个用户注册，**When** 查 `me`，**Then** `is_admin=true`；**Given** 升级已有旧库，**When** 启动服务，**Then** 自动选出 admin 且只有 1 个；**Given** 非 admin 用户，**When** 访问备份端点，**Then** 403 |

#### M1-2 一致性快照

| 项 | 内容 |
|---|---|
| 需求 | 系统应能生成账本数据的一致性快照，且快照可用于完整还原 |
| 规则 | ① 使用 `VACUUM INTO`（或等价备份 API），**禁止直接复制 db 文件**；② 快照完成后校验：打开快照库、跑 `PRAGMA integrity_check`、比对核心表行数与原库一致；③ 校验不通过则删除该快照并记错误日志，不保留坏备份；④ 同一时刻只允许一个备份任务（互斥锁）；⑤ 快照生成期间不阻塞正常读写（`VACUUM INTO` 为读操作，可接受） |
| 验收 | **Given** 库处于 WAL 模式且有未 checkpoint 的写入，**When** 生成快照，**Then** 快照中这些写入**存在**且 `integrity_check=ok`，行数与原库一致（此条专门验证 §2.3 的坑）；**Given** 快照生成中磁盘写满，**When** 失败，**Then** 不留残缺文件、返回明确错误、服务不崩溃 |

#### M1-3 自动定时备份

| 项 | 内容 |
|---|---|
| 需求 | 系统应每日自动生成一次快照，不依赖外部 cron |
| 规则 | ① Go 内部定时器（`time.Ticker`），默认每日 1 次；② 时间点可配（`LEDGER_BACKUP_HOUR`，默认 3 点，按容器时区）；③ **补跑机制**：服务启动时检查「距上次成功备份是否已超过设定间隔」，超过则立即补一次（NAS 常关机，错过定时点很常见）；④ 启动补跑有随机延迟（0~60 秒）避免多实例同时打库；⑤ 服务关闭时优雅停止定时器 |
| 验收 | **Given** 上次备份在 30 小时前，**When** 服务启动，**Then** 自动触发一次备份；**Given** 服务连续运行跨过设定时间点，**When** 到点，**Then** 自动生成快照且日志可查；**Given** 备份失败，**When** 下次到点，**Then** 仍然重试且失败原因记录完整 |

#### M1-4 保留策略

| 项 | 内容 |
|---|---|
| 需求 | 备份不应无限增长，应按策略清理旧快照 |
| 规则 | ① 双约束取交集保留：**最近 N 份每日备份**（默认 14）+ **最近 M 份月度首日备份**（默认 12）；② `LEDGER_BACKUP_KEEP_DAILY` / `LEDGER_BACKUP_KEEP_MONTHLY` 可配；③ 清理只在**新备份成功后**执行（避免先删后建导致空窗）；④ 清理按清单精确删除（`.db` + 同名 `.beancount` + manifest 条目），**禁止通配符删除整个目录**；⑤ 手动备份（manual）默认不参与自动清理 |
| 验收 | **Given** 已有 20 份每日备份，**When** 新备份成功，**Then** 最旧的 7 份被清理，剩 14 份；**Given** 有月度首日备份，**When** 清理，**Then** 月度备份不受日份数限制影响；**Given** 清理过程中出错，**When** 下次运行，**Then** 不误删非备份文件 |

#### M1-5 备份查询与下载 API

| 项 | 内容 |
|---|---|
| 需求 | 管理员应能查看备份列表并下载任意一份 |
| 规则 | ① `GET /api/backup/list` → 返回各备份的名称、时间、大小、SHA256、记录数、触发方式；② `GET /api/backup/download?name=` → 下载，**必须校验 name 白名单**（防路径穿越，只允许匹配 `ledger-*.db` / `ledger-*.beancount` 的 basename，拒绝任何 `..` 或路径分隔符）；③ `POST /api/backup/run` → 手动触发一次并返回结果；④ 全部端点 `requireAdmin`；⑤ 下载响应设置 `Content-Disposition: attachment`；⑥ 下载动作写审计日志（时间、用户、文件名、IP） |
| 验收 | **Given** 管理员，**When** 查列表，**Then** 返回按时间倒序的备份清单；**When** 下载，**Then** 得到完整文件且 SHA256 与列表一致；**Given** 传入 `name=../../etc/passwd`，**Then** 返回 400 且不泄露文件；**Given** 非管理员，**Then** 403 |

> 安全提示：路径穿越是这类下载接口的经典漏洞。白名单 + 只取 basename，两道都要。

#### M1-6 前端备份入口

| 项 | 内容 |
|---|---|
| 需求 | 管理员应能在设置页看到备份状态并操作 |
| 规则 | ① 设置页新增「数据备份」卡片（仅 `is_admin` 可见）：最近成功备份时间、总份数、占用空间；② 「立即备份」「下载最新备份」两个按钮；③ 备份列表可展开，逐条下载；④ 备份失败时卡片显示红色状态与失败原因；⑤ 非管理员不显示该卡片（不是灰掉，是不渲染） |
| 验收 | **Given** 管理员登录，**When** 进设置页，**Then** 可见备份卡片与最近备份时间；**When** 点「立即备份」，**Then** 按钮进入 loading、完成后时间刷新；**Given** 普通成员，**Then** 设置页无备份卡片 |

#### M1-7 备份审计

| 项 | 内容 |
|---|---|
| 需求 | 全库级敏感操作应留痕 |
| 规则 | ① 记录：自动备份成功/失败、手动备份、下载（含文件名与操作人）、清理动作；② 写结构化日志（时间戳、级别、事件、用户、结果）；③ 保留在日志文件，不单独建表（M1 阶段避免过度设计，M3 再做审计表） |
| 验收 | **Given** 管理员下载一份备份，**When** 查日志，**Then** 有对应记录含用户名与文件名 |

### 2.6 M1 与 M2 的边界（防范围蔓延）

| 事项 | 归属 | 原因 |
|---|---|---|
| 自动快照、下载、保留策略、权限 | **M1** | 本节范围 |
| 从备份**还原** | M2 | 涉及停机窗口、还原前快照、迁移版本兼容 |
| Beancount **导入** | M2 | 涉及解析、冲突、未知指令保留 |
| 备份加密 | 暂不做 | 内网 + 自持，加密会带来密钥管理问题；公开后重新评估 |
| 备份异地同步（S3/WebDAV） | 暂不做 | 用户可自行把 `backup` 目录挂到同步盘，产品不做集成 |

### 2.7 M1 工作量

| 原估 | 修正后 | 说明 |
|---|---|---|
| 1.5~2 天 | **2~2.5 天**（估算） | 增量：① 引入 admin 角色（迁移 + 中间件 + 前端适配）；② manifest 与完整性校验；③ 补跑机制与互斥；④ 路径穿越防护 |

**原子任务拆分**：

| 序号 | 任务 | 估时 |
|---|---|---|
| M1-a | 迁移 008 + admin 角色 + `requireAdmin` 中间件 | 0.5 天 |
| M1-b | 快照引擎（VACUUM INTO + 校验 + manifest） | 0.5~1 天 |
| M1-c | 定时器 + 补跑 + 保留清理 | 0.5 天 |
| M1-d | 三个 API + 安全防护 + 审计日志 | 0.3 天 |
| M1-e | 前端备份卡片 + 列表 + 下载 | 0.5 天 |

---

## 三、执行顺序

```
M0-a (bean-check 严格化)  ← 先做，后面所有改动才有护栏
      ↓
M0-b (静态托管) → M0-c (Dockerfile) → M0-d (版本治理) → M0-e (workflow/compose/Makefile)
      ↓
M1-a (admin) → M1-b (快照引擎) → M1-c (定时/清理) → M1-d (API) → M1-e (前端)
```

**为什么 M0-a 必须第一**：它成本最低（半天）、保护范围最大（覆盖之后所有账芯与导出改动）。**它不完成，M0 其余部分做得再好都是砂上建塔。**

**为什么 M1-a 在 M1-b 前**：备份端点的权限模型必须先定，否则快照做好了也没法安全地给出去。

---

## 四、风险登记册（M0/M1 范围）

| 编号 | 风险 | 类别 | 概率 | 影响 | 等级 | 应对 |
|---|---|---|---|---|---|---|
| RK-M01 | 沿用 master 的 CI 配置误以为已就绪，实际技术栈完全不同 | 需求 | 4 | 3 | **12 中** | §1.4 对照表逐项核对；不把「master 有」当「可复用」 |
| RK-M02 | `bean-check` 在 CI 静默跳过，护栏形同虚设 | 技术 | 5 | 4 | **20 高** | M0-6 严格模式，找不到即 fail；CI 显式 `pip install beancount` |
| RK-M03 | 用 `cp` 备份 SQLite，快照残缺（WAL 场景实测丢 98%） | 数据 | 4 | 5 | **20 高** | 强制 `VACUUM INTO` + 快照后完整性校验与行数比对 |
| RK-M04 | 备份下载端点路径穿越致任意文件读取 | 安全 | 3 | 5 | **15 高** | basename 白名单 + 拒绝 `..` 与分隔符 + 单测覆盖 |
| RK-M05 | 推送到 master 覆盖 MoneyMint 历史 | 进度/数据 | 3 | 4 | **12 中** | 推送前由老板确认仓库归属（§0.1 三选一） |
| RK-M06 | Flutter in Docker 构建超时或镜像过大 | 技术 | 3 | 3 | 9 中 | buildx gha 缓存；builder 与 runtime 严格分离；实测体积并记录 |
| RK-M07 | NAS arm64 上镜像不可用（本地只测了 amd64） | 技术 | 3 | 4 | **12 中** | CI 双平台构建；至少在 arm64 环境 `docker pull + run` 实测一次 |
| RK-M08 | 定时备份与前端请求并发导致锁等待 | 技术 | 2 | 3 | 6 低 | 备份互斥锁 + `VACUUM INTO` 为读操作 + 失败重试 |

---

## 五、待确认（3 项，需老板拍板）

| # | 问题 | 影响 | 我的建议 |
|---|---|---|---|
| Q-1 | **FamBean 的代码放哪个仓库/分支？** master 现在是 MoneyMint | 决定 M0-5 的 workflow 落在哪、是否会覆盖老项目 | 新建独立仓库（最干净）；若复用现有仓库，则 FamBean 放独立分支，master 保持不动 |
| Q-2 | **Docker Hub 镜像名取什么？** MoneyMint 用的是 `{user}/money-mint` | 决定 compose 与 workflow 里的镜像名 | 与仓库名统一，避免以后再次改名；若计划把项目正式叫 FamBean，就用 `fambel`/`famben`？**需你定名** |
| Q-3 | **Docker Hub 的 `DOCKER_USERNAME` / `DOCKERHUB_TOKEN` 是否已在新仓库配好？** | 没配的话 CI 推不上去 | 沿用原命名，重新在该仓库 Settings → Secrets 里配一次 |

---

## 六、附录

### 6.1 验收总表（M0 + M1 一起看）

| 编号 | 验收项 | 可自动化 |
|---|---|---|
| M0-1 | tag 与 CHANGELOG/pubspec 不一致时构建失败 | ✅ CI |
| M0-2 | 单端口访问前端 + SPA 路由回退正常 | ⚠️ 半自动（curl 断言） |
| M0-3 | 单镜像 `docker run` 后完整可用、重启数据不丢、非 root | ✅ 脚本 |
| M0-4 | compose 一键起停，数据持久 | ✅ 脚本 |
| M0-5 | tag 推送产出 amd64 + arm64 双架构镜像 | ✅ CI |
| M0-6 | 严格模式下 bean-check 缺失即 fail；单文件与多文件导出均实校 | ✅ CI |
| M0-7 | `make check` / `make smoke` 一条命令跑通 | ✅ 本地 |
| M1-1 | 首个用户自动为 admin；旧库升级自动补 | ✅ 测试 |
| M1-2 | WAL 未 checkpoint 数据在快照中存在；快照 integrity_check 通过 | ✅ 测试（关键） |
| M1-3 | 启动补跑；到点自动备份 | ⚠️ 半自动（缩短间隔实测） |
| M1-4 | 双约束保留；只删白名单文件 | ✅ 测试 |
| M1-5 | 列表/下载/手动备份；路径穿越被拒 | ✅ 测试 |
| M1-6 | 管理员可见备份卡片，普通成员不渲染 | ⚠️ 手测 |
| M1-7 | 下载留痕 | ✅ 日志断言 |

### 6.2 工作量汇总

| 里程碑 | 原子任务数 | 合计（估算，自然日） |
|---|---|---|
| M0 | 5 | 2 ~ 2.5 |
| M1 | 5 | 2 ~ 2.5 |
| **合计** | **10** | **4 ~ 5** |

> 估算基于单人全栈、含自测，不含返工。碎片时间推进时按原子任务为单位，任一任务完成即是一个可交付状态。

### 6.3 上游一致性

- 本拆解对应 `docs/product-review-2026-11.md` v2.0 的 M0 / M1
- 与 v2.0 的差异：M0 工作量从 0.5~1 天上调至 2~2.5 天（原因见 §1.5）；M1 从 1.5~2 天上调至 2~2.5 天（原因见 §2.7）；新增「系统管理员角色」需求（原 v2.0 未识别此缺口）
- 待 v2.0 文档下次修订时同步更新工作量

> 合规声明：本方案为产品设计参考，具体实施请结合公司合规与法务要求确认。
