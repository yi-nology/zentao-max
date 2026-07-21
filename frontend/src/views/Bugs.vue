<template>
  <div class="page-container">
    <div class="filter-card">
      <el-form :inline="true" :model="filterForm" class="filter-form">
        <el-form-item label="指派人">
          <el-select
            v-model="filterForm.assignedTo"
            placeholder="请选择或输入指派人"
            clearable
            filterable
            style="width: 160px"
          >
            <el-option
              v-for="item in assignedToOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="版本">
          <el-select
            v-model="filterForm.version"
            placeholder="请选择版本"
            clearable
            filterable
            style="width: 160px"
          >
            <el-option
              v-for="item in versionOptions"
              :key="item.id"
              :label="item.name"
              :value="item.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="filterForm.status"
            placeholder="请选择状态"
            clearable
            style="width: 120px"
          >
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select
            v-model="filterForm.type"
            placeholder="请选择类型"
            clearable
            filterable
            style="width: 140px"
          >
            <el-option
              v-for="item in typeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="filterForm.dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            style="width: 240px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">
            <el-icon><Search /></el-icon>查询
          </el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="table-card">
      <div class="table-header">
        <span v-if="selectedBugs.length > 0">已选择 {{ selectedBugs.length }} 个Bug</span>
        <span v-else class="result-count">共 {{ pagination.total }} 条</span>
        <div class="header-actions">
          <ColumnSettings
            :columns="columns"
            @toggle="toggleColumn"
            @show-all="showAllColumns"
            @reset="resetColumns"
          />
          <el-button type="primary" size="small" @click="handleViewDetails" :disabled="selectedBugs.length === 0">
            查看详情
          </el-button>
          <el-dropdown split-button type="success" size="small" @click="handleExport('excel')" @command="handleExport" :disabled="selectedBugs.length === 0">
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
      <el-table
        v-loading="loading"
        :data="filteredBugList"
        border
        stripe
        style="width: 100%"
        :default-sort="defaultSort"
        @select="handleSelect"
        @select-all="handleSelectAll"
        @sort-change="handleSortChange"
      >
        <el-table-column type="selection" width="55" fixed="left" />
        <template v-for="col in visibleColumns" :key="col.key">
          <el-table-column
            :prop="col.key"
            :label="col.label"
            :width="col.width"
            :min-width="col.minWidth"
            :align="col.align || 'center'"
            :sortable="col.sortable || false"
            :show-overflow-tooltip="col.showOverflowTooltip"
          >
            <template v-if="col.key === 'title'" #default="{ row }">
              <a href="javascript:void(0)" @click="openZentaoLink(buildZentaoUrl(`bug-view-${row.id}.html`))" class="bug-title">
                {{ row.title }}
              </a>
            </template>
            <template v-else-if="col.key === 'openedBuild'" #default="{ row }">
              <template v-if="row.openedBuild && row.openedBuild.length > 0">
                <el-tag v-for="build in row.openedBuild" :key="build" size="small" type="info" style="margin-right: 2px;">
                  {{ build }}
                </el-tag>
              </template>
              <span v-else>-</span>
            </template>
            <template v-else-if="col.key === 'status'" #default="{ row }">
              <el-tag :type="getStatusType(row.status)">
                {{ getStatusLabel(row.status) }}
              </el-tag>
            </template>
            <template v-else-if="col.key === 'severity'" #default="{ row }">
              <el-tag :type="getSeverityType(row.severity)">
                {{ getSeverityLabel(row.severity) }}
              </el-tag>
            </template>
            <template v-else-if="col.key === 'type'" #default="{ row }">
              <el-tag v-if="row.type" type="info" size="small">{{ getTypeLabel(row.type) }}</el-tag>
              <span v-else>-</span>
            </template>
            <template v-else-if="col.key === 'assignedTo'" #default="{ row }">
              {{ row.assignedTo?.realname || row.assignedTo?.account || row.assignedTo || '-' }}
            </template>
            <template v-else-if="col.key === 'openedDate'" #default="{ row }">
              {{ formatDate(row.openedDate) }}
            </template>
          </el-table-column>
        </template>
        <el-table-column label="操作" width="80" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleViewDetail(row)">
              查看
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </div>

    <el-dialog v-model="detailDialogVisible" title="Bug详情" width="80%" destroy-on-close>
      <div v-if="currentBug" class="bug-detail">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="ID">{{ currentBug.id }}</el-descriptions-item>
          <el-descriptions-item label="标题">{{ currentBug.title }}</el-descriptions-item>
          <el-descriptions-item label="产品">{{ productMap[currentBug.product] || currentBug.product }}</el-descriptions-item>
          <el-descriptions-item label="项目">{{ currentBug.project }}</el-descriptions-item>
          <el-descriptions-item label="版本">
            <template v-if="currentBug.openedBuild && currentBug.openedBuild.length > 0">
              {{ currentBug.openedBuild.join(', ') }}
            </template>
            <template v-else>-</template>
          </el-descriptions-item>
          <el-descriptions-item label="状态">{{ getStatusLabel(currentBug.status) }}</el-descriptions-item>
          <el-descriptions-item label="严重程度">{{ currentBug.severity }}</el-descriptions-item>
          <el-descriptions-item label="类型">{{ currentBug.type ? getTypeLabel(currentBug.type) : '-' }}</el-descriptions-item>
          <el-descriptions-item label="指派人">{{ currentBug.assignedTo?.realname || currentBug.assignedTo?.account || '-' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatDate(currentBug.openedDate) }}</el-descriptions-item>
          <el-descriptions-item label="描述" :span="2">
            <div v-html="sanitizeHtml(currentBug.steps)"></div>
          </el-descriptions-item>
        </el-descriptions>
        <div class="bug-actions">
          <el-button size="small" @click="handleBugAction('confirm', currentBug.id)">确认</el-button>
          <el-button size="small" type="warning" @click="handleBugAction('resolve', currentBug.id)">解决</el-button>
          <el-button size="small" type="success" @click="handleBugAction('close', currentBug.id)">关闭</el-button>
          <el-button size="small" @click="handleBugAction('activate', currentBug.id)">激活</el-button>
          <el-button size="small" @click="handleBugAction('assign', currentBug.id)">指派</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- 简单动作对话框（确认/解决/关闭/激活/指派 通用） -->
    <el-dialog v-model="actionDialogVisible" :title="actionTitle" width="480px" append-to-body>
      <el-form label-width="90px">
        <el-form-item v-if="actionType === 'resolve'" label="解决方案">
          <el-select v-model="actionForm.resolution" style="width: 100%">
            <el-option label="已解决(bydesign)" value="bydesign" />
            <el-option label="重复(duplicate)" value="duplicate" />
            <el-option label="外部原因(external)" value="external" />
            <el-option label="修复(fixed)" value="fixed" />
            <el-option label="不予解决(notrepro)" value="notrepro" />
            <el-option label="延期(postponed)" value="postponed" />
            <el-option label="不予修复(willnotfix)" value="willnotfix" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="actionType === 'resolve'" label="解决版本">
          <el-input v-model="actionForm.resolvedBuild" placeholder="如 mainV3_Build06" />
        </el-form-item>
        <el-form-item v-if="actionType === 'assign'" label="指派给">
          <el-input v-model="actionForm.assignedTo" placeholder="账号" />
        </el-form-item>
        <el-form-item v-if="actionType === 'activate'" label="指派给">
          <el-input v-model="actionForm.assignedTo" placeholder="账号（可空）" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="actionForm.comment" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="actionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="actionLoading" @click="submitAction">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, inject, watch } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { sanitizeHtml } from '@/utils/sanitize'
