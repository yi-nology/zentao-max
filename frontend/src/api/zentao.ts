import type {
  Product,
  Project,
  Execution,
  Bug,
  Story,
  Task,
  User,
  TimelogAnalysis,
  TimelogDashboard,
  TimelogEffort,
  EffortCalendarResult,
  SelectOption,
  PaginatedResponse,
  ApiResponse,
  DashboardData,
  SearchResult
} from '@/types/api'
import api from './api'

interface BugParams {
  productId?: number
  projectId?: number
  assignedTo?: string
  status?: string
  version?: string
  type?: string
  startDate?: string
  endDate?: string
  specificDate?: string
  page?: number
  pageSize?: number
}

export interface Build {
  id: number
  project: number
  product: number
  name: string
  date: string
}

interface StoryParams {
  productId?: number
  projectId?: number
  executionId?: number
  assignedTo?: string
  status?: string
  page?: number
  pageSize?: number
}

interface TaskParams {
  productId?: number
  executionId?: number
  assignedTo?: string
  status?: string
  startDate?: string
  endDate?: string
  page?: number
  pageSize?: number
}

interface TimelogParams {
  productId?: number
  projectId?: number
  executionId?: number
  assignedTo?: string
  dateFrom?: string
  dateTo?: string
}

interface CacheItem<T> {
  data: T
  timestamp: number
}

interface UserCache {
  data: Record<string, CacheItem<unknown>>
  set: <T>(key: string, data: T) => void
  get: <T>(key: string) => T | null
}

export const getProducts = (): Promise<ApiResponse<Product[]>> => {
  return api.get('/products')
}

export const getProjects = (params: Record<string, unknown> = {}): Promise<ApiResponse<Project[]>> => {
  return api.get('/projects', { params })
}

export const getExecutions = (params: Record<string, unknown> = {}): Promise<ApiResponse<Execution[]>> => {
  return api.get('/executions', { params })
}

export const getBugs = (params: BugParams = {}): Promise<ApiResponse<PaginatedResponse<Bug>>> => {
  const apiParams: Record<string, unknown> = {}
  if (params.productId) apiParams.productId = params.productId
  if (params.projectId) apiParams.projectId = params.projectId
  if (params.assignedTo) apiParams.assignedTo = params.assignedTo
  if (params.status) apiParams.status = params.status
  if (params.version) apiParams.version = params.version
  if (params.type) apiParams.type = params.type
  if (params.startDate) apiParams.startDate = params.startDate
  if (params.endDate) apiParams.endDate = params.endDate
  if (params.specificDate) apiParams.specificDate = params.specificDate
  if (params.page) apiParams.page = params.page
  if (params.pageSize) apiParams.pageSize = params.pageSize
  return api.get('/bugs', { params: apiParams })
}

export const getBuildsByProject = (projectId: number): Promise<ApiResponse<Build[]>> => {
  return api.get('/builds/project', { params: { projectId } })
}

export const getBuildsByExecution = (executionId: number): Promise<ApiResponse<Build[]>> => {
  return api.get('/builds/execution', { params: { executionId } })
}

export const getStories = (params: StoryParams = {}): Promise<ApiResponse<PaginatedResponse<Story>>> => {
  const apiParams: Record<string, unknown> = {}
  if (params.productId) apiParams.productId = params.productId
  if (params.projectId) apiParams.projectId = params.projectId
  if (params.executionId) apiParams.executionId = params.executionId
  if (params.assignedTo) apiParams.assignedTo = params.assignedTo
  if (params.status) apiParams.status = params.status
  if (params.page) apiParams.page = params.page
  if (params.pageSize) apiParams.pageSize = params.pageSize
  return api.get('/stories', { params: apiParams })
}

export const getTasks = (params: TaskParams = {}): Promise<ApiResponse<PaginatedResponse<Task>>> => {
  const apiParams: Record<string, unknown> = {}
  if (params.productId) apiParams.productId = params.productId
  if (params.executionId) apiParams.executionId = params.executionId
  if (params.assignedTo) apiParams.assignedTo = params.assignedTo
  if (params.status) apiParams.status = params.status
  if (params.startDate) apiParams.startDate = params.startDate
  if (params.endDate) apiParams.endDate = params.endDate
  if (params.page) apiParams.page = params.page
  if (params.pageSize) apiParams.pageSize = params.pageSize
  return api.get('/tasks', { params: apiParams })
}

