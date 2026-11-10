/// <reference types="vite/client" />

/**
 * 构建期注入的应用版本号（来自 frontend/package.json 的 version 字段）。
 * 由 vite.config.ts 的 define.__APP_VERSION__ 提供。
 */
declare const __APP_VERSION__: string
