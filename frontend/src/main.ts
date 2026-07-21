import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import { initTheme } from './composables/useTheme'

// 启动时立即应用主题，避免首次加载白闪
initTheme()

const app = createApp(App)
app.use(router)
app.use(ElementPlus)
app.mount('#app')
