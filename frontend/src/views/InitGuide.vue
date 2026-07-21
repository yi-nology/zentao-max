<template>
  <div class="init-guide">
    <div class="init-card">
      <div class="init-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="32" height="32">
          <path d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
        </svg>
      </div>
      <h1 class="init-title">系统初始化</h1>

      <!-- 登录方式切换：账号密码 / 浏览器 zentaosid -->
      <div class="mode-switch">
        <el-radio-group v-model="loginMode" size="small">
          <el-radio-button label="password">账号密码登录</el-radio-button>
          <el-radio-button label="zentaosid">从浏览器导入 zentaosid</el-radio-button>
        </el-radio-group>
      </div>

      <!-- 账号密码登录表单 -->
      <form v-if="loginMode === 'password'" @submit.prevent="submitLogin" class="init-form login-form">
        <div class="field">
          <label class="field-label">禅道地址</label>
          <input
            v-model.trim="loginForm.domain"
            type="text"
            class="field-input"
            placeholder="https://your-zentao-server.com"
            autocomplete="url"
          />
        </div>

        <div class="field">
          <label class="field-label">账号</label>
          <input
            v-model.trim="loginForm.account"
            type="text"
            class="field-input"
            placeholder="<user-account>"
            autocomplete="username"
          />
        </div>

        <div class="field">
          <label class="field-label">密码</label>
          <input
            v-model="loginForm.password"
            type="password"
            class="field-input"
            placeholder="••••••••"
            autocomplete="current-password"
          />
        </div>

        <div class="field">
          <label class="field-label">认证域</label>
          <select v-model="loginForm.realm" class="field-input">
            <option value="">禅道本地账号 (local)</option>
            <option value="kydc">受限实例 SSO (kydc)</option>
          </select>
          <p class="field-hint">
            会话模式登录：local 走禅道本地账号（md5 混淆），kydc 走受限实例 SSO（密码明文）。<br />
            适用于所有标准禅道实例以及禁用 REST API 的实例（如受限实例 your-zentao-server.com）。
          </p>
        </div>

        <button type="submit" class="init-btn" :disabled="loading || !canSubmitLogin">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>

      <!-- 从浏览器导入 zentaosid -->
      <div v-else class="init-form sidd-form">
        <div class="sid-steps">
          <p class="step-title">如何获取 zentaosid？</p>
          <ol>
            <li>用浏览器正常打开 <code>{{ zentaoBaseUrl }}</code> 并登录</li>
            <li>F12 打开 DevTools → Application 标签 → Cookies → 选 <code>your-zentao-server.com</code></li>
            <li>找 <code>zentaosid</code> 这一行，复制它的 Value（32 位 hex 字符串）</li>
            <li>粘贴到下面输入框，点"导入并启动"</li>
          </ol>
          <p class="step-warning">
            <strong>为什么要这样做？</strong>server 端每次登录会消耗受限实例 SSO 配额，
            连续失败会触发"密码尝试次数太多"长期 lock（10 分钟）。
            复用浏览器已经验证的 zentaosid cookie，server 不再调 login()，根本绕开这个限制。
          </p>
        </div>

        <div class="field">
          <label class="field-label">zentaosid（32 位 hex）</label>
          <input
            v-model.trim="zentaosidForm.zentaosid"
            type="text"
            class="field-input sid-input"
            placeholder="08e4dde1d0718437ca8f6fc2ea865a9d"
            autocomplete="off"
            spellcheck="false"
          />
        </div>
        <div class="field">
          <label class="field-label">备注（可选）</label>
          <input
            v-model.trim="zentaosidForm.note"
            type="text"
            class="field-input"
            placeholder="从 Chrome 浏览器复制的 zentaosid"
            autocomplete="off"
          />
        </div>

        <button
          type="button"
          class="init-btn"
          :disabled="loading || !canImportSID"
          @click="submitImportZentaoSID"
        >
          {{ loading ? '导入中...' : '导入并启动' }}
        </button>
      </div>

      <div v-if="error" class="message error">{{ error }}</div>
      <div v-if="success" class="message success">
        {{ success }}
        <div class="success-actions">
          <router-link to="/" class="action-btn primary">进入系统</router-link>
          <button @click="testZentao" class="action-btn secondary" :disabled="testing">
            {{ testing ? '测试中...' : '测试禅道连接' }}
          </button>
        </div>
      </div>
      <div v-if="testResult" class="result-box">
        <h3>测试结果</h3>
        <pre>{{ testResult }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElRadioGroup, ElRadioButton } from 'element-plus'
