# iOS 14.8 白屏：真机诊断结论、已落地兜底与后续计划

> 2026-11-09 · v1.1 · 状态：**真机数据已闭环，P1 兼容兜底已落地并验证；未提交**
>
> v1.0 是基于官方支持矩阵的推断，v1.1 用真机诊断数据把失败点收敛到了一个具体 API，并把方案 B 从"供评审的片段"变成可运行代码。

## 一、结论先行

**不是记账代码的 bug，是一个缺失的浏览器 API。** 真机（iOS 14.8 / WebKit 14.1.2）实测 **不支持 `WebAssembly.compileStreaming`**；而本项目 `flutter_bootstrap.js` 加载 CanvasKit 时正是用它编译 `canvaskit.wasm`，**且没有回退到 `WebAssembly.compile`**。于是 CanvasKit 初始化失败、`initializeEngine` 拒绝、`runApp()` 不执行，页面什么都不渲染 —— 纯白，连 Flutter 的红屏都没有。

**最短修复仍然是升级系统，但现在有硬证据支撑**：Apple 官方 Safari 15 Release Notes 明确写了「WebAssembly 新增了流式编译支持」「增加了对 WebGL 2 的支持」，MDN 兼容表为 Safari / Safari on iOS **15+**。也就是说一次升级会同时补上流式编译和 WebGL 2 两道坎。所有能跑 iOS 14.8 的 iPhone（6s / 6s Plus / 7 / 7 Plus / SE 一代）都在 iOS 15 支持列表内。

**已同步落地 P1**：新增 ES5 零依赖的兼容门槛脚本，iOS 14.8 打开首页会看到「当前浏览器版本过低 + 升级指引」而不是白屏；达标设备不会看到任何额外内容。

## 二、真机实测数据（2026-11-09）

诊断页 `compat.html` 在出问题的手机上复制回来的完整结果，关键项如下：

| 项 | 实测值 | 对白屏的影响 |
|---|---|---|
| User-Agent | `iPhone OS 14_8` / `Version/14.1.2 Safari/604.1` | iOS 14.8、WebKit 14.1.2，低于官方 Safari 15.6 门槛 |
| 安全上下文 HTTPS | **否** | **不是白屏原因**；但 SW / Cache API 被浏览器禁用，离线与 PWA 失效 |
| Service Worker / Cache API | 不支持 | 同上，HTTP 非安全上下文下的正常表现（iOS 14 的 Safari 本身是支持 SW 的） |
| WebAssembly | 支持 | 基础能力没问题 |
| **`WebAssembly.compileStreaming`** | **不支持** | **直接根因**，见第三节 |
| WASM GC（skwasm 必需） | 不支持 | 不影响：本构建是 `dart2js + canvaskit`，不走 skwasm |
| WebGL | WebGL 1.0（仅基础） | 第二道坎，CanvasKit 在 Safari 15 的 WebGL 2 上才稳 |
| 动态 `import()` / `??=` / `?.` | 全部支持 | **排除语法兼容问题** |
| `Intl.Segmenter` / `ResizeObserver` | 支持 | 无影响 |
| `OffscreenCanvas` / `ImageDecoder` / `structuredClone` / `SharedArrayBuffer` | 不支持 | 只影响 CanvasKit 变体选择，不阻断 |
| 资源 `main.dart.js` / `flutter_bootstrap.js` / `canvaskit.wasm` | 404 | **假信号**，见第四节 |
| 资源 `index.html` | 200 | 同上 |
| 页面错误 | 无 | **正常**：诊断页不加载 Flutter 引擎，本来就不会有主应用报错 |

## 三、精确根因链（已定位到代码行）

`frontend/build/web/flutter_bootstrap.js` 里的 CanvasKit 加载函数：

```js
var k = i => {
  let e = WebAssembly.compileStreaming(fetch(i));   // ← 无 try / 无回退
  return (r, t) => ((async () => {
    let n = await e, s = await WebAssembly.instantiate(n, r);
    t(s, n);
  })(), {});
};
var E = (i, e, r, t) => (window.flutterCanvasKitLoaded = (async () => {
  ...
  let d = k(c(a, "canvaskit.wasm"));                // ← 此处抛 TypeError（在 async IIFE 内转为 rejected Promise）
  let u = await import(o);
  ...
})(), window.flutterCanvasKitLoaded);
```

执行链：

```
index.html → flutter_bootstrap.js (async)
  → _flutter.loader.load()
  → builds 只有 [dart2js + canvaskit]
  → E() 调用 k(canvaskit.wasm)
  → Safari 14 无 compileStreaming ⇒ TypeError
  → window.flutterCanvasKitLoaded = rejected Promise
  → 按 bootstrap 代码 main.dart.js 仍会被注入，但 initializeEngine 等待 CanvasKit 失败
  → runApp() 不执行 ⇒ 页面无 flt-glass-pane、无 canvas ⇒ 纯白屏
```