export const getBugStatusOptions = (): SelectOption[] => {
  return [
    { label: '激活', value: 'active' },
    { label: '已解决', value: 'resolved' },
    { label: '已关闭', value: 'closed' },
    { label: '评审中', value: 'reviewing' }
  ]
}

export const getBugSeverityOptions = (): SelectOption[] => {
  return [
    { label: '1', value: 1 },
    { label: '2', value: 2 },
    { label: '3', value: 3 },
    { label: '4', value: 4 }
  ]
}

export const getStoryStatusOptions = (): SelectOption[] => {
  return [
    { label: '草稿', value: 'draft' },
    { label: '激活', value: 'active' },
    { label: '已变更', value: 'changed' },
    { label: '已关闭', value: 'closed' }
  ]
}

export const getStoryStageOptions = (): SelectOption[] => {
  return [
    { label: '等待', value: 'wait' },
    { label: '已计划', value: 'planned' },
    { label: '已立项', value: 'projected' },
    { label: '研发中', value: 'developing' },
    { label: '研发完毕', value: 'developed' },
    { label: '测试中', value: 'testing' },
    { label: '测试完毕', value: 'tested' },
    { label: '已验收', value: 'verified' },
    { label: '已发布', value: 'released' }
  ]
}

export const getTaskStatusOptions = (): SelectOption[] => {
  return [
    { label: '未开始', value: 'wait' },
    { label: '进行中', value: 'doing' },
    { label: '已完成', value: 'done' },
    { label: '已暂停', value: 'pause' },
    { label: '已取消', value: 'cancel' },
    { label: '已关闭', value: 'closed' }
  ]
}

const userCache: UserCache = {
  data: {},
  set<T>(key: string, data: T): void {
    this.data[key] = {
      data,
      timestamp: Date.now()
    }
  },
  get<T>(key: string): T | null {
    const cached = this.data[key] as CacheItem<T> | undefined
    if (cached) {
      if (Date.now() - cached.timestamp < 5 * 60 * 1000) {
        return cached.data
      } else {
        delete this.data[key]
      }
    }
    return null
  }
}

export const getUsers = (_params: Record<string, unknown> = {}): Promise<User[]> => {
  const cacheKey = 'users_all'

  const cachedData = userCache.get<User[]>(cacheKey)
  if (cachedData) {
    return Promise.resolve(cachedData)
  }

  return api.get('/users/all').then((response) => {
    const res = response as unknown as ApiResponse<{ users: User[] }>
    const users: User[] = res?.data?.users || []
    userCache.set(cacheKey, users)
    return users
  })
}

export const getTimelogAnalysis = (params: TimelogParams = {}): Promise<ApiResponse<TimelogAnalysis>> => {
  const apiParams: Record<string, unknown> = {}
  if (params.productId) apiParams.productId = params.productId
  if (params.projectId) apiParams.projectId = params.projectId
  if (params.executionId) apiParams.executionId = params.executionId
  if (params.assignedTo) apiParams.assignedTo = params.assignedTo
  if (params.dateFrom) apiParams.dateFrom = params.dateFrom
  if (params.dateTo) apiParams.dateTo = params.dateTo
  return api.get('/timelog/analysis', { params: apiParams })
}

export const getTimelogDashboard = (params: TimelogParams = {}): Promise<ApiResponse<TimelogDashboard>> => {
  const apiParams: Record<string, unknown> = {}
  if (params.productId) apiParams.productId = params.productId
  if (params.projectId) apiParams.projectId = params.projectId
  if (params.executionId) apiParams.executionId = params.executionId
  if (params.assignedTo) apiParams.assignedTo = params.assignedTo
  if (params.dateFrom) apiParams.dateFrom = params.dateFrom
  if (params.dateTo) apiParams.dateTo = params.dateTo
  return api.get('/timelog/dashboard', { params: apiParams })
}

