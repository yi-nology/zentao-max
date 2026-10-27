import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'
import { initTheme } from './composables/useTheme'

// 启动时立即应用主题，避免首次加载白闪
initTheme()

const app = createApp(App)
app.use(router)
// 界面文案是中文，但 Element Plus 内置弹窗（MessageBox 确认/取消、分页、日期选择等）默认英文，统一配置中文 locale
app.use(ElementPlus, { locale: zhCn })
app.mount('#app')
