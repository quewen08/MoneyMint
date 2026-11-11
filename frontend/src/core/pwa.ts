/**
 * PWA 注册与更新提示（批 5，E-04 / AC-35）
 *
 * - 仅生产环境注册（dev 无 SW，避免 Vite HMR 与缓存打架）；
 * - 新 SW 安装完成进入 waiting 时**不自动激活**，回调通知 UI 出
 *   「有新版本」横幅；用户确认后才 skipWaiting 并在 controllerchange 后刷新；
 * - Safari 14.1 兼容：只用最基础的 SW API，无 BroadcastChannel / locks。
 */

export interface PwaHandle {
  /** 用户确认更新：激活等待中的 SW 并刷新页面。 */
  confirmUpdate: () => void
}

export function registerServiceWorker(onUpdateAvailable: () => void): PwaHandle | null {
  if (typeof navigator === 'undefined' || navigator.serviceWorker === undefined) {
    return null
  }
  if (import.meta.env.DEV) {
    return null
  }

  let waitingWorker: ServiceWorker | null = null

  const confirmUpdate = (): void => {
    if (waitingWorker === null) return
    waitingWorker.postMessage('SKIP_WAITING')
    // 新 SW 接管后刷新一次，加载新产物。
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      window.location.reload()
    }, { once: true })
  }

  navigator.serviceWorker
    .register('/sw.js')
    .then((registration) => {
      if (registration.waiting !== null && navigator.serviceWorker.controller !== null) {
        waitingWorker = registration.waiting
        onUpdateAvailable()
      }
      registration.addEventListener('updatefound', () => {
        const installing = registration.installing
        if (installing === null) return
        installing.addEventListener('statechange', () => {
          if (installing.state === 'installed' && navigator.serviceWorker.controller !== null) {
            waitingWorker = installing
            onUpdateAvailable()
          }
        })
      })
    })
    .catch(() => {
      // SW 注册失败不影响功能（如非 HTTPS 内网环境）。
    })

  return { confirmUpdate }
}
