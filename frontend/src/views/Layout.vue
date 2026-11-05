<template>
  <div class="layout-container">
    <aside class="aside">
      <div class="logo">
        <div class="logo-icon">Z</div>
        <span class="logo-text">禅道 Max</span>
      </div>
      <nav class="nav-menu">
        <router-link
          v-for="item in menuItems"
          :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: $route.path === item.path }"
        >
          <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path :d="item.icon" />
          </svg>
          <span>{{ item.label }}</span>
        </router-link>
      </nav>
      <div class="sidebar-footer">
        <div class="sidebar-auth">
          <template v-if="authStatus.authenticated">
            <span class="sidebar-auth-user" :title="'已登录：' + authStatus.username">{{ authStatus.username }}</span>
            <button class="sidebar-auth-btn" :disabled="loggingOut" @click="handleLogout">
              {{ loggingOut ? '退出中' : '退出' }}
            </button>
          </template>
          <router-link v-else class="sidebar-auth-btn" to="/login">登录</router-link>
        </div>
        <router-link v-if="updateAvailable" to="/settings" class="update-badge" title="前往设置查看更新">
          新版本 {{ updateBadgeVersion }} 可用
        </router-link>
        <span class="version-info">v{{ appVersion }}</span>
        <span>©2024-2026 <a href="https://murphyyi.com" target="_blank" rel="noopener noreferrer">murphyyi.com</a></span>
      </div>
    </aside>
    <div class="main-area">
      <header class="header">
        <h1 class="header-title">{{ pageTitle }}</h1>
        <div class="header-actions">
          <!-- 禅道账号信息仅在平台登录后显示：匿名只读态下易误读为已登录 -->
          <div class="account-info" v-if="authStatus.authenticated && accountInfo">
            <span class="account-status-dot" :class="accountInfo.connected ? 'connected' : 'disconnected'"></span>
            <span class="account-text">{{ accountInfo.account }}</span>
            <span class="account-domain" v-if="accountInfo.domain">@ {{ accountInfo.domain.replace(/^https?:\/\//, '') }}</span>
          </div>
          <GlobalSearch />
          <ProductSelector
            :model-value="globalSelection"
            @update:model-value="handleSelectionChange"
          />
        </div>
      </header>
      <main class="main">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, provide, reactive, ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import GlobalSearch from '@/components/GlobalSearch.vue'
import ProductSelector from '@/components/ProductSelector.vue'
import { getAccountInfo } from '@/api/zentao'
import api from '@/api/api'
import { useDesktopNotification } from '@/composables/useDesktopNotification'
import { useAuth } from '@/composables/useAuth'
import type { ApiResponse, GlobalSelection } from '@/types/api'

// 启用桌面通知监听（仅在 Wails 模式生效）
useDesktopNotification()

// 平台访问控制：匿名只读，管理员读写
const { authStatus, refreshAuthStatus, logout } = useAuth()
const loggingOut = ref(false)

const handleLogout = async (): Promise<void> => {
  loggingOut.value = true
  try {
    await logout()
  } finally {
    loggingOut.value = false
    router.push('/')
  }
}

interface SelectionChangePayload {
  product: number | null
  project: number | null
}

interface MenuItem {
  path: string
  label: string
  icon: string
}

const route = useRoute()
const router = useRouter()

// globalSelection 持久化到 localStorage：用户选过的 product/project 在刷新页面后保留。
// 跨页面（bugs / stories / tasks / dashboard 等）都用同一个 selection，刷新丢失会
// 让所有列表页清空，必须持久化。优先级：URL query > localStorage > 默认 null。
const GLOBAL_SELECTION_KEY = 'zentao_max:global_selection'
const loadPersistedSelection = (): GlobalSelection => {
  try {
    const raw = localStorage.getItem(GLOBAL_SELECTION_KEY)
    if (!raw) return { product: null, project: null }
    const parsed = JSON.parse(raw) as Partial<GlobalSelection>
    return {
      product: typeof parsed.product === 'number' ? parsed.product : null,
      project: typeof parsed.project === 'number' ? parsed.project : null
    }
  } catch { return { product: null, project: null } }
}
const globalSelection = reactive<GlobalSelection>(loadPersistedSelection())
const appVersion = ref('...')
const accountInfo = ref<{ domain: string; account: string; connected: boolean } | null>(null)

// Search state

// ─── 新版本角标：拉取版本检查状态（静默失败即可，非关键） ───
const updateAvailable = ref(false)
const updateBadgeVersion = ref('')
const fetchUpdateBadge = async (): Promise<void> => {
  try {
    const res = await api.get('/update/status') as { data?: { hasUpdate?: boolean; latest?: string } }
    if (res?.data?.hasUpdate && res.data.latest) {
      updateAvailable.value = true
      updateBadgeVersion.value = res.data.latest
    }
  } catch { /* 忽略 */ }
}

onMounted(async () => {
  fetchUpdateBadge()

  const qProduct = route.query.product
  const qProject = route.query.project
  if (qProduct) globalSelection.product = Number(qProduct)
  if (qProject) globalSelection.project = Number(qProject)

  try {
    const res = await api.get('/version') as ApiResponse<{ version: string }>
    if (res?.data?.version) appVersion.value = res.data.version
  } catch { appVersion.value = 'dev' }

  try {
    const res = await getAccountInfo()
    if (res.code === 200 && res.data) {
      accountInfo.value = res.data
    }
  } catch { /* ignore */ }

  refreshAuthStatus()
})

const menuItems: MenuItem[] = [
  { path: '/dashboard', label: '仪表盘', icon: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z' },
  { path: '/bugs', label: 'Bug 查询', icon: 'M12 22C6.477 22 2 17.523 2 12S6.477 2 12 2s10 4.477 10 10-4.477 10-10 10zm0-2a8 8 0 100-16 8 8 0 000 16zm-1-5h2v2h-2v-2zm0-8h2v6h-2V7z' },
  { path: '/stories', label: '需求查询', icon: 'M4 5a2 2 0 012-2h12a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V5zm4 0v4h8V5H8zm0 6v4h8v-4H8zm0 6v2h5v-2H8z' },
  { path: '/tasks', label: '任务查询', icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4' },
  { path: '/cases', label: '测试用例', icon: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z' },
  { path: '/timelog', label: '工时统计', icon: 'M12 22C6.477 22 2 17.523 2 12S6.477 2 12 2s10 4.477 10 10-4.477 10-10 10zm0-2a8 8 0 100-16 8 8 0 000 16zm1-13h-2v6l5.25 3.15.75-1.23-4-2.42V7z' },
  { path: '/effort-calendar', label: '工时日历', icon: 'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z' },
  { path: '/scheduler', label: '定时任务', icon: 'M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z' },
  { path: '/health-check', label: '心跳检测', icon: 'M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z' },
  { path: '/logs', label: '系统日志', icon: 'M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
  { path: '/mcp-guide', label: 'MCP 对接', icon: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z' },
  { path: '/settings', label: '设置', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z' },
]

const pageTitle = computed<string>(() => {
  const current = menuItems.find(item => item.path === route.path)
  return current?.label || '禅道 Max'
})

const handleSelectionChange = (selection: SelectionChangePayload): void => {
  globalSelection.product = selection.product
  globalSelection.project = selection.project
}

watch(() => [globalSelection.product, globalSelection.project], () => {
  // 1. 持久化到 localStorage（页面刷新后保留）
  try {
    const payload: Partial<GlobalSelection> = {}
    if (globalSelection.product !== null) payload.product = globalSelection.product
    if (globalSelection.project !== null) payload.project = globalSelection.project
    if (Object.keys(payload).length > 0) {
      localStorage.setItem(GLOBAL_SELECTION_KEY, JSON.stringify(payload))
    } else {
      localStorage.removeItem(GLOBAL_SELECTION_KEY)
    }
  } catch { /* localStorage 不可用时静默 */ }

  // 2. 同步到 URL query
  const childQuery = { ...route.query }
  delete childQuery.product
  delete childQuery.project
  const merged = { ...childQuery } as Record<string, string | string[]>
  if (globalSelection.product) merged.product = String(globalSelection.product)
  if (globalSelection.project) merged.project = String(globalSelection.project)
  router.replace({ query: merged })
})

provide<GlobalSelection>('globalSelection', globalSelection)
</script>

<style scoped>
.layout-container {
  height: 100vh;
  display: flex;
  overflow: hidden;
  background-color: var(--color-bg);
}

/* Sidebar */
.aside {
  width: 220px;
  flex-shrink: 0;
  background-color: var(--color-sidebar);
  display: flex;
  flex-direction: column;
  transition: width 0.3s ease;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 20px;
  border-bottom: 1px solid #334155;
}

.logo-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  background-color: var(--color-primary);
  color: var(--color-text-on-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  font-weight: 700;
  font-family: var(--font-heading);
}

.logo-text {
  color: var(--color-text-on-dark);
  font-size: 16px;
  font-weight: 600;
  font-family: var(--font-heading);
}

/* Navigation */
.nav-menu {
  flex: 1;
  padding: 12px 10px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow-y: auto;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-radius: var(--radius-sm);
  color: var(--color-text-on-dark);
  text-decoration: none;
  font-size: 14px;
  font-weight: 400;
  transition: all var(--transition-fast);
  opacity: 0.7;
}

.nav-item:hover {
  background-color: var(--color-sidebar-hover);
  opacity: 1;
}

.nav-item.active {
  background-color: var(--color-sidebar-active);
  color: var(--color-text-on-primary);
  opacity: 1;
}

.nav-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

/* Sidebar Footer */
.update-badge {
  display: inline-block;
  padding: 3px 10px;
  margin-bottom: 6px;
  background: #fef3c7;
  border: 1px solid #fcd34d;
  border-radius: 10px;
  font-size: 11px;
  color: #92400e;
  text-decoration: none;
}
:global(.dark) .update-badge {
  background: #451a03;
  border-color: #78350f;
  color: #fcd34d;
}

.sidebar-footer {
  padding: 12px 16px;
  border-top: 1px solid #334155;
  text-align: center;
  font-size: 11px;
  color: var(--color-text-on-dark);
  opacity: 0.5;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.version-info {
  font-family: monospace;
  font-size: 11px;
  opacity: 0.7;
}

.sidebar-footer a {
  color: var(--color-text-on-dark);
  text-decoration: none;
  transition: opacity 0.2s;
}

.sidebar-footer a:hover {
  opacity: 1;
  text-decoration: underline;
}

/* Main Area */
.main-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Header */
.header {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 28px;
  background-color: var(--color-bg-card);
  border-bottom: 1px solid var(--color-border-light);
  flex-shrink: 0;
}

.header-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  font-family: var(--font-heading);
  white-space: nowrap;
  flex-shrink: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.auth-area {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 登录者用户名：安静文本展示，不做徽章（仅头部禅道账号区旁不用；登录态展示移至侧边栏底部） */
.auth-user {
  font-size: 12px;
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.sidebar-auth {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding-top: 8px;
  margin-bottom: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.sidebar-auth-user {
  font-size: 12px;
  color: var(--color-text-on-dark, #e2e8f0);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 120px;
}

.sidebar-auth-btn {
  font-size: 12px;
  line-height: 1;
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: transparent;
  color: var(--color-text-on-dark, #e2e8f0);
  white-space: nowrap;
  cursor: pointer;
  text-decoration: none;
  opacity: 0.8;
  transition: all 0.15s;
}

.sidebar-auth-btn:hover {
  opacity: 1;
  border-color: rgba(255, 255, 255, 0.35);
}

.sidebar-auth-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.account-info {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: var(--radius-sm);
  background-color: var(--color-bg-hover);
  font-size: 12px;
  color: var(--color-text-secondary);
  min-width: 0;
}

.account-status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.account-status-dot.connected {
  background-color: #22C55E;
  box-shadow: 0 0 4px rgba(34, 197, 94, 0.4);
}

.account-status-dot.disconnected {
  background-color: #EF4444;
  box-shadow: 0 0 4px rgba(239, 68, 68, 0.4);
}

.account-text {
  color: var(--color-text-primary);
  font-weight: 500;
  white-space: nowrap;
}

.account-domain {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 160px;
  color: var(--color-text-tertiary);
}

.search-hints kbd {
  display: inline-block;
  padding: 1px 6px;
  margin-right: 4px;
  background: var(--color-bg-hover);
  border: 1px solid var(--color-border);
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 10px;
  color: var(--color-text-secondary);
}

.search-page-info {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

/* Content */
.main {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-lg);
  background-color: var(--color-bg);
}

/* Responsive */
@media screen and (max-width: 1500px) {
  /* 依次让渡次要信息，避免到处出现省略号：先隐藏服务器域名，再缩搜索框 */
  .account-domain {
    display: none;
  }

  .search-input {
    width: 240px;
  }
}

@media screen and (max-width: 1240px) {
  .search-input {
    width: 200px;
  }

  .header-actions :deep(.el-select) {
    width: 170px !important;
  }
}

@media screen and (max-width: 768px) {
  .aside {
    width: 60px;
  }

  .logo-text {
    display: none;
  }

  .nav-item span {
    display: none;
  }

  .nav-item {
    justify-content: center;
    padding: 12px;
  }

  .header {
    padding: 0 16px;
  }

  .search-input {
    width: 160px;
  }

  .search-dropdown {
    width: 320px;
  }
}
</style>
