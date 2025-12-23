// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2024-04-03',
  devtools: { enabled: true },
  modules: [
    '@nuxtjs/tailwindcss', 
    '@pinia/nuxt',
  ],
  runtimeConfig: {
    public: {
      apiBaseUrl: '/api',
      version: process.env.npm_package_version || '1.0.0',
      // 使用 NUXT_PUBLIC_ 前缀的环境变量会自动被添加，这里保留作为兼容
      buildDate: process.env.NUXT_PUBLIC_BUILD_DATE || process.env.BUILD_DATE || new Date().toISOString(),
      buildHash: process.env.NUXT_PUBLIC_BUILD_HASH || process.env.BUILD_HASH || 'dev',
      gitBranch: process.env.NUXT_PUBLIC_GIT_BRANCH || process.env.GIT_BRANCH || 'dev'
    }
  },
  css: [
    '~/assets/css/main.css',
  ],
  ssr: false, // 开启服务端渲染
  nitro: {
    routeRules: {
      '/api/**': {
        proxy: 'http://localhost:5000/api/**'
      }
    }
  },
  devServer: {
    port: 3000,
    host: '0.0.0.0'
  },
  app: {
    head: {
      title: 'MoneyMint 记账',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'description', content: 'MoneyMint 是一个基于 Vue 3 和 Nuxt 3 的个人记账系统' }
      ]
    }
  },
  // 编译时删除 console.log 语句
  vite: {
    esbuild: {
      drop: ['debugger'],
      pure: process.env.NODE_ENV === 'production' ? ['console.log'] : [],
    },
    build: {
      minify: true,
    },
  },
})