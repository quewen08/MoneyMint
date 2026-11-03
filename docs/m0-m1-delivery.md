# M0 + M1 交付报告

> 版本：v1.0 ｜ 日期：2026-11-03
> 上游：`docs/product-review-2026-11.md` v2.1（路线图）→ `docs/m0-m1-requirements.md`（产品拆解）→ `docs/m0-m1-dev-tasks.md`（开发任务书）
> 状态：**M0 全部完成、M1 全部完成**，后端 6 个包测试全绿，前端 `flutter analyze` 零问题

---

## 一、一句话结论

工程护栏、容器化、数据备份三块底座已落地并实测通过。产品从「自己玩、丢了算自己的」变成「可以给家人开账号、数据丢了能救」。

**唯一未验证项**：Docker 镜像构建（本机无 docker/podman/nerdctl），需在 CI 或你的机器上跑一次。

---

## 二、完成清单

### M0 工程护栏 + 容器化

| 编号 | 内容 | 关键文件 | 状态 |
|---|---|---|---|
| M0-a | `bean-check` 严格化 + 多文件目录导出首次纳入校验 | `internal/export/beancheck_test.go`、`archive_test.go`、`scripts/check-bean.sh` | ✅ |
| M0-b | Go 托管前端静态产物（SPA 回退 + 分级缓存 + 防穿越） | `internal/server/static.go`、`middleware.go`、`config.go` | ✅ |
| M0-c | 三阶段 Dockerfile + 构建信息注入 + 优雅停机 | `Dockerfile`、`cmd/server/main.go` | ✅ |
| M0-d | 版本号三处治理（tag / CHANGELOG / pubspec） | `scripts/verify-version.sh`、`CHANGELOG.md`、`pubspec.yaml` | ✅ |
| M0-e | GitHub Actions（双架构推 Docker Hub）+ compose + Makefile + local.sh | `.github/workflows/docker-image.yml`、`docker-compose.yml`、`Makefile`、`scripts/local.sh` | ✅ |

### M1 数据备份底座

| 编号 | 内容 | 关键文件 | 状态 |
|---|---|---|---|
| M1-a | 系统管理员角色（迁移 008 + 旧库自动补选） | `internal/db/migrate.go`、`repository/user.go`、`service/auth.go` | ✅ |
| M1-b | 一致性快照引擎（`VACUUM INTO` + 完整性校验 + manifest） | `internal/backup/engine.go` | ✅ |
| M1-c | 定时调度（启动补跑）+ 双约束保留清理 | `internal/backup/scheduler.go`、`retention.go` | ✅ |
| M1-d | 备份 API + 四层路径穿越防护 + 审计日志 | `internal/handler/backup.go`、`server/router.go` | ✅ |
| M1-e | 前端备份卡片（仅管理员渲染） | `frontend/lib/screens/settings/backup_card.dart`、`ledger_api.dart`、`sync_models.dart` | ✅ |

---

## 三、验证结果

### 3.1 自动化测试

```
ok  familyledger/internal/backup    5.584s
ok  familyledger/internal/db        0.942s
ok  familyledger/internal/export    2.080s
ok  familyledger/internal/handler   3.223s
ok  familyledger/internal/server    1.846s
ok  familyledger/internal/service   4.371s
```

**测试数量变化**：从 2 个测试文件（8 个用例）→ 8 个测试文件、**30+ 用例**。

新增关键测试：

| 测试 | 覆盖什么 |
|---|---|
| `TestSnapshotIncludesUncheckpointedWAL` | WAL 数据进快照（含反面对照：cp 主文件丢数据） |
| `TestValidSnapshotName` | 8 个合法 + 8 个非法文件名 |
| `TestBackupDownloadPathTraversal` | 10 种路径穿越写法全部 400 |
| `TestRetentionKeepsMonthlyHistory` | 月度历史不被每日窗口挤掉 |
| `TestMigrate008BackfillsAdmin` | 旧库升级自动补选管理员 |
| `TestPathTraversalBlocked` | 静态托管的 6 种穿越写法 |
| `TestSpaFallbackForClientRoute` | 前端路由回退 |
| `TestStrictBeanCheckEnv` | 严格模式解析（防护栏静默失效） |