import { getBugs, getBuildsByProject, getBugStatusOptions, getUsers, getProducts, resolveBug, closeBug, assignBug, confirmBug, activateBug } from '@/api/zentao'
import type { Build } from '@/api/zentao'
import { useZentaoConfig } from '@/composables/useZentaoConfig'
import { useTableColumns, type ColumnConfig } from '@/composables/useTableColumns'
import ColumnSettings from '@/components/ColumnSettings.vue'
import type { Bug, User, SelectOption } from '@/types/api'
import { useRoute, useRouter } from 'vue-router'

interface GlobalSelection {
  product: number | null
  project: number | null
  execution: number | null
}

interface FilterForm {
  assignedTo: string
  status: string
  type: string
  version: string
  dateRange: [string, string] | []
  specificDate: string
}

interface Pagination {
  page: number
  pageSize: number
  total: number
}

const globalSelection = inject<GlobalSelection>('globalSelection')!
const route = useRoute()
const router = useRouter()
const { buildUrl: buildZentaoUrl } = useZentaoConfig()

// ----- 表格列配置（自定义列 + 本地排序） -----
interface BugColumnConfig extends ColumnConfig {
  align?: 'left' | 'center' | 'right'
  showOverflowTooltip?: boolean
}

const defaultBugColumns: BugColumnConfig[] = [
  { key: 'id', label: 'ID', visible: true, width: 80, sortable: true },
  { key: 'title', label: '标题', visible: true, minWidth: 200, align: 'left', showOverflowTooltip: true },
  { key: 'openedBuild', label: '版本', visible: true, width: 120 },
  { key: 'status', label: '状态', visible: true, width: 90, sortable: true },
  { key: 'severity', label: '严重程度', visible: true, width: 90, sortable: true },
  { key: 'type', label: '类型', visible: true, width: 110 },
  { key: 'assignedTo', label: '指派人', visible: true, width: 100 },
  { key: 'openedDate', label: '创建时间', visible: true, width: 150, sortable: true }
]

