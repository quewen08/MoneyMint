# CLAUDE.md — FamBean 工作约定

> 给 Claude 系工具的项目速查。**权威技术上下文在 [`AGENTS.md`](AGENTS.md)**；本文只记「怎么干活」与「不要做什么」，不重复全文。

## 先读这三件事

1. **M7 全量重构已 cutover 收官（批 0~6，2026-11-12）**：**Vue 3 + Vite SPA 是唯一前端**（`frontend/src/` → `frontend/dist`，由 `LEDGER_WEB_ROOT` 托管）。Flutter Web 源码与构建链已 `git rm`（commit `d297318`，需时从 git 历史找回），`frontend/dist/legacy/` 是**冻结的回滚基准**（仅本机留存、不入库、不再重建）。
2. **别把方案当现状，也别把旧文档当现状**：批 0~6 均已实现——20 页全量覆盖、36 端点、`core/Money`、离线同步内核 `core/sync/`、迁移 013/014、M2 恢复/导入四端点、PWA `public/sw.js`、CI `web-verify` job、Dockerfile Node 构建、工具链钉版。判断「有没有」**以代码为准**。
3. **账芯不可动**：复式借贷平衡、append-only 不可变交易、导出必须过 `bean-check`、定点十进制金额。改的是前端，不是全栈。

## 命令速查

统一入口 `bash scripts/local.sh <cmd>`（Windows 无 make；Linux/CI 等价目标在 `Makefile`）。

| 目的 | 命令 |
|---|---|
| 后端全量测试（严格 bean-check） | `bash scripts/local.sh test` |
| CI 同款全量校验 | `bash scripts/local.sh check` |
| 端到端冒烟 | `bash scripts/local.sh smoke` |
| 起后端（托管 `frontend/dist`） | `bash scripts/local.sh dev` |
| Vue 构建 / 开发服务器 | `bash scripts/local.sh web-build` / `web-dev` |
| Vue 单测 / 类型 / 兼容扫描 | `bash scripts/local.sh web-test` / `web-typecheck` / `web-compat` |
| 校验冻结的 `/legacy/` 回滚基准 | `bash scripts/local.sh check-legacy` |

- `web-dev` 的 `/api` 已由 `vite.config.ts` 代理到 `127.0.0.1:8080`，**前端代码一律用相对 `/api`**。
- **`web-build` 会清空 `frontend/dist`（含本机留存的 `dist/legacy/`）**。需要保留回滚基准时**先备份该目录**；构建源 `frontend/build/legacy-web` 仍在盘上，可直接拷回，无需 Flutter。
- 本机 Git Bash 可能需 `export PATH="$PATH:/d/Environment/go/bin"`。Node 需 22.x（`engines >=22.22 <23`，`engine-strict=true`）。
- Python 只用于 `bean-check`，**一律用项目内 `.venv`**：`.venv/Scripts/python.exe -m beancount.scripts.check <file>`。不在项目外创建 venv / 装包 / 跑脚本。

## 关键文件索引

| 关心什么 | 看哪里 |
|---|---|
| 后端路由与端点（36 条） | `internal/server/router.go`、`AGENTS.md` §7 |
| CORS / CSRF / Cookie 鉴权 | `internal/server/middleware.go`、`internal/config/config.go` |
| 认证与会话 | `internal/handler/auth.go`、`internal/service/auth.go` |
| 交易与幂等 | `internal/service/transaction.go`、`internal/repository/transaction.go`、`internal/db/migrate013.go` |
| 迁移清单（下一个编号 **015**） | `internal/db/migrate.go` 的 `migrations` 数组（当前至 `014_db_instance_id`） |
| 库世代 | `internal/db/db_instance.go`、`internal/service/instance.go`、`GET /api/db/instance` |
| 恢复 / 导入 | `internal/restore/`、`internal/beancount/`、`internal/service/{restore,imports}.go`、`internal/handler/{restore,import}.go` |
| 同步裁决 | `internal/service/sync.go`（push 稳定码 `account_not_found` / `account_closed`） |
| 静态托管 / SPA 回退 | `internal/server/static.go` |
| 导出器 | `internal/export/{beancount,files}.go` |
| Vue 定点金额 | `frontend/src/core/Money.ts` + `Money.test.ts` |
| Vue 端点契约 | `frontend/src/api/{types,routes,errors,client}.ts` |
| 离线同步内核 | `frontend/src/core/sync/{models,localStore,idbStore,engine}.ts` + `fixtures/`（8 份真实契约样本） |
| 构建与兼容基线 | `frontend/vite.config.ts`、`frontend/package.json`、`frontend/.nvmrc` |
| CI 前端门禁 | `.github/workflows/docker-image.yml` 的 `web-verify` job、`scripts/check-test-floor.mjs` |
| 旧版回滚（冻结基准） | `scripts/check-legacy.sh`、`frontend/dist/legacy/` |

