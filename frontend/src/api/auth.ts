import api from './api'
import type { ApiResponse } from '@/types/api'

export interface AuthStatusData {
  authenticated: boolean
  role: 'readonly' | 'admin'
  username: string
}

export interface LoginResult {
  token: string
  username: string
  role: string
}

// 平台访问控制：匿名只读，登录管理员读写
export const getAuthStatus = (): Promise<ApiResponse<AuthStatusData>> => {
  return api.get('/auth/status')
}

export const login = (username: string, password: string): Promise<ApiResponse<LoginResult>> => {
  return api.post('/auth/login', { username, password })
}

export const logout = (): Promise<ApiResponse<unknown>> => {
  return api.post('/auth/logout')
}