const {
  columns,
  visibleColumns,
  toggleColumn,
  showAllColumns,
  resetColumns
} = useTableColumns<BugColumnConfig>('zentao-max-bug-columns', defaultBugColumns)

// 排序状态
const sortState = reactive<{ prop: string; order: 'ascending' | 'descending' | null }>({
  prop: '',
  order: null
})
const defaultSort = computed(() => {
  if (!sortState.prop || !sortState.order) return undefined
  return { prop: sortState.prop, order: sortState.order }
})

const handleSortChange = ({ prop, order }: { prop: string; order: 'ascending' | 'descending' | null }): void => {
  sortState.prop = prop
  sortState.order = order
}

const filterForm = reactive<FilterForm>({
  assignedTo: '',
  status: '',
  type: '',
  version: '',
  dateRange: [],
  specificDate: ''
})

const statusOptions = ref<SelectOption[]>(getBugStatusOptions())
const productMap = ref<Record<number, string>>({})

const fetchProductNames = async (): Promise<void> => {
  try {
    const res = await getProducts()
    const products = res.data || []
    const map: Record<number, string> = {}
    products.forEach(p => { map[p.id] = p.name })
    productMap.value = map
  } catch { /* ignore */ }
}
const typeOptions = computed(() => {
  const types = new Map<string, { value: string; label: string }>()
  bugList.value.forEach((bug: Bug) => {
    if (bug.type && !types.has(bug.type)) {
      types.set(bug.type, {
        value: bug.type,
        label: getTypeLabel(bug.type)
      })
    }
  })
  return Array.from(types.values()).sort((a, b) => a.label.localeCompare(b.label))
})
const userOptions = ref<User[]>([])
const bugList = ref<Bug[]>([])
const loading = ref<boolean>(false)
const selectedBugs = ref<Bug[]>([])
const detailDialogVisible = ref<boolean>(false)
const currentBug = ref<Bug | null>(null)
// 版本下拉：优先用 builds 接口，失败(如禅道权限403)时从 bug 列表的 openedBuild 兜底提取
const buildVersions = ref<Build[]>([])
const fallbackVersions = ref<Build[]>([])
const versionOptions = computed<Build[]>(() => {
  return buildVersions.value.length > 0 ? buildVersions.value : fallbackVersions.value
})

const pagination = reactive<Pagination>({
  page: 1,
  pageSize: 20,
  total: 0
})

const assignedToOptions = computed(() => {
  const assignees = new Map<string, { value: string; label: string }>()
  userOptions.value.forEach((user: User) => {
    if (user.account) {
      assignees.set(user.account, {
        value: user.account,
        label: user.realname || user.account
      })
    }
  })
  return Array.from(assignees.values()).sort((a, b) => a.label.localeCompare(b.label))
})

