<template>
  <div class="setup-container">
    <div class="setup-card">
      <div class="setup-logo">
        <div class="logo-icon">Z</div>
        <h1 class="setup-title">zentao-max 初始设置</h1>
        <p class="setup-subtitle">首次启动向导：连接禅道 → 配置登录 → 开始使用</p>
      </div>

      <!-- 步骤指示 -->
      <div class="steps">
        <div class="step" :class="{ active: step === 1, done: step > 1 }">1 连接禅道</div>
        <div class="step" :class="{ active: step === 2, done: step > 2 }">2 登录方式</div>
        <div class="step" :class="{ active: step === 3 }">3 完成</div>
      </div>

      <!-- 步骤 1：禅道连接 -->
      <form v-if="step === 1" class="setup-form" @submit.prevent="handleSaveConnection">
        <div class="form-field">
          <label class="field-label">禅道地址</label>
          <input
            v-model="form.domain"
            class="field-input"
            type="text"
            placeholder="https://zentao.example.com"
            autocomplete="url"
            :disabled="saving"
          />
        </div>
        <div class="form-field">
          <label class="field-label">账号</label>
          <input
            v-model="form.account"
            class="field-input"
            type="text"
            placeholder="禅道账号"
            autocomplete="username"
            :disabled="saving"
          />
        </div>
        <div class="form-field">
          <label class="field-label">密码</label>
          <input
            v-model="form.password"
            class="field-input"
            type="password"
            placeholder="禅道密码"
            autocomplete="current-password"
            :disabled="saving"
          />
        </div>
        <div class="form-field">
          <label class="field-label">认证域</label>
          <select v-model="form.realm" class="field-input" :disabled="saving">
            <option value="">标准（密码 MD5 摘要登录）</option>
            <option value="kydc">kydc（统一认证域，密码明文）</option>
          </select>
          <p class="field-hint">受限实例（如 pm.kylin.com）选 kydc；不确定先用标准，验证失败再切换。</p>
        </div>

        <div v-if="error" class="message error">{{ error }}</div>

        <button class="setup-btn" type="submit" :disabled="saving || !form.domain || !form.account || !form.password">
          {{ saving ? '正在验证并保存…' : '验证并保存，下一步' }}
        </button>
        <p class="field-hint">保存时会真实登录一次禅道验证凭据；成功后自动进入下一步。</p>
      </form>

      <!-- 步骤 2：登录方式（SSO） -->
      <form v-else-if="step === 2" class="setup-form" @submit.prevent="handleSaveSSO">
        <div class="form-field">
          <label class="checkbox-row">
            <input v-model="sso.enabled" type="checkbox" />
            <span>启用公司 SSO 登录（统一认证平台，用户名 + 密码 + 短信验证码）</span>
          </label>
          <p class="field-hint">平台已移除本地账号密码登录，SSO 是唯一登录通道；不启用则所有人都只能只读浏览，无法执行写操作。</p>
        </div>
        <template v-if="sso.enabled">
          <div class="form-field">
            <label class="field-label">SSO 平台地址</label>
            <input
              v-model="sso.baseUrl"
              class="field-input"
              type="text"
              placeholder="https://sso.example.com"
              :disabled="saving"
            />
          </div>
          <div class="form-field">
            <label class="field-label">认证页地址（可选）</label>
            <input
              v-model="sso.authPageUrl"
              class="field-input"
              type="text"
              placeholder="留空 = {SSO平台地址}/sso/auth"
              :disabled="saving"
            />
          </div>
        </template>

        <div v-if="error" class="message error">{{ error }}</div>

        <div class="btn-row">
          <button class="setup-btn" type="submit" :disabled="saving || (sso.enabled && !sso.baseUrl)">
            {{ saving ? '保存中…' : sso.enabled ? '保存 SSO 配置，下一步' : '下一步（暂不启用 SSO）' }}
          </button>
        </div>
      </form>

      <!-- 步骤 3：完成 -->
      <div v-else class="setup-form">
        <div class="done-icon">✓</div>
        <p class="done-text">初始设置完成</p>
        <div class="done-info">
          <p>禅道：<strong>{{ savedDomain }}</strong>（{{ savedConnected ? '已连接' : '未连接' }}）</p>
          <p v-if="savedSSO">SSO 登录：已启用</p>
          <p v-else class="warn">SSO 登录：未启用（只能只读浏览，写操作需要登录）——可随时在「设置」页开启</p>
        </div>
        <router-link to="/dashboard" class="setup-btn done-link">进入仪表盘 →</router-link>
      </div>

      <div class="setup-footer">
        <router-link v-if="step < 3" to="/dashboard" class="skip-link">暂不设置，先浏览 →</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getAccountInfo, updateZentaoConfig } from '@/api/zentao'
import { getSSOConfig, updateSSOConfig } from '@/api/auth'

const step = ref(1)
const saving = ref(false)
const error = ref('')

const form = ref({ domain: '', account: '', password: '', realm: '' })
const sso = ref({ enabled: false, baseUrl: '', authPageUrl: '' })

const savedDomain = ref('')
const savedConnected = ref(false)
const savedSSO = ref(false)

