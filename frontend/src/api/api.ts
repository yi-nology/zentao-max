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
      // 其余 401 视为禅道连接配置失效 → 仅提示错误
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
      ElMessage.error(message)
      return Promise.reject(error)
    }

    if (status === 403) {
      ElMessage.error(message)
      return Promise.reject(error)
    }

    if (status >= 500) {
      ElMessage.error(message)
    }

    return Promise.reject(error)
  }
)

/**
 * 从 axios 错误里提取后端语义化错误消息（供业务 catch 展示 4xx 等
 * 非拦截器覆盖场景）。无后端消息时返回空串，调用方自行给兜底文案。
 * 职责约定（issue #6）：拦截器负责网络错误/401/403/5xx 的 toast，
 * 业务 catch 只处理需要额外语义文案的失败。
 */
export function extractApiError(err: unknown): string {
  const anyErr = err as { response?: { data?: { message?: string } }; message?: string }
  return anyErr?.response?.data?.message || ''
}

export default api