import {
  loginWithCredentials,
  testZentaoConnection,
  importZentaoSID,
  type LoginPayload
} from '@/api/zentao'

const router = useRouter()

const loading = ref<boolean>(false)
const error = ref<string>('')
const success = ref<string>('')
const testing = ref<boolean>(false)
const testResult = ref<string>('')

// 登录表单：domain/account 默认为空，请用户填入受限实例地址和账号。
const loginForm = reactive<LoginPayload>({
  domain: 'https://your-zentao-server.com',
  account: '',
  password: '',
  realm: 'kydc'
})

const canSubmitLogin = computed<boolean>(() =>
  loginForm.domain !== '' && loginForm.account !== '' && loginForm.password !== ''
)

const submitLogin = async (): Promise<void> => {
  if (!canSubmitLogin.value) { error.value = '请填写禅道地址、账号和密码'; return }
  loading.value = true; error.value = ''; success.value = ''
  try {
    // local 域 = Token 模式，传 realm 空串让后端走 REST。
    const payload: LoginPayload = { ...loginForm }
    if (payload.realm === 'local') payload.realm = ''
    const response = await loginWithCredentials(payload)
    if (response.code !== 200) throw new Error(response.message || '登录失败')
    success.value = '登录成功！系统已准备就绪，即将跳转到主页...'
    setTimeout(() => { router.push('/') }, 1500)
  } catch (err) {
    // 后端登录失败返回 400 + message，已被 axios reject；这里直接展示。
    // 注意：登录失败不会触发 401 全局重定向（后端用 400 而非 401）。
    const anyErr = err as { response?: { data?: { message?: string } } }
    const msg = anyErr?.response?.data?.message || (err instanceof Error ? err.message : String(err))
    error.value = msg
  } finally { loading.value = false }
}

const testZentao = async (): Promise<void> => {
  testing.value = true; testResult.value = ''
  try {
    const response = await testZentaoConnection()
    if (response.code !== 200) throw new Error(response.message || '测试失败')
    testResult.value = JSON.stringify(response, null, 2)
  } catch (err) {
    testResult.value = '测试失败: ' + (err instanceof Error ? err.message : String(err))
  } finally { testing.value = false }
}

// === zentaosid 导入模式 ===
//
// 避免受限实例 SSO "密码尝试次数太多" 长期 lock 的关键。
// 用户在浏览器手动登录一次拿到 zentaosid，粘贴到这里 → server 持久化
// + 立刻注入到当前 client → 后续所有禅道请求复用这个 cookie，不再 login。

const loginMode = ref<'password' | 'zentaosid'>('password')
const zentaoBaseUrl = ref<string>('https://your-zentao-server.com')

const zentaosidForm = reactive<{ zentaosid: string; note: string }>({
  zentaosid: '',
  note: '',
})

const canImportSID = computed<boolean>(() => {
  const sid = zentaosidForm.zentaosid.trim()
  // 32 位 hex 是最常见的（md5(secret)），但实际禅道是 hex 16-64 字符都接受
  return /^[a-f0-9]{16,64}$/i.test(sid)
})