补充两点：

- 失败发生在 **Promise 链内部**，页面没有 `window.onerror` 兜底，所以用户看不到任何文字，只在 console 里可能留一条 unhandled rejection。
- Service Worker 注册失败**不会导致白屏**：bootstrap 里 `loadServiceWorker(...).catch(console.warn)` 把错误吞掉了，只是打印警告。所以 HTTP 访问不会挡住页面，只让离线能力失效。

### 分级判断（v1.1 更新）

| 级别 | 判断 |
|---|---|
| **确定（真机 + 产物代码双证）** | 缺 `WebAssembly.compileStreaming`；bootstrap 无回退；CanvasKit 初始化失败；官方要求 Safari 15.6+；iOS 换浏览器无效 |
| **确定（已落地缓解）** | 兼容门槛脚本会把白屏变成可读提示页 |
| **确定（非白屏原因）** | HTTP 非安全上下文、SW/Cache API 不可用、动态 `import()` 与 `??=` 语法、产物 hash 错配（实测 md5 一致） |
| **待真机验证** | 升级到 iOS 15.x 后是否正常渲染；门槛提示页在 iOS 14.8 上的实际显示效果 |
| **条件分支** | 若白屏地址连 `/flutter_bootstrap.js` 都返回 404，则第一根因是部署缺产物，与本结论无关（见第四节） |

## 四、三个 404 的解读：诊断页打开的位置不是完整部署

老板补充「三个 404 可能是假的，我没起 flutter」，与本地核对结果一致：

- `frontend/web/`（源码目录）里**只有** `index.html`、`compat.html`、`compat-gate.js`、`manifest.json`、图标 —— 恰好对应 `index.html 200`、三个 Flutter 资源 404 的结果。
- `frontend/build/web/`（产物目录）里 `main.dart.js` 3.4 MB、`flutter_bootstrap.js` 9.7 KB、`canvaskit.wasm` 7.1 MB 都在，Dockerfile 与 README 也都把 `LEDGER_WEB_ROOT` 指向产物目录。

**所以这些 404 不构成"部署坏了"的证据**，只是诊断页当时被 serve 在源码目录/预览服务上。要把它变成确定结论，只需在**白屏的那个地址**上做一次判据测试：

| 判据 | 结论 |
|---|---|
| `<地址>/flutter_bootstrap.js` 返回 JS 内容（200） | 部署完整，白屏走本报告的浏览器基线结论 |
| 返回 404 或被 SPA 回退成 HTML | **第一根因是部署错目录或缺产物**，先把 `frontend/build/web` 整个目录部署到 `LEDGER_WEB_ROOT` |

本地用真实 Go 后端（`LEDGER_WEB_ROOT=frontend/build/web`）实测的资源表，可作为对照：

| 路径 | 状态 | Cache-Control | Content-Type |
|---|---|---|---|
| `/` | 200 | `no-cache` | `text/html` |
| `/compat-gate.js` | 200 | `no-cache` | `text/javascript` |
| `/compat.html` | 200 | `no-cache` | `text/html` |
| `/flutter_bootstrap.js` | 200 | `no-cache` | `text/javascript` |
| `/main.dart.js` | 200 | `no-cache` | `text/javascript` |
| `/canvaskit/canvaskit.wasm` | 200 | `max-age=31536000, immutable` | **`application/wasm`** |
| `/missing-file.js` | 404 | — | 带扩展名不回退，404 判定可信 |
| `/some-client-route` | 200 | `no-cache` | SPA 回退 index.html |

`application/wasm` 这个 MIME 也顺带排除了一个隐患：`compileStreaming` 要求响应 MIME 必须是 `application/wasm`，否则会 reject。升级到 iOS 15 后不会因为服务端 MIME 再挂一次。

## 五、HTTPS 不是白屏原因，但会让「离线优先」失效

这是真机数据里第二个值得单独拎出来的结论，**和升级系统是两件事**：

| 现象 | 原因 | 后果 |
|---|---|---|
| `isSecureContext = false` | 当前是局域网 `http://` | 浏览器按规范隐藏 `navigator.serviceWorker`、`caches` |
| SW / Cache API 不支持 | 上一条，不是 iOS 14 的能力缺陷 | `flutter_bootstrap.js` 注册 SW 失败并 `console.warn` 吞掉 |
| 页面仍可显示（在达标浏览器上） | SW 失败不阻断启动 | **不白屏** |
| 离线记账 / PWA 安装 | 依赖 SW | **完全不可用**，项目的核心卖点在 HTTP 下是关着的 |

