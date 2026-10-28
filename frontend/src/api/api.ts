import axios, { type AxiosInstance, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'

const api: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 150000,
  headers: {
    'Content-Type': 'application/json'
  }
})

api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

let isRedirecting = false

function redirectToInit(message: string) {
  if (isRedirecting) return
  isRedirecting = true
  ElMessage.error(message)
  setTimeout(() => {
    window.location.href = '/init-guide'
  }, 1500)
}

api.interceptors.response.use(
  (response: AxiosResponse) => {
    return response.data
  },
  (error) => {
    if (!error.response) {
      ElMessage.error('无法连接到服务器，请检查后端服务是否运行')
      return Promise.reject(error)
    }

    const { status, data } = error.response
    const message: string = data?.message || error.message || '请求失败'

    if (status === 401) {
      // 登录/登出接口自身的 401（密码错误等）由调用方（登录表单）展示，不触发跳转
      const reqUrl: string = error.config?.url || ''
      if (reqUrl.includes('/auth/login') || reqUrl.includes('/auth/logout') || reqUrl.includes('/auth/sso/')) {
        return Promise.reject(error)
      }
      // 平台认证 401（后端 auth 中间件的提示语含"管理员登录"）→ 跳登录页，登录后回到当前页；
      // 其余 401 视为禅道连接配置失效 → 跳初始化引导
      if (message.includes('管理员登录')) {
        if (!isRedirecting) {
          isRedirecting = true
          ElMessage.warning(message || '该操作需要管理员登录')
          const redirect = encodeURIComponent(window.location.pathname + window.location.search)
          setTimeout(() => {
            window.location.href = '/login?redirect=' + redirect
          }, 1200)
        }
        return Promise.reject(error)
      }
      redirectToInit('认证失败，请重新配置')
      return Promise.reject(error)
    }

    if (status === 403) {
      redirectToInit('认证失败，请重新配置')
      return Promise.reject(error)
    }

    if (status >= 500) {
      ElMessage.error(message)
    }

    return Promise.reject(error)
  }
)

export default api
