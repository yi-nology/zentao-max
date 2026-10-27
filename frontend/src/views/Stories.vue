<template>
  <div class="page-container">
    <!-- Stats Summary：分段式统计卡，点击段落即按状态筛选 -->
    <div class="stats-bar">
      <button type="button" class="stat-seg" :class="{ active: filterForm.status === '' }" title="点击清除状态筛选" @click="setStatusFilter('')">
        <span class="stat-seg-value">{{ pagination.total }}</span>
        <span class="stat-seg-label">总计</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--draft" :class="{ active: filterForm.status === 'draft' }" title="点击只看草稿" @click="setStatusFilter('draft')">
        <span class="stat-seg-value">{{ statusCounts.draft }}</span>
        <span class="stat-seg-label">草稿</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--active" :class="{ active: filterForm.status === 'active' }" title="点击只看激活" @click="setStatusFilter('active')">
        <span class="stat-seg-value">{{ statusCounts.active }}</span>
        <span class="stat-seg-label">激活</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--reviewing" :class="{ active: filterForm.status === 'reviewing' }" title="点击只看评审中" @click="setStatusFilter('reviewing')">
        <span class="stat-seg-value">{{ statusCounts.reviewing }}</span>
        <span class="stat-seg-label">评审中</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--changed" :class="{ active: filterForm.status === 'changed' }" title="点击只看已变更" @click="setStatusFilter('changed')">
        <span class="stat-seg-value">{{ statusCounts.changed }}</span>
        <span class="stat-seg-label">已变更</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--closed" :class="{ active: filterForm.status === 'closed' }" title="点击只看已关闭" @click="setStatusFilter('closed')">
        <span class="stat-seg-value">{{ statusCounts.closed }}</span>
        <span class="stat-seg-label">已关闭</span>
      </button>
    </div>

    <div class="filter-card">
      <el-form :inline="true" :model="filterForm" class="filter-form">
        <el-form-item label="产品">
          <el-select v-model="filterForm.productId" placeholder="请选择产品" clearable filterable style="width: 200px" @change="handleProductChange">
            <el-option v-for="p in productOptions" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="指派人">
          <el-select v-model="filterForm.assignedTo" placeholder="请选择指派人" clearable filterable style="width: 160px">
            <el-option v-for="item in userOptions" :key="item.account" :label="item.realname || item.account" :value="item.account" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filterForm.status" placeholder="请选择状态" clearable style="width: 120px">
            <el-option v-for="item in storyStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-date-picker v-model="filterForm.dateRange" type="daterange" range-separator="至" start-placeholder="开始日期" end-placeholder="结束日期" :shortcuts="dateShortcuts" style="width: 240px" />
        </el-form-item>
        <el-form-item label="具体日期">
          <el-date-picker v-model="filterForm.specificDate" type="date" placeholder="选择日期" style="width: 160px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch"><el-icon><Search /></el-icon>查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="table-card">
      <div class="table-header">
        <span v-if="selectedStories.length > 0">已选择 {{ selectedStories.length }} 个需求</span>
        <span v-else class="result-count">共 {{ pagination.total }} 条</span>
        <div class="header-actions">
          <el-button type="primary" size="small" @click="handleViewDetails" :disabled="selectedStories.length === 0">查看详情</el-button>
          <span v-if="selectedStories.length > 0 || pagination.total > 0" class="export-hint">{{ selectedStories.length > 0 ? `导出已选的 ${selectedStories.length} 条` : `导出当前筛选的全部 ${pagination.total} 条` }}</span>
          <el-dropdown split-button type="success" size="small" :loading="exporting" :disabled="selectedStories.length === 0 && pagination.total === 0" @click="handleExport('excel')" @command="handleExport">
            导出 Excel
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="excel">导出 Excel (.xlsx)</el-dropdown-item>
                <el-dropdown-item command="csv">导出 CSV (.csv)</el-dropdown-item>
                <el-dropdown-item command="pdf">导出 PDF (.pdf)</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
      <el-table v-loading="loading" :data="filteredStoryList" border stripe style="width: 100%" :empty-text="listEmptyText" @select="handleSelect" @select-all="handleSelectAll" @row-click="handleRowClick">
        <el-table-column type="selection" width="55" />
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <a href="javascript:void(0)" @click="openZentaoLink(buildZentaoUrl(`story-view-${row.id}.html`))" class="story-title">{{ row.title }}</a>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">{{ getStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="pri" label="优先级" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="getPriorityType(row.pri)">{{ row.pri }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="stage" label="阶段" width="100" align="center">
          <template #default="{ row }">{{ getStageLabel(row.stage) }}</template>
        </el-table-column>
        <el-table-column prop="assignedTo" label="指派人" width="100" align="center">
          <template #default="{ row }">{{ row.assignedTo?.realname || row.assignedTo?.account || row.assignedTo || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleViewDetail(row)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-wrapper">
        <el-pagination v-model:current-page="pagination.page" v-model:page-size="pagination.pageSize" :page-sizes="[10, 20, 50, 100]" :total="pagination.total" layout="total, sizes, prev, pager, next, jumper" @size-change="handleSizeChange" @current-change="handlePageChange" />
      </div>
    </div>

    <el-dialog v-model="detailDialogVisible" title="需求详情" width="80%" destroy-on-close>
      <div v-if="currentStory" class="story-detail">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="ID">{{ currentStory.id }}</el-descriptions-item>
          <el-descriptions-item label="标题">{{ currentStory.title }}</el-descriptions-item>
          <el-descriptions-item label="产品">{{ (currentStory.product as unknown as { name?: string })?.name }}</el-descriptions-item>
          <el-descriptions-item label="项目">{{ (currentStory as unknown as { project?: { name?: string } }).project?.name }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ getStatusLabel(currentStory.status) }}</el-descriptions-item>
          <el-descriptions-item label="阶段">{{ getStageLabel(currentStory.stage) }}</el-descriptions-item>
          <el-descriptions-item label="优先级">{{ currentStory.pri }}</el-descriptions-item>
          <el-descriptions-item label="指派人">{{ currentStory.assignedTo?.realname || currentStory.assignedTo?.account || '-' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ currentStory.openedDate }}</el-descriptions-item>
          <el-descriptions-item label="描述" :span="2"><div v-html="sanitizeHtml(currentStory.spec)"></div></el-descriptions-item>
        </el-descriptions>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, inject, watch, computed } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { sanitizeHtml } from '@/utils/sanitize'
