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

// ---- SSO 登录（麒麟统一认证：用户名+密码+短信验证码） ----

export interface SendCodeResult {
  mobile: string
  hint: string
}

/** 触发 SSO 短信验证码发送，返回脱敏手机号（186****1498） */
export const sendSsoCode = (username: string, password: string): Promise<ApiResponse<SendCodeResult>> => {
  return api.post('/auth/sso/send-code', { username, password })
}

/** SSO 验证码登录，成功签发平台会话 */
export const ssoLogin = (username: string, password: string, code: string): Promise<ApiResponse<LoginResult>> => {
  return api.post('/auth/sso/login', { username, password, code })
}

export const logout = (): Promise<ApiResponse<unknown>> => {
  return api.post('/auth/logout')
}