onMounted(async () => {
  // 已保存的配置回填（yaml bootstrap 的 SSO 也可视化确认）
  try {
    const res = await getSSOConfig()
    if (res.data) {
      sso.value.enabled = res.data.enabled
      sso.value.baseUrl = res.data.baseUrl
      sso.value.authPageUrl = res.data.authPageUrl
    }
  } catch { /* 拉不到保持空表单 */ }
  try {
    const acc = await getAccountInfo()
    savedDomain.value = acc.data?.domain || ''
    savedConnected.value = !!acc.data?.connected
    if (acc.data?.domain && step.value === 1) {
      form.value.domain = acc.data.domain
    }
  } catch { /* ignore */ }
})

async function handleSaveConnection(): Promise<void> {
  error.value = ''
  saving.value = true
  try {
    const res = await updateZentaoConfig({
      domain: form.value.domain.trim(),
      account: form.value.account.trim(),
      password: form.value.password,
      realm: form.value.realm
    })
    if (res.code === 200) {
      savedDomain.value = res.data?.domain || form.value.domain
      savedConnected.value = !!res.data?.connected
      step.value = 2
    } else {
      error.value = res.message || '保存失败'
    }
  } catch (e) {
    error.value = extractErr(e)
  } finally {
    saving.value = false
  }
}

async function handleSaveSSO(): Promise<void> {
  error.value = ''
  if (!sso.value.enabled) {
    savedSSO.value = false
    step.value = 3
    return
  }
  saving.value = true
  try {
    const res = await updateSSOConfig({
      enabled: true,
      baseUrl: sso.value.baseUrl.trim(),
      authPageUrl: sso.value.authPageUrl.trim()
    })
    if (res.code === 200) {
      savedSSO.value = true
      step.value = 3
    } else {
      error.value = res.message || 'SSO 配置保存失败'
    }
  } catch (e) {
    error.value = extractErr(e)
  } finally {
    saving.value = false
  }
}

function extractErr(e: unknown): string {
  const anyErr = e as { response?: { data?: { message?: string } }; message?: string }
  return anyErr?.response?.data?.message || (e instanceof Error ? e.message : '操作失败')
}
</script>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--color-bg-page);
}

.setup-card {
  width: 100%;
  max-width: 520px;
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  padding: 32px;
}

.setup-logo {
  text-align: center;
  margin-bottom: 20px;
}

.logo-icon {
  width: 44px;
  height: 44px;
  margin: 0 auto 10px;
  border-radius: 10px;
  background: var(--color-primary);
  color: #fff;
  font-size: 22px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}

.setup-title {
  font-size: 20px;
  margin: 0 0 6px;
  color: var(--color-text-primary);
}

.setup-subtitle {
  font-size: 13px;
  color: var(--color-text-secondary);
  margin: 0;
}

.steps {
  display: flex;
  gap: 8px;
  margin-bottom: 22px;
}

.step {
  flex: 1;
  text-align: center;
  font-size: 12px;
  padding: 7px 0;
  border-radius: var(--radius-sm);
  background: var(--color-bg-hover);
  color: var(--color-text-secondary);
}

.step.active {
  background: var(--color-primary);
  color: #fff;
}

.step.done {
  background: var(--color-primary-light, #e8f0ff);
  color: var(--color-primary);
}

.setup-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.field-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.field-input {
  height: 38px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: 0 12px;
  font-size: 14px;
  background: var(--color-bg-page);
  color: var(--color-text-primary);
}

.field-hint {
  font-size: 12px;
  color: var(--color-text-tertiary, var(--color-text-secondary));
  margin: 2px 0 0;
}

.checkbox-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 13px;
  color: var(--color-text-primary);
  cursor: pointer;
}

.message.error {
  background: var(--color-danger-light, #fdecec);
  color: var(--color-danger, #d33);
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  font-size: 13px;
}

.setup-btn {
  height: 40px;
  border: none;
  border-radius: var(--radius-sm);
  background: var(--color-primary);
  color: #fff;
  font-size: 14px;
  cursor: pointer;
  text-align: center;
}

.setup-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.btn-row {
  display: flex;
  justify-content: flex-end;
}

.done-icon {
  width: 54px;
  height: 54px;
  margin: 6px auto 10px;
  border-radius: 50%;
  background: var(--color-success-light, #e8f7ee);
  color: var(--color-success, #2ba471);
  font-size: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.done-text {
  text-align: center;
  font-size: 16px;
  color: var(--color-text-primary);
  margin: 0 0 14px;
}

.done-info {
  background: var(--color-bg-hover);
  border-radius: var(--radius-sm);
  padding: 12px 14px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.done-info p {
  margin: 4px 0;
}

.done-info .warn {
  color: var(--color-warning, #b8860b);
}

.done-link {
  display: block;
  margin-top: 14px;
  line-height: 40px;
  text-decoration: none;
}

.setup-footer {
  margin-top: 16px;
  text-align: center;
}

.skip-link {
  font-size: 12px;
  color: var(--color-text-tertiary, var(--color-text-secondary));
  text-decoration: none;
}
</style>