export const getTimelogEfforts = (params: TimelogParams = {}): Promise<ApiResponse<TimelogEffort[]>> => {
  const apiParams: Record<string, unknown> = {}
  if (params.productId) apiParams.productId = params.productId
  if (params.projectId) apiParams.projectId = params.projectId
  if (params.executionId) apiParams.executionId = params.executionId
  if (params.assignedTo) apiParams.assignedTo = params.assignedTo
  if (params.dateFrom) apiParams.dateFrom = params.dateFrom
  if (params.dateTo) apiParams.dateTo = params.dateTo
  return api.get('/timelog/efforts', { params: apiParams })
}

export const getTimelogExecutions = (params: Record<string, unknown> = {}): Promise<ApiResponse<Execution[]>> => {
  return api.get('/executions', { params })
}

// === 工时日历 ===
//
// 后端: GET /api/v1/effort-calendar?user=&year=
// 数据源: your-zentao-server.com 的 /effort-ajaxGetEfforts-{userID}-{year}.html
// 模式:  Session 模式专属（受限实例等禁用 REST API 的实例）。
//        Token 模式（标准禅道）暂未实现，后端返 501。
// 缓存:  后端 5min fresh / 1h stale。
//
// 一次拉全年的 entries（~300 条 / 80KB），前端按月切片渲染。
export const getEffortCalendar = (params: { user?: string; year?: number } = {}): Promise<ApiResponse<EffortCalendarResult>> => {
  const apiParams: Record<string, unknown> = {}
  if (params.user) apiParams.user = params.user
  if (params.year) apiParams.year = params.year
  return api.get('/effort-calendar', { params: apiParams })
}

// 批量创建工时（受限实例 batchCreate 端点转发）
//
// 后端: POST /api/v1/efforts/batch
// body: { date: 'YYYY-MM-DD', items: [{ work, objectType: 'task_xxx'|'bug_xxx', execution, consumed, left }, ...] }
// 返回: { date, created, skipped }
export interface BatchEffortItem {
  work: string
  objectType: string  // 'task_617525' 或 'bug_xxx' 或空（占位行会被跳过）
  execution: number
  consumed: number
  left: number
}

export interface BatchEffortPayload {
  date: string
  items: BatchEffortItem[]
}

export interface BatchEffortResult {
  date: string
  created: number
  skipped: number
}

export const batchCreateEffort = (payload: BatchEffortPayload): Promise<ApiResponse<BatchEffortResult>> => {
  // 注意：后端注册在 /api/v1 前缀（routes.go apiV1 组），baseURL 已是 /api
  return api.post('/v1/efforts/batch', payload)
}

// === Daily note（"这天我干了什么"） ===
//
// 后端:
//   GET    /api/v1/effort/notes?user=&year=&month=  → 拉某月 notes
//   GET    /api/v1/effort/notes/{user}/{date}      → 拉单条
//   PUT    /api/v1/effort/notes/{user}/{date}      → upsert
//   DELETE /api/v1/effort/notes/{user}/{date}      → 删
//
// 跟禅道 effort 互补：effort 是 task 维度 ("X 任务 Y 小时")，note 是日维度总结
// ("今天整体 / 会议 / 明日计划")。完全本地存储，不打禅道。
export interface DailyNote {
  user: string
  date: string
  content: string
  createdAt?: string
  updatedAt?: string
}

export const listDailyNotes = (params: { user?: string; year?: number; month?: number } = {}): Promise<ApiResponse<{ notes: DailyNote[] }>> => {
  const apiParams: Record<string, unknown> = {}
  if (params.user) apiParams.user = params.user
  if (params.year) apiParams.year = params.year
  if (params.month) apiParams.month = params.month
  return api.get('/effort/notes', { params: apiParams })
}

export const getDailyNote = (user: string, date: string): Promise<ApiResponse<DailyNote & { exists: boolean }>> => {
  return api.get(`/effort/notes/${encodeURIComponent(user)}/${date}`)
}

export const putDailyNote = (user: string, date: string, content: string): Promise<ApiResponse<DailyNote>> => {
  return api.put(`/effort/notes/${encodeURIComponent(user)}/${date}`, { content })
}

export const deleteDailyNote = (user: string, date: string): Promise<ApiResponse<{ deleted: boolean }>> => {
  return api.delete(`/effort/notes/${encodeURIComponent(user)}/${date}`)
}

