/**
 * FamBean Service Worker（批 5，E-04 / AC-26 / AC-35 / AC-36）
 *
 * 铁律：**只缓存应用壳与静态资源，API 响应一律不缓存**——本地真相源是
 * IndexedDB（core/sync），缓存 API 响应会与同步协议打架。
 * - `/api/*` 与 `/legacy/*` 完全不拦截（网络直连 / 回滚基准原样）；
 * - 导航请求：network-first，失败回落缓存的 `/`（离线打开应用壳，AC-26）；
 * - `/assets/*`（Vite 哈希文件名，不可变）与其余同源静态：cache-first；
 * - 更新流程：新 SW 安装后**不自动 skipWaiting**（不得打断正在录入的记账，
 *   AC-35 / S-A02），postMessage 通知页面出「有新版本」提示，用户确认后才
 *   skipWaiting + 刷新。
 */
const SHELL_CACHE = 'fambean-shell-v1'
const ASSET_CACHE = 'fambean-assets-v1'

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(SHELL_CACHE).then((cache) => cache.addAll(['/'])))
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      const names = await caches.keys()
      await Promise.all(
        names
          .filter((n) => n !== SHELL_CACHE && n !== ASSET_CACHE)
          .map((n) => caches.delete(n)),
      )
      // 无旧客户端接管需求：等待下一次导航自然使用新 SW。
      await self.clients.claim()
    })(),
  )
})

self.addEventListener('message', (event) => {
  if (event.data === 'SKIP_WAITING' && self.skipWaiting !== undefined) {
    self.skipWaiting()
  }
})

self.addEventListener('fetch', (event) => {
  const req = event.request
  if (req.method !== 'GET') return
  const url = new URL(req.url)
  if (url.origin !== self.location.origin) return
  // AC-36：API 与 /legacy 回滚基准绝不拦截。
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/legacy/')) return

  // 导航：network-first，断网回落缓存的壳。
  if (req.mode === 'navigate') {
    event.respondWith(
      (async () => {
        try {
          const fresh = await fetch(req)
          const cache = await caches.open(SHELL_CACHE)
          cache.put('/', fresh.clone())
          return fresh
        } catch {
          const cached = await caches.match('/')
          return (
            cached ??
            new Response('离线且无缓存壳', { status: 503, headers: { 'Content-Type': 'text/plain' } })
          )
        }
      })(),
    )
    return
  }

  // 哈希命名的构建产物与其余同源静态：cache-first。
  event.respondWith(
    (async () => {
      const cached = await caches.match(req)
      if (cached !== undefined) return cached
      try {
        const fresh = await fetch(req)
        if (fresh.ok) {
          const cache = await caches.open(ASSET_CACHE)
          cache.put(req, fresh.clone())
        }
        return fresh
      } catch {
        return new Response('离线且无缓存资源', { status: 503 })
      }
    })(),
  )
})
