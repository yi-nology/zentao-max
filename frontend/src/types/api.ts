export interface UserRef {
  id: number
  account: string
  avatar: string
  realname: string
}

export interface Product {
  id: number
  name: string
  code: string
  type: string
  status: string
  desc: string
}

export interface Project {
  id: number
  name: string
  code: string
  model: string
  type: string
  status: string
  pm: unknown
  begin: string
  end: string
  progress: unknown
}

export interface Execution {
  id: number
  project: number
  name: string
  code: string
  status: string
  type: string
  begin: string
  end: string
}

export interface Bug {
  id: number
  project: number
  product: number
  title: string
  keywords: string
  severity: number
  pri: number
  type: string
  os: string
  browser: string
  hardware: string
  steps: string
  status: string
  subStatus: string
  color: string
  confirmed: number
  planTime: string
  openedBy: UserRef
  openedDate: string
  openedBuild: string[]
  assignedTo: UserRef
  assignedDate: string
  deadline: unknown
  resolvedBy: unknown
  resolution: string
  resolvedBuild: string
  resolvedDate: unknown
  closedBy: unknown
  closedDate: unknown
  statusName: string
  lifeCycle: string
}

export interface Story {
  id: number
  product: number
  module: number
  plan: string
  source: string
  title: string
  spec: string
  verify: string
  type: string
  status: string
  stage: string
  pri: number
  estimate: number
  version: number
  openedBy: UserRef
  openedDate: string
  assignedTo: UserRef
  assignedDate: string
  closedBy: unknown
  closedDate: string
  closedReason: string
}

export interface Task {
  id: number
  project: number
  execution: number
  name: string
  type: string
  pri: number
  status: string
  assignedTo: UserRef
  estStarted: string
  deadline: string
  estimate: number
  consumed: number
  left: number
  desc: string
  openedBy: unknown
  openedDate: string
  finishedBy: unknown
  finishedDate: unknown
  closedBy: unknown
  closedDate: string
  statusName: string
}

export interface User {
  id: number
  account: string
  realname: string
}

export interface TimelogAnalysis {
  totalHours: number
  effortCount: number
  taskCount: number
  byProject: TimelogByProject[]
  byType: TimelogByType[]
  byDate: TimelogByDate[]
  efforts: TimelogEffort[]
}

export interface TimelogDashboard {
  totalHours: number
  effortCount: number
  taskCount: number
  byProject: TimelogByProject[]
  byType: TimelogByType[]
  byDate: TimelogByDate[]
}

export interface TimelogByProject {
  name: string
  hours: number
  count: number
}

export interface TimelogByType {
  name: string
  hours: number
  count: number
}

export interface TimelogByDate {
  date: string
  hours: number
  count: number
}

export interface TimelogEffort {
  id: number
  taskId: number
  taskName: string
  taskType: string
  project: string
  execution: string
  account: string
  date: string
  consumed: number
  work: string
}

// === 工时日历（your-zentao-server.com/effort-calendar.html）===

export interface EffortCalendarEntry {
  id: number
  title: string
  objectType: 'task' | 'bug' | 'story' | 'project' | 'other' | string
  start: string
  end: string
  consumed: number
  url: string
}

export interface EffortCalendarMonthStat {
  month: string
  totalHours: number
  recordCount: number
  workDays: number
}

export interface EffortCalendarTypeStat {
  count: number
  hours: number
}

export interface EffortCalendarSummary {
  totalHours: number
  totalRecords: number
  workDays: number
  avgDailyHours: number
  maxDailyHours: number
  maxDailyDate: string
  byMonth: EffortCalendarMonthStat[]
  byType: Record<string, EffortCalendarTypeStat>
}

export interface EffortCalendarResult {
  entries: EffortCalendarEntry[]
  summary: EffortCalendarSummary
  year: number
  userId: number
  userName?: string
}

export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

export interface PaginatedResponse<T> {
  list: T[]
  total: number
  page: number
  pageSize: number
}

// Dashboard types
export interface BugStats {
  total: number
  active: number
  resolved: number
  closed: number
  bySeverity: Record<string, number>
  byType: Record<string, number>
}

export interface StoryStats {
  total: number
  draft: number
  active: number
  closed: number
  byStage: Record<string, number>
}

export interface TaskStats {
  total: number
  wait: number
  doing: number
  done: number
  closed: number
}

export interface ProjectHours {
  projectId: number
  projectName: string
  hours: number
}

export interface TimelogSummary {
  totalHours: number
  thisWeekHours: number
  byProject: ProjectHours[]
}

export interface DashboardData {
  bugs: BugStats
  stories: StoryStats
  tasks: TaskStats
  timelog: TimelogSummary
  recentBugs: Bug[]
  recentTasks: Task[]
  // 缓存元信息
  fromCache?: boolean
  stale?: boolean
}

export interface SearchItem {
  type: 'bug' | 'story' | 'task'
  id: number
  title: string
  status: string
  project: string
  extra?: string
}

export interface SearchResult {
  total: number
  items: SearchItem[]
}

export interface SelectOption {
  label: string
  value: string | number
}
