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

/** SSO 启用状态（登录页检测，匿名可访问） */
export const getSSOStatus = (): Promise<ApiResponse<{ enabled: boolean }>> => {
  return api.get('/auth/sso/status')
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

// ---- SSO 登录配置（设置页「SSO 登录配置」，保存即热生效） ----

export interface SSOConfigData {
  enabled: boolean
  /** 统一认证平台根地址 */
  baseUrl: string
  /** 登录页完整地址；空串 = 回退 {baseUrl}/sso/auth */
  authPageUrl: string
  /** true = 数据库保存过（重启仍生效）；false = 来自 config.yaml bootstrap */
  managed: boolean
  /** 'db' | 'yaml' */
  source: 'db' | 'yaml'
}

export const getSSOConfig = (): Promise<ApiResponse<SSOConfigData>> => {
  return api.get('/v1/sso-config')
}

export interface SSOConfigPayload {
  enabled: boolean
  baseUrl: string
  authPageUrl: string
}

export const updateSSOConfig = (payload: SSOConfigPayload): Promise<ApiResponse<SSOConfigData>> => {
  return api.post('/v1/sso-config', payload)
}