const submitImportZentaoSID = async (): Promise<void> => {
  if (!canImportSID.value) {
    error.value = 'zentaosid 格式不对（应是 16-64 位 hex 字符串）'
    return
  }
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    const response = await importZentaoSID({
      zentaosid: zentaosidForm.zentaosid.trim(),
      note: zentaosidForm.note.trim() || undefined,
    })
    if (response.code !== 200) throw new Error(response.message || '导入失败')
    if (response.data?.injected) {
      success.value = '导入成功！已注入到当前 server session。即将跳转...'
    } else {
      success.value = '已保存到磁盘（注入当前 client 失败，重启后自动生效）。'
    }
    setTimeout(() => { router.push('/') }, 1500)
  } catch (err) {
    const anyErr = err as { response?: { data?: { message?: string } } }
    const msg = anyErr?.response?.data?.message || (err instanceof Error ? err.message : String(err))
    error.value = msg
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.init-guide {
  min-height: 100vh;
  display: flex;
  justify-content: center;
  align-items: center;
  background-color: #F1F5F9;
  padding: 20px;
}

.init-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  padding: 48px;
  max-width: 440px;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 28px;
}

.init-icon {
  width: 64px;
  height: 64px;
  border-radius: var(--radius-lg);
  background-color: var(--color-primary-light);
  color: var(--color-primary);
  display: flex;
  align-items: center;
  justify-content: center;
}

.init-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0;
}

.mode-switch {
  display: flex;
  justify-content: center;
}

.sidd-form {
  width: 100%;
}

.sid-steps {
  background: var(--color-bg);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-sm);
  padding: 12px 16px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-secondary);
  width: 100%;
}

.sid-steps .step-title {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.sid-steps ol {
  margin: 0 0 8px;
  padding-left: 22px;
}

.sid-steps code {
  background: var(--color-bg-hover);
  padding: 1px 4px;
  border-radius: 3px;
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--color-primary);
}

.sid-steps .step-warning {
  margin: 8px 0 0;
  padding: 8px 10px;
  background-color: rgba(245, 158, 11, 0.08);
  border-left: 3px solid #f59e0b;
  border-radius: 3px;
  font-size: 11px;
}

.sid-input {
  font-family: var(--font-mono);
  font-size: 12px;
  letter-spacing: 0.5px;
}

.init-form {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.login-form .field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-secondary);
}

.field-input {
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-size: 14px;
  color: var(--color-text-primary);
  background-color: var(--color-bg-card);
  transition: border-color var(--transition-fast);
  font-family: inherit;
}

.field-input:focus {
  outline: none;
  border-color: var(--color-primary);
}

.field-hint {
  font-size: 11px;
  color: var(--color-text-tertiary);
  margin: 2px 0 0;
  line-height: 1.5;
}

.init-btn {
  width: 100%;
  padding: 14px 32px;
  background-color: var(--color-primary);
  color: var(--color-text-on-primary);
  border: none;
  border-radius: var(--radius-sm);
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color var(--transition-fast);
}

.init-btn:hover:not(:disabled) {
  background-color: var(--color-primary-hover);
}

.init-btn:disabled {
  background-color: #CBD5E1;
  cursor: not-allowed;
}

.message {
  padding: 16px;
  border-radius: var(--radius-sm);
  font-size: 14px;
  text-align: center;
}

.message.error {
  background-color: var(--color-danger-light);
  color: #991B1B;
}

.message.success {
  background-color: var(--color-success-light);
  color: #166534;
}

.success-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
  justify-content: center;
}

.action-btn {
  padding: 10px 20px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  text-decoration: none;
  border: none;
  transition: all var(--transition-fast);
}

.action-btn.primary {
  background-color: var(--color-primary);
  color: var(--color-text-on-primary);
}

.action-btn.secondary {
  background-color: var(--color-bg-card);
  color: var(--color-text-primary);
  border: 1px solid var(--color-border);
}

.result-box {
  width: 100%;
  padding: 16px;
  background-color: var(--color-bg);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-sm);
  font-size: 13px;
}

.result-box h3 {
  margin: 0 0 8px;
  font-size: 14px;
  color: var(--color-text-primary);
}

.result-box pre {
  margin: 0;
  padding: 12px;
  background-color: var(--color-bg-hover);
  border-radius: var(--radius-sm);
  font-size: 12px;
  overflow-x: auto;
  white-space: pre-wrap;
  font-family: var(--font-mono);
}
</style>
