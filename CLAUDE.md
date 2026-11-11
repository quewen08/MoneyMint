# CLAUDE.md — FamBean 工作约定

> 给 Claude 系工具的项目速查。**权威技术上下文在 [`AGENTS.md`](AGENTS.md)，重构方案在 [`docs/next-phase-frontend-plan.md`](docs/next-phase-frontend-plan.md)（v2.5）**；本文只记「怎么干活」与「不要做什么」，不重复全文。

## 先读这三件事

1. **当前是 M7 全量重构的双前端并存期**：默认生产前端仍是 **Flutter Web**（`frontend/lib/` → `frontend/build/web`）；**Vue 3 + Vite SPA**（`frontend/src/` → `frontend/dist`）已落地批 0~2.5（57 文件 / 14,067 行 / 11 页 / Vitest 202 用例全过），**尚未 cutover**。
2. **别把方案当现状**：方案 v2.5 的 20 个页面只完成 11 个；离线内核、迁移 014、M2 四端点、PWA、Dockerfile Node 化**都还没实现**；CI `web-verify` 门禁与工具链钉版按 2026-11-11 决策**暂缓**（不是「已做」）。判断「有没有」以代码为准，不以方案章节为准。
3. **账芯不可动**：复式借贷平衡、append-only 不可变交易、导出必须过 `bean-check`、定点十进制金额。改的是前端，不是全栈。

## 命令速查

统一入口 `bash scripts/local.sh <cmd>`（Windows 无 make；Linux/CI 等价目标在 `Makefile`）。

| 目的 | 命令 |
|---|---|
| 后端全量测试（严格 bean-check） | `bash scripts/local.sh test` |
| CI 同款全量校验 | `bash scripts/local.sh check` |
| 端到端冒烟 | `bash scripts/local.sh smoke` |
| 起后端（托管 Flutter 旧版产物） | `bash scripts/local.sh dev` |
| Vue 构建 / 开发服务器 | `bash scripts/local.sh web-build` / `web-dev` |
| Vue 单测 / 类型 / 兼容扫描 | `bash scripts/local.sh web-test` / `web-typecheck` / `web-compat` |
| 重建 `/legacy/` 回滚产物 | `bash scripts/local.sh web-legacy` |
| 旧版 Flutter 构建 / 测试 | `bash scripts/local.sh web` / `test-web` |

- `web-dev` 的 `/api` 已由 `vite.config.ts` 代理到 `127.0.0.1:8080`，**前端代码一律用相对 `/api`**。
- **`web-build` 会清空 `frontend/dist`，每次构建后必须补跑 `web-legacy`**，否则 `/legacy/` 回滚目录消失。
- 本机 Git Bash 可能需 `export PATH="$PATH:/d/Environment/go/bin:/d/Environment/flutter/bin"`。
- Python 只用于 `bean-check`，**一律用项目内 `.venv`**：`.venv/Scripts/python.exe -m beancount.scripts.check <file>`。不在项目外创建 venv / 装包 / 跑脚本。

## 关键文件索引

| 关心什么 | 看哪里 |
|---|---|
| 后端路由与端点 | `internal/server/router.go` |
| CORS / CSRF / Cookie 鉴权 | `internal/server/middleware.go`、`internal/config/config.go` |
| 认证与会话 | `internal/handler/auth.go`、`internal/service/auth.go` |
| 交易与幂等 | `internal/service/transaction.go`、`internal/repository/transaction.go`、`internal/db/migrate013.go` |
| 迁移清单（下一个编号 014） | `internal/db/migrate.go` 的 `migrations` 数组 |
| 静态托管 / SPA 回退 | `internal/server/static.go` |
| 导出器 | `internal/export/{beancount,files}.go` |
| Vue 定点金额 | `frontend/src/core/Money.ts` + `Money.test.ts` |
| Vue 端点契约 | `frontend/src/api/{types,routes,errors,client}.ts` |
| 构建与兼容基线 | `frontend/vite.config.ts`、`frontend/package.json` |
| 旧版回滚 | `scripts/build-legacy.sh`、`scripts/check-legacy.sh`、`frontend/dist/legacy/` |

## 硬约束（违反即返工）