### 3.2 端到端冒烟（`bash scripts/local.sh smoke`）

```
✅ 编译并启动服务
✅ GET /api/health → {"status":"ok"}
✅ 注册成功 + is_admin: true（首个用户自动管理员）
✅ 建账户 id=36/37
✅ 记账成功
✅ 不平衡交易被拒绝（HTTP 400）
✅ 未登录 → 401
✅ 单文件导出 1469 字节 → 通过 bean-check
✅ 目录导出 19 个 .bean → 通过 bean-check
✅ 手动备份 ledger-20261103-133250.db
✅ 备份下载 143360 字节
✅ 路径穿越被拒绝（HTTP 400）
🎉 冒烟全部通过
```

### 3.3 前端

- `flutter analyze lib` → **No issues found!**
- `flutter build web --release` → **Built build\web**（49.6s）
- `dart:html` wasm 警告为既有做法（JS 目标正常），非本次引入

### 3.4 实测硬数据（两处，都已写进代码注释与文档）

| 场景 | 数据 |
|---|---|
| WAL 备份坑（引擎测试） | 主库 188 KB / WAL 4.1 MB，300 条交易；`VACUUM INTO` 得 300 条 ✅；**直接复制主文件只得 236 条** ❌ |
| WAL 备份坑（独立验证） | 主库 4 KB / WAL 2 MB，500 条交易；`VACUUM INTO` 得 500 条 ✅；**直接复制只得 0 条** ❌ |

> 两次数据不同是因为 checkpoint 进度不同——**丢多少取决于时机**，这种"有时丢一点"比"全丢"更危险，因为可能一直没人发现。

### 3.5 `VACUUM INTO` 能力确认

| 验证项 | 结果 |
|---|---|
| 字面量路径 | ✅ |
| **参数化 `VACUUM INTO ?`** | ✅（无需字符串拼接，无注入风险） |
| 目标已存在 | ❌ 报错 `output file already exists`（故必须先确认文件名不冲突） |

---

## 四、过程中发现的真实问题

| # | 问题 | 处理 |
|---|---|---|
| 1 | **`bean-check` 测试会静默跳过**，CI 里形成假绿灯 | 已改 `BEAN_CHECK_STRICT=1` 缺失即 fail；CI 显式装 beancount |
| 2 | **`authMiddleware` 对所有非白名单路径强制鉴权**，静态托管会 401 | 已收窄为「只对 `/api/` 前缀校验」 |
| 3 | **`POST /api/transactions` 不支持 `tags`**，且 `DisallowUnknownFields` 导致带标签直接 400；标签只支持离线 sync/push | **未修**（不属 M0/M1 范围），已记入 CHANGELOG「已知限制」，建议归入 M2 或独立小任务 |
| 4 | 冒烟脚本建的账户名与 seed 默认账户（`Expenses:Food`）撞名导致 409 | 已改用 `Smoke` 前缀 |
| 5 | 版本号三处漂移（CHANGELOG 0.4.0 / pubspec 1.0.0 / 无 tag） | 已统一到 0.5.0，校验脚本上线即拦 |
| 6 | `main.go` 无优雅停机 | 已改 `http.Server` + `Shutdown`，给备份任务留收尾窗口 |

---

## 五、未验证项与已知限制

| # | 项目 | 原因 / 影响 | 建议 |
|---|---|---|---|
| 1 | **Docker 镜像构建** | 本机无 docker / podman / nerdctl | 在 CI 手动 `workflow_dispatch` 试构建一次；或本机 `docker build -t famben:dev .` |
| 2 | arm64 镜像实跑 | 同上 | 推 tag 后在 NAS 上 `docker pull` 验证 |
| 3 | CI 本身未跑过 | 需推送到 GitHub 并打 tag | 首次发布前手动 dispatch 验证 |
| 4 | 前端备份卡片 UI 手测 | 只做了静态分析与构建 | 起服务用管理员登录点一遍 |
| 5 | 定时备份的真实触发 | 单测覆盖了补跑逻辑，但未跨天实测 | 可临时把 `LEDGER_BACKUP_HOUR` 设为当前小时+1 观察 |