所以 P2 的 HTTPS 不是"顺手优化"，是解锁离线能力的前置条件。iOS 14 的 Safari 自 11.3 起就支持 Service Worker，升级系统解决不了这一条。

## 六、方案对比（状态已更新）

| 方案 | 做法 | 成本 | 效果 | 结论 / 状态 |
|---|---|---|---|---|
| **A 升级系统** | iOS 14.8 → iOS 15.x | 0 | 补上 `compileStreaming` + WebGL 2，达到 Safari 15.6+ | **首选**，待老板在真机上执行 |
| **B 兼容兜底页** | `index.html` 前置 ES5 门槛脚本，不达标显示可读提示；达标设备加启动失败兜底 | 低 | 白屏变提示页；老设备仍不能用 | **✅ 已落地并验证**（P1） |
| **C 降级 Flutter 换 HTML renderer** | 退到 3.27 及更早重建 Web 端 | 高 | 可能兼容，但丢功能、`pubspec.lock` 与 Docker 内 3.35.1 冲突、与移动端共用代码断裂 | **不推荐**，`--web-renderer` 在 3.29 移除、3.32 删代码，本地 3.38.9 无开关 |
| **D 轻量 H5 只读页** | 给老设备单独做非 Flutter 简化页 | 高 | 真兼容 | 二期，家庭自用场景不划算 |
| **E HTTPS + SW 版本化** | 反代加证书、旧缓存升级验证 | 中 | 解锁离线与 PWA；不解决浏览器基线 | **提为 P2 必做**，与白屏解耦 |

## 七、分阶段计划

| 阶段 | 内容 | 状态 | 验收（GWT） |
|---|---|---|---|
| **P0 诊断** | 真机取数据；确认失败点在 `compileStreaming`；澄清 404 性质 | ✅ 完成 | Given 真机诊断结果，When 读到 `compileStreaming 不支持`，Then 能定位到 bootstrap 的具体一行 |
| **P1 兼容兜底** | 新增 `compat-gate.js`，`index.html` 前置引入；诊断页判定文案精确化 | ✅ 完成 | Given iOS 14.8 Safari，When 打开首页，Then 3 秒内看到「当前浏览器版本过低」提示页而非白屏；Given Safari 15.6+，When 打开，Then 正常渲染且无兜底提示 |
| **P1 剩余** | 把「最低 Safari 15.6」写进 README / AGENTS.md | ⏳ 待 Q1 拍板 | 文档与门槛提示文案一致 |
| **P2 部署** | NAS 上 HTTPS；确认 `LEDGER_WEB_ROOT` 指向 `build/web`；旧 SW 缓存升级验证 | ⏳ 待办 | Given 已缓存旧 SW，When 部署新版本首次访问，Then 自动更新且不白屏；Given HTTPS 访问，Then `isSecureContext = true` 且 SW 注册成功 |
| **P3 二期（按需）** | 若确认必须支持 iOS 14，再评估方案 D；否则正式宣布最低支持版本 | ⏳ 决策留档 | 不因一台设备改动主架构 |

## 八、P1 已落地实现说明

### 文件与职责

| 文件 | 作用 |
|---|---|
| `frontend/web/compat-gate.js` | **新增**。纯 ES5、零依赖、同步加载于 `<head>`，不修改也不拦截 `flutter_bootstrap.js` |
| `frontend/web/index.html` | 在 `<title>` 之后、Flutter bootstrap 之前加一行 `<script src="compat-gate.js"></script>` |
| `frontend/web/compat.html` | 判定文案精确化；补 404 解读；安全上下文改标 warn 并注明「不影响页面显示」；复制结果附注意事项 |
| `tmp/compat_gate_selftest.js` | 一次性 Node + DOM stub 分支自测（非交付物，不进 CI） |

### 判定与兜底逻辑

| 分支 | 条件 | 行为 |
|---|---|---|
| **能力不达标** | 缺 `WebAssembly` / `compileStreaming` / `fetch` / WebGL 任一 | 立即显示「当前浏览器版本过低」页：缺失能力明细、iOS 15 升级指引、换浏览器无效的说明、查看完整诊断链接、忽略提示按钮；**不再安装错误监听**，避免提示被覆盖 |
| **能力达标** | 四项齐备 | 不显示任何内容；改为监听 `error` / `unhandledrejection`，并在 20 秒后若仍未检测到画面则显示「应用没能启动」页（含资源自检清单、重新加载按钮） |
| **自动纠正** | 提示已显示但引擎随后画出 `canvas` / `flt-glass-pane` | 轮询每 500 ms 检测一次，检测到即移除提示，最多 40 秒；避免误判挡住正常用户 |

