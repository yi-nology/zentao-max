export interface WebhookConfig {
  id: string
  name: string
  url: string
  enabled: boolean
  platform: string
  secret: string
  skipSSL: boolean
}

export interface SchedulerTask {
  id: string
  name: string
  enabled: boolean
  cronExpr: string
  webhooks: WebhookConfig[]
  projectId: number
  productId: number
  projectName: string
  productName: string
  statusFilter: string
  reportType: string
  agingDays: number
  priorityAssignees: string[]
  messageHeader: string
  keyword: string
  externalInfo: string
  lastRunAt: string | null
  lastRunStatus: string
  createdAt: string
  updatedAt: string
}

export interface WebhookResult {
  webhookId: string
  webhookName: string
  webhookUrl: string
  success: boolean
  statusCode: number
  error: string
}

export interface TaskExecutionLog {
  id: string
  taskId: string
  taskName: string
  startedAt: string
  finishedAt: string | null
  status: 'running' | 'success' | 'partial' | 'failed'
  bugTotal: number
  highSeverity: number
  assigneeCount: number
  webhookResults: WebhookResult[]
  error: string
}

export interface AssigneeBugStats {
  assignee: string
  account: string
  total: number
  highSeverity: number
  fatal: number
  serious: number
  moderate: number
  minor: number
}

export interface BugReport {
  title: string
  timestamp: string
  projectName: string
  total: number
  highSeverity: number
  statusBreakdown: Record<string, number>
  details: AssigneeBugStats[]
  message: string
}

export interface BugAgingItem {
  id: number
  title: string
  severity: string
  openedDate: string
  daysOpen: number
}

export interface AssigneeBugAgingStats {
  assignee: string
  account: string
  total: number
  bugs: BugAgingItem[]
}

export interface BugAgingReport {
  title: string
  timestamp: string
  projectName: string
  total: number
  agingDays: number
  details: AssigneeBugAgingStats[]
  message: string
}

export const CRON_PRESETS = [
  { label: '每天 9:00', expr: '0 9 * * *' },
  { label: '工作日 9:00', expr: '0 9 * * 1-5' },
  { label: '每周一 9:00', expr: '0 9 * * 1' },
  { label: '每天 9+14点', expr: '0 9,14 * * *' },
  { label: '每8小时', expr: '0 */8 * * *' },
  { label: '每30分钟', expr: '*/30 * * * *' },
] as const

export const STATUS_OPTIONS = [
  { label: '活跃 (active)', value: 'active' },
  { label: '已解决 (resolved)', value: 'resolved' },
  { label: '已关闭 (closed)', value: 'closed' },
  { label: '全部', value: 'all' },
] as const

export const REPORT_TYPE_OPTIONS = [
  { label: 'Bug 分布报告', value: 'bug', icon: '🐛', color: '#EF4444' },
  { label: '需求进度播报', value: 'requirement', icon: '📋', color: '#4F6BF6' },
  { label: '任务进度播报', value: 'task', icon: '✅', color: '#22C55E' },
  { label: 'Bug 超时提醒', value: 'bug-aging', icon: '⏰', color: '#F59E0B' },
] as const

export const AGING_DAYS_OPTIONS = [
  { label: '3 天', value: 3 },
  { label: '5 天', value: 5 },
  { label: '7 天', value: 7 },
  { label: '14 天', value: 14 },
  { label: '30 天', value: 30 },
] as const

export interface AssigneeStoryStats {
  assignee: string
  account: string
  total: number
  active: number
  changed: number
  closed: number
  resolved: number
  accepted: number
}

export interface RequirementReport {
  title: string
  timestamp: string
  projectName: string
  productName: string
  total: number
  statusBreakdown: Record<string, number>
  details: AssigneeStoryStats[]
  message: string
}

export interface TaskProgressStats {
  assignee: string
  account: string
  total: number
  wait: number
  doing: number
  done: number
  paused: number
  cancelled: number
  estimate: number
  consumed: number
  progress: number
}

export interface TaskProgressReport {
  title: string
  timestamp: string
  projectName: string
  productName: string
  total: number
  statusBreakdown: Record<string, number>
  totalEstimate: number
  totalConsumed: number
  overallProgress: number
  details: TaskProgressStats[]
  message: string
}