const filteredBugList = computed(() => {
  const list = bugList.value.filter((bug: Bug) => {
    if (filterForm.assignedTo) {
      const assigned = bug.assignedTo
      if (!assigned) return false
      const account = typeof assigned === 'object' ? assigned.account : assigned
      const realname = typeof assigned === 'object' ? assigned.realname : assigned
      if (account !== filterForm.assignedTo && realname !== filterForm.assignedTo) return false
    }
    // 状态、版本、类型已由后端筛选，这里不再二次过滤，以保证分页 total 准确
    return true
  })

  // 本地排序（仅对当前页数据生效）
  if (sortState.prop && sortState.order) {
    const factor = sortState.order === 'ascending' ? 1 : -1
    return [...list].sort((a: any, b: any) => {
      const va = a[sortState.prop]
      const vb = b[sortState.prop]
      if (va == null && vb == null) return 0
      if (va == null) return 1
      if (vb == null) return -1
      // severity 是数字字符串（1-4）
      if (typeof va === 'number' || typeof vb === 'number') {
        return (Number(va) - Number(vb)) * factor
      }
      // 时间字段直接字符串比较（ISO 格式可比较）
      const sa = String(va)
      const sb = String(vb)
      return sa.localeCompare(sb) * factor
    })
  }
  return list
})

const fetchUsers = async (): Promise<void> => {
  try {
    const users = await getUsers()
    userOptions.value = users || []
  } catch (error) {
    console.error('获取用户列表失败:', error)
    ElMessage.error('获取用户列表失败，请刷新页面重试')
  }
}

const fetchBuilds = async (): Promise<void> => {
  if (!globalSelection.project) {
    buildVersions.value = []
    return
  }
  try {
    const res = await getBuildsByProject(globalSelection.project!)
    buildVersions.value = res.data || []
  } catch {
    // builds 接口可能因禅道权限返回失败(403)，此时依赖 bug 列表兜底提取版本
    buildVersions.value = []
  }
}