export interface DashboardTimeRange {
  productId: number
  startDate?: string
  endDate?: string
}

export const getDashboard = (
  productId: number,
  timeRange?: { startDate?: string; endDate?: string }
): Promise<ApiResponse<DashboardData>> => {
  const params: Record<string, any> = { productId }
  if (timeRange?.startDate) params.startDate = timeRange.startDate
  if (timeRange?.endDate) params.endDate = timeRange.endDate
  return api.get('/dashboard', { params })
}

export interface InitStatusData {
  isFirstStart: boolean
  hasConfig: boolean
  message: string
}

export const getInitStatus = (): Promise<ApiResponse<InitStatusData>> => {
  return api.get('/init/status')
}

export interface AccountInfo {
  domain: string
  account: string
  connected: boolean
  // 认证模式：'token'（REST API）或 'session'（PHP *.json 端点）。
  mode?: string
  // 会话模式下的认证域（如 'kydc'）。token 模式为空。
  realm?: string
}

export const getAccountInfo = (): Promise<ApiResponse<AccountInfo>> => {
  return api.get('/init/account')
}

export const search = (params: {
  keyword: string
  productId?: number
  page?: number
  pageSize?: number
}): Promise<ApiResponse<SearchResult>> => {
  const apiParams: Record<string, unknown> = { keyword: params.keyword }
  if (params.productId) apiParams.productId = params.productId
  if (params.page) apiParams.page = params.page
  if (params.pageSize) apiParams.pageSize = params.pageSize
  return api.get('/search', { params: apiParams })
}

// ---- Phase2b 新增实体（cases/plans/programs/releases/testtasks）----
// 这些实体的 wrapper 方法同时支持 token 和 session 两种认证模式。

export interface ExtendedEntityParams {
  productId?: number
  page?: number
  pageSize?: number
}

export const getCases = (params: ExtendedEntityParams = {}): Promise<ApiResponse<unknown>> => {
  return api.get('/cases', { params })
}

export const getPlans = (params: ExtendedEntityParams = {}): Promise<ApiResponse<unknown>> => {
  return api.get('/plans', { params })
}

export const getPrograms = (params: ExtendedEntityParams = {}): Promise<ApiResponse<unknown>> => {
  return api.get('/programs', { params })
}

export const getReleases = (params: ExtendedEntityParams = {}): Promise<ApiResponse<unknown>> => {
  return api.get('/releases', { params })
}

export const getTestTasks = (params: ExtendedEntityParams = {}): Promise<ApiResponse<unknown>> => {
  return api.get('/testtasks', { params })
}

// ---- Phase2c 写操作（bug/task/story，token + session 双模式）----
// 这些端点经 wrapper 的写方法分发，session 模式下走禅道 PHP 表单端点。

// Bug 写操作
export const resolveBug = (id: number, data: { resolution: string; resolvedBuild?: string; comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/bugs/${id}/resolve`, data)
}
export const closeBug = (id: number, data: { comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/bugs/${id}/close`, data)
}
export const assignBug = (id: number, data: { assignedTo: string; comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/bugs/${id}/assign`, data)
}
export const confirmBug = (id: number, data: { comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/bugs/${id}/confirm`, data)
}
export const activateBug = (id: number, data: { assignedTo?: string; comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/bugs/${id}/activate`, data)
}

// Task 写操作
export const startTask = (id: number, data: { realStarted?: string; consumed?: number; left?: number; comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/tasks/${id}/start`, data)
}
export const finishTask = (id: number, data: { consumed?: number; finishedDate?: string; comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/tasks/${id}/finish`, data)
}
export const pauseTask = (id: number, data: { comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/tasks/${id}/pause`, data)
}
export const assignTask = (id: number, data: { assignedTo: string; left?: number; comment?: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/tasks/${id}/assign`, data)
}

// Story 写操作
export const changeStory = (id: number, data: { spec: string; verify: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/stories/${id}/change`, data)
}

// ---- Phase2c 扩展写操作（全实体 CRUD）----

