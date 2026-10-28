<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-logo">
        <div class="logo-icon">Z</div>
        <h1 class="login-title">禅道 Max</h1>
        <p class="login-subtitle">登录后可执行写操作</p>
      </div>

      <!-- SSO 登录（默认）：用户名 + 密码 + 短信验证码（麒麟统一认证） -->
      <form v-if="mode === 'sso'" class="login-form" @submit.prevent="handleSsoLogin">
        <div class="form-field">
          <label class="field-label" for="sso-username">用户名</label>
          <input
            id="sso-username"
            v-model="ssoUsername"
            class="field-input"
            type="text"
            autocomplete="username"
            placeholder="统一认证账号"
            :disabled="loading || sendingCode"
          />
        </div>
        <div class="form-field">
          <label class="field-label" for="sso-password">密码</label>
          <input
            id="sso-password"
            v-model="ssoPassword"
            class="field-input"
            type="password"
            autocomplete="current-password"
            placeholder="统一认证密码"
            :disabled="loading || sendingCode"
          />
        </div>
        <div class="form-field">
          <label class="field-label" for="sso-code">短信验证码</label>
          <div class="code-row">
            <input
              id="sso-code"
              v-model="ssoCode"
              class="field-input code-input"
              type="text"
              inputmode="numeric"
              maxlength="6"
              placeholder="6 位验证码"
              :disabled="loading"
            />
            <button
              class="code-btn"
              type="button"
              :disabled="sendingCode || countdown > 0 || !ssoUsername || !ssoPassword"
              @click="handleSendCode"
            >
              {{ sendingCode ? '发送中...' : countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
            </button>
          </div>
          <p v-if="mobileHint" class="mobile-hint">验证码已发送至 {{ mobileHint }}</p>
        </div>

        <div v-if="error" class="message error">{{ error }}</div>

        <button class="login-btn" type="submit" :disabled="loading || !ssoUsername || !ssoPassword || !ssoCode">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>

      <!-- 本地管理员登录（备用） -->
      <form v-else class="login-form" @submit.prevent="handleLogin">
        <div class="form-field">
          <label class="field-label" for="username">用户名</label>
          <input
            id="username"
            v-model="username"
            class="field-input"
            type="text"
            autocomplete="username"
            placeholder="管理员用户名"
            :disabled="loading"
          />
        </div>
        <div class="form-field">
          <label class="field-label" for="password">密码</label>
          <input
            id="password"
            v-model="password"
            class="field-input"
            type="password"
            autocomplete="current-password"
            placeholder="管理员密码"
            :disabled="loading"
          />
        </div>

        <div v-if="error" class="message error">{{ error }}</div>

        <button class="login-btn" type="submit" :disabled="loading || !username || !password">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>

      <div class="login-footer">
        <a href="javascript:void(0)" class="back-link" @click="toggleMode">
          {{ mode === 'sso' ? '使用本地管理员账号登录 →' : '使用统一认证（SSO）登录 →' }}
        </a>
        <router-link to="/" class="back-link readonly-link">暂不登录，继续浏览 →</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const route = useRoute()
const router = useRouter()
const { login, ssoLogin, sendSsoCode } = useAuth()

// 模式：sso（默认，统一认证）| local（本地管理员）
const mode = ref<'sso' | 'local'>('sso')

// SSO 表单
const ssoUsername = ref('')
const ssoPassword = ref('')
const ssoCode = ref('')
const mobileHint = ref('')
const sendingCode = ref(false)
let countdownTimer: ReturnType<typeof setInterval> | null = null
const countdown = ref(0)

// 本地表单
const username = ref('')
const password = ref('')

const loading = ref(false)
const error = ref('')

const toggleMode = (): void => {
  mode.value = mode.value === 'sso' ? 'local' : 'sso'
  error.value = ''
}

const startCountdown = (): void => {
  countdown.value = 60
  if (countdownTimer) clearInterval(countdownTimer)
  countdownTimer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0 && countdownTimer) {
      clearInterval(countdownTimer)
      countdownTimer = null
    }
  }, 1000)
}

onBeforeUnmount(() => {
  if (countdownTimer) clearInterval(countdownTimer)
})

