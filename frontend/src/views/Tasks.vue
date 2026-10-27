<template>
  <div class="page-container">
    <!-- Stats Summary：分段式统计卡，点击段落即按状态筛选 -->
    <div class="stats-bar">
      <button type="button" class="stat-seg" :class="{ active: filterForm.status === '' }" title="点击清除状态筛选" @click="setStatusFilter('')">
        <span class="stat-seg-value">{{ pagination.total }}</span>
        <span class="stat-seg-label">总计</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--doing" :class="{ active: filterForm.status === 'doing' }" title="点击只看进行中" @click="setStatusFilter('doing')">
        <span class="stat-seg-value">{{ statusCounts.doing }}</span>
        <span class="stat-seg-label">进行中</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--wait" :class="{ active: filterForm.status === 'wait' }" title="点击只看待开始" @click="setStatusFilter('wait')">
        <span class="stat-seg-value">{{ statusCounts.wait }}</span>
        <span class="stat-seg-label">待开始</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--done" :class="{ active: filterForm.status === 'done' }" title="点击只看已完成" @click="setStatusFilter('done')">
        <span class="stat-seg-value">{{ statusCounts.done }}</span>
        <span class="stat-seg-label">已完成</span>
      </button>
      <div class="stat-divider" />
      <button type="button" class="stat-seg stat-seg--closed" :class="{ active: filterForm.status === 'closed' }" title="点击只看已关闭" @click="setStatusFilter('closed')">
        <span class="stat-seg-value">{{ statusCounts.closed }}</span>
        <span class="stat-seg-label">已关闭</span>
      </button>
    </div>

    <div class="filter-card">
      <el-form :inline="true" :model="filterForm" class="filter-form">
        <el-form-item label="执行/迭代">
          <el-select v-model="filterForm.execution" placeholder="请选择执行/迭代" clearable style="width: 180px">
            <el-option v-for="item in executionOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="指派人">
          <el-select v-model="filterForm.assignedTo" placeholder="请选择或输入指派人" clearable filterable style="width: 150px">
            <el-option v-for="item in assignedToOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filterForm.status" placeholder="请选择状态" clearable style="width: 130px">
            <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-date-picker v-model="filterForm.dateRange" type="daterange" range-separator="至" start-placeholder="开始日期" end-placeholder="结束日期" :shortcuts="dateShortcuts" style="width: 240px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch"><el-icon><Search /></el-icon>查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="table-card">
      <div class="table-header">
        <span class="result-count">共 {{ pagination.total }} 条</span>
        <div class="header-actions">
          <span v-if="pagination.total > 0" class="export-hint">导出当前筛选的全部 {{ pagination.total }} 条</span>
          <el-dropdown split-button type="success" size="small" :loading="exporting" :disabled="pagination.total === 0" @click="handleExport('excel')" @command="handleExport">
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
      <el-table v-loading="loading" :data="taskList" border stripe style="width: 100%" :empty-text="listEmptyText" :row-class-name="tableRowClassName" @row-click="handleRowClick">
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="name" label="标题" min-width="220">
          <template #default="{ row }">
            <a href="javascript:void(0)" @click="openZentaoTask(row.id)" class="task-title">{{ row.name }}</a>
            <span v-if="overdueDays(row) > 0" class="overdue-flag">已超期{{ overdueDays(row) }}天</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">{{ getStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="指派给" width="110" align="center">
          <template #default="{ row }">
            <span class="assignee-cell">{{ row.assignedTo?.realname || row.assignedTo?.account || '未指派' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="estimate" label="预估" width="80" align="center">
          <template #default="{ row }">
            <span :class="{ 'text-over-hours': isOverHours(row) }">{{ row.estimate || 0 }}h</span>
          </template>
        </el-table-column>
        <el-table-column prop="consumed" label="消耗" width="80" align="center">
          <template #default="{ row }">
            <span :class="{ 'text-over-hours': isOverHours(row) }">{{ row.consumed || 0 }}h</span>
          </template>
        </el-table-column>
        <el-table-column label="进度" width="130" align="center">
          <template #default="{ row }">
            <el-progress :percentage="getProgress(row.estimate, row.consumed)" :status="getProgressStatus(row)" :stroke-width="8" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openTaskDetail(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-wrapper">
        <el-pagination v-model:current-page="pagination.page" v-model:page-size="pagination.pageSize" :page-sizes="[10, 20, 50, 100]" :total="pagination.total" layout="total, sizes, prev, pager, next, jumper" @size-change="handleSizeChange" @current-change="handlePageChange" />
      </div>
    </div>

    <el-dialog v-model="detailDialogVisible" :title="`任务详情 - ID: ${currentTask?.id}`" width="80%" destroy-on-close>
      <div v-if="currentTask" class="task-detail">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="ID">{{ currentTask.id }}</el-descriptions-item>
          <el-descriptions-item label="标题">{{ currentTask.name }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusType(currentTask.status)">{{ getStatusLabel(currentTask.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="指派给">{{ currentTask.assignedTo?.realname || currentTask.assignedTo?.account || '未指派' }}</el-descriptions-item>
          <el-descriptions-item label="所属执行">{{ executionName(currentTask.execution) }}</el-descriptions-item>
          <el-descriptions-item label="预估工时">{{ currentTask.estimate || 0 }}h</el-descriptions-item>
          <el-descriptions-item label="消耗工时">{{ currentTask.consumed || 0 }}h</el-descriptions-item>
          <el-descriptions-item label="预计开始">{{ currentTask.estStarted || '—' }}</el-descriptions-item>
          <el-descriptions-item label="截止日期">{{ currentTask.deadline || '—' }}</el-descriptions-item>
          <el-descriptions-item label="进度">
            <el-progress :percentage="getProgress(currentTask.estimate, currentTask.consumed)" :status="getProgressStatus(currentTask)" :stroke-width="10" style="max-width: 300px" />
          </el-descriptions-item>
          <el-descriptions-item label="描述" :span="2"><div v-html="sanitizeHtml(currentTask.desc || '无')" /></el-descriptions-item>
        </el-descriptions>
        <div class="dialog-actions">
          <el-button @click="openZentaoTask(currentTask.id)">在禅道中查看</el-button>
          <el-button size="small" type="primary" @click="handleTaskAction('start', currentTask.id)">开始</el-button>
          <el-button size="small" type="success" @click="handleTaskAction('finish', currentTask.id)">完成</el-button>
          <el-button size="small" type="warning" @click="handleTaskAction('pause', currentTask.id)">暂停</el-button>
          <el-button size="small" @click="handleTaskAction('activate', currentTask.id)">激活</el-button>
          <el-button size="small" @click="handleTaskAction('effort', currentTask.id)">记录工时</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- 任务动作对话框（开始/完成/暂停/激活/工时） -->
    <el-dialog v-model="taskActionDialogVisible" :title="taskActionTitle" width="480px" append-to-body>
      <el-form label-width="90px">
        <el-form-item v-if="taskActionType === 'start' || taskActionType === 'finish' || taskActionType === 'activate'" label="本次消耗">
          <el-input-number v-model="taskActionForm.consumed" :min="0" :step="0.5" :precision="1" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="taskActionType === 'start' || taskActionType === 'activate'" label="剩余">
          <el-input-number v-model="taskActionForm.left" :min="0" :step="0.5" :precision="1" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="taskActionType === 'effort'" label="日期">
          <el-date-picker v-model="taskActionForm.date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="taskActionType === 'effort'" label="工作内容">
          <el-input v-model="taskActionForm.work" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item v-if="taskActionType !== 'effort'" label="备注">
          <el-input v-model="taskActionForm.comment" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="taskActionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="taskActionLoading" @click="submitTaskAction">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, inject, watch, computed } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { sanitizeHtml } from '@/utils/sanitize'
import { getExecutions, getTasks, getTaskStatusOptions, getUsers, startTask, finishTask, pauseTask, activateTask, recordEffort } from '@/api/zentao'
import { useZentaoConfig } from '@/composables/useZentaoConfig'
import type { Task, User, Execution, SelectOption } from '@/types/api'
import { useRoute, useRouter } from 'vue-router'

interface GlobalSelection { product: number | null; project: number | null; execution: number | null }
interface FilterForm { execution: number | null; assignedTo: string; status: string; dateRange: [string, string] | [] }
interface Pagination { page: number; pageSize: number; total: number }

const globalSelection = inject<GlobalSelection>('globalSelection')!
const route = useRoute()
const router = useRouter()
const { buildUrl: buildZentaoUrl } = useZentaoConfig()
const filterForm = reactive<FilterForm>({ execution: null, assignedTo: '', status: '', dateRange: [] })
const executionOptions = ref<Execution[]>([])
const statusOptions = ref<SelectOption[]>(getTaskStatusOptions())
const userOptions = ref<User[]>([])
const taskList = ref<Task[]>([])
const loading = ref<boolean>(false)
const detailDialogVisible = ref<boolean>(false)
const currentTask = ref<Task | null>(null)
const pagination = reactive<Pagination>({ page: 1, pageSize: 20, total: 0 })

// 统计分段卡：服务端按全量（分页前）统计，避免随翻页跳变
const statusCounts = ref<Record<'doing' | 'wait' | 'done' | 'closed', number>>({ doing: 0, wait: 0, done: 0, closed: 0 })
const exporting = ref<boolean>(false)
// 导出走全量分页拉取（后端单页上限 100）
const EXPORT_PAGE_SIZE = 100

const dateShortcuts = [
  { text: '近7天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 6); return [start, end] } },
  { text: '近30天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 29); return [start, end] } },
  { text: '近90天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 89); return [start, end] } }
]

// 未选产品/执行时不发请求，页面展示引导文案
const hasScope = (): boolean => !!(globalSelection.product || filterForm.execution)

const listEmptyText = computed(() => (hasScope() ? '暂无数据' : '请先在顶部选择产品，或用执行/迭代筛选'))

const assignedToOptions = computed(() => {
  const assignees = new Map<string, { value: string; label: string }>()
  userOptions.value.forEach((user: User) => { if (user.account) assignees.set(user.account, { value: user.account, label: user.realname || user.account }) })
  return Array.from(assignees.values()).sort((a, b) => a.label.localeCompare(b.label))
})

const fetchExecutions = async (): Promise<void> => {
  try {
    const params: { projectId?: number; productId?: number } = {}
    if (globalSelection.project) params.projectId = globalSelection.project
    else if (globalSelection.product) params.productId = globalSelection.product
    const res = await getExecutions(params)
    executionOptions.value = res.data || []
  } catch (error) { console.error('获取执行/迭代列表失败:', error) }
}

const fetchUsers = async (): Promise<void> => {
  try { userOptions.value = (await getUsers()) || [] } catch (error) { console.error('获取用户列表失败:', error) }
}

// 导出当前筛选条件下的全部任务（而非仅当前页）
const handleExport = async (format: 'excel' | 'csv' | 'pdf'): Promise<void> => {
  if (exporting.value) return
  if (pagination.total === 0) return
  const { exportData, timestampedFilename } = await import('@/utils/export')
  type ExportColumn<T> = import('@/utils/export').ExportColumn<T>
  const cols: ExportColumn<Task>[] = [
    { header: 'ID', access: t => t.id },
    { header: '标题', access: t => t.name },
    { header: '状态', access: t => getStatusLabel(t.status) },
    { header: '指派给', access: t => t.assignedTo?.realname || t.assignedTo?.account || '' },
    { header: '预估工时', access: t => t.estimate ?? 0 },
    { header: '消耗工时', access: t => t.consumed ?? 0 },
    { header: '剩余工时', access: t => t.left ?? 0 },
    { header: '截止日期', access: t => t.deadline || '' }
  ]
  exporting.value = true
  try {
    const baseParams = { productId: globalSelection.product ?? undefined, executionId: filterForm.execution ?? undefined, assignedTo: filterForm.assignedTo, status: filterForm.status, startDate: filterForm.dateRange[0] || '', endDate: filterForm.dateRange[1] || '' }
    const pageCount = Math.ceil(pagination.total / EXPORT_PAGE_SIZE)
    const all: Task[] = []
    for (let p = 1; p <= pageCount; p++) {
      const res = await getTasks({ ...baseParams, page: p, pageSize: EXPORT_PAGE_SIZE })
      const list = res.data.list || []
      all.push(...list)
      if (list.length < EXPORT_PAGE_SIZE) break
    }
    if (all.length === 0) { ElMessage.warning('没有可导出的数据'); return }
    await exportData(timestampedFilename('任务列表'), all, cols, format, { title: '任务列表' })
    if (all.length < pagination.total) ElMessage.warning(`筛选结果 ${pagination.total} 条，已导出前 ${all.length} 条`)
    else ElMessage.success(`已导出全部 ${all.length} 个任务`)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '导出失败'
    ElMessage.error(msg)
  } finally { exporting.value = false }
}

// 切换产品/项目后自动刷新列表（与 Bugs/Stories 页行为一致）
watch(() => [globalSelection.product, globalSelection.project], () => {
  filterForm.execution = null
  pagination.page = 1
  fetchExecutions()
  fetchTasks()
}, { deep: true })

const fetchTasks = async (): Promise<void> => {
  if (!hasScope()) {
    taskList.value = []
    pagination.total = 0
    statusCounts.value = { doing: 0, wait: 0, done: 0, closed: 0 }
    return
  }
  loading.value = true
  try {
    const params = { productId: globalSelection.product ?? undefined, page: pagination.page, pageSize: pagination.pageSize, executionId: filterForm.execution ?? undefined, assignedTo: filterForm.assignedTo, status: filterForm.status, startDate: filterForm.dateRange[0] || '', endDate: filterForm.dateRange[1] || '' }
    const res = await getTasks(params)
    taskList.value = res.data.list || []
    pagination.total = res.data.total || 0
    // 统计卡为服务端按全量（分页前）统计，避免随翻页跳变
    if (res.data.statusCounts) {
      statusCounts.value = {
        doing: res.data.statusCounts.doing ?? 0,
        wait: res.data.statusCounts.wait ?? 0,
        done: res.data.statusCounts.done ?? 0,
        closed: res.data.statusCounts.closed ?? 0
      }
    }
  } catch (error) { console.error('获取任务列表失败:', error); ElMessage.error('获取任务列表失败') } finally { loading.value = false }
}

// 点击统计段落 = 快捷设置状态筛选（总计 = 清除状态）
const setStatusFilter = (status: string): void => {
  if (filterForm.status === status) return
  filterForm.status = status
  pagination.page = 1
  syncRoute()
  fetchTasks()
}

const handleSearch = (): void => { pagination.page = 1; syncRoute(); fetchTasks() }
const handleReset = (): void => { filterForm.execution = null; filterForm.assignedTo = ''; filterForm.status = ''; filterForm.dateRange = []; pagination.page = 1; syncRoute(); fetchTasks() }
const handleSizeChange = (size: number): void => { pagination.pageSize = size; pagination.page = 1; syncRoute(); fetchTasks() }
const handlePageChange = (page: number): void => { pagination.page = page; syncRoute(); fetchTasks() }

const syncRoute = (): void => {
  const q: Record<string, string> = {}
  if (filterForm.execution != null) q.execution = String(filterForm.execution)
  if (filterForm.assignedTo) q.assignedTo = filterForm.assignedTo
  if (filterForm.status) q.status = filterForm.status
  if (filterForm.dateRange[0]) q.startDate = filterForm.dateRange[0]
  if (filterForm.dateRange[1]) q.endDate = filterForm.dateRange[1]
  if (pagination.page > 1) q.page = String(pagination.page)
  if (pagination.pageSize !== 20) q.pageSize = String(pagination.pageSize)
  router.replace({ query: q })
}
const getStatusType = (status: string): string => ({ wait: 'info', doing: 'primary', done: 'success', pause: 'warning', cancel: 'info', closed: 'info' }[status] || 'info')
const getStatusLabel = (status: string): string => ({ wait: '未开始', doing: '进行中', done: '已完成', pause: '已暂停', cancel: '已取消', closed: '已关闭' }[status] || status)
const getProgress = (estimate: number, consumed: number): number => { if (!estimate || estimate === 0) return 0; return Math.min(Math.round((consumed / estimate) * 100), 100) }
// 活跃任务消耗超预估才标红提示；已完成/已关闭任务一律中性/成功色，避免整页红色告警噪音
const isActiveTask = (row: Task): boolean => ['wait', 'doing', 'pause'].includes(row.status)
const isOverHours = (row: Task): boolean => isActiveTask(row) && row.estimate > 0 && row.consumed > row.estimate
const getProgressStatus = (row: Task): string => {
  if (row.status === 'done' || row.status === 'closed') return 'success'
  if (!row.estimate || row.estimate === 0) return ''
  const ratio = row.consumed / row.estimate
  if (ratio > 1) return 'exception'
  if (ratio >= 0.8) return 'warning'
  return ''
}
const tableRowClassName = ({ row }: { row: Task }) => { if (isOverHours(row)) return 'overdue-row'; return '' }
const executionName = (executionId?: number): string => {
  if (!executionId) return '—'
  return executionOptions.value.find(item => item.id === executionId)?.name || `#${executionId}`
}
// 真正的"超期"：活跃任务（未完成/未关闭）且截止日期已过，返回超期天数
const DAY_MS = 24 * 60 * 60 * 1000
const overdueDays = (row: Task): number => {
  if (!isActiveTask(row) || !row.deadline) return 0
  const deadline = new Date(`${row.deadline}T00:00:00`)
  if (Number.isNaN(deadline.getTime())) return 0
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  return Math.round((today.getTime() - deadline.getTime()) / DAY_MS)
}
// 点击行任意处打开详情（标题链接与按钮除外）
const handleRowClick = (row: Task, _column: unknown, event?: Event): void => {
  const target = event?.target as HTMLElement | null
  if (target?.closest('a, button, label, .el-checkbox')) return
  openTaskDetail(row)
}
const openTaskDetail = (task: Task): void => { currentTask.value = task; detailDialogVisible.value = true }
const openZentaoTask = async (taskId: number): Promise<void> => {
  const url = buildZentaoUrl(`task-view-${taskId}.html`)
  if (!url) { ElMessage.warning('禅道地址未配置，请检查系统设置'); return }
  try {
    const { openExternalLink } = await import('@/composables/useExternalLink')
    await openExternalLink(url)
  } catch { window.open(url, '_blank', 'noopener,noreferrer') }
}

// ----- Task 写操作（开始/完成/暂停/激活/记录工时）-----
type TaskActionType = 'start' | 'finish' | 'pause' | 'activate' | 'effort'
const taskActionDialogVisible = ref(false)
const taskActionLoading = ref(false)
const taskActionType = ref<TaskActionType>('start')
const taskActionTaskId = ref(0)
const taskActionForm = reactive<{ consumed: number; left: number; date: string; work: string; comment: string }>({
  consumed: 0, left: 0, date: '', work: '', comment: ''
})
const taskActionTitle = computed(() => {
  const map: Record<TaskActionType, string> = {
    start: '开始任务', finish: '完成任务', pause: '暂停任务', activate: '激活任务', effort: '记录工时'
  }
  return map[taskActionType.value]
})

const handleTaskAction = (type: TaskActionType, id: number): void => {
  taskActionType.value = type
  taskActionTaskId.value = id
  taskActionForm.consumed = 0
  taskActionForm.left = 0
  taskActionForm.date = ''
  taskActionForm.work = ''
  taskActionForm.comment = ''
  taskActionDialogVisible.value = true
}

const submitTaskAction = async (): Promise<void> => {
  taskActionLoading.value = true
  try {
    const id = taskActionTaskId.value
    if (taskActionType.value === 'start') {
      await startTask(id, { consumed: taskActionForm.consumed, left: taskActionForm.left, comment: taskActionForm.comment })
    } else if (taskActionType.value === 'finish') {
      await finishTask(id, { consumed: taskActionForm.consumed, comment: taskActionForm.comment })
    } else if (taskActionType.value === 'pause') {
      await pauseTask(id, { comment: taskActionForm.comment })
    } else if (taskActionType.value === 'activate') {
      await activateTask(id, { consumed: taskActionForm.consumed, left: taskActionForm.left })
    } else if (taskActionType.value === 'effort') {
      await recordEffort(id, { date: taskActionForm.date, consumed: taskActionForm.consumed, left: taskActionForm.left, work: taskActionForm.work })
    }
    ElMessage.success('操作成功')
    taskActionDialogVisible.value = false
    fetchTasks()
  } catch (err) {
    const anyErr = err as { response?: { data?: { message?: string } } }
    ElMessage.error(anyErr?.response?.data?.message || '操作失败')
  } finally {
    taskActionLoading.value = false
  }
}

onMounted(() => {
  const q = route.query
  if (q.execution) filterForm.execution = Number(q.execution)
  if (q.assignedTo) filterForm.assignedTo = String(q.assignedTo)
  if (q.status) filterForm.status = String(q.status)
  if (q.startDate || q.endDate) filterForm.dateRange = [String(q.startDate || ''), String(q.endDate || '')] as [string, string]
  if (q.page) pagination.page = Number(q.page) || 1
  if (q.pageSize) pagination.pageSize = Number(q.pageSize) || 20

  fetchExecutions()
  fetchUsers()
  fetchTasks()
})
</script>

<style scoped>
.page-container {
  max-width: 1200px;
}

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

.stat-seg--doing .stat-seg-value { color: var(--color-primary); }
.stat-seg--wait .stat-seg-value { color: var(--color-warning); }
.stat-seg--done .stat-seg-value { color: var(--color-success); }
.stat-seg--closed .stat-seg-value { color: var(--color-text-tertiary); }

.stat-divider {
  width: 1px;
  height: 28px;
  background: var(--color-border);
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

/* Filter */
.filter-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  padding: var(--space-md) var(--space-lg);
  margin-bottom: var(--space-md);
  box-shadow: var(--shadow-sm);
}

/* Table */
.table-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

.task-title { color: var(--color-primary); text-decoration: none; cursor: pointer; transition: color var(--transition-fast); font-weight: 500; }
.task-title:hover { text-decoration: underline; color: var(--color-primary-hover); }

.assignee-cell {
  font-size: 13px;
}

.text-over-hours {
  color: var(--color-danger);
  font-weight: 600;
}

.overdue-flag {
  display: inline-block;
  margin-left: 8px;
  padding: 0 6px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  line-height: 18px;
  white-space: nowrap;
  color: var(--color-danger);
  background: var(--color-danger-light);
  vertical-align: 1px;
}

:deep(.el-table .overdue-row) {
  background-color: #fef2f2 !important;
}

:deep(.el-table .overdue-row:hover > td) {
  background-color: #fee2e2 !important;
}

/* Pagination */
.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  padding: var(--space-md);
}

/* Dialog */
.task-detail { line-height: 1.6; padding: 8px; }
.task-detail :deep(.el-descriptions__label) { font-weight: 600; color: var(--color-text-primary); }
.dialog-actions { margin-top: 20px; display: flex; justify-content: flex-end; }

/* Responsive */
@media screen and (max-width: 768px) {
  .stats-bar {
    flex-wrap: wrap;
  }
  .filter-card :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}
</style>