- **金额**：禁止浮点参与运算/存储。旧版 `package:decimal`，新版只走 `core/Money.ts`（`decimal.js`，只收 string、`ROUND_HALF_UP`）；后端写定点十进制字符串。
- **Beancount**：改导出器后必须重跑 `bean-check`；`BEAN_CHECK_STRICT=1` 找不到校验器就必须失败，不许改回静默跳过。
- **迁移**：schema 改动一律追加 `migrate.go` 迁移并记 `schema_migrations`，不手改线上库。幂等索引用 `ON CONFLICT(ledger_id, client_request_id) WHERE client_request_id IS NOT NULL`。
- **认证**：token 不落 `localStorage` / URL；Cookie 路径非 GET 必须带 `X-CSRF`；`SameSite=None` 禁用；**Bearer 兜底分支观察期结束前不得删**。
- **多账本**：业务请求必须带 `X-Ledger-Id`；新 API 客户端缺失即抛错，**绝不静默回退默认账本 id=1**。
- **posting 字段**：在线 `POST /api/transactions` 用 `account_id`（整数）；离线 `sync/push` 用 `account_uuid`（字符串）。混用必错。
- **不可变**：改账 = 新增冲正交易，删除 = 软删；交易不原地改。
- **兼容性（iOS 14.8 / Safari 14.1.2）**：`build.target: 'es2015'` + plugin-legacy；禁 WASM / WebGL / CanvasKit / OffscreenCanvas；禁 `structuredClone` / `crypto.randomUUID` / `.at()` / `findLast` / `Object.hasOwn` / `showModal` / `BroadcastChannel` / `navigator.locks`；CSS 不用 `aspect-ratio` / `:has()` / `dvh` / `inset`。
- **依赖钉版**：按方案 §14.7。**禁装** ECharts / Vuetify / Framework7 / moment / monaco / leaflet / swiper；图表用 Canvas 2D 自绘。升级依赖必须过 `web-compat` + 真机冒烟，不允许「本地能跑就升」。
- **备份**：只能用 `VACUUM INTO` + `integrity_check` + 行数比对，**禁止复制 db 文件**（WAL 会静默丢数据）。备份/下载/恢复校验 `users.is_admin`，不是账本 owner。
- **注释**：代码注释用中文。

## 不要做（未经明确指示）

- 不要把 Vue 产物输出到 `frontend/web/`（那是 Flutter 源目录，`emptyOutDir` 会清空源 `index.html` / manifest / icons）。正确落点是 `frontend/dist/`。
- 不要直接搬 `frontend/build/web` 当 `/legacy/`：`<base href="/">` 会把资源打到根路径被 Vue 接管。必须 `--base-href /legacy/` 重建后复制。
- 不要把 `Dockerfile` 改成 Node 构建、不要删 `frontend/lib/` 等 Dart 资产、不要删 `compat-gate.js` / `compat.html` / `compat-gate-selftest.js`——这些分别是批 6 与 D-5 的动作，**需老板逐个确认**。
- 不要提前引入 SSR / Nuxt / Node 运行时；不要为「顺手」加预算、AI 识别、深色模式、附件、2FA（方案已明确不做）。
- 不要跳过旧功能：CSV 导出、分类拖拽排序、导出不平告警、注册开关、默认账户管理在新版一律 **Must**（方案 §14.5）。

## 改完怎么验

按项目规则：**先本地测试通过，才允许提交；测试前不要 commit。**

1. 后端改动 → `bash scripts/local.sh check`。
2. 前端改动 → `web-test` + `web-typecheck` + `web-build` + `web-compat` 四项；改了构建产物就补 `web-legacy`。
3. 改了导出器或账芯 → 追加 `bean-check`（严格模式）与导出 round-trip。
4. 涉及认证 / 幂等 / 同步 → 跑 `internal/server/*_test.go` 与 `internal/db/migrate013_test.go`，并核对 Bearer 兼容路径未被破坏。
5. 真机门禁：涉及前端构建或兼容基线的改动，cutover 前必须在 iOS 14.8 Safari 跑全主流程冒烟。

> 文档收口后工作区应干净；**改动前仍先看 `git status`**，不要把 docs 与代码改动混在同一次提交。CI `web-verify` 门禁与工具链钉版按 2026-11-11 决策暂缓，别把「方案里写了」当成「CI 已经在跑」。
