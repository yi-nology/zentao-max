<template>
  <div
    class="timelog-container"
    v-loading="loading"
    element-loading-text="正在获取数据..."
  >
    <div v-if="myEffortMode" class="my-effort-note">当前实例为受限模式：工时数据来自你本人（{{ analysisData.account || '当前账号' }}）的工时日历（跨产品），执行/产品筛选不参与</div>
    <div v-if="loading" class="cold-load-hint">正在拉取工时日历（秒级）</div>
    <div class="filter-card">
      <div class="quick-btns">
        <span class="quick-label">快捷选择:</span>
        <button v-for="range in quickRanges" :key="range.value" class="quick-btn" :class="{ active: selectedRange === range.value }" @click="setQuickRange(range.value)">
          {{ range.label }}
        </button>
      </div>
      <el-form :inline="true" class="filter-form">
        <el-form-item v-if="!myEffortMode" label="执行/迭代">
          <el-select v-model="filters.executionId" placeholder="全部执行" style="width: 160px">
            <el-option value="" label="全部执行" />
            <el-option v-for="execution in executions" :key="execution.id" :value="execution.id" :label="execution.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户">
          <el-select v-model="filters.assignedTo" placeholder="全部用户" filterable style="width: 140px">
            <el-option value="" label="全部用户" />
            <el-option v-for="user in users" :key="user.account" :value="user.account" :label="user.realname || user.account" />
          </el-select>
        </el-form-item>
        <el-form-item label="开始日期">
          <el-date-picker v-model="filters.dateFrom" type="date" placeholder="选择日期" value-format="YYYY-MM-DD" style="width: 140px" />
        </el-form-item>
        <el-form-item label="结束日期">
          <el-date-picker v-model="filters.dateTo" type="date" placeholder="选择日期" value-format="YYYY-MM-DD" style="width: 140px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="queryTimelog" :loading="loading">查询统计</el-button>
        </el-form-item>
      </el-form>
    </div>

    <template v-if="showResult">
      <div class="summary-cards">
        <div class="stat-card">
          <div class="stat-value primary">{{ analysisData.totalHours.toFixed(1) }}</div>
          <div class="stat-label">总工时 (小时)</div>
        </div>
        <div class="stat-card">
          <div class="stat-value success">{{ analysisData.effortCount || 0 }}</div>
          <div class="stat-label">工时记录数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value warning">{{ (analysisData.byProject || []).length }}</div>
          <div class="stat-label">涉及项目</div>
        </div>
        <div class="stat-card">
          <div class="stat-value danger">{{ avgHours }}</div>
          <div class="stat-label">日均工时</div>
        </div>
      </div>

      <div class="charts-container">
        <div class="chart-card full-width">
          <h3 class="chart-title">每日工时</h3>
          <div class="chart-wrapper"><canvas ref="dailyChart"></canvas></div>
        </div>
        <div class="chart-card">
          <h3 class="chart-title">按项目分布</h3>
          <div class="chart-wrapper"><canvas ref="projectChart"></canvas></div>
        </div>
        <div class="chart-card">
          <h3 class="chart-title">按任务类型分布</h3>
          <div class="chart-wrapper"><canvas ref="typeChart"></canvas></div>
        </div>
      </div>

      <div class="table-card">
        <div class="table-header">
          <el-input v-model="tableSearch" placeholder="搜索任务名称/工作内容..." :prefix-icon="Search" style="width: 300px" />
          <span class="table-count">{{ filteredEfforts.length }} 条</span>
          <div class="header-actions">
            <el-dropdown split-button type="success" size="small" @click="handleExport('excel')" @command="handleExport" :disabled="filteredEfforts.length === 0">
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
        <el-table :data="filteredEfforts" style="width: 100%" @sort-change="handleSortChange">
          <el-table-column prop="date" label="日期" sortable width="120" />
          <el-table-column prop="taskName" label="任务名称" min-width="200">
            <template #default="scope">{{ scope.row.taskName.length > 40 ? scope.row.taskName.substring(0, 40) + '...' : scope.row.taskName }}</template>
          </el-table-column>
          <el-table-column prop="taskType" label="类型" width="100">
            <template #default="scope">{{ getTaskTypeLabel(scope.row.taskType) }}</template>
          </el-table-column>
          <el-table-column prop="project" label="项目" width="150" />
          <el-table-column prop="execution" label="执行" width="150" />
          <el-table-column prop="account" label="人员" width="100">
            <template #default="scope">{{ getUserName(scope.row.account) }}</template>
          </el-table-column>
          <el-table-column prop="consumed" label="消耗(h)" sortable width="100">
            <template #default="scope"><strong>{{ scope.row.consumed.toFixed(1) }}</strong></template>
          </el-table-column>
          <el-table-column prop="work" label="工作内容" min-width="250">
            <template #default="scope">{{ (scope.row.work || '').length > 60 ? (scope.row.work || '').substring(0, 60) + '...' : (scope.row.work || '-') }}</template>
          </el-table-column>
        </el-table>
      </div>
    </template>

    <div v-if="!showResult && !loading" class="empty-state">
      <el-empty description="请选择筛选条件并点击查询统计" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, inject, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import type { Ref, ComputedRef } from 'vue'
