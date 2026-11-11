/**
 * 兼容门禁（P1 修复）：用当前源码产物做语法扫描 + API/CSS 特性扫描。
 *
 * v2.6（2026-11-11）：语法扫描目标由 es2015 抬到 **es2020**，只为放行
 * Vite 路由懒加载生成的动态 `import()`（ES2020 语法，Safari 14.1 真机支持）。
 * **产物基线不变**：`build.target: 'es2015'` 与 plugin-legacy 的
 * Safari >= 14 polyfills 一个字未动。语法门禁放宽后，用 §14.7 承诺过的
 * API / CSS 特性扫描补位，避免「语法门禁变松、API 层裸奔」。
 *
 * 背景：旧 `npm run compat` 直接扫 `dist/assets/*.js`，那是批 0 陈旧产物，
 * 对批 1 代码零证明力；而 `vite build` 的 `emptyOutDir:true` 会清空
 * `dist/legacy/`（Flutter 回滚兜底），因此绝不能对 dist 原地重建。
 *
 * 本脚本：
 *  1) 只写 `frontend/.compat-tmp/`，绝不写 `frontend/dist/` 与 `dist/legacy/`；
 *  2) 校验临时产物确实来自当前源码（批 1 特征串命中，否则门禁失败）；
 *  3) 前后比对 `dist/legacy/` 文件清单，断言未被改动；
 *  4) 无论成败都清理临时目录；
 *  5) 只调用本地 vite / es-check 的 bin，不新增任何依赖。
 */
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, readdirSync, readFileSync, rmSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const tmpDir = join(frontendRoot, '.compat-tmp')
const tmpAssetsDir = join(tmpDir, 'assets')
const distAssetsDir = join(frontendRoot, 'dist', 'assets')
const legacyDir = join(frontendRoot, 'dist', 'legacy')
const viteBin = join(frontendRoot, 'node_modules', 'vite', 'bin', 'vite.js')
const esCheckBin = join(frontendRoot, 'node_modules', 'es-check', 'index.js')

/** 批 1 引入、批 0 陈旧产物不可能包含的特征串。 */
const BATCH1_MARKERS = [
  'ledger_id_required',
  'X-CSRF',
  'Idempotency-Key',
  'fambean.record_draft',
  'idempotency_key_deleted',
]

/**
 * §14.7 禁用清单：Safari 14.1 缺失或 15.4+ 才有的 API / 语法特性。
 * 语法门禁抬到 es2020 后由这层兜底，命中即失败（AC-19）。
 */