const handleSendCode = async (): Promise<void> => {
  if (!ssoUsername.value || !ssoPassword.value || sendingCode.value || countdown.value > 0) return
  sendingCode.value = true
  error.value = ''
  try {
    const mobile = await sendSsoCode(ssoUsername.value, ssoPassword.value)
    mobileHint.value = mobile
    startCountdown()
  } catch (err) {
    const resp = (err as { response?: { data?: { message?: string } } })?.response
    error.value = resp?.data?.message || (err instanceof Error ? err.message : '验证码发送失败')
  } finally {
    sendingCode.value = false
  }
}

const handleSsoLogin = async (): Promise<void> => {
  if (!ssoUsername.value || !ssoPassword.value || !ssoCode.value) return
  loading.value = true
  error.value = ''
  try {
    await ssoLogin(ssoUsername.value, ssoPassword.value, ssoCode.value.trim())
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    router.push(redirect)
  } catch (err) {
    const resp = (err as { response?: { data?: { message?: string } } })?.response
    const backendMsg = resp?.data?.message
    error.value = backendMsg || (err instanceof Error ? err.message : '登录失败')
  } finally {
    loading.value = false
  }
}

const handleLogin = async (): Promise<void> => {
  if (!username.value || !password.value) return
  loading.value = true
  error.value = ''
  try {
    await login(username.value, password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    router.push(redirect)
  } catch (err) {
    // 优先展示后端返回的中文提示（如"用户名或密码错误"），axios 网络错误才用通用文案
    const resp = (err as { response?: { data?: { message?: string } } })?.response
    const backendMsg = resp?.data?.message
    const msg = err instanceof Error && !backendMsg ? err.message : String(err)
    error.value = backendMsg || (msg.includes('Network') ? '网络错误，请检查网络连接后重试。' : '登录失败：' + msg)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg, #f5f7fa);
  padding: 24px;
}

.login-card {
  width: 380px;
  background: var(--color-bg-card, #fff);
  border: 1px solid var(--color-border, #e4e7ed);
  border-radius: 12px;
  padding: 36px 32px 28px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.06);
}

.login-logo {
  text-align: center;
  margin-bottom: 28px;
}

.logo-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto 12px;
  border-radius: 12px;
  background: linear-gradient(135deg, #4f6ef7, #6a4ff7);
  color: #fff;
  font-size: 24px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}

.login-title {
  margin: 0;
  font-size: 20px;
  color: var(--color-text-primary, #303133);
}

.login-subtitle {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--color-text-tertiary, #909399);
  line-height: 1.6;
}

.form-field {
  margin-bottom: 16px;
}

.field-label {
  display: block;
  font-size: 13px;
  color: var(--color-text-secondary, #606266);
  margin-bottom: 6px;
}

.field-input {
  width: 100%;
  box-sizing: border-box;
  height: 38px;
  padding: 0 12px;
  border: 1px solid var(--color-border, #dcdfe6);
  border-radius: 8px;
  font-size: 14px;
  color: var(--color-text-primary, #303133);
  background: var(--color-bg-card, #fff);
  outline: none;
  transition: border-color 0.2s;
}

.field-input:focus {
  border-color: #4f6ef7;
}

.code-row {
  display: flex;
  gap: 8px;
}

.code-input {
  flex: 1;
  letter-spacing: 2px;
}

.code-btn {
  flex-shrink: 0;
  height: 38px;
  padding: 0 14px;
  border: 1px solid var(--color-primary, #4f6ef7);
  border-radius: 8px;
  background: var(--color-bg-card, #fff);
  color: var(--color-primary, #4f6ef7);
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
  transition: opacity 0.2s;
}

.code-btn:hover:not(:disabled) {
  opacity: 0.85;
}

.code-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  border-color: var(--color-border, #dcdfe6);
  color: var(--color-text-tertiary, #909399);
}

.mobile-hint {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--color-success, #67c23a);
}

.message.error {
  background: #fef0f0;
  color: #f56c6c;
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 13px;
  margin-bottom: 16px;
  line-height: 1.6;
}

.login-btn {
  width: 100%;
  height: 40px;
  border: none;
  border-radius: 8px;
  background: #4f6ef7;
  color: #fff;
  font-size: 14px;
  cursor: pointer;
  transition: opacity 0.2s;
}

.login-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.login-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.login-footer {
  margin-top: 20px;
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.back-link {
  font-size: 13px;
  color: var(--color-text-tertiary, #909399);
  text-decoration: none;
}

.back-link:hover {
  color: #4f6ef7;
}

.readonly-link {
  font-size: 12px;
}
</style>
