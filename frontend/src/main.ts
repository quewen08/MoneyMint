import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import router from './router'
import './styles/tokens.css'

// 应用入口：Pinia 状态容器 + hash 路由 + 设计 token 样式。
const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
