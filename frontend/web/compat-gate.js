/*
 * FamBean Web 兼容门槛
 *
 * 目的：在 Flutter 引擎启动之前，把「不支持的浏览器白屏」变成一页可读的提示。
 *
 * 依据（2026-11-09 真机实测 + 产物代码核对）：
 *   - iOS 14.8 / WebKit 14.1.2 没有 WebAssembly.compileStreaming；
 *   - 本项目 flutter_bootstrap.js 里 CanvasKit 的加载是
 *       WebAssembly.compileStreaming(fetch("canvaskit/canvaskit.wasm"))
 *     且没有回退到 WebAssembly.compile；
 *   - 于是 window.flutterCanvasKitLoaded 变成 rejected Promise，
 *     initializeEngine 失败、runApp() 不执行，页面什么都不渲染 —— 纯白。
 *   - Apple Safari 15 Release Notes：「WebAssembly 新增了流式编译支持」
 *     「增加了对 WebGL 2 的支持」；MDN 兼容表 Safari on iOS 15+。
 *     所以升级到 iOS 15.x 能同时补上两道坎。
 *
 * v2（不升级 iOS 的方案）：
 *   Safari 14 只是缺 compileStreaming，但 WebAssembly.compile / Response.arrayBuffer()
 *   都是 2017 年就有的老 API。这里在 bootstrap 之前注入一个 compileStreaming polyfill：
 *       fetch(url) → Response.arrayBuffer() → WebAssembly.compile(bytes)
 *   不改 Flutter 生成的任何文件、不动构建流程、不影响 SW 资源清单。
 *   能力补齐后让引擎尝试启动；真跑不起来再由 watchdog 显示兜底页。
 *   用 ?nocompat=1 可关闭补丁做 A/B 对照。
 *
 * 约束：
 *   - 纯 ES5、零依赖、同步加载于 <head>，老 WebKit 必须能解析；
 *   - 不修改、不拦截 Flutter 生成的 flutter_bootstrap.js；
 *   - 只在原生缺 compileStreaming 时打补丁，绝不覆盖原生实现；
 *   - 能力达标时不打扰（只在明确报错或超时后才提示）；
 *   - 引擎一旦画出画面，提示自动消失，避免误判挡住正常用户。
 */