const FORBIDDEN_JS = [
  { name: 'structuredClone', re: /\bstructuredClone\s*\(/ },
  { name: 'crypto.randomUUID', re: /\bcrypto\.randomUUID\s*\(/ },
  { name: '.at()', re: /\.at\s*\(/ },
  { name: 'findLast / findLastIndex', re: /\.findLast(?:Index)?\s*\(/ },
  { name: 'Object.hasOwn', re: /\bObject\.hasOwn\s*\(/ },
  { name: 'showModal', re: /\.showModal\s*\(/ },
  { name: 'BroadcastChannel', re: /\bBroadcastChannel\b/ },
  { name: 'navigator.locks', re: /\bnavigator\.locks\b/ },
  { name: 'OffscreenCanvas', re: /\bOffscreenCanvas\b/ },
  {
    name: 'WASM / WebGL / CanvasKit',
    re: /\bWebAssembly\b|\bCanvasKit\b|WebGLRenderingContext|\bWebGL2?\b/,
  },
]

const FORBIDDEN_CSS = [
  { name: 'aspect-ratio', re: /aspect-ratio\s*:/ },
  { name: ':has()', re: /:has\s*\(/ },
  { name: 'dvh', re: /\bdvh\b/ },
  { name: 'inset', re: /(^|[^\w-])inset\s*:/ },
]

/** 返回命中的禁用特性名（去重）。 */
function scanForbidden(content, rules) {
  const hits = []
  for (const rule of rules) {
    if (rule.re.test(content)) {
      hits.push(rule.name)
    }
  }
  return hits
}

/** 自检：扫描器必须能命中样本，防止规则写错变成「永远通过」的假绿灯。 */
function selfCheckScanner() {
  const jsSample = 'structuredClone(x); const last = arr.at(-1); new BroadcastChannel("a");'
  const cssSample = '.card { aspect-ratio: 1; inset: 0; }'
  return (
    scanForbidden(jsSample, FORBIDDEN_JS).length >= 3 &&
    scanForbidden(cssSample, FORBIDDEN_CSS).length >= 2
  )
}

/** §14.7：扫描临时产物是否命中禁用 API / CSS 特性；命中即门禁失败（AC-19）。 */
function scanFeatures(assetsDir) {
  // 自检：扫描器必须能命中样本，否则规则写错会变成「永远通过」的假绿灯。
  if (!selfCheckScanner()) {
    fail('禁用特性扫描器自检失败：规则未能命中样本，疑似失效')
    return false
  }
  const files = existsSync(assetsDir)
    ? readdirSync(assetsDir)
        .filter((name) => name.endsWith('.js') || name.endsWith('.css'))
        .sort()
    : []
  let bad = 0
  let skipped = 0
  for (const name of files) {
    // 跳过 @vitejs/plugin-legacy 按 target 自动产出的 polyfills / legacy 垫片 chunk：
    // 它们不是业务源码，只做能力探测与补缺，会引用 "WebAssembly" 等字符串做特性检测，
    // 扫描会误伤；AC-19 约束的是「我们的应用代码」，不约束插件垫片。
    if (/^(polyfills|legacy)[-.]/.test(name)) {
      skipped += 1
      continue
    }
    const content = readFileSync(join(assetsDir, name), 'utf8')
    const jsHits = name.endsWith('.js') ? scanForbidden(content, FORBIDDEN_JS) : []
    const cssHits = name.endsWith('.css') ? scanForbidden(content, FORBIDDEN_CSS) : []
    const hits = [...new Set([...jsHits, ...cssHits])]
    if (hits.length > 0) {
      bad += 1
      console.error(`  [compat] 命中禁用特性 ${name}: ${hits.join(', ')}`)
    }
  }
  if (bad > 0) {
    fail(`共 ${bad} 个产物文件命中 §14.7 禁用 API/CSS 特性（Safari 14.1 不支持），门禁失败`)
    return false
  }
  console.log(
    `[compat] §14.7 API/CSS 特性扫描：当前源码产物未命中禁用清单` +
      `${skipped > 0 ? `（已跳过 ${skipped} 个插件垫片 chunk）` : ''}`,
  )
  return true
}

function fail(message) {
  console.error(`[compat] FAIL: ${message}`)
  process.exitCode = 1
}

/**
 * 递归列出目录内文件（相对路径 + 字节数 + mtime 毫秒 + 内容 sha256）。
 * 内容哈希用 Node 内置 crypto，零新增依赖；即便 size 与 mtime 被精确恢复，
 * 内容替换也会被 sha256 检出，真正满足「内容与时间戳不变」的验收口径。
 */
function snapshot(dirPath) {
  if (!existsSync(dirPath)) {
    return null
  }
  const entries = []
  const walk = (current) => {
    for (const name of readdirSync(current)) {
      const full = join(current, name)
      const info = statSync(full)
      if (info.isDirectory()) {
        walk(full)
      } else {
        const digest = createHash('sha256').update(readFileSync(full)).digest('hex')
        entries.push(`${relative(dirPath, full)}:${info.size}:${info.mtimeMs}:${digest}`)
      }
    }
  }
  walk(dirPath)
  return entries.sort().join('\n')
}

/** 返回目录内 JS 文件的最新修改时间（ISO 字符串），用于佐证产物新旧。 */
function latestJsMtime(dirPath) {
  if (!existsSync(dirPath)) {
    return null
  }
  const files = readdirSync(dirPath).filter((name) => name.endsWith('.js'))
  let latest = 0
  for (const name of files) {
    const mtime = statSync(join(dirPath, name)).mtimeMs
    if (mtime > latest) {
      latest = mtime
    }
  }
  return latest === 0 ? null : new Date(latest).toISOString()
}

function run(label, args) {
  console.log(`[compat] ${label}: ${process.execPath} ${args.join(' ')}`)
  const result = spawnSync(process.execPath, args, {
    cwd: frontendRoot,
    stdio: 'inherit',
  })
  if (result.error) {
    fail(`${label} 启动失败：${result.error.message}`)
    return false
  }
  if (result.status !== 0) {
    fail(`${label} 退出码 ${result.status}`)
    return false
  }
  return true
}

function main() {
  const legacyBefore = snapshot(legacyDir)
  const staleDistTime = latestJsMtime(distAssetsDir)
  // 保证每次都是全新构建，避免复用上一次残留产物。
  rmSync(tmpDir, { recursive: true, force: true })

  try {
    const bins = [
      ['vite', viteBin],
      ['es-check', esCheckBin],
    ]
    for (const [name, bin] of bins) {
      if (!existsSync(bin)) {
        fail(`找不到本地 ${name} 入口 ${bin}，请先执行 npm install`)
        return
      }
    }

    // 步骤 1：只构建到临时目录，dist/ 与 dist/legacy/ 完全不参与。
    if (
      !run('构建当前源码（临时目录）', [
        viteBin,
        'build',
        '--outDir',
        '.compat-tmp',
        '--emptyOutDir',
      ])
    ) {
      return
    }

    // 步骤 2：列出本次实际扫描的文件，佐证不是 11-09 的陈旧 dist 产物。
    const jsFiles = existsSync(tmpAssetsDir)
      ? readdirSync(tmpAssetsDir).filter((name) => name.endsWith('.js')).sort()
      : []
    if (jsFiles.length === 0) {
      fail('临时产物中没有 JS 文件，无法证明扫描的是当前源码')
      return
    }
    const freshTime = latestJsMtime(tmpAssetsDir)
    console.log('[compat] 本次 es-check 扫描的文件（刚由当前源码构建）：')
    for (const name of jsFiles) {
      const size = statSync(join(tmpAssetsDir, name)).size
      console.log(`  .compat-tmp/assets/${name}  ${size} bytes`)
    }
    console.log(`[compat] 旧 dist/assets JS 时间戳：${staleDistTime === null ? '无' : staleDistTime}（不参与本次扫描）`)
    console.log(`[compat] 本次产物时间戳：${freshTime === null ? '无' : freshTime}`)

    // 步骤 3：特征串命中校验 —— 命中不足说明扫到的仍是陈旧产物，直接失败。
    const combined = jsFiles
      .map((name) => readFileSync(join(tmpAssetsDir, name), 'utf8'))
      .join('\n')
    const hits = BATCH1_MARKERS.filter((marker) => combined.includes(marker))
    console.log(
      `[compat] 批 1 特征串命中 ${hits.length}/${BATCH1_MARKERS.length}：${hits.join(', ')}`,
    )
    if (hits.length < 3) {
      fail('批 1 特征串命中不足，产物疑似陈旧，门禁拒绝通过')
      return
    }

    // 步骤 4：对临时产物执行语法门禁（v2.6 由 es2015 抬到 **es2020**，
    // 只为放行路由懒加载生成的动态 import()；产物基线仍是 es2015，
    // Safari 14 polyfills 一个字未动）。
    if (
      !run('es2020 语法扫描（临时产物）', [
        esCheckBin,
        'es2020',
        '.compat-tmp/assets/*.js',
        '--module',
      ])
    ) {
      return
    }

    // 步骤 4.5：§14.7 API/CSS 特性扫描，放行动态 import 后由这层兜底
    // Safari 14.1 缺失的 API / CSS 特性（AC-19）。
    if (!scanFeatures(tmpAssetsDir)) {
      return
    }

    // 步骤 5：证明 dist/legacy/ 回滚目录内容（sha256）、字节数与时间戳一字未动。
    const legacyAfter = snapshot(legacyDir)
    if (legacyBefore !== null) {
      if (legacyBefore !== legacyAfter) {
        fail('frontend/dist/legacy/ 内容 sha256 / 字节数 / 时间戳发生变化，门禁失败')
        return
      }
      const lines = legacyBefore === '' ? [] : legacyBefore.split('\n')
      const newest = lines.reduce((max, line) => {
        const parts = line.split(':')
        const mtime = Number(parts[parts.length - 2])
        return Number.isFinite(mtime) && mtime > max ? mtime : max
      }, 0)
      const digest = createHash('sha256').update(legacyBefore).digest('hex')
      console.log(
        `[compat] frontend/dist/legacy/ 未改动（${lines.length} 个文件，内容 sha256+字节数+mtime 前后一致；` +
          `快照摘要 ${digest}；最新 mtime ${newest === 0 ? '无' : new Date(newest).toISOString()}）`,
      )
    } else {
      console.log('[compat] frontend/dist/legacy/ 不存在（跳过比对）')
    }
    console.log('[compat] PASS：es-check 扫描的是当前源码新产物；dist/ 与 dist/legacy/ 均未触碰')
  } finally {
    rmSync(tmpDir, { recursive: true, force: true })
    console.log('[compat] 已清理临时目录 .compat-tmp/')
  }
}

main()
