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
      <h3 class="section-title">服务与开机启动</h3>
      <div class="settings-card">
        <div class="info-row">
          <span class="info-label">平台</span>
          <span class="info-value">{{ svcInfo.platform }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">安装状态</span>
          <span class="info-value">
            <el-tag v-if="svcInfo.installed" type="success" size="small">已安装（开机自启）</el-tag>
            <el-tag v-else type="info" size="small">未安装</el-tag>
            <el-tag v-if="svcInfo.running" type="success" size="small" style="margin-left:6px">运行中</el-tag>
          </span>
        </div>
        <div class="info-row" v-if="svcInfo.execPath">
          <span class="info-label">可执行文件</span>
          <span class="info-value update-notes">{{ svcInfo.execPath }}</span>
        </div>
        <div class="info-row" v-if="svcInfo.workDir">
          <span class="info-label">工作目录</span>
          <span class="info-value update-notes">{{ svcInfo.workDir }}（数据目录随此路径）</span>
        </div>
        <div class="info-row" v-if="svcInfo.detail">
          <span class="info-label">说明</span>
          <span class="info-value update-notes">{{ svcInfo.detail }}</span>
        </div>
        <div class="update-actions">
          <el-button v-if="!svcInfo.installed && !svcInfo.devMode" size="small" type="primary"
            :loading="svcAction === 'install'" @click="svcDo('install')">
            一键安装（开机自启）
          </el-button>
          <template v-if="svcInfo.installed">
            <el-button size="small" :loading="svcAction === 'restart'" @click="svcDo('service-restart')">重启服务</el-button>
            <el-button size="small" :loading="svcAction === 'stop'" @click="svcDo('service-stop')">停止服务</el-button>
            <el-button size="small" :loading="svcAction === 'start'" @click="svcDo('service-start')">启动服务</el-button>
            <el-button size="small" type="danger" :loading="svcAction === 'uninstall'" @click="svcDo('uninstall')">卸载服务</el-button>
          </template>
        </div>
        <div class="update-notes" style="margin-top:10px">
          安装后将注册为系统服务：开机/登录自启、崩溃自动拉起（Linux）。环境变量会固化进服务定义，
          修改 ZENTAO_ENCRYPTION_KEY 等后需重新安装。⚠️ 安装后请停止手动运行的实例，避免端口冲突。
        </div>
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">软件更新</h3>
      <div class="settings-card">
        <div class="info-row">
          <span class="info-label">当前版本</span>
          <span class="info-value">v{{ appVersion }}
            <el-tag v-if="updateInfo?.hasUpdate" type="warning" size="small" style="margin-left:6px">
              可更新至 {{ updateInfo.latest }}
            </el-tag>
          </span>
        </div>
        <div class="info-row" v-if="updateInfo?.latest">
          <span class="info-label">最新版本</span>
          <span class="info-value">{{ updateInfo.latest }}</span>
        </div>
        <div class="info-row" v-if="updateInfo?.notes">
          <span class="info-label">更新说明</span>
          <span class="info-value update-notes">{{ updateInfo.notes }}</span>
        </div>
        <div class="info-row" v-if="updateInfo?.downloadState === 'downloading'">
          <span class="info-label">下载进度</span>
          <span class="info-value update-notes" style="min-width:180px">
            <el-progress :percentage="updateInfo.downloadPercent" :stroke-width="10" />
          </span>
        </div>
        <div class="info-row" v-if="updateInfo?.downloadState === 'done'">
          <span class="info-label">已下载</span>
          <span class="info-value update-notes">{{ updateInfo.downloadPath }}（替换二进制后重启生效）</span>
        </div>
        <div class="info-row" v-if="updateInfo?.downloadState === 'failed'">
          <span class="info-label">下载失败</span>
          <span class="info-value update-notes">{{ updateInfo.downloadError }}</span>
        </div>
        <div class="info-row" v-if="updateInfo?.lastError">
          <span class="info-label">上次检查</span>
          <span class="info-value update-notes">{{ updateInfo.lastError }}</span>
        </div>
        <div class="update-actions">
          <el-button size="small" :loading="checking" @click="checkUpdate">
            {{ checking ? '检查中...' : '检查更新' }}
          </el-button>
          <el-button
            v-if="updateInfo?.hasUpdate && updateInfo.downloadUrl && updateInfo.downloadState !== 'done'"
            size="small" type="primary"
            :loading="updateInfo.downloadState === 'downloading'"
            @click="downloadUpdate"
          >
            {{ updateInfo.downloadState === 'downloading' ? '下载中...' : '下载更新包' }}
          </el-button>
          <el-button
            v-if="updateInfo?.downloadState === 'done' && !updateInfo.applied"
            size="small" type="warning" :loading="applying"
            @click="applyingUpdate"
          >
            应用更新
          </el-button>
          <el-button
            v-if="updateInfo?.applied && updateInfo.restartSupported"
            size="small" type="success" :loading="restarting"
            @click="restartApp"
          >
            重启应用
          </el-button>
          <el-button v-if="updateInfo?.downloadUrl" size="small" @click="openDownloadPage">在浏览器打开</el-button>
        </div>
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

// ─── 软件更新 ───
interface UpdateInfo {
  current: string
  latest: string
  hasUpdate: boolean
  downloadUrl: string
  notes: string
  source: string
  checkedAt: string | null
  lastError: string
  checking: boolean
  os: string
  arch: string
  downloadState: 'none' | 'downloading' | 'done' | 'failed'
  downloadPath: string
  downloadError: string
  downloadTotal: number
  downloaded: number
  downloadPercent: number
  applied: boolean
  newBinaryPath: string
  restartSupported: boolean
}
interface SvcStatus {
  platform: string
  installed: boolean
  running: boolean
  execPath: string
  workDir: string
  devMode: boolean
  unitPath: string
  detail: string
  serviceArgs: string
}
const svcInfo = ref<SvcStatus>({ platform: '', installed: false, running: false, execPath: '', workDir: '', devMode: false, unitPath: '', detail: '', serviceArgs: '' })
const svcAction = ref('')

const fetchSvcStatus = async (): Promise<void> => {
  try {
    const res = await api.get('/system/install-status') as unknown as { data?: SvcStatus }
    if (res?.data) svcInfo.value = res.data
  } catch { /* 忽略 */ }
}

const svcDo = async (action: string): Promise<void> => {
  svcAction.value = action
  try {
    const path = action === 'install' || action === 'uninstall'
      ? `/system/${action}`
      : `/system/service/${action.replace('service-', '')}`
    const res = await api.post(path, {}) as unknown as { data?: { message?: string } }
    ElMessage.success(res?.data?.message || '操作成功')
    await fetchSvcStatus()
  } catch (e) {
    const resp = (e as { response?: { data?: { message?: string } } })?.response
    ElMessage.error(resp?.data?.message || (e instanceof Error ? e.message : '操作失败'))
  } finally {
    svcAction.value = ''
  }
}

const updateInfo = ref<UpdateInfo | null>(null)
const checking = ref(false)

const fetchUpdateStatus = async (): Promise<void> => {
  try {
    const res = await api.get('/update/status') as unknown as { data?: UpdateInfo }
    updateInfo.value = res?.data || null
  } catch { /* 状态获取失败不提示，手动检查时再反馈 */ }
}

const checkUpdate = async (): Promise<void> => {
  checking.value = true
  try {
    const res = await api.post('/update/check', {}) as unknown as { data?: UpdateInfo }
    updateInfo.value = res?.data || null
    if (res?.data?.hasUpdate) ElMessage.success(`发现新版本 ${res.data.latest}`)
    else if (res?.data?.lastError) ElMessage.warning(`检查失败：${res.data.lastError}`)
    else ElMessage.success('当前已是最新版本')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '检查更新失败')
  } finally {
    checking.value = false
  }
}