import { getStories, getUsers, getProducts, getStoryStatusOptions } from '@/api/zentao'
import { useZentaoConfig } from '@/composables/useZentaoConfig'
import type { Story, User } from '@/types/api'
import { useRoute, useRouter } from 'vue-router'

interface GlobalSelection { product: number | null; project: number | null; execution: number | null }
interface ProductOption { id: number; name: string }
interface FilterForm { productId: number | null; assignedTo: string; status: string; dateRange: [string, string] | []; specificDate: string }
interface Pagination { page: number; pageSize: number; total: number }

const globalSelection = inject<GlobalSelection>('globalSelection')!
const route = useRoute()
const router = useRouter()
const { buildUrl: buildZentaoUrl } = useZentaoConfig()
// productId 优先级：filterForm 选的产品 > 顶部 globalSelection 选的产品
const filterForm = reactive<FilterForm>({ productId: null, assignedTo: '', status: '', dateRange: [], specificDate: '' })
const productOptions = ref<ProductOption[]>([])
const userOptions = ref<User[]>([])
const storyStatusOptions = ref(getStoryStatusOptions())
const storyList = ref<Story[]>([])
const loading = ref<boolean>(false)
const selectedStories = ref<Story[]>([])
const detailDialogVisible = ref<boolean>(false)
const currentStory = ref<Story | null>(null)
const pagination = reactive<Pagination>({ page: 1, pageSize: 20, total: 0 })