---

## 六、关键改动对现有行为的影响

| 变更 | 影响 | 兼容性 |
|---|---|---|
| `authMiddleware` 只对 `/api/` 鉴权 | 非 API 路径不再 401 | 前端原本单独托管，无影响 |
| `/api/` 未知路径：未登录 401 / 已登录 404 JSON | 更安全（不泄露接口存在性） | — |
| `newRouter(h, svc)` → `newRouter(h, svc, webRoot)` | 内部签名变化 | 无外部调用方 |
| `handler.New(svc)` → `handler.New(svc, engine)` | 内部签名变化 | 无外部调用方 |
| `CreateUser` 返回值改为 `(id, isAdmin, err)` | 内部签名变化 | 唯一调用点在 service |
| 迁移 008 | 旧库自动补选管理员 | 幂等，重复执行不改变已有管理员 |
| 备份目录默认 `{DBPath 目录}/backups` | 首次启动自动创建 | `data/` 已在 .gitignore |

---

## 七、下一步

1. **推送到仓库并打 tag `v0.5.0`** 触发 CI（需先确认仓库归属：`origin/master` 目前是 MoneyMint）
2. **验证镜像**：CI 产物在 NAS 上跑起来，确认前端可访问、数据卷持久化、非 root
3. **手测备份卡片**：管理员可见、普通成员不可见、备份/下载可用
4. **进入 M2**：恢复 + Beancount 导入（备份已为恢复铺好路——快照本身就是一个可直接打开的完整库）

---

## 八、文件清单

### 新增

```
Dockerfile
docker-compose.yml
Makefile
.github/workflows/docker-image.yml
scripts/check-bean.sh
scripts/verify-version.sh
scripts/smoke.sh
scripts/local.sh
internal/backup/engine.go
internal/backup/scheduler.go
internal/backup/retention.go
internal/backup/engine_test.go
internal/backup/retention_test.go
internal/db/migrate_test.go
internal/export/beancheck_test.go
internal/export/archive_test.go
internal/handler/backup.go
internal/handler/backup_test.go
internal/server/static.go
internal/server/static_test.go
frontend/lib/screens/settings/backup_card.dart
docs/product-review-2026-11.md
docs/m0-m1-requirements.md
docs/m0-m1-dev-tasks.md
```

### 修改

```
cmd/server/main.go                                  # 构建信息 + 优雅停机 + 调度器
internal/config/config.go                           # WebRoot + 备份配置 + Validate
internal/db/migrate.go                              # 迁移 008
internal/domain/models.go                           # User.IsAdmin
internal/repository/user.go                         # CreateUser 原子判断首个用户 + is_admin 查询
internal/service/auth.go                            # UserInfo.IsAdmin + IsAdmin()
internal/service/service_test.go                    # 首个用户管理员测试
internal/handler/handler.go                         # Handler 持有 backup.Engine
internal/server/middleware.go                       # 鉴权收窄 + 备份端点放行
internal/server/router.go                           # 静态托管 + 备份路由
internal/server/server.go                           # 配置校验 + 引擎装配
internal/export/beancount_test.go                   # 改用 runBeanCheck
frontend/lib/core/api/ledger_api.dart               # isAdmin + 3 个备份方法
frontend/lib/core/api/sync_models.dart              # BackupItem
frontend/lib/screens/settings/settings_screen.dart  # 插入 BackupCard
frontend/pubspec.yaml                               # 版本 0.5.0+1
CHANGELOG.md                                        # 0.5.0 完整记录
AGENTS.md                                           # 端点表 / 环境变量 / 命令入口 / 硬约束
.dockerignore                                       # 补充 CI、文档、脚本
```

---

> 合规声明：本方案为产品设计参考，具体实施请结合公司合规与法务要求确认。
