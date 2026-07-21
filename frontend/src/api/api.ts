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

    if (status === 401 || status === 403) {
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
