<template>
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
</template>

<script setup lang="ts">
import { computed, inject, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { search } from '@/api/zentao'
import type { GlobalSelection, SearchItem } from '@/types/api'

/**
 * 全局搜索（issue #7-3：从 Layout.vue 拆出，208-395 行 + 搜索样式）。
 * Ctrl+K 聚焦 / 点击外部关闭 / 防抖 / 键盘上下翻页选择 / Enter 跳转。
 * 跳转带 ?id=，落地页（Bugs/Stories/Tasks）onMounted 自动打开详情弹窗。
 */

const router = useRouter()
const globalSelection = inject<GlobalSelection>('globalSelection')!

interface ResultGroup {
  type: string
  label: string
  icon: string
  color: string
  items: SearchItem[]
}

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

// 事件监听收敛到组件自身（原在 Layout onMounted/onBeforeUnmount 里注册）
onMounted(() => {
  document.addEventListener('keydown', onGlobalKeydown)
  document.addEventListener('click', onDocClick)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onGlobalKeydown)
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>

<style scoped>
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
</style>