const fetchBugs = async (): Promise<void> => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      pageSize: pagination.pageSize,
      productId: globalSelection.product ?? undefined,
      projectId: globalSelection.project ?? undefined,
      status: filterForm.status,
      version: filterForm.version,
      type: filterForm.type,
      startDate: filterForm.dateRange[0] || '',
      endDate: filterForm.dateRange[1] || '',
      specificDate: filterForm.specificDate
    }
    const res = await getBugs(params)
    const paginatedData = res.data
    bugList.value = paginatedData.list || []
    pagination.total = paginatedData.total || 0
    // 从 bug 列表提取版本作为兜底（当 builds 接口因权限不可用时）
    const seen = new Map<string, Build>()
    bugList.value.forEach((bug: Bug) => {
      ;(bug.openedBuild || []).forEach((name: string) => {
        if (name && !seen.has(name)) {
          seen.set(name, { id: 0, project: 0, product: 0, name, date: '' })
        }
      })
    })
    fallbackVersions.value = Array.from(seen.values()).sort((a, b) => a.name.localeCompare(b.name))
  } catch (error) {
    console.error('获取 Bug 列表失败:', error)
    ElMessage.error('获取 Bug 列表失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = (): void => {
  if (!globalSelection.product) {
    ElMessage.warning('请先在顶部选择产品')
    return
  }
  pagination.page = 1
  syncRoute()
  fetchBugs()
}

const handleReset = (): void => {
  filterForm.assignedTo = ''
  filterForm.status = ''
  filterForm.type = ''
  filterForm.version = ''
  filterForm.dateRange = []
  filterForm.specificDate = ''
  pagination.page = 1
  syncRoute()
  fetchBugs()
}

// ----- Bug 写操作（确认/解决/关闭/激活/指派）-----
type BugActionType = 'confirm' | 'resolve' | 'close' | 'activate' | 'assign'
const actionDialogVisible = ref(false)
const actionLoading = ref(false)
const actionType = ref<BugActionType>('confirm')
const actionBugId = ref(0)
const actionForm = reactive<{ resolution: string; resolvedBuild: string; assignedTo: string; comment: string }>({
  resolution: 'fixed',
  resolvedBuild: '',
  assignedTo: '',
  comment: ''
})
const actionTitle = computed(() => {
  const map: Record<BugActionType, string> = {
    confirm: '确认 Bug', resolve: '解决 Bug', close: '关闭 Bug', activate: '激活 Bug', assign: '指派 Bug'
  }
  return map[actionType.value]
})

const handleBugAction = (type: BugActionType, id: number): void => {
  actionType.value = type
  actionBugId.value = id
  actionForm.resolution = 'fixed'
  actionForm.resolvedBuild = ''
  actionForm.assignedTo = ''
  actionForm.comment = ''
  actionDialogVisible.value = true
}

const submitAction = async (): Promise<void> => {
  actionLoading.value = true
  try {
    const id = actionBugId.value
    if (actionType.value === 'confirm') {
      await confirmBug(id, { comment: actionForm.comment })
    } else if (actionType.value === 'resolve') {
      await resolveBug(id, { resolution: actionForm.resolution, resolvedBuild: actionForm.resolvedBuild, comment: actionForm.comment })
    } else if (actionType.value === 'close') {
      await closeBug(id, { comment: actionForm.comment })
    } else if (actionType.value === 'activate') {
      await activateBug(id, { assignedTo: actionForm.assignedTo, comment: actionForm.comment })
    } else if (actionType.value === 'assign') {
      await assignBug(id, { assignedTo: actionForm.assignedTo, comment: actionForm.comment })
    }
    ElMessage.success('操作成功')
    actionDialogVisible.value = false
    fetchBugs()
  } catch (err) {
    const anyErr = err as { response?: { data?: { message?: string } } }
    ElMessage.error(anyErr?.response?.data?.message || '操作失败')
  } finally {
    actionLoading.value = false
  }
}

const handleSizeChange = (size: number): void => {
  if (!globalSelection.product) return
  pagination.pageSize = size
  pagination.page = 1
  syncRoute()
  fetchBugs()
}

const handlePageChange = (page: number): void => {
  if (!globalSelection.product) return
  pagination.page = page
  syncRoute()
  fetchBugs()
}

const syncRoute = (): void => {
  const q: Record<string, string> = {}
  if (filterForm.assignedTo) q.assignedTo = filterForm.assignedTo
  if (filterForm.status) q.status = filterForm.status
  if (filterForm.type) q.type = filterForm.type
  if (filterForm.version) q.version = filterForm.version
  if (filterForm.dateRange[0]) q.startDate = filterForm.dateRange[0]
  if (filterForm.dateRange[1]) q.endDate = filterForm.dateRange[1]
  if (filterForm.specificDate) q.specificDate = filterForm.specificDate
  if (pagination.page > 1) q.page = String(pagination.page)
  if (pagination.pageSize !== 20) q.pageSize = String(pagination.pageSize)
  router.replace({ query: q })
}

watch(() => globalSelection.product, (newProduct: number | null) => {
  if (newProduct) {
    pagination.page = 1
    bugList.value = []
    pagination.total = 0
    fetchBugs()
  } else {
    bugList.value = []
    pagination.total = 0
  }
}, { immediate: true })

watch(() => globalSelection.project, () => {
  if (globalSelection.product) {
    pagination.page = 1
    fetchBugs()
  }
  fetchBuilds()
})

const getStatusType = (status: string): string => {
  const types: Record<string, string> = {
    active: 'danger',
    resolved: 'success',
    closed: 'info'
  }
  return types[status] || 'info'
}

const getStatusLabel = (status: string): string => {
  const labels: Record<string, string> = {
    active: '激活',
    resolved: '已解决',
    closed: '已关闭'
  }
  return labels[status] || status
}

const getSeverityType = (severity: number): string => {
  if (severity === 1) return 'danger'
  if (severity === 2) return 'warning'
  if (severity === 3) return 'primary'
  return 'info'
}

const getSeverityLabel = (severity: number): string => {
  const labels: Record<number, string> = { 1: '致命', 2: '严重', 3: '一般', 4: '轻微', 5: '建议' }
  return labels[severity] || String(severity)
}

const getTypeLabel = (type: string): string => {
  const labels: Record<string, string> = {
    codeerror: '代码错误',
    configerror: '配置错误',
    security: '安全问题',
    performance: '性能问题',
    standard: '标准规范',
    designdefect: '设计缺陷',
    ui: '界面问题',
    install: '安装部署',
    automation: '自动化',
    other: '其他'
  }
  return labels[type] || type
}

const formatDate = (dateStr: string): string => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const handleSelect = (selection: Bug[], _row: Bug): void => {
  selectedBugs.value = selection
}

const handleSelectAll = (selection: Bug[]): void => {
  selectedBugs.value = selection
}

const handleViewDetails = (): void => {
  if (selectedBugs.value.length > 0) {
    currentBug.value = selectedBugs.value[0]
    detailDialogVisible.value = true
  }
}

const handleViewDetail = (row: Bug): void => {
  currentBug.value = row
  detailDialogVisible.value = true
}

const handleExport = async (format: 'excel' | 'csv' | 'pdf'): Promise<void> => {
  if (selectedBugs.value.length === 0) return
  const { exportData, timestampedFilename } = await import('@/utils/export')
  type ExportColumn<T> = import('@/utils/export').ExportColumn<T>
  const cols: ExportColumn<Bug>[] = [
    { header: 'ID', access: bug => bug.id },
    { header: '标题', access: bug => bug.title },
    { header: '链接地址', access: bug => buildZentaoUrl(`bug-view-${bug.id}.html`) },
    { header: '产品', access: bug => productMap.value[bug.product] || String(bug.product) },
    { header: '项目', access: bug => String(bug.project) },
    { header: '版本', access: bug => (bug.openedBuild || []).join(', ') },
    { header: '状态', access: bug => getStatusLabel(bug.status) },
    { header: '严重程度', access: bug => getSeverityLabel(bug.severity) },
    { header: '类型', access: bug => getTypeLabel(bug.type) },
    { header: '指派人', access: bug => bug.assignedTo?.realname || bug.assignedTo?.account || '' },
    { header: '创建时间', access: bug => formatDate(bug.openedDate) },
    { header: '描述', access: bug => bug.steps || '' }
  ]
  const filename = timestampedFilename('Bug列表')
  try {
    await exportData(filename, selectedBugs.value, cols, format, { title: 'Bug 列表' })
    ElMessage.success(`导出 ${selectedBugs.value.length} 个Bug成功`)
  } catch (error) {
    console.error('导出失败:', error)
    const msg = error instanceof Error ? error.message : '导出失败'
    ElMessage.error(msg)
  }
}

const openZentaoLink = async (url: string): Promise<void> => {
  if (!url) {
    ElMessage.warning('禅道地址未配置，请检查系统设置')
    return
  }
  try {
    const { openExternalLink } = await import('@/composables/useExternalLink')
    await openExternalLink(url)
  } catch (error) {
    console.error('打开链接失败:', error)
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

onMounted(() => {
  const q = route.query
  if (q.assignedTo) filterForm.assignedTo = String(q.assignedTo)
  if (q.status) filterForm.status = String(q.status)
  if (q.type) filterForm.type = String(q.type)
  if (q.version) filterForm.version = String(q.version)
  if (q.startDate || q.endDate) filterForm.dateRange = [String(q.startDate || ''), String(q.endDate || '')] as [string, string]
  if (q.specificDate) filterForm.specificDate = String(q.specificDate)
  if (q.page) pagination.page = Number(q.page) || 1
  if (q.pageSize) pagination.pageSize = Number(q.pageSize) || 20

  fetchUsers()
  fetchProductNames()
  fetchBuilds()
})
</script>

<style scoped>
.bug-title {
  color: var(--color-primary);
  text-decoration: none;
  cursor: pointer;
  transition: color var(--transition-fast);
}

.bug-title:hover {
  text-decoration: underline;
  color: var(--color-primary-hover);
}

.result-count {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.table-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bug-detail {
  line-height: 1.6;
  padding: 8px;
}

.bug-detail :deep(.el-descriptions__content) {
  word-break: break-word;
  line-height: 1.8;
}

.bug-detail :deep(.el-descriptions__label) {
  font-weight: 600;
  color: var(--color-text-primary);
}

.bug-actions {
  display: flex;
  gap: 8px;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border-light);
  flex-wrap: wrap;
}
</style>
