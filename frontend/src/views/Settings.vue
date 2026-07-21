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
              <el-tag :type="accountInfo.connected ? 'success' : 'danger'" size="small">
                {{ accountInfo.connected ? '已连接' : '未连接' }}
              </el-tag>
            </span>
          </div>
          <div class="info-row">
            <span class="info-label">检测延迟</span>
            <span class="info-value">{{ latency }}ms</span>
          </div>
        </template>
        <div v-else class="empty-text">无法获取连接信息</div>
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
          <div>
            <el-switch v-model="notificationEnabled" @change="onNotificationChange" />
            <el-button v-if="notificationEnabled && notifPermission !== 'granted'" size="small" link type="primary" @click="requestNotifPermission" style="margin-left: 12px">
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
import { computed, ref, onMounted } from 'vue'
import { getAccountInfo, testZentaoConnection } from '@/api/zentao'
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
const latency = ref(0)
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
    }
  } catch { /* ignore */ }

  try {
    const start = Date.now()
    await testZentaoConnection()
    latency.value = Date.now() - start
  } catch { latency.value = -1 }

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
</style>