// Bug CRUD
export const createBug = (productId: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.post(`/bugs?productId=${productId}`, data)
}
export const updateBug = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.put(`/bugs/${id}`, data)
}
export const deleteBug = (id: number): Promise<ApiResponse<unknown>> => {
  return api.delete(`/bugs/${id}`)
}

// Task CRUD + activate + effort
export const createTask = (executionId: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.post(`/tasks?executionId=${executionId}`, data)
}
export const updateTask = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.put(`/tasks/${id}`, data)
}
export const deleteTask = (id: number): Promise<ApiResponse<unknown>> => {
  return api.delete(`/tasks/${id}`)
}
export const activateTask = (id: number, data: { consumed?: number; left?: number }): Promise<ApiResponse<unknown>> => {
  return api.post(`/tasks/${id}/activate`, data)
}
export const recordEffort = (id: number, data: { date?: string; consumed: number; left: number; work: string }): Promise<ApiResponse<unknown>> => {
  return api.post(`/tasks/${id}/effort`, data)
}

// Story CRUD
export const createStory = (data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.post('/stories', data)
}
export const updateStory = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.put(`/stories/${id}`, data)
}
export const deleteStory = (id: number): Promise<ApiResponse<unknown>> => {
  return api.delete(`/stories/${id}`)
}

// Plan CRUD + link/unlink
export const createPlan = (productId: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.post(`/plans?productId=${productId}`, data)
}
export const updatePlan = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.put(`/plans/${id}`, data)
}
export const deletePlan = (id: number): Promise<ApiResponse<unknown>> => {
  return api.delete(`/plans/${id}`)
}
export const linkStoriesToPlan = (id: number, storyIds: number[]): Promise<ApiResponse<unknown>> => {
  return api.post(`/plans/${id}/link-stories`, { storyIds })
}
export const unlinkStoriesFromPlan = (id: number, storyIds: number[]): Promise<ApiResponse<unknown>> => {
  return api.post(`/plans/${id}/unlink-stories`, { storyIds })
}
export const linkBugsToPlan = (id: number, bugIds: number[]): Promise<ApiResponse<unknown>> => {
  return api.post(`/plans/${id}/link-bugs`, { bugIds })
}
export const unlinkBugsFromPlan = (id: number, bugIds: number[]): Promise<ApiResponse<unknown>> => {
  return api.post(`/plans/${id}/unlink-bugs`, { bugIds })
}

// Case CRUD
export const createCase = (productId: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.post(`/cases?productId=${productId}`, data)
}
export const updateCase = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => {
  return api.put(`/cases/${id}`, data)
}
export const deleteCase = (id: number): Promise<ApiResponse<unknown>> => {
  return api.delete(`/cases/${id}`)
}

// Product / Project / Program / Execution / Build / User CRUD
export const createProduct = (data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.post('/products', data)
export const updateProduct = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.put(`/products/${id}`, data)
export const deleteProduct = (id: number): Promise<ApiResponse<unknown>> => api.delete(`/products/${id}`)

export const createProject = (data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.post('/projects', data)
export const updateProject = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.put(`/projects/${id}`, data)
export const deleteProject = (id: number): Promise<ApiResponse<unknown>> => api.delete(`/projects/${id}`)

export const createProgram = (data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.post('/programs', data)
export const updateProgram = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.put(`/programs/${id}`, data)
export const deleteProgram = (id: number): Promise<ApiResponse<unknown>> => api.delete(`/programs/${id}`)

export const createExecution = (projectId: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.post(`/executions?projectId=${projectId}`, data)
export const updateExecution = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.put(`/executions/${id}`, data)
export const deleteExecution = (id: number): Promise<ApiResponse<unknown>> => api.delete(`/executions/${id}`)

export const createBuild = (projectId: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.post(`/builds?projectId=${projectId}`, data)
export const updateBuild = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.put(`/builds/${id}`, data)
export const deleteBuild = (id: number): Promise<ApiResponse<unknown>> => api.delete(`/builds/${id}`)

export const createUser = (data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.post('/users', data)
export const updateUser = (id: number, data: Record<string, unknown>): Promise<ApiResponse<unknown>> => api.put(`/users/${id}`, data)
export const deleteUser = (id: number): Promise<ApiResponse<unknown>> => api.delete(`/users/${id}`)