import Chart from 'chart.js/auto'
import type { Chart as ChartType } from 'chart.js/auto'
import { getTimelogDashboard, getTimelogEfforts, getTimelogExecutions, getUsers } from '../api/zentao'
import type { Execution, User, TimelogAnalysis, TimelogEffort } from '../types/api'
import { useRoute, useRouter } from 'vue-router'

interface QuickRange { label: string; value: string }
interface Filters { executionId: string; assignedTo: string; dateFrom: string; dateTo: string }
interface ChartInstances { dailyChart?: ChartType<'bar'>; projectChart?: ChartType<'bar'>; typeChart?: ChartType<'doughnut'> }
interface GlobalSelection { product: number | null; project: number | null }
interface SortParams { prop: string; order: string | null }

const globalSelection = inject<GlobalSelection>('globalSelection') as GlobalSelection
const route = useRoute()
const router = useRouter()
const loading: Ref<boolean> = ref(false)
const showResult: Ref<boolean> = ref(false)
const selectedRange: Ref<string> = ref('thisMonth')
const tableSearch: Ref<string> = ref('')
const filters: Ref<Filters> = ref({ executionId: '', assignedTo: '', dateFrom: '', dateTo: '' })
const executions: Ref<Execution[]> = ref([])
const users: Ref<User[]> = ref([])
const analysisData: Ref<TimelogAnalysis> = ref({ totalHours: 0, effortCount: 0, taskCount: 0, byDate: [], byProject: [], byType: [], efforts: [] })
const myEffortMode = ref(false) // 受限实例：calendar 源（当前账号维度）
const dailyChart: Ref<HTMLCanvasElement | null> = ref(null)
const projectChart: Ref<HTMLCanvasElement | null> = ref(null)
const typeChart: Ref<HTMLCanvasElement | null> = ref(null)
const chartInstances: Ref<ChartInstances> = ref({})
const quickRanges: QuickRange[] = [{ label: '本周', value: 'thisWeek' }, { label: '上周', value: 'lastWeek' }, { label: '本月', value: 'thisMonth' }, { label: '上月', value: 'lastMonth' }]

const avgHours: ComputedRef<string> = computed(() => { const days = (analysisData.value.byDate || []).length; return days > 0 ? (analysisData.value.totalHours / days).toFixed(1) : '0' })
const filteredEfforts: ComputedRef<TimelogEffort[]> = computed(() => {
  if (!analysisData.value.efforts) return []
  const keyword = tableSearch.value.toLowerCase()
  if (!keyword) return analysisData.value.efforts
  return analysisData.value.efforts.filter((effort: TimelogEffort) => (effort.taskName || '').toLowerCase().includes(keyword) || (effort.work || '').toLowerCase().includes(keyword) || (effort.project || '').toLowerCase().includes(keyword))
})

const setQuickRange = (range: string): void => {
  selectedRange.value = range; const today = new Date(); let from: Date, to: Date
  switch (range) {
    case 'thisWeek': { const d = today.getDay() || 7; from = new Date(today); from.setDate(today.getDate() - d + 1); to = new Date(from); to.setDate(from.getDate() + 6); break }
    case 'lastWeek': { const d = today.getDay() || 7; from = new Date(today); from.setDate(today.getDate() - d - 6); to = new Date(from); to.setDate(from.getDate() + 6); break }
    case 'thisMonth': { from = new Date(today.getFullYear(), today.getMonth(), 1); to = today; break }
    case 'lastMonth': { from = new Date(today.getFullYear(), today.getMonth() - 1, 1); to = new Date(today.getFullYear(), today.getMonth(), 0); break }
    default: return
  }
  filters.value.dateFrom = from.toISOString().split('T')[0]; filters.value.dateTo = to.toISOString().split('T')[0]
}

