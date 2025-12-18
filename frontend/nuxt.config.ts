// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2024-04-03',
  devtools: { enabled: true },
  modules: ['@nuxtjs/tailwindcss', '@pinia/nuxt'],
  runtimeConfig: {
    public: {
      apiBaseUrl: '/api'
    }
  },
  css: ['~/assets/css/main.css'],
  ssr: false, // 开启服务端渲染
  nitro: {
    routeRules: {
      '/api/**': {
        proxy: 'http://localhost:5000/api/**'
      }
    }
  },
  app: {
    head: {
      title: 'MoneyMint 记账',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'description', content: '个人记账系统' }
      ]
    }
  }
})