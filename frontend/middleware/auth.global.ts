import { useNuxtApp } from '#app'
import { useRouter } from 'vue-router'

export default defineNuxtRouteMiddleware((to, from) => {
  const { $api } = useNuxtApp()
  const router = useRouter()
  
  // 如果用户未登录且访问的不是登录页面，跳转到登录页面
  if (!$api.user.value && to.path !== '/login') {
    return navigateTo('/login')
  }
  
  // 如果用户已登录且访问的是登录页面，跳转到首页
  if ($api.user.value && to.path === '/login') {
    return navigateTo('/')
  }
})