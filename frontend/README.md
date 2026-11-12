# frontend — 家庭记账 Web 前端（Vue 3 SPA，cutover 后唯一前端）

类 Beancount 的 NAS 个人云记账应用的 Web 前端（PWA）。与 Go 后端通过 REST 通信，
支持离线记账 + 增量同步（`src/core/sync/` 离线内核），对齐 `../docs/prototype/index.html` 设计稿。

## 技术栈

- Vue 3.5 + Vue Router 4.5 + Pinia 2.3（全量重写，M7；原 Flutter 版已于批 6 cutover 清退）
- Vite 5（`build.target: 'es2015'` + `@vitejs/plugin-legacy`，Safari 14.1 / iOS 14.8 兼容基线）
- TypeScript ~5.6 + vue-tsc；Vitest 2.1（289+ 用例）
- 金额运算统一走 `decimal.js` 封装（`src/core/Money`），禁止裸浮点加减

## 命令

```bash
npm ci            # 安装依赖（lock 已含 Linux 平台 optional 二进制）
npm run dev       # 开发服务器（/api 代理到 127.0.0.1:8080）
npm run build     # vue-tsc + vite build → dist/
npm run test      # Vitest 全量
npm run typecheck # vue-tsc --noEmit
npm run compat    # es2020 语法门禁 + §14.7 API/CSS 特性扫描（扫描 dist/）
```

或经仓库根 `bash scripts/local.sh web-dev|web-build|web-test|web-typecheck|web-compat`。

## 目录结构

```
frontend/
├── index.html            # 入口（Vite root = frontend/）
├── public/               # SW（sw.js）、manifest、图标：原样拷入 dist/
├── src/
│   ├── api/              # client.ts（X-Ledger-Id 注入/断言、401/428、FormData）、routes、types
│   ├── core/             # Money / batch3 / m2 纯逻辑 + sync/（离线内核：models/localStore/idbStore/engine）
│   ├── stores/           # Pinia：session / privacy / recordSheet / sync
│   ├── views/            # 20 页（登录、流水、账户、报表、恢复/导入向导、同步队列管理等）
│   ├── components/       # AppLayout / RecordSheet / SyncBadge 等
│   └── router/           # 懒加载路由 + adminOnly 守卫
└── dist/                 # 构建产物（不入库）；dist/legacy/ 为冻结的旧版回滚基准（不再重建）
```

## 兼容基线（硬约束，CI 门禁）

- 语法门禁 es2020（放行路由懒加载动态 `import()`）；API/CSS 特性扫描见 `scripts/compat.mjs`
- 禁用 `.at()` / `structuredClone` / `crypto.randomUUID` / `BroadcastChannel` / `navigator.locks` 等
  Safari 14.1 缺失 API；禁装 ECharts / Vuetify / moment 等（§14.7 清单）
- Service Worker 只缓存应用壳与静态资源，`/api/*` 与 `/legacy/*` 绝不拦截

## 版本号

`package.json` 的 `version` 与根 `CHANGELOG.md` 顶部版本、git tag 三处一致
（`bash scripts/verify-version.sh` 校验），并经 `__APP_VERSION__` 注入前端展示。