const queryTimelog = async (): Promise<void> => {
  if (!filters.value.dateFrom || !filters.value.dateTo) { ElMessage.warning('请选择时间范围'); return }
  if (!globalSelection.product) { ElMessage.warning('请先在顶部选择产品'); return }
  syncRoute()
  loading.value = true
  try {
    const dashRes = await getTimelogDashboard({ productId: globalSelection.product ?? undefined, executionId: filters.value.executionId ? parseInt(filters.value.executionId, 10) : undefined, assignedTo: filters.value.assignedTo, dateFrom: filters.value.dateFrom, dateTo: filters.value.dateTo })
    const effortRes = await getTimelogEfforts({ productId: globalSelection.product ?? undefined, executionId: filters.value.executionId ? parseInt(filters.value.executionId, 10) : undefined, assignedTo: filters.value.assignedTo, dateFrom: filters.value.dateFrom, dateTo: filters.value.dateTo })
    analysisData.value = { ...dashRes.data, efforts: effortRes.data };
    myEffortMode.value = (dashRes.data as { source?: string })?.source === 'my-effort-calendar';
    showResult.value = true
    await nextTick(); renderCharts()
  } catch (error) { console.error('查询工时统计失败:', error); ElMessage.error('查询失败') } finally { loading.value = false }
}

const syncRoute = (): void => {
  const q: Record<string, string> = {}
  if (filters.value.executionId) q.executionId = filters.value.executionId
  if (filters.value.assignedTo) q.assignedTo = filters.value.assignedTo
  if (filters.value.dateFrom) q.dateFrom = filters.value.dateFrom
  if (filters.value.dateTo) q.dateTo = filters.value.dateTo
  if (selectedRange.value !== 'thisMonth') q.range = selectedRange.value
  router.replace({ query: q })
}

const renderCharts = (): void => { renderDailyChart(); renderProjectChart(); renderTypeChart() }
const getColor = (i: number): string => ['#4F6BF6', '#22C55E', '#F59E0B', '#EF4444', '#6B7280', '#8B5CF6', '#EC4899', '#14B8A6'][i % 8]

const renderDailyChart = (): void => {
  if (chartInstances.value.dailyChart) chartInstances.value.dailyChart.destroy()
  if (!dailyChart.value) return
  chartInstances.value.dailyChart = new Chart(dailyChart.value.getContext('2d') as CanvasRenderingContext2D, {
    type: 'bar', data: { labels: analysisData.value.byDate.map(d => d.date), datasets: [{ label: '工时', data: analysisData.value.byDate.map(d => parseFloat(d.hours.toFixed(1))), backgroundColor: '#4F6BF6', borderRadius: 4 }] },
    options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false } }, scales: { y: { beginAtZero: true, title: { display: true, text: '小时' } }, x: { ticks: { maxRotation: 45 } } } }
  })
}

const renderProjectChart = (): void => {
  if (chartInstances.value.projectChart) chartInstances.value.projectChart.destroy()
  if (!projectChart.value) return
  const items = [...analysisData.value.byProject].sort((a, b) => b.hours - a.hours)
  chartInstances.value.projectChart = new Chart(projectChart.value.getContext('2d') as CanvasRenderingContext2D, {
    type: 'bar', data: { labels: items.map(i => i.name), datasets: [{ label: '工时', data: items.map(i => parseFloat(i.hours.toFixed(1))), backgroundColor: items.map((_, idx) => getColor(idx)), borderRadius: 4 }] },
    options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false } }, scales: { y: { beginAtZero: true } } }
  })
}

const renderTypeChart = (): void => {
  if (chartInstances.value.typeChart) chartInstances.value.typeChart.destroy()
  if (!typeChart.value) return
  const items = [...analysisData.value.byType].sort((a, b) => b.hours - a.hours)
  chartInstances.value.typeChart = new Chart(typeChart.value.getContext('2d') as CanvasRenderingContext2D, {
    type: 'doughnut', data: { labels: items.map(i => i.name), datasets: [{ data: items.map(i => parseFloat(i.hours.toFixed(1))), backgroundColor: items.map((_, idx) => getColor(idx)), borderWidth: 2, borderColor: '#fff' }] },
    options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { position: 'right', labels: { padding: 12, font: { size: 13 } } } } }
  })
}

const handleSortChange = (sort: SortParams): void => {
  const { prop, order } = sort; if (!prop) return
  const sorted = [...analysisData.value.efforts].sort((a, b) => { let va = a[prop as keyof TimelogEffort] as string | number; let vb = b[prop as keyof TimelogEffort] as string | number; if (typeof va === 'string') va = va.toLowerCase(); if (typeof vb === 'string') vb = vb.toLowerCase(); return va < vb ? (order === 'ascending' ? -1 : 1) : va > vb ? (order === 'ascending' ? 1 : -1) : 0 })
  analysisData.value.efforts = sorted
}
const getUserName = (account: string): string => { const user = users.value.find(u => u.account === account); return user ? (user.realname || account) : account }
const getTaskTypeLabel = (type: string): string => ({ devel: '开发', test: '测试', design: '设计', study: '学习', discuss: '讨论', docs: '文档', meeting: '会议' }[type] || type)