// 统计分段卡：服务端按全量（分页前）统计，避免随翻页跳变。
// max 后端 status 过滤在客户端进行，统计口径为"除状态外筛选后的全量"
const statusCounts = ref<Record<'draft' | 'active' | 'reviewing' | 'changed' | 'closed', number>>({ draft: 0, active: 0, reviewing: 0, changed: 0, closed: 0 })
const exporting = ref<boolean>(false)
// 导出走全量分页拉取（后端单页上限 100）
const EXPORT_PAGE_SIZE = 100

const dateShortcuts = [
  { text: '近7天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 6); return [start, end] } },
  { text: '近30天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 29); return [start, end] } },
  { text: '近90天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 89); return [start, end] } }
]

// 未选产品/项目时不发请求，页面展示引导文案
const hasScope = (): boolean => !!(globalSelection.product || globalSelection.project || filterForm.productId)

const listEmptyText = computed(() => (hasScope() ? '暂无数据' : '请先在顶部选择产品或项目'))

// 点击统计段落 = 快捷设置状态筛选（总计 = 清除状态）。
// max 的状态过滤在客户端 computed 完成，无需重新拉取
const setStatusFilter = (status: string): void => {
  if (filterForm.status === status) return
  filterForm.status = status
  syncRoute()
}

// 点击行任意处打开详情（标题链接、勾选框、按钮除外）
const handleRowClick = (row: Story, _column: unknown, event?: Event): void => {
  const target = event?.target as HTMLElement | null
  if (target?.closest('a, button, label, .el-checkbox')) return
  handleViewDetail(row)
}

const fetchUsers = async (): Promise<void> => {
  try { userOptions.value = (await getUsers()) || [] } catch (error) { console.error('获取用户列表失败:', error) }
}

const fetchProducts = async (): Promise<void> => {
  try {
    const res = await getProducts()
    const list = (res && Array.isArray(res.data)) ? res.data : []
    productOptions.value = list.map((p: { id: number; name: string }) => ({ id: p.id, name: p.name }))
    // 页面首次加载时，如果 filterForm.productId 还没设，用 globalSelection.product 兜底
    if (filterForm.productId === null && globalSelection.product !== null) {
      filterForm.productId = globalSelection.product
    }
  } catch (error) { console.error('获取产品列表失败:', error) }
}

const handleProductChange = (newProductId: number | null): void => {
  // 在页内选 product 时也同步到 globalSelection，让顶部和其他页面跟着变
  if (newProductId !== null) {
    globalSelection.product = newProductId
  }
  pagination.page = 1
  storyList.value = []
  pagination.total = 0
  if (newProductId !== null) {
    fetchStories()
  }
}

const fetchStories = async (): Promise<void> => {
  loading.value = true
  try {
    // productId 优先级：filterForm.productId > globalSelection.product
    const effectiveProductId = filterForm.productId ?? globalSelection.product ?? undefined
    const effectiveProjectId = globalSelection.project ?? undefined
    if (effectiveProductId === undefined && effectiveProjectId === undefined) {
      // 没有 product 也没 project：清空 list，不调 API，让用户选
      storyList.value = []
      pagination.total = 0
      statusCounts.value = { draft: 0, active: 0, reviewing: 0, changed: 0, closed: 0 }
      loading.value = false
      return
    }
    const params = { page: pagination.page, pageSize: pagination.pageSize, productId: effectiveProductId, projectId: effectiveProjectId, assignedTo: filterForm.assignedTo, startDate: filterForm.dateRange[0] || '', endDate: filterForm.dateRange[1] || '', specificDate: filterForm.specificDate }
    const res = await getStories(params)
    storyList.value = res.data.list || []
    pagination.total = res.data.total || 0
    // 统计卡为服务端按全量（分页前）统计，避免随翻页跳变
    if (res.data.statusCounts) {
      statusCounts.value = {
        draft: res.data.statusCounts.draft ?? 0,
        active: res.data.statusCounts.active ?? 0,
        reviewing: res.data.statusCounts.reviewing ?? 0,
        changed: res.data.statusCounts.changed ?? 0,
        closed: res.data.statusCounts.closed ?? 0
      }
    }
  } catch (error) { console.error('获取需求列表失败:', error); ElMessage.error('获取需求列表失败') } finally { loading.value = false }
}

watch(() => globalSelection.product, (newProduct: number | null) => {
  // 顶部 product 变了 → 同步到 filterForm
  if (newProduct !== null && filterForm.productId === null) {
    filterForm.productId = newProduct
  }
  if (newProduct) { pagination.page = 1; fetchStories() } else if (filterForm.productId === null) { storyList.value = []; pagination.total = 0 }
}, { immediate: true })

watch(() => globalSelection.project, () => { if (globalSelection.product || filterForm.productId) { pagination.page = 1; fetchStories() } })

const filteredStoryList = computed(() => {
  return storyList.value.filter((story: Story) => {
    if (filterForm.status && story.status !== filterForm.status) return false
    return true
  })
})

const handleSearch = (): void => {
  if (!globalSelection.product && !globalSelection.project) { ElMessage.warning('请先在顶部选择产品或项目'); return }
  pagination.page = 1; syncRoute(); fetchStories()
}
const handleReset = (): void => { filterForm.assignedTo = ''; filterForm.status = ''; filterForm.dateRange = []; filterForm.specificDate = ''; pagination.page = 1; syncRoute(); fetchStories() }
const handleSizeChange = (size: number): void => { pagination.pageSize = size; pagination.page = 1; syncRoute(); fetchStories() }
const handlePageChange = (page: number): void => { pagination.page = page; syncRoute(); fetchStories() }

const syncRoute = (): void => {
  const q: Record<string, string> = {}
  if (filterForm.assignedTo) q.assignedTo = filterForm.assignedTo
  if (filterForm.status) q.status = filterForm.status
  if (filterForm.dateRange[0]) q.startDate = filterForm.dateRange[0]
  if (filterForm.dateRange[1]) q.endDate = filterForm.dateRange[1]
  if (filterForm.specificDate) q.specificDate = filterForm.specificDate
  if (pagination.page > 1) q.page = String(pagination.page)
  if (pagination.pageSize !== 20) q.pageSize = String(pagination.pageSize)
  router.replace({ query: q })
}
const getStatusType = (status: string): string => ({ draft: 'info', active: 'success', changed: 'warning', closed: 'info' }[status] || 'info')
const getStatusLabel = (status: string): string => ({ draft: '草稿', active: '激活', changed: '已变更', closed: '已关闭' }[status] || status)
const getPriorityType = (pri: number): string => (pri === 1 ? 'danger' : pri === 2 ? 'warning' : pri === 3 ? 'primary' : 'info')
const getStageLabel = (stage: string): string => ({ wait: '等待', planned: '已计划', projected: '已立项', developing: '研发中', developed: '研发完毕', testing: '测试中', tested: '测试完毕', verified: '已验收', released: '已发布' }[stage] || stage)
const handleSelect = (selection: Story[]): void => { selectedStories.value = selection }
const handleSelectAll = (selection: Story[]): void => { selectedStories.value = selection }
const handleViewDetails = (): void => { if (selectedStories.value.length > 0) { currentStory.value = selectedStories.value[0]; detailDialogVisible.value = true } }
const handleViewDetail = (row: Story): void => { currentStory.value = row; detailDialogVisible.value = true }

const handleExport = async (format: 'excel' | 'csv' | 'pdf'): Promise<void> => {
  if (exporting.value) return
  if (selectedStories.value.length === 0 && pagination.total === 0) return
  const { exportData, timestampedFilename } = await import('@/utils/export')
  type ExportColumn<T> = import('@/utils/export').ExportColumn<T>
  const cols: ExportColumn<Story>[] = [
    { header: 'ID', access: s => s.id },
    { header: '标题', access: s => s.title },
    { header: '状态', access: s => getStatusLabel(s.status) },
    { header: '阶段', access: s => getStageLabel(s.stage) },
    { header: '优先级', access: s => s.pri },
    { header: '指派人', access: s => s.assignedTo?.realname || s.assignedTo?.account || '' }
  ]
  exporting.value = true
  try {
    // 导出：有勾选导勾选，无勾选导当前筛选全量（而非仅当前页）
    let list: Story[] = selectedStories.value
    if (list.length === 0) {
      const baseParams = {
        productId: filterForm.productId ?? globalSelection.product ?? undefined,
        projectId: globalSelection.project ?? undefined,
        assignedTo: filterForm.assignedTo,
        startDate: filterForm.dateRange[0] || '',
        endDate: filterForm.dateRange[1] || '',
        specificDate: filterForm.specificDate
      }
      const pageCount = Math.ceil(pagination.total / EXPORT_PAGE_SIZE)
      list = []
      for (let p = 1; p <= pageCount; p++) {
        const res = await getStories({ ...baseParams, page: p, pageSize: EXPORT_PAGE_SIZE })
        const pageList = res.data.list || []
        list.push(...pageList)
        if (pageList.length < EXPORT_PAGE_SIZE) break
      }
    }
    if (list.length === 0) { ElMessage.warning('没有可导出的数据'); return }
    await exportData(timestampedFilename('需求列表'), list, cols, format, { title: '需求列表' })
    if (list === selectedStories.value) ElMessage.success(`导出已选 ${list.length} 个需求成功`)
    else if (list.length < pagination.total) ElMessage.warning(`筛选结果 ${pagination.total} 条，已导出前 ${list.length} 条`)
    else ElMessage.success(`已导出全部 ${list.length} 个需求`)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '导出失败'
    ElMessage.error(msg)
  } finally { exporting.value = false }
}

const openZentaoLink = async (url: string): Promise<void> => {
  if (!url) { ElMessage.warning('禅道地址未配置，请检查系统设置'); return }
  try {
    const { openExternalLink } = await import('@/composables/useExternalLink')
    await openExternalLink(url)
  } catch { window.open(url, '_blank', 'noopener,noreferrer') }
}

onMounted(() => {
  const q = route.query
  if (q.assignedTo) filterForm.assignedTo = String(q.assignedTo)
  if (q.status) filterForm.status = String(q.status)
  if (q.startDate || q.endDate) filterForm.dateRange = [String(q.startDate || ''), String(q.endDate || '')] as [string, string]
  if (q.specificDate) filterForm.specificDate = String(q.specificDate)
  if (q.page) pagination.page = Number(q.page) || 1
  if (q.pageSize) pagination.pageSize = Number(q.pageSize) || 20

  fetchProducts()
  fetchUsers()
})
</script>

<style scoped>
/* Stats Bar：单卡片分段式统计，段落可点击筛选 */
.stats-bar {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-sm) var(--space-md);
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  margin-bottom: var(--space-md);
  flex-wrap: wrap;
}

.stat-seg {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 6px 22px;
  border: none;
  background: transparent;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font: inherit;
  transition: background var(--transition-fast);
}

.stat-seg:hover {
  background: var(--color-bg-hover);
}

.stat-seg.active {
  background: var(--color-primary-light);
}

.stat-seg-value {
  font-size: 20px;
  font-weight: 700;
  line-height: 1.3;
  color: var(--color-text-primary);
}

.stat-seg-label {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.stat-seg--draft .stat-seg-value { color: var(--color-text-tertiary); }
.stat-seg--active .stat-seg-value { color: var(--color-success); }
.stat-seg--reviewing .stat-seg-value { color: var(--color-warning, #e6a23c); }
.stat-seg--changed .stat-seg-value { color: var(--color-primary); }
.stat-seg--closed .stat-seg-value { color: var(--color-text-tertiary); }

.stat-divider {
  width: 1px;
  height: 28px;
  background: var(--color-border);
}

.result-count {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.export-hint {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-right: 8px;
}

/* 点击行打开详情 */
:deep(.el-table__row) {
  cursor: pointer;
}

.story-title { color: var(--color-primary); text-decoration: none; cursor: pointer; transition: color var(--transition-fast); }
.story-title:hover { text-decoration: underline; color: var(--color-primary-hover); }
.story-detail { line-height: 1.6; padding: 8px; }
.story-detail :deep(.el-descriptions__label) { font-weight: 600; color: var(--color-text-primary); }
</style>