设计取舍：**用能力检测而不是 UA 版本号做门槛**。UA 判断会把 Safari 15.0–15.5 这类"版本够但官方未支持"的情况误放行，也会在未来新版 WebKit 上误伤；能力项直接对应 bootstrap 的必经代码路径。达标设备不预挂载任何提示，是为了不引入首屏闪烁和误报。

### 缓存与部署

- `compat-gate.js` 走 `internal/server/static.go` 的 default 分支 → `Cache-Control: no-cache`，改了即时生效。
- `flutter build web` 会把它和 `compat.html` 一起复制进 `build/web`，并写进 `flutter_service_worker.js` 的 RESOURCES 清单（按 md5）；实测两次构建的 md5 记录与磁盘文件完全一致。
- **部署必须整目录覆盖 `frontend/build/web`**，只拷 `compat.html` 会让门槛脚本 404，兜底页等于没上。

## 九、验证结果（全部本地实测）

| 验证项 | 命令 / 方法 | 结果 |
|---|---|---|
| ES5 语法 | `node --check frontend/web/compat-gate.js`；关键词扫描无 `let/const/class/=>/模板串/可选链` | ✅ 通过 |
| 门槛控制流 | `node tmp/compat_gate_selftest.js` | ✅ 5/5 场景通过：不达标立即提示、引擎异常兜底、画面出现后自动移除、已启动不误报、超时兜底、忽略后不再弹出 |
| 诊断页语法 | Node 提取内联 `<script>` 后 `new Function` 解析 | ✅ 通过 |
| 前端打包 | `flutter build web --release`（两次） | ✅ 退出码 0，`√ Built build/web` |
| 产物完整性 | 六类资源存在性 + 源码/产物 md5 比对 + SW 清单 md5 比对 | ✅ 全部一致 |
| 加载顺序 | 产物 `index.html` 第 43 行 gate（`head`，同步）、第 47 行 bootstrap（`body`，async） | ✅ 正确 |
| 静态托管 | 真实 Go 后端 `:18080`，8 个路径 HEAD 探测 | ✅ 见第四节表格 |
| 全量校验 | `bash scripts/local.sh check` | ✅ 全过，版本 0.5.0 三处一致，bean-check 自检通过 |
| 端到端冒烟 | `bash scripts/smoke.sh` | ✅ 10 步全过（导出 bean-check、备份、路径穿越防护） |
| 前端测试 | `bash scripts/local.sh test-web` | ✅ 33 tests 全过 |

**未覆盖**：真机上提示页的实际渲染效果、升级 iOS 15 后的完整启动。这两条只能在老板的手机上验，本地没有 iOS 14 WebKit 可模拟。

## 十、待老板确认 / 拍板

| # | 问题 | 暂定建议 |
|---|---|---|
| Q1 | 是否接受「最低 Safari 15.6」并放弃 iOS 14 | 接受；接受后我把这条写进 README / AGENTS.md |
| Q2 | **这台手机从来没成功打开过，还是之前能开、某次更新后白屏？** | 若"曾经能开"，需另查 SW 缓存与部署变更；当前证据指向"从来没开过" |
| Q3 | **白屏的那个地址，和诊断页地址是同一个吗？** `<地址>/flutter_bootstrap.js` 是 200 还是 404？ | 用第四节判据表一测即可定性 |
| Q4 | 升级到 iOS 15.x 后重开，是否正常渲染、门槛提示是否消失 | 升级后回我一句结果，我收口 |
| Q5 | NAS 是否上 HTTPS | 建议 P2 必做，与白屏解耦，目的是解锁离线/PWA |

## 十一、本次改动清单

| 文件 | 说明 |
|---|---|
| `frontend/web/compat-gate.js` | **新增**，ES5 兼容门槛与启动失败兜底 |
| `frontend/web/index.html` | **修改**，前置引入门槛脚本（+8 行注释与 script 标签） |
| `frontend/web/compat.html` | **修改**，判定精确化、404 解读、HTTPS 语义修正、复制文本注释 |
| `docs/ios14-white-screen-plan.md` | **修改**，v1.0 → v1.1 本文档 |
| `frontend/build/web/*` | 构建产物自动刷新（该目录被 `.gitignore` 忽略，不在提交范围） |
| `tmp/compat_gate_selftest.js` | 一次性自测脚本，非交付物 |

后端、数据库、账本核心、同步协议、Beancount 导出**均未改动**；**未提交**。