// 导出当前过滤后的工时数据
const handleExport = async (format: 'excel' | 'csv' | 'pdf'): Promise<void> => {
  const rows = filteredEfforts.value
  if (rows.length === 0) return
  const { exportData, timestampedFilename } = await import('@/utils/export')
  type ExportColumn<T> = import('@/utils/export').ExportColumn<T>
  const cols: ExportColumn<TimelogEffort>[] = [
    { header: '日期', access: e => e.date },
    { header: '任务名称', access: e => e.taskName },
    { header: '类型', access: e => getTaskTypeLabel(e.taskType) },
    { header: '项目', access: e => e.project },
    { header: '执行', access: e => e.execution },
    { header: '账号', access: e => e.account },
    { header: '姓名', access: e => getUserName(e.account) },
    { header: '消耗工时', access: e => e.consumed },
    { header: '工作内容', access: e => e.work }
  ]
  try {
    await exportData(timestampedFilename('工时明细'), rows, cols, format, { title: '工时明细' })
    ElMessage.success(`导出 ${rows.length} 条工时成功`)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '导出失败'
    ElMessage.error(msg)
  }
}

const onProductChange = async (): Promise<void> => {
  filters.value.executionId = ''; executions.value = []
  if (globalSelection.product) { try { const res = await getTimelogExecutions({ projectId: '' }); executions.value = res.data || [] } catch { console.error('获取执行列表失败') } }
}

watch(() => globalSelection.product, (newProduct) => {
  if (newProduct) {
    onProductChange()
  } else {
    executions.value = []
    filters.value.executionId = ''
  }
}, { immediate: true })

onMounted(async () => {
  const q = route.query
  if (q.executionId) filters.value.executionId = String(q.executionId)
  if (q.assignedTo) filters.value.assignedTo = String(q.assignedTo)

  const hasDateParams = q.dateFrom && q.dateTo
  const rangeParam = q.range ? String(q.range) : 'thisMonth'
  selectedRange.value = rangeParam
  if (hasDateParams) {
    filters.value.dateFrom = String(q.dateFrom)
    filters.value.dateTo = String(q.dateTo)
  } else {
    setQuickRange(rangeParam)
  }

  if (globalSelection.product) onProductChange()
  try { users.value = (await getUsers()) || [] } catch { console.error('加载数据失败') }

  if (hasDateParams) queryTimelog()
})
</script>

<style scoped>
.timelog-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.my-effort-note {
  padding: 8px 12px;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 4px;
  font-size: 12px;
  color: #1d4ed8;
}
:global(.dark) .my-effort-note {
  background: #172554;
  border-color: #1e3a8a;
  color: #93c5fd;
}

.cold-load-hint {
  padding: 8px 12px;
  background: #fffbeb;
  border: 1px solid #fde68a;
  border-radius: 4px;
  font-size: 12px;
  color: #92400e;
}
:global(.dark) .cold-load-hint {
  background: #451a03;
  border-color: #78350f;
  color: #fcd34d;
}

.filter-card {
  background: var(--color-bg-card);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border-light);
  box-shadow: var(--shadow-sm);
}

.quick-btns {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  gap: 8px;
}

.quick-label {
  font-weight: 500;
  color: var(--color-text-primary);
  font-size: 13px;
}

.quick-btn {
  padding: 6px 14px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.quick-btn:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.quick-btn.active {
  background-color: var(--color-primary);
  color: var(--color-text-on-primary);
  border-color: var(--color-primary);
}

.filter-form {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: flex-end;
}

.filter-form .el-form-item {
  margin-bottom: 0;
}

.summary-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.stat-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border-light);
  padding: 20px 24px;
  text-align: center;
}

.stat-value {
  font-size: 32px;
  font-weight: 700;
  margin-bottom: 4px;
  font-family: var(--font-heading);
}

.stat-value.primary { color: var(--color-primary); }
.stat-value.success { color: var(--color-success); }
.stat-value.warning { color: var(--color-warning); }
.stat-value.danger { color: var(--color-danger); }

.stat-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.charts-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.chart-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border-light);
  padding: 20px;
  box-shadow: var(--shadow-sm);
}

.chart-card.full-width {
  grid-column: 1 / -1;
}

.chart-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px;
}

.chart-wrapper {
  height: 260px;
  position: relative;
}

.table-header {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  margin-bottom: 12px;
}
.table-header .header-actions {
  margin-left: auto;
}

.table-count {
  color: var(--color-text-tertiary);
  font-size: 13px;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: var(--color-text-tertiary);
}

@media (max-width: 1200px) {
  .summary-cards { grid-template-columns: repeat(2, 1fr); }
  .charts-container { grid-template-columns: 1fr; }
}

@media (max-width: 768px) {
  .summary-cards { grid-template-columns: 1fr; }
  .filter-form { flex-direction: column; align-items: flex-start; }
}
</style>