const downloadUpdate = async (): Promise<void> => {
  try {
    await api.post('/update/download', {})
    ElMessage.success('下载已开始，完成后会显示保存路径')
    // 轮询下载状态（含进度百分比）
    const timer = setInterval(async () => {
      await fetchUpdateStatus()
      if (updateInfo.value?.downloadState !== 'downloading') clearInterval(timer)
    }, 1500)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '下载失败')
  }
}

const applying = ref(false)
const applyingUpdate = async (): Promise<void> => {
  applying.value = true
  try {
    const res = await api.post('/update/apply', {}) as unknown as { data?: { message?: string } }
    ElMessage.success(res?.data?.message || '更新已应用')
    await fetchUpdateStatus()
  } catch (e) {
    const resp = (e as { response?: { data?: { message?: string } } })?.response
    ElMessage.error(resp?.data?.message || (e instanceof Error ? e.message : '应用更新失败'))
  } finally {
    applying.value = false
  }
}

const restarting = ref(false)
const restartApp = async (): Promise<void> => {
  try {
    await ElMessageBox.confirm('将重启应用以加载新版本（数秒不可用），继续？', '重启确认', { type: 'warning' })
  } catch { return }
  restarting.value = true
  try {
    await api.post('/update/restart', {})
    ElMessage.success('重启中，页面将在数秒后自动恢复...')
    // 轮询后端恢复后刷新页面（新版本生效）
    for (let i = 0; i < 30; i++) {
      await new Promise((r) => setTimeout(r, 2000))
      try {
        await api.get('/version')
        location.reload()
        return
      } catch { /* 未恢复，继续等 */ }
    }
    ElMessage.warning('未检测到服务恢复，请手动刷新页面并检查服务')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '重启失败')
  } finally {
    restarting.value = false
  }
}

const openDownloadPage = (): void => {
  if (updateInfo.value?.downloadUrl) window.open(updateInfo.value.downloadUrl, '_blank')
}

onMounted(() => {
  themeMode.value = getStoredThemeMode()
  notificationEnabled.value = isNotificationEnabled()
  notifPermission.value = typeof Notification !== 'undefined' ? Notification.permission : 'default'
  fetchAccountInfo()
  fetchCacheStatus()
  fetchUpdateStatus()
  fetchSvcStatus()
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

.update-actions {
  margin-top: 10px;
  display: flex;
  gap: 8px;
}

.update-notes {
  font-weight: 400;
  word-break: break-all;
  white-space: pre-wrap;
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
