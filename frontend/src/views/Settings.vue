<template>
  <div class="page-container">
    <div class="settings-section">
      <h3 class="section-title">禅道连接信息</h3>
      <div class="settings-card">
        <div v-if="loading" class="loading-text">加载中...</div>
        <template v-else-if="accountInfo">
          <div class="info-row">
            <span class="info-label">服务器地址</span>
            <span class="info-value">{{ accountInfo.domain }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">登录账号</span>
            <span class="info-value">{{ accountInfo.account }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">认证模式</span>
            <span class="info-value">
              <el-tag size="small" type="warning">
                {{ authModeLabel }}
              </el-tag>
            </span>
          </div>
          <div class="info-row">
            <span class="info-label">连接状态</span>
            <span class="info-value">
              <el-tag :type="healthZentaoOk ? 'success' : 'danger'" size="small">
                {{ healthZentaoOk ? '已连接' : (healthLoading ? '检测中' : '未连接') }}
              </el-tag>
            </span>
          </div>
          <div class="info-row">
            <span class="info-label">检测延迟</span>
            <span class="info-value">{{ healthZentaoOk ? healthLatency + 'ms' : (healthLoading ? '检测中' : '—') }}</span>
          </div>
        </template>
        <div v-else class="empty-text">无法获取连接信息</div>
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">连接配置</h3>
      <div class="settings-card">
        <div class="config-hint">
          填写禅道基础域名与账号密码，先验证连接再加密保存，保存后立即生效（无需重启）。
          公网部署时可直接在此切换连接目标，无需登录服务器改配置文件。修改需要平台管理员登录。
        </div>
        <div class="config-form">
          <div class="form-item">
            <label class="form-label">基础域名</label>
            <el-input v-model="configForm.domain" placeholder="https://demo.zentao.net" clearable />
          </div>
          <div class="form-item">
            <label class="form-label">账号</label>
            <el-input v-model="configForm.account" placeholder="禅道登录账号" clearable />
          </div>
          <div class="form-item">
            <label class="form-label">密码</label>
            <el-input
              v-model="configForm.password"
              type="password"
              show-password
              placeholder="必填；不回显已保存的密码"
            />
          </div>
          <div class="form-item">
            <label class="form-label">认证域</label>
            <el-select v-model="configForm.realm" style="width: 100%">
              <el-option label="kydc（受限实例 SSO 会话模式）" value="kydc" />
              <el-option label="local（Token 模式 REST API）" value="local" />
            </el-select>
          </div>
          <div class="form-actions">
            <el-button type="primary" size="small" :loading="configSaving" @click="saveZentaoConfig">
              保存并检测
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">显示设置</h3>
      <div class="settings-card">
        <div class="info-row">
          <span class="info-label">主题模式</span>
          <el-radio-group v-model="themeMode" @change="onThemeChange">
            <el-radio-button label="light">浅色</el-radio-button>
            <el-radio-button label="dark">深色</el-radio-button>
            <el-radio-button label="auto">跟随系统</el-radio-button>
          </el-radio-group>
        </div>
        <div class="info-row">
          <span class="info-label">桌面通知</span>
          <div class="notif-controls">
            <el-switch v-model="notificationEnabled" @change="onNotificationChange" />
            <el-button v-if="notificationEnabled && notifPermission !== 'granted'" size="small" link type="primary" @click="requestNotifPermission">
              授权系统通知
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">离线缓存</h3>
      <div class="settings-card">
        <div v-if="cacheLoading" class="loading-text">加载中...</div>
        <template v-else-if="cacheStatus">
          <div class="info-row">
            <span class="info-label">缓存条目</span>
            <span class="info-value">{{ cacheStatus.entryCount }} 条</span>
          </div>
          <div class="info-row">
            <span class="info-label">占用空间</span>
            <span class="info-value">{{ formatBytes(cacheStatus.totalBytes) }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">最后更新</span>
            <span class="info-value">{{ cacheStatus.lastUpdateAt ? formatTime(cacheStatus.lastUpdateAt) : '从未' }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">数据库路径</span>
            <span class="info-value mono">{{ cacheStatus.dbPath }}</span>
          </div>
          <div class="info-row actions">
            <el-button size="small" @click="fetchCacheStatus">刷新</el-button>
            <el-button size="small" type="danger" @click="clearCache">清空缓存</el-button>
          </div>
        </template>
        <div v-else class="empty-text">缓存未启用（SQLite 初始化失败）</div>
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">系统信息</h3>
      <div class="settings-card">
        <div class="info-row">
          <span class="info-label">应用版本</span>
          <span class="info-value">v{{ appVersion }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">前端框架</span>
          <span class="info-value">Vue 3 + Element Plus</span>
        </div>
        <div class="info-row">
          <span class="info-label">后端框架</span>
          <span class="info-value">Go + Hertz</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted } from 'vue'
import { getAccountInfo, updateZentaoConfig } from '@/api/zentao'
import api from '@/api/api'
import type { ApiResponse } from '@/types/api'
import { getStoredThemeMode, setThemeMode, type ThemeMode } from '@/composables/useTheme'
import {
  isNotificationEnabled,
  setNotificationEnabled,
  ensurePermission
} from '@/composables/useDesktopNotification'
import { ElMessage, ElMessageBox } from 'element-plus'

interface AccountInfo {
  domain: string
  account: string
  connected: boolean
  mode?: string
  realm?: string
}

interface CacheStatus {
  entryCount: number
  totalBytes: number
  lastUpdateAt: string
  dbPath: string
}

const loading = ref(true)
const accountInfo = ref<AccountInfo | null>(null)
const healthZentaoOk = ref(false)
const healthLatency = ref(0)
const healthLoading = ref(false)
const appVersion = ref('dev')
const themeMode = ref<ThemeMode>('auto')

// 认证模式标签：统一会话模式（kydc 走受限实例 SSO，local 走禅道本地）。
const authModeLabel = computed<string>(() => {
  const info = accountInfo.value
  if (!info) return ''
  return info.realm ? `会话模式 (${info.realm})` : '会话模式'
})
const notificationEnabled = ref(false)
const notifPermission = ref<NotificationPermission>('default')

const onNotificationChange = async (val: boolean | string | number) => {
  const enabled = Boolean(val)
  setNotificationEnabled(enabled)
  if (enabled) {
    // 用户开启通知时自动请求权限
    const granted = await ensurePermission()
    notifPermission.value = granted ? 'granted' : (typeof Notification !== 'undefined' ? Notification.permission : 'default')
    if (!granted) {
      ElMessage.info('已开启应用内通知，但需要授权才能显示桌面通知')
    }
  }
}

const requestNotifPermission = async () => {
  const granted = await ensurePermission()
  notifPermission.value = granted ? 'granted' : (typeof Notification !== 'undefined' ? Notification.permission : 'default')
  if (granted) {
    ElMessage.success('已授权桌面通知')
  } else {
    ElMessage.warning('授权失败，可在浏览器/系统设置中手动开启')
  }
}

const cacheLoading = ref(true)
const cacheStatus = ref<CacheStatus | null>(null)

const fetchAccountInfo = async () => {
  loading.value = true
  try {
    const res = await getAccountInfo()
    if (res.code === 200 && res.data) {
      accountInfo.value = res.data
      prefillConfigForm(res.data)
    }
  } catch { /* ignore */ }

  // 连接状态用 healthz 实时探测（与心跳页同源），避免 init/account 的
  // connected 标志滞后导致误报"未连接"
  healthLoading.value = true
  try {
    const start = Date.now()
    const hz = await api.get('/healthz') as ApiResponse<{ zentao?: { status?: string; latencyMs?: number } }>
    const zentao = hz?.data?.zentao
    healthZentaoOk.value = zentao?.status === 'ok'
    healthLatency.value = zentao?.latencyMs ?? (Date.now() - start)
  } catch {
    healthZentaoOk.value = false
    healthLatency.value = -1
  } finally {
    healthLoading.value = false
  }

  try {
    const res = await api.get('/version') as ApiResponse<{ version: string }>
    if (res?.data?.version) appVersion.value = res.data.version
  } catch { /* ignore */ }

  loading.value = false
}

const fetchCacheStatus = async () => {
  cacheLoading.value = true
  try {
    const res = await api.get('/cache/status') as ApiResponse<CacheStatus>
    if (res?.data) cacheStatus.value = res.data
  } catch {
    // 缓存可能未启用（404 或 500），静默处理
    cacheStatus.value = null
  } finally {
    cacheLoading.value = false
  }
}

// ─── 连接配置表单 ───

const configForm = reactive({
  domain: '',
  account: '',
  password: '',
  realm: 'local'
})
const configSaving = ref(false)

// 以当前连接预填（密码不回显）；已填过的字段不覆盖，避免打断用户输入
const prefillConfigForm = (info: AccountInfo) => {
  if (!configForm.domain) configForm.domain = info.domain || ''
  if (!configForm.account) configForm.account = info.account || ''
  configForm.realm = info.realm || 'local'
}

const saveZentaoConfig = async () => {
  if (!configForm.domain || !configForm.account || !configForm.password) {
    ElMessage.warning('基础域名、账号、密码均必填')
    return
  }
  configSaving.value = true
  try {
    const res = await updateZentaoConfig({ ...configForm })
    if (res.code === 200) {
      ElMessage.success(res.message || '连接成功，配置已保存')
      configForm.password = ''
      fetchAccountInfo()
    } else {
      ElMessage.error(res.message || '保存失败')
    }
  } catch {
    // api.ts 拦截器已展示后端错误（401 会引导跳登录页）
  } finally {
    configSaving.value = false
  }
}

const clearCache = async () => {
  try {
    await ElMessageBox.confirm('确定要清空所有离线缓存吗？断网时将无法查看历史数据。', '清空确认', {
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await api.delete('/cache')
    ElMessage.success('缓存已清空')
    fetchCacheStatus()
  } catch (e) {
    const msg = e instanceof Error ? e.message : '清空失败'
    ElMessage.error(msg)
  }
}

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const formatTime = (iso: string): string => {
  if (!iso) return ''
  try {
    const d = new Date(iso)
    return d.toLocaleString('zh-CN')
  } catch {
    return iso
  }
}

const onThemeChange = (val: ThemeMode) => {
  setThemeMode(val)
}

onMounted(() => {
  themeMode.value = getStoredThemeMode()
  notificationEnabled.value = isNotificationEnabled()
  notifPermission.value = typeof Notification !== 'undefined' ? Notification.permission : 'default'
  fetchAccountInfo()
  fetchCacheStatus()
})
</script>

<style scoped>
.settings-section {
  margin-bottom: var(--space-xl);
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 var(--space-md) 0;
}

.settings-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  padding: var(--space-lg);
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid var(--color-border-light);
}

.info-row:last-child {
  border-bottom: none;
}

.info-label {
  font-size: 14px;
  color: var(--color-text-secondary);
  font-weight: 500;
}

.info-value {
  font-size: 14px;
  color: var(--color-text-primary);
  font-weight: 600;
}

.notif-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.notif-controls .el-button {
  flex-shrink: 0;
  white-space: nowrap;
}

.loading-text, .empty-text {
  text-align: center;
  padding: var(--space-lg);
  color: var(--color-text-tertiary);
  font-size: 14px;
}

.info-row.actions {
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}

.info-value.mono {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 12px;
  font-weight: 400;
  word-break: break-all;
  max-width: 60%;
  text-align: right;
}

.config-hint {
  font-size: 13px;
  color: var(--color-text-tertiary);
  line-height: 1.6;
  margin-bottom: var(--space-md);
}

.config-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 13px;
  color: var(--color-text-secondary);
  font-weight: 500;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
}
</style>
