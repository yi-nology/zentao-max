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
        <span class="version-info">v{{ appVersion }}</span>
        <span>©2024-2026 <a href="https://murphyyi.com" target="_blank" rel="noopener noreferrer">murphyyi.com</a></span>
      </div>
    </aside>
    <div class="main-area">
      <header class="header">
        <h1 class="header-title">{{ pageTitle }}</h1>
        <div class="header-actions">
          <div class="auth-area">
            <template v-if="authStatus.authenticated">
              <span class="auth-user">{{ authStatus.username }}</span>
              <button class="auth-btn" :disabled="loggingOut" @click="handleLogout">
                {{ loggingOut ? '退出中...' : '退出' }}
              </button>
            </template>
            <template v-else>
              <router-link class="auth-btn" to="/login">登录</router-link>
            </template>
          </div>
          <!-- 禅道账号信息仅在平台登录后显示：匿名只读态下与"登录"按钮并存易误读为已登录 -->
          <div class="account-info" v-if="authStatus.authenticated && accountInfo">
            <span class="account-status-dot" :class="accountInfo.connected ? 'connected' : 'disconnected'"></span>
            <span class="account-text">{{ accountInfo.account }}</span>
            <span class="account-domain" v-if="accountInfo.domain">@ {{ accountInfo.domain.replace(/^https?:\/\//, '') }}</span>
          </div>
          <div class="search-wrapper" ref="searchWrapperRef">
            <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="11" cy="11" r="8" />
              <path d="m21 21-4.35-4.35" />
            </svg>
            <input
              class="search-input"
              type="text"
              name="global-search"
              placeholder="搜索 Bug / 需求 / 任务…"
              title="快捷键 Ctrl+K 聚焦搜索"
              v-model="searchKeyword"
              @focus="onSearchFocus"
              @keydown.escape="closeSearch"
              @keydown.down.prevent="highlightNext"
              @keydown.up.prevent="highlightPrev"
              @keydown.enter.prevent="enterHighlighted"
              @keydown.page-down.prevent="searchNextPage"
              @keydown.page-up.prevent="searchPrevPage"
            />
            <!-- Search Results Dropdown -->
            <div v-if="searchOpen" class="search-dropdown">
              <div v-if="searchLoading" class="search-loading">搜索中...</div>
              <div v-else-if="searchItems.length === 0 && searchKeyword.trim()" class="search-empty">未找到相关内容</div>
              <template v-else>
                <div
                  v-for="group in groupedResults"
                  :key="group.type"
                  class="search-group"
                >
                  <div class="search-group-header">
                    <span class="search-group-icon" :style="{ backgroundColor: group.color }">{{ group.icon }}</span>
                    <span class="search-group-label">{{ group.label }}</span>
                    <span class="search-group-count">{{ group.items.length }}</span>
                  </div>
                  <div
                    v-for="item in group.items"
                    :key="item.type + '-' + item.id"
                    class="search-result-item"
                    :class="{ highlighted: flatResults.indexOf(item) === searchHighlightIndex }"
                    @click="navigateTo(item)"
                  >
                    <span class="search-item-type" :style="{ backgroundColor: group.color + '20', color: group.color }">
                      {{ typeLabel(item.type) }} #{{ item.id }}
                    </span>
                    <span class="search-item-title">{{ item.title }}</span>
                    <span class="search-item-status" :class="'status-' + item.status">{{ item.status }}</span>
                  </div>
                </div>
                <!-- Pagination -->
                <div v-if="searchTotal > searchPageSize" class="search-pagination">
                  <button
                    class="search-page-btn"
                    :disabled="searchPage <= 1"
                    @click="searchPrevPage"
                  >上一页</button>
                  <span class="search-page-info">{{ searchPage }} / {{ totalPages }}</span>
                  <button
                    class="search-page-btn"
                    :disabled="searchPage >= totalPages"
                    @click="searchNextPage"
                  >下一页</button>
                </div>
                <!-- 快捷键提示条 -->
                <div class="search-hints">
                  <span><kbd>↑↓</kbd>选择</span>
                  <span><kbd>Enter</kbd>跳转</span>
                  <span><kbd>PgUp/PgDn</kbd>翻页</span>
                  <span><kbd>Esc</kbd>关闭</span>
                </div>
              </template>
            </div>
          </div>
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
import { computed, provide, reactive, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ProductSelector from '@/components/ProductSelector.vue'
import { search, getAccountInfo } from '@/api/zentao'
import api from '@/api/api'
import { useDesktopNotification } from '@/composables/useDesktopNotification'
import { useAuth } from '@/composables/useAuth'
import type { SearchItem, ApiResponse } from '@/types/api'

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

interface GlobalSelection {
  product: number | null
  project: number | null
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

interface ResultGroup {
  type: string
  label: string
  icon: string
  color: string
  items: SearchItem[]
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
const searchKeyword = ref('')
const searchOpen = ref(false)
const searchLoading = ref(false)
const searchItems = ref<SearchItem[]>([])
const searchTotal = ref(0)
const searchPage = ref(1)
const searchPageSize = 20
const searchHighlightIndex = ref(-1)
const searchWrapperRef = ref<HTMLElement | null>(null)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

onMounted(async () => {
  document.addEventListener('keydown', onGlobalKeydown)

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

  document.addEventListener('click', onDocClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onGlobalKeydown)
  if (debounceTimer) clearTimeout(debounceTimer)
})

const onDocClick = (e: MouseEvent) => {
  if (searchWrapperRef.value && !searchWrapperRef.value.contains(e.target as Node)) {
    closeSearch()
  }
}

const onGlobalKeydown = (e: KeyboardEvent) => {
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault()
    const input = searchWrapperRef.value?.querySelector('input') as HTMLInputElement | null
    if (input) {
      input.focus()
      searchOpen.value = true
    }
  }
}

const onSearchFocus = () => {
  searchOpen.value = true
  searchHighlightIndex.value = -1
}

const closeSearch = () => {
  searchOpen.value = false
}

const doSearch = async () => {
  const kw = searchKeyword.value.trim()
  if (!kw) {
    searchItems.value = []
    searchTotal.value = 0
    return
  }
  searchLoading.value = true
  searchOpen.value = true
  try {
    const productId = globalSelection.product ?? undefined
    const res = await search({
      keyword: kw,
      productId,
      page: searchPage.value,
      pageSize: searchPageSize
    })
    const data = res.data
    searchItems.value = data.items || []
    searchTotal.value = data.total || 0
  } catch {
    searchItems.value = []
    searchTotal.value = 0
  } finally {
    searchLoading.value = false
  }
}

watch(searchKeyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    searchPage.value = 1
    doSearch()
  }, 300)
})

const totalPages = computed(() => Math.ceil(searchTotal.value / searchPageSize))

const flatResults = computed(() => {
  const items: SearchItem[] = []
  groupedResults.value.forEach(g => items.push(...g.items))
  return items
})

const highlightNext = () => {
  if (flatResults.value.length === 0) return
  searchHighlightIndex.value = Math.min(searchHighlightIndex.value + 1, flatResults.value.length - 1)
}

const highlightPrev = () => {
  searchHighlightIndex.value = Math.max(searchHighlightIndex.value - 1, -1)
}

const enterHighlighted = () => {
  if (searchHighlightIndex.value >= 0 && searchHighlightIndex.value < flatResults.value.length) {
    navigateTo(flatResults.value[searchHighlightIndex.value])
  }
}

// 翻页：上一页（PgUp 或点击）
const searchPrevPage = () => {
  if (searchPage.value > 1) {
    searchPage.value--
    searchHighlightIndex.value = -1
    doSearch()
  }
}

// 翻页：下一页（PgDn 或点击）
const searchNextPage = () => {
  if (searchPage.value < totalPages.value) {
    searchPage.value++
    searchHighlightIndex.value = -1
    doSearch()
  }
}

const groupedResults = computed<ResultGroup[]>(() => {
  const groups: ResultGroup[] = [
    { type: 'bug', label: 'Bug', icon: '🐛', color: '#EF4444', items: [] },
    { type: 'story', label: '需求', icon: '📋', color: '#4F6BF6', items: [] },
    { type: 'task', label: '任务', icon: '✅', color: '#22C55E', items: [] }
  ]
  const map: Record<string, ResultGroup> = { bug: groups[0], story: groups[1], task: groups[2] }
  for (const item of searchItems.value) {
    const g = map[item.type]
    if (g) g.items.push(item)
  }
  return groups.filter(g => g.items.length > 0)
})

const typeLabel = (type: string) => {
  const m: Record<string, string> = { bug: 'Bug', story: '需求', task: '任务' }
  return m[type] || type
}

const navigateTo = (item: SearchItem) => {
  const routeMap: Record<string, string> = { bug: '/bugs', story: '/stories', task: '/tasks' }
  const path = routeMap[item.type]
  if (path) {
    router.push({ path, query: { id: String(item.id) } })
  }
  closeSearch()
}

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
  { path: '/init-guide', label: '重新初始化', icon: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15' }
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

/* 登录者用户名：安静文本展示，不做徽章 */
.auth-user {
  font-size: 12px;
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.auth-btn {
  font-size: 12px;
  line-height: 1;
  padding: 6px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  white-space: nowrap;
  flex-shrink: 0;
  cursor: pointer;
  text-decoration: none;
  transition: all 0.15s;
}

.auth-btn:hover {
  color: var(--color-primary, #4f6ef7);
  border-color: var(--color-primary, #4f6ef7);
}

.auth-btn:disabled {
  opacity: 0.6;
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

/* Search */
.search-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 10px;
  width: 16px;
  height: 16px;
  color: var(--color-text-tertiary);
  pointer-events: none;
  z-index: 1;
}

.search-input {
  width: 260px;
  height: 34px;
  padding: 0 12px 0 32px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-size: 13px;
  color: var(--color-text-primary);
  background-color: var(--color-bg);
  outline: none;
  transition: all var(--transition-fast);
}

.search-input:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(79, 107, 246, 0.12);
}

.search-input::placeholder {
  color: var(--color-text-tertiary);
}

.search-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  width: 480px;
  max-height: 420px;
  overflow-y: auto;
  background-color: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg);
  z-index: 100;
  padding: 8px 0;
}

.search-loading,
.search-empty {
  padding: 24px;
  text-align: center;
  color: var(--color-text-tertiary);
  font-size: 13px;
}

.search-group-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.search-group-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 4px;
  font-size: 11px;
  color: #fff;
}

.search-group-count {
  margin-left: auto;
  font-weight: 400;
  color: var(--color-text-tertiary);
}

.search-result-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  cursor: pointer;
  transition: background-color var(--transition-fast);
}

.search-result-item:hover,
.search-result-item.highlighted {
  background: var(--color-bg-hover);
}

.search-item-type {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
}

.search-item-title {
  flex: 1;
  font-size: 13px;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.search-item-status {
  flex-shrink: 0;
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 100px;
  background-color: var(--color-info-light);
  color: var(--color-info);
}

.search-item-status.status-active,
.search-item-status.status-doing {
  background-color: var(--color-primary-light);
  color: var(--color-primary);
}

.search-item-status.status-resolved,
.search-item-status.status-done,
.search-item-status.status-closed {
  background-color: var(--color-success-light);
  color: var(--color-success);
}

.search-item-status.status-draft,
.search-item-status.status-wait {
  background-color: var(--color-info-light);
  color: var(--color-info);
}

.search-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 10px 14px;
  border-top: 1px solid var(--color-border-light);
}

.search-page-btn {
  padding: 4px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.search-page-btn:hover:not(:disabled) {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.search-page-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.search-hints {
  display: flex;
  justify-content: center;
  gap: 12px;
  padding: 6px 14px 10px;
  font-size: 11px;
  color: var(--color-text-tertiary);
  border-top: 1px dashed var(--color-border-light);
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
