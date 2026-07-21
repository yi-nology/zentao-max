<template>
  <div class="page-container">
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
          <el-date-picker v-model="filterForm.dateRange" type="daterange" range-separator="至" start-placeholder="开始日期" end-placeholder="结束日期" style="width: 240px" />
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
        <div class="header-actions">
          <el-button type="primary" size="small" @click="handleViewDetails" :disabled="selectedStories.length === 0">查看详情</el-button>
          <el-dropdown split-button type="success" size="small" @click="handleExport('excel')" @command="handleExport" :disabled="selectedStories.length === 0">
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
      <el-table v-loading="loading" :data="filteredStoryList" border stripe style="width: 100%" @select="handleSelect" @select-all="handleSelectAll">
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
      loading.value = false
      return
    }
    const params = { page: pagination.page, pageSize: pagination.pageSize, productId: effectiveProductId, projectId: effectiveProjectId, assignedTo: filterForm.assignedTo, startDate: filterForm.dateRange[0] || '', endDate: filterForm.dateRange[1] || '', specificDate: filterForm.specificDate }
    const res = await getStories(params)
    storyList.value = res.data.list || []
    pagination.total = res.data.total || 0
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
  if (selectedStories.value.length === 0) return
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
  try {
    await exportData(timestampedFilename('需求列表'), selectedStories.value, cols, format, { title: '需求列表' })
    ElMessage.success(`导出 ${selectedStories.value.length} 个需求成功`)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '导出失败'
    ElMessage.error(msg)
  }
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
.story-title { color: var(--color-primary); text-decoration: none; cursor: pointer; transition: color var(--transition-fast); }
.story-title:hover { text-decoration: underline; color: var(--color-primary-hover); }
.story-detail { line-height: 1.6; padding: 8px; }
.story-detail :deep(.el-descriptions__label) { font-weight: 600; color: var(--color-text-primary); }
</style>
