import { ref } from 'vue'
import { getAuthStatus, login as loginApi, logout as logoutApi, ssoLogin as ssoLoginApi, sendSsoCode as sendSsoCodeApi } from '@/api/auth'
import type { AuthStatusData } from '@/api/auth'

// 全局认证状态：anonymous=只读，admin=读写。
// Layout 挂载时刷新，登录/登出后调用 login/logout 更新。
const authStatus = ref<AuthStatusData>({ authenticated: false, role: 'readonly', username: '' })
const loaded = ref(false)

async function refreshAuthStatus(): Promise<void> {
  try {
    const res = await getAuthStatus()
    if (res.code === 200 && res.data) {
      authStatus.value = res.data
    }
  } catch {
    // 后端不可用时保持只读态
    authStatus.value = { authenticated: false, role: 'readonly', username: '' }
  } finally {
    loaded.value = true
  }
}

async function login(username: string, password: string): Promise<void> {
  const res = await loginApi(username, password)
  if (res.code === 200 && res.data) {
    authStatus.value = { authenticated: true, role: 'admin', username: res.data.username }
  }
}

/** 触发 SSO 短信验证码发送，返回脱敏手机号 */
async function sendSsoCode(username: string, password: string): Promise<string> {
  const res = await sendSsoCodeApi(username, password)
  if (res.code === 200 && res.data) {
    return res.data.mobile
  }
  throw new Error(res.message || '验证码发送失败')
}

/** SSO 短信验证码登录（麒麟统一认证），成功置管理员态 */
async function ssoLogin(username: string, password: string, code: string): Promise<void> {
  const res = await ssoLoginApi(username, password, code)
  if (res.code === 200 && res.data) {
    authStatus.value = { authenticated: true, role: 'admin', username: res.data.username }
  }
}

async function logout(): Promise<void> {
  try {
    await logoutApi()
  } finally {
    authStatus.value = { authenticated: false, role: 'readonly', username: '' }
  }
}

export function useAuth() {
  return { authStatus, loaded, refreshAuthStatus, login, ssoLogin, sendSsoCode, logout }
}