## 硬约束（违反即返工）

- **金额**：禁止浮点参与运算/存储。只走 `core/Money.ts`（`decimal.js`，只收 string、`ROUND_HALF_UP`）；后端写定点十进制字符串。
- **Beancount**：改导出器后必须重跑 `bean-check`；`BEAN_CHECK_STRICT=1` 找不到校验器就必须失败，不许改回静默跳过。
- **迁移**：schema 改动一律追加 `migrate.go` 迁移并记 `schema_migrations`，不手改线上库。幂等索引用 `ON CONFLICT(ledger_id, client_request_id) WHERE client_request_id IS NOT NULL`。
- **认证**：token 不落 `localStorage` / URL；Cookie 路径非 GET 必须带 `X-CSRF`；`SameSite=None` 禁用；**服务端 Bearer 双读兜底保留**（`LEDGER_AUTH_EXPOSE_TOKEN` 默认 `false`，响应体不再向 JS 暴露 token；仅回滚 `/legacy/` 时显式设 `true`）。
- **多账本**：业务请求必须带 `X-Ledger-Id`；新 API 客户端缺失即抛错，**绝不静默回退默认账本 id=1**。`/api/import/*` 与 `/api/db/*` 属账本上下文豁免路径。
- **posting 字段**：在线 `POST /api/transactions` 用 `account_id`（整数）；离线 `sync/push` 用 `account_uuid`（字符串）。混用必错。
- **不可变**：改账 = 新增冲正交易，删除 = 软删；交易不原地改。
- **兼容性（iOS 14.8 / Safari 14.1.2）**：`build.target: 'es2015'` + plugin-legacy；禁 WASM / WebGL / CanvasKit / OffscreenCanvas；禁 `structuredClone` / `crypto.randomUUID` / `.at()` / `findLast` / `Object.hasOwn` / `showModal` / `BroadcastChannel` / `navigator.locks`；CSS 不用 `aspect-ratio` / `:has()` / `dvh` / `inset`。
- **依赖钉版**：按方案 §14.7，`.nvmrc` / `engines` / Dockerfile ARG 三处同步。**禁装** ECharts / Vuetify / Framework7 / moment / monaco / leaflet / swiper；图表用 Canvas 2D 自绘。升级依赖必须过 `web-compat` + 真机冒烟。
- **备份**：只能用 `VACUUM INTO` + `integrity_check` + 行数比对，**禁止复制 db 文件**（WAL 会静默丢数据）。备份/下载/恢复校验 `users.is_admin`，不是账本 owner。
- **注释**：代码注释用中文。

## 不要做（未经明确指示）

- 不要试图恢复 Flutter 构建链 / 重建 `pubspec.*` / 再引入 `web-legacy` 目标——批 6 已清退，回滚基准是冻结产物。
- 不要把 Vue 产物输出到 `frontend/web/`，正确落点是 `frontend/dist/`（`emptyOutDir` 会清空目标目录）。
- 不要提前引入 SSR / Nuxt / Node 运行时；不要为「顺手」加预算、AI 识别、深色模式、附件、2FA（方案已明确不做）。
- 不要把 `LEDGER_AUTH_EXPOSE_TOKEN` 默认值改回 `true`（除非明确要回滚 `/legacy/`）。
- 不要跳过旧功能：CSV 导出、分类拖拽排序、导出不平告警、注册开关、默认账户管理在新版一律 **Must**（方案 §14.5）。

## 改完怎么验

按项目规则：**先本地测试通过，才允许提交；测试前不要 commit。**

1. 后端改动 → `bash scripts/local.sh check`。
2. 前端改动 → `web-test` + `web-typecheck` + `web-build` + `web-compat` 四项；CI 的 `web-verify` job 会再跑一遍并校验**用例数下限**（`scripts/check-test-floor.mjs`，防「文件加载失败只剩少量用例」的假绿）。
3. 改了导出器或账芯 → 追加 `bean-check`（严格模式）与导出 round-trip。
4. 涉及认证 / 幂等 / 同步 → 跑 `internal/server/*_test.go`、`internal/db/migrate013_test.go` 与 `frontend/src/core/sync/engine.test.ts`，并核对 Cookie / Bearer 双通道未被破坏。
5. 涉及前端构建或兼容基线 → iOS 14.8 Safari 真机全主流程冒烟（AC-19）。
6. 版本号三处一致：`git tag` / `CHANGELOG.md` / `frontend/package.json`（`bash scripts/verify-version.sh`）。

> 文档收口后工作区应干净；**改动前仍先看 `git status`**，不要把 docs 与代码改动混在同一次提交。CI `web-verify` 与 Dockerfile Node 构建的 Linux 实机首跑需 push 后观察，别把「配置写了」当成「已经跑过」。
