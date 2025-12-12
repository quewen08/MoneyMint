import { defineNuxtConfig } from "nuxt/config";

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-12-09',
  devtools: { enabled: true },
  modules: [],
  css: ['~/assets/css/main.css'],

  devServer: {
    host: '0.0.0.0',
    port: 3000
  }
})