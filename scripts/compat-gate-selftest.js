/*
 * compat-gate.js 控制流自测（无第三方依赖）
 * 运行：node scripts/compat-gate-selftest.js
 *
 * 用极简 DOM stub 覆盖兼容补丁与兜底页的所有分支：
 *   A Safari 14 画像：无 compileStreaming → 自动补上，让引擎尝试启动
 *   B ?nocompat=1 对照：关闭补丁 → 回到直接提示
 *   C WebAssembly 不可写：补丁失败 → 提示并说明原因
 *   D 原生支持：绝不覆盖原生实现
 *   E 补丁生效但引擎异常 → 加载失败页，画面出现后自动移除
 *   F 补丁生效但迟迟无画面 → 20 秒超时兜底
 *   G 忽略提示后不再打扰
 *   H 缺 WebGL 等硬能力 → 即使补丁生效仍然提示
 */
"use strict";

const assert = require("assert");
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const CODE = fs.readFileSync(
  path.join(__dirname, "..", "frontend", "web", "compat-gate.js"),
  "utf8"
);

const GATE_ID = "fb-compat-gate";

// 1. 门槛脚本必须是纯 ES5（要在 WebKit 14 上解析）
const es5Tokens = /(^|[^\w])(let|const|class)\s|=>|`|\?\.|\?\?/;
assert(!es5Tokens.test(CODE), "门槛脚本中出现了非 ES5 语法");
console.log("ES5 检查：关键词扫描通过");

function makeEnv(opts) {
  opts = opts || {};
  const byId = {};
  const timers = [];
  const intervals = [];
  const listeners = { win: {}, doc: {} };
  let canvasPresent = !!opts.canvas;
  let reloaded = false;

  function makeEl(tag) {
    const e = {
      tagName: tag,
      _html: "",
      children: [],
      parentNode: null,
      textContent: "",
      onclick: null,
      style: {},
      className: "",
      setAttribute(k, v) { if (k === "id") { e.id = v; } },
      appendChild(c) { c.parentNode = e; e.children.push(c); return c; },
      removeChild(c) {
        const i = e.children.indexOf(c);
        if (i >= 0) { e.children.splice(i, 1); }
        c.parentNode = null;
        if (c.id && byId[c.id] === c) { delete byId[c.id]; }
        return c;
      },
      getContext() { return opts.webgl === false ? null : {}; }
    };
    let _id = "";
    Object.defineProperty(e, "id", {
      get() { return _id; },
      set(v) { _id = v; if (v) { byId[v] = e; } }
    });
    Object.defineProperty(e, "innerHTML", {
      get() { return e._html; },
      set(v) {
        e._html = v;
        const re = /id="([^"]+)"/g;
        let m;
        while ((m = re.exec(v)) !== null) {
          const child = makeEl("div");
          child.id = m[1];
          child.parentNode = e;
          e.children.push(child);
        }
      }
    });
    return e;
  }

  const document = {
    head: makeEl("head"),
    body: makeEl("body"),
    createElement: makeEl,
    createTextNode: (t) => ({ text: t, nodeType: 3, parentNode: null }),
    getElementById: (id) => byId[id] || null,
    querySelector: () => (canvasPresent ? makeEl("canvas") : null),
    addEventListener: (t, fn) => {
      (listeners.doc[t] = listeners.doc[t] || []).push(fn);
    }
  };

  const window = {
    fetch: opts.fetch === false ? undefined : () => {},
    isSecureContext: opts.secure === true,
    location: {
      href: "http://192.168.1.9:8080/",
      search: opts.query || "",
      reload() { reloaded = true; }
    },
    addEventListener: (t, fn) => {
      (listeners.win[t] = listeners.win[t] || []).push(fn);
    }
  };

  const WebAssembly = {
    validate: () => true,
    compile: () => Promise.resolve({ __fakeModule: true })
  };
  if (opts.streaming !== false) {
    WebAssembly.compileStreaming = () => Promise.resolve({ __native: true });
  }
  if (opts.noCompile) { delete WebAssembly.compile; }
  if (opts.frozen) { Object.freeze(WebAssembly); }

  const sandbox = {
    window,
    document,
    WebAssembly,
    Promise,
    console,
    setTimeout: (fn, ms) => {
      const t = { fn, ms, cleared: false };
      timers.push(t);
      return t;
    },
    setInterval: (fn, ms) => {
      const t = { fn, ms, cleared: false };
      intervals.push(t);
      return t;
    },
    clearInterval: (t) => { if (t) { t.cleared = true; } }
  };

  return {
    sandbox,
    byId,
    listeners,
    setCanvas: (v) => { canvasPresent = v; },
    wasReloaded: () => reloaded,
    fireWin: (type, ev) => {
      (listeners.win[type] || []).forEach((fn) => fn(ev));
    },
    fireDoc: (type) => {
      (listeners.doc[type] || []).forEach((fn) => fn({}));
    },
    runTimeout: (ms) => {
      timers.filter((t) => t.ms === ms && !t.cleared).forEach((t) => t.fn());
    },
    tick: () => {
      intervals.filter((t) => !t.cleared).forEach((t) => t.fn());
    }
  };
}

let chain = Promise.resolve();

function run(name, opts, fn) {
  chain = chain.then(() => {
    const env = makeEnv(opts);
    vm.runInNewContext(CODE, env.sandbox);
    return Promise.resolve(fn(env)).then(() => {
      console.log("PASS  " + name);
    });
  });
}

// A：iOS 14.8 真机画像 —— 有 WebAssembly/WebGL1/fetch，唯独没有 compileStreaming
run("A Safari 14 → 自动补 compileStreaming 并尝试启动", { streaming: false }, (env) => {
  assert(!env.byId[GATE_ID], "补丁生效后不应直接显示不支持提示");
  assert(env.sandbox.window.__fambeanStreamingPolyfilled === true, "应标记补丁已启用");
  assert(typeof env.sandbox.WebAssembly.compileStreaming === "function", "应补上 compileStreaming");
  assert(env.listeners.win.unhandledrejection, "应安装 watchdog 而非直接判死");
  // 验证补丁本身的调用链：Promise<Response> → arrayBuffer → compile
  return env.sandbox.WebAssembly.compileStreaming(Promise.resolve({
    arrayBuffer: () => Promise.resolve(new ArrayBuffer(8))
  })).then((mod) => {
    assert(mod && mod.__fakeModule, "应走到 WebAssembly.compile 并返回模块");
  });
});

// B：?nocompat=1 用于真机 A/B 对照
run("B ?nocompat=1 → 关闭补丁，回到直接提示", { streaming: false, query: "?nocompat=1" }, (env) => {
  const gate = env.byId[GATE_ID];
  assert(gate && /跑不了当前版本/.test(gate.innerHTML), "应显示兼容提示页");
  assert(/compileStreaming/.test(env.byId["fb-gate-detail"].textContent), "应列出缺失能力");
  assert(/nocompat=1/.test(env.byId["fb-gate-detail"].textContent), "应说明补丁已关闭");
  assert(!env.sandbox.window.__fambeanStreamingPolyfilled, "不应注入补丁");
});

// C：WebAssembly 命名空间不可写
run("C WebAssembly 不可写 → 提示并说明补丁失败", { streaming: false, frozen: true }, (env) => {
  assert(env.byId[GATE_ID], "应显示兼容提示页");
  assert(/启用失败/.test(env.byId["fb-gate-detail"].textContent), "应说明补丁启用失败");
});

// D：原生支持时绝不覆盖
run("D 原生支持 → 不覆盖原生实现", { streaming: true }, (env) => {
  const nativeFn = env.sandbox.WebAssembly.compileStreaming;
  assert(typeof nativeFn === "function");
  env.fireWin("error", { message: "boom" });
  assert(env.byId[GATE_ID], "出错时应显示加载失败页");
  assert(/原生支持/.test(env.byId["fb-gate-detail"].textContent), "应标明补丁未启用");
  assert(env.sandbox.WebAssembly.compileStreaming === nativeFn, "原生实现不得被替换");
});

// E：补丁生效但引擎异常 → 兜底页 → 画面出现后自动移除
run("E 引擎异常 → 兜底页，画面出现后自动移除", { streaming: false }, (env) => {
  assert(!env.byId[GATE_ID]);
  env.fireWin("unhandledrejection", { reason: new Error("CanvasKit init failed") });
  assert(env.byId[GATE_ID] && /应用没能启动/.test(env.byId[GATE_ID].innerHTML));
  assert(/已启用/.test(env.byId["fb-gate-detail"].textContent), "应标明补丁已启用");
  env.setCanvas(true);
  env.tick();
  assert(!env.byId[GATE_ID], "画面出现后提示应自动消失");
});

// F：补丁生效但迟迟没画面
run("F 补丁生效但无画面 → 20 秒超时兜底", { streaming: false }, (env) => {
  env.runTimeout(20000);
  assert(env.byId[GATE_ID], "应出现超时提示");
  assert(/20 秒/.test(env.byId["fb-gate-detail"].textContent));
  env.byId["fb-gate-reload"].onclick();
  assert(env.wasReloaded(), "重新加载按钮应生效");
});

// G：用户忽略后不再打扰
run("G 忽略提示 → 移除且不再弹出", { streaming: false }, (env) => {
  env.fireWin("error", { message: "main.dart.js 404" });
  assert(env.byId[GATE_ID]);
  env.byId["fb-gate-dismiss"].onclick();
  assert(!env.byId[GATE_ID]);
  env.fireWin("error", { message: "again" });
  env.runTimeout(20000);
  assert(!env.byId[GATE_ID]);
});

// H：WebGL 等硬能力缺失
run("H 缺 WebGL → 补丁生效也仍提示", { streaming: false, webgl: false }, (env) => {
  assert(env.byId[GATE_ID], "硬能力缺失应显示提示页");
  const detail = env.byId["fb-gate-detail"].textContent;
  assert(/WebGL/.test(detail), "应列出 WebGL 缺失");
  assert(/已启用/.test(detail), "应同时说明补丁已启用");
});

chain.then(() => {
  console.log("\n全部 8 个场景通过");
}).catch((err) => {
  console.error("\nFAIL  " + (err && err.stack ? err.stack : err));
  process.exit(1);
});
