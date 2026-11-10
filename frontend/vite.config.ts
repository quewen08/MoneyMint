import { readFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'

import legacy from '@vitejs/plugin-legacy'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

// 从 package.json 读取版本号并注入到前端（顶部版本标识用）。
const pkg = JSON.parse(
  readFileSync(new URL('./package.json', import.meta.url), 'utf-8'),
) as { version: string }

// Vite 配置：入口为 frontend/index.html，产物输出到 frontend/dist。
// base 用相对路径，方便反代到任意子路径；legacy 兜底 Safari 14。
export default defineConfig({
  base: './',
  plugins: [
    vue(),
    legacy({
      targets: ['Safari >= 14'],
      // 不生成旧浏览器 chunk（现代构建本身已降到 es2015），只补现代浏览器缺失的 polyfill。
      renderLegacyChunks: false,
      modernPolyfills: true,
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  build: {
    // 注意：不能输出到 ../web（那是 Flutter 源目录），统一落在 frontend/dist。
    // Vite 的 root 是 frontend/，此处写 'dist' 即计划里的 frontend/dist；
    // 若照字面写 '../dist' 会落到仓库根目录（与后端二进制目录冲突）。
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2015',
  },
  server: {
    proxy: {
      // 开发期跨域：前端始终请求相对 /api，由 Vite 转发到本机后端。
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
    // Windows 下多 worker 并发写 vite-node 的 SSR 临时缓存会触发 EPERM，
    // 导致部分测试文件加载失败；顺序执行测试文件即可稳定全绿。
    fileParallelism: false,
  },
})