(function () {
  "use strict";

  var GATE_ID = "fb-compat-gate";
  var STYLE_ID = "fb-compat-gate-style";
  var LOAD_TIMEOUT_MS = 20000;
  var POLL_MS = 500;
  var POLL_MAX = 80;

  var pending = null;
  var mounted = false;
  var dismissed = false;
  var pollTimer = null;
  var patchNote = "";

  var CSS = [
    "#" + GATE_ID + "{--fb-bg:#f4f7f8;--fb-card:#fff;--fb-text:#1b252c;--fb-sub:#5c6b73;--fb-line:#e2e9ec;--fb-accent:#00897b;--fb-accent-text:#fff;position:fixed;top:0;left:0;right:0;bottom:0;z-index:2147483647;box-sizing:border-box;overflow:auto;-webkit-overflow-scrolling:touch;background:var(--fb-bg);color:var(--fb-text);font-family:-apple-system,BlinkMacSystemFont,\"PingFang SC\",\"Microsoft YaHei\",sans-serif;padding:32px 16px}",
    "@media (prefers-color-scheme:dark){#" + GATE_ID + "{--fb-bg:#11181c;--fb-card:#1b252b;--fb-text:#e9f0f2;--fb-sub:#a3b2ba;--fb-line:#2c3a41;--fb-accent:#4db6ac;--fb-accent-text:#10201e}}",
    "#" + GATE_ID + " *{box-sizing:border-box}",
    "#" + GATE_ID + " .fb-card{max-width:520px;margin:0 auto;background:var(--fb-card);border:1px solid var(--fb-line);border-radius:14px;padding:28px 22px;box-shadow:0 8px 28px rgba(0,0,0,.08)}",
    "#" + GATE_ID + " .fb-badge{display:inline-block;font-size:12px;letter-spacing:.06em;color:var(--fb-accent);border:1px solid var(--fb-accent);border-radius:999px;padding:3px 10px;margin-bottom:16px}",
    "#" + GATE_ID + " h1{font-size:21px;line-height:1.4;margin:0 0 12px;font-weight:600}",
    "#" + GATE_ID + " p{font-size:15px;line-height:1.7;margin:0 0 12px;color:var(--fb-sub)}",
    "#" + GATE_ID + " ol{font-size:15px;line-height:1.8;margin:0 0 16px;padding-left:22px;color:var(--fb-text)}",
    "#" + GATE_ID + " li{margin-bottom:6px}",
    "#" + GATE_ID + " .fb-detail{font-size:13px;line-height:1.6;background:var(--fb-bg);border:1px solid var(--fb-line);border-radius:8px;padding:10px 12px;margin:0 0 16px;color:var(--fb-sub);word-break:break-word}",
    "#" + GATE_ID + " .fb-actions{margin-top:20px;display:flex;flex-wrap:wrap;align-items:center}",
    "#" + GATE_ID + " .fb-btn{display:inline-block;font-size:15px;font-weight:600;text-decoration:none;border-radius:8px;padding:10px 16px;margin:0 10px 10px 0;border:1px solid var(--fb-accent);background:var(--fb-accent);color:var(--fb-accent-text);cursor:pointer;font-family:inherit}",
    "#" + GATE_ID + " .fb-btn.fb-ghost{background:transparent;color:var(--fb-accent)}",
    "#" + GATE_ID + " .fb-foot{font-size:12px;line-height:1.6;color:var(--fb-sub);margin-top:18px;border-top:1px solid var(--fb-line);padding-top:14px}"
  ].join("\n");

  var UNSUPPORTED_HTML = [
    '<div class="fb-card">',
    '<div class="fb-badge">兼容性提示</div>',
    "<h1>这台设备暂时跑不了当前版本</h1>",
    "<p>页面已经尝试自动补齐缺失的 WebAssembly 流式编译能力，仍未能完成引擎初始化，所以显示这页提示而不是白屏。</p>",
    '<div class="fb-detail" id="fb-gate-detail"></div>',
    "<ol>",
    "<li>打开<a href=\"compat.html\">完整诊断页</a>，点「兼容补丁实测」，把结果发给管理员，判断是否值得继续适配。</li>",
    "<li>若这台设备必须能用，请让管理员启用<b>轻量记账页</b>（不依赖 WebGL / WASM）。</li>",
    "<li>或改用 Safari 15.6 以上的设备、桌面浏览器访问同一地址。</li>",
    "</ol>",
    "<p>iOS 上的 Chrome、Firefox、Edge 都使用系统自带 WebKit，换浏览器不会改变结果；升级系统最省事，但不是唯一选择。</p>",
    '<div class="fb-actions">',
    '<a class="fb-btn fb-ghost" href="compat.html">查看完整诊断</a>',
    '<button class="fb-btn fb-ghost" type="button" id="fb-gate-dismiss">忽略提示</button>',
    "</div>",
    '<div class="fb-foot" id="fb-gate-foot"></div>',
    "</div>"
  ].join("");

  var FAILED_HTML = [
    '<div class="fb-card">',
    '<div class="fb-badge">加载失败</div>',
    "<h1>应用没能启动</h1>",
    "<p>浏览器能力达标，但 Flutter 引擎初始化失败。通常是前端产物没有部署完整、资源被缓存错配，或者网络中断。</p>",
    '<div class="fb-detail" id="fb-gate-detail"></div>',
    "<ol>",
    "<li>确认这个地址能直接打开 <b>flutter_bootstrap.js</b>、<b>main.dart.js</b>、<b>canvaskit/canvaskit.wasm</b>。</li>",
    "<li>打开完整诊断页，核对资源可达性与页面报错。</li>",
    "<li>确认服务端部署的是 <b>frontend/build/web</b> 整个目录，而不是 frontend/web 源码目录。</li>",
    "</ol>",
    '<div class="fb-actions">',
    '<button class="fb-btn" type="button" id="fb-gate-reload">重新加载</button>',
    '<a class="fb-btn fb-ghost" href="compat.html">查看完整诊断</a>',
    '<button class="fb-btn fb-ghost" type="button" id="fb-gate-dismiss">忽略提示</button>',
    "</div>",
    '<div class="fb-foot" id="fb-gate-foot"></div>',
    "</div>"
  ].join("");

  // Safari 14 / WebKit 14.1 缺的是 compileStreaming；WebAssembly.compile 与
  // Response.arrayBuffer() 都是 2017 年的老 API。用老路径实现同名方法，
  // Flutter 生成的 bootstrap 一行都不用改。
  function installStreamingPolyfill() {
    if (typeof WebAssembly === "undefined") {
      return { installed: false, native: false, reason: "浏览器没有 WebAssembly" };
    }
    if (typeof WebAssembly.compileStreaming === "function") {
      return { installed: false, native: true, reason: "原生已支持" };
    }
    if (typeof WebAssembly.compile !== "function" || typeof window.fetch !== "function") {
      return { installed: false, native: false, reason: "缺少 WebAssembly.compile 或 fetch" };
    }
    var poly = function (source) {
      return Promise.resolve(source).then(function (resp) {
        if (!resp || typeof resp.arrayBuffer !== "function") {
          throw new TypeError("compileStreaming 兼容实现：需要 Response 对象");
        }
        return resp.arrayBuffer();
      }).then(function (bytes) {
        return WebAssembly.compile(bytes);
      });
    };
    try {
      WebAssembly.compileStreaming = poly;
      if (WebAssembly.compileStreaming !== poly) {
        return { installed: false, native: false, reason: "WebAssembly 对象不可写" };
      }
      window.__fambeanStreamingPolyfilled = true;
      return { installed: true, native: false, reason: "" };
    } catch (e) {
      return { installed: false, native: false, reason: String((e && e.message) || e) };
    }
  }

  function compatDisabled() {
    try {
      return /[?&]nocompat=1(?:&|$)/.test(window.location.search || "");
    } catch (e) {
      return false;
    }
  }

  function capability() {
    var wasm = typeof WebAssembly !== "undefined";
    var streaming = wasm && typeof WebAssembly.compileStreaming === "function";
    var fetchOk = typeof window.fetch === "function";
    var gl = false;
    try {
      var c = document.createElement("canvas");
      gl = !!(c.getContext("webgl2") ||
              c.getContext("webgl") ||
              c.getContext("experimental-webgl"));
    } catch (e) {
      gl = false;
    }
    return { wasm: wasm, streaming: streaming, fetch: fetchOk, gl: gl };
  }

  function missingList(c) {
    var m = [];
    if (!c.wasm) { m.push("WebAssembly"); }
    if (c.wasm && !c.streaming) {
      m.push("WebAssembly.compileStreaming（Safari 15 起原生支持，兼容补丁未生效）");
    }
    if (!c.fetch) { m.push("fetch"); }
    if (!c.gl) { m.push("WebGL"); }
    return m;
  }

  function injectStyle() {
    if (document.getElementById(STYLE_ID)) { return; }
    var s = document.createElement("style");
    s.id = STYLE_ID;
    s.appendChild(document.createTextNode(CSS));
    document.head.appendChild(s);
  }

  function engineStarted() {
    try {
      return !!document.querySelector("flt-glass-pane, flutter-view, canvas");
    } catch (e) {
      return false;
    }
  }

  function removeGate() {
    var g = document.getElementById(GATE_ID);
    if (g && g.parentNode) { g.parentNode.removeChild(g); }
  }

  function remove() {
    dismissed = true;
    removeGate();
    mounted = false;
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
  }

  function startPolling() {
    if (pollTimer) { return; }
    var n = 0;
    pollTimer = setInterval(function () {
      n += 1;
      if (engineStarted()) {
        // 引擎真的画出画面了：说明能力判断是误报，或用户忽略提示后启动成功。
        removeGate();
        mounted = false;
        clearInterval(pollTimer);
        pollTimer = null;
        return;
      }
      if (n >= POLL_MAX) { clearInterval(pollTimer); pollTimer = null; }
    }, POLL_MS);
  }

  function render(kind, detail) {
    injectStyle();
    var gate = document.createElement("div");
    gate.id = GATE_ID;
    gate.setAttribute("role", "alert");
    gate.innerHTML = kind === "unsupported" ? UNSUPPORTED_HTML : FAILED_HTML;
    document.body.appendChild(gate);

    var d = document.getElementById("fb-gate-detail");
    if (d) { d.textContent = detail || ""; }

    var foot = document.getElementById("fb-gate-foot");
    if (foot) {
      var notes = [];
      if (window.isSecureContext === false) {
        notes.push("当前通过 HTTP 访问，离线与 PWA 能力不可用（不影响页面显示）。");
      }
      notes.push("页面地址：" + window.location.href);
      foot.textContent = notes.join("  ");
    }

    var dismiss = document.getElementById("fb-gate-dismiss");
    if (dismiss) { dismiss.onclick = remove; }
    var reload = document.getElementById("fb-gate-reload");
    if (reload) {
      reload.onclick = function () { window.location.reload(); };
    }

    mounted = true;
    startPolling();
  }

  function flush() {
    if (!pending || mounted || dismissed || !document.body) { return; }
    var p = pending;
    pending = null;
    render(p.kind, p.detail);
  }

  function show(kind, detail) {
    if (dismissed || mounted) { return; }
    pending = {
      kind: kind,
      detail: detail + (patchNote ? "\n兼容补丁：" + patchNote : "")
    };
    flush();
  }

  function installWatchdog() {
    window.addEventListener("error", function (ev) {
      var msg = "资源或脚本加载失败";
      if (ev && ev.target && ev.target.src) { msg += "：" + ev.target.src; }
      else if (ev && ev.message) { msg += "：" + ev.message; }
      show("failed", msg);
    }, true);

    window.addEventListener("unhandledrejection", function (ev) {
      var r = ev && ev.reason;
      var msg = (r && r.message) ? r.message : String(r || "未知 Promise 错误");
      show("failed", "引擎初始化异常：" + msg);
    });

    setTimeout(function () {
      if (!engineStarted()) {
        show("failed", "等待 " + (LOAD_TIMEOUT_MS / 1000) + " 秒后仍未检测到应用画面。");
      }
    }, LOAD_TIMEOUT_MS);
  }

  function init() {
    var patch;
    if (compatDisabled()) {
      patchNote = "关闭（?nocompat=1，用于对照测试）";
    } else {
      patch = installStreamingPolyfill();
      if (patch.native) {
        patchNote = "未启用（浏览器原生支持）";
      } else if (patch.installed) {
        patchNote = "已启用：fetch → arrayBuffer → WebAssembly.compile";
      } else {
        patchNote = "启用失败：" + patch.reason;
      }
    }

    // 注意：capability() 必须在补丁之后调用，streaming 才会被补上。
    var miss = missingList(capability());
    if (miss.length > 0) {
      // 补丁也没救回来：给出可读提示，不再监听引擎错误（避免被覆盖）。
      show("unsupported", "缺失能力：" + miss.join("、"));
    } else {
      // 能力齐备（可能是补丁补齐的）：让引擎尝试启动，真失败再由兜底页接住。
      installWatchdog();
    }

    document.addEventListener("DOMContentLoaded", flush);
    window.addEventListener("load", flush);
    setTimeout(flush, 0);
  }

  init();
})();
