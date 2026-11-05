<template>
  <div class="dashboard-container">
    <!-- 顶部 Toolbar：缓存徽章 + 时间范围 -->
    <div class="dashboard-toolbar">
      <div class="toolbar-left">
        <div v-if="data?.fromCache" class="cache-chip" :class="{ 'is-stale': data?.stale }">
          <span class="cache-dot"></span>
          <span>{{ data?.stale ? '统计更新中 · 当前为上次结果' : '缓存数据' }}</span>
        </div>
        <div v-else-if="data" class="cache-chip is-live">
          <span class="cache-dot"></span>
          <span>实时数据</span>
        </div>
      </div>
      <div class="toolbar-right">
        <span class="time-range-label">时间范围</span>
        <el-radio-group v-model="timeRange" size="default">
          <el-radio-button value="7d">近 7 天</el-radio-button>
          <el-radio-button value="30d">近 30 天</el-radio-button>
          <el-radio-button value="90d">近 90 天</el-radio-button>
          <el-radio-button value="all">全部</el-radio-button>
          <el-radio-button value="custom">自定义</el-radio-button>
        </el-radio-group>
        <el-tooltip content="跳过缓存，立即回源统计（大实例约需 1 分钟）" placement="top">
          <el-button size="default" :loading="refreshing" @click="fetchData({ refresh: true })">
            <svg v-if="!refreshing" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width:14px;height:14px;margin-right:4px">
              <path d="M21 12a9 9 0 1 1-2.64-6.36" /><path d="M21 3v6h-6" />
            </svg>
            刷新
          </el-button>
        </el-tooltip>
        <el-date-picker
          v-if="timeRange === 'custom'"
          v-model="customDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始"
          end-placeholder="结束"
          size="default"
          class="time-range-picker"
        />
      </div>
    </div>

    <!-- 加载骨架屏 -->
    <div v-if="loading && !data" class="skeleton-wrap">
      <div class="cold-load-hint">大实例首次聚合需遍历全部执行（可能需要 90 秒），之后走缓存秒开；超时会返回已收集的部分数据</div>
      <div class="skeleton-row stats-skeleton">
        <div v-for="i in 4" :key="i" class="skeleton-card"></div>
      </div>
      <div class="skeleton-row charts-skeleton">
        <div v-for="i in 3" :key="i" class="skeleton-card skeleton-card--tall"></div>
      </div>
    </div>

    <!-- 错误 -->
    <div v-else-if="error" class="error-banner">
      <span>加载仪表盘数据失败：{{ error }}</span>
      <el-button type="primary" size="small" @click="fetchData">重试</el-button>
    </div>

    <template v-else-if="data">
      <!-- Stat Cards -->
      <div v-if="data?.incomplete" class="cold-load-hint">
        ⚠️ 本次统计未在时限内完成（大实例逐执行遍历耗时较长），部分数字可能偏小。30 分钟窗口内会自动后台更新，或点击"刷新"立即重算。
      </div>
      <div class="stats-grid">
        <div class="stat-card stat-card--bug">
          <div class="stat-accent"></div>
          <div class="stat-head">
            <div class="stat-icon stat-icon--bug">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="13" r="4" />
                <path d="M12 9V5M9 5h6M7.5 7.5L5 5M16.5 7.5L19 5M5 13H2M22 13h-3M4.5 19l-2 2M19.5 19l2 2M9 17l-1 5M15 17l1 5" />
              </svg>
            </div>
            <span class="stat-label">Bug</span>
          </div>
          <div class="stat-value-row">
            <span class="stat-value">{{ animatedBugs }}</span>
            <span class="stat-suffix">总数</span>
          </div>
          <div class="stat-dist">
            <div class="dist-row" v-for="(seg, idx) in bugDist" :key="idx">
              <span class="dist-label">{{ seg.label }}</span>
              <div class="dist-bar">
                <div class="dist-fill" :class="`dist-fill--bug-${seg.level}`" :style="{ width: seg.pct + '%' }"></div>
              </div>
              <span class="dist-num">{{ seg.count }}</span>
            </div>
          </div>
        </div>

        <div class="stat-card stat-card--story">
          <div class="stat-accent"></div>
          <div class="stat-head">
            <div class="stat-icon stat-icon--story">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                <polyline points="14 2 14 8 20 8" />
                <line x1="9" y1="13" x2="15" y2="13" />
                <line x1="9" y1="17" x2="15" y2="17" />
              </svg>
            </div>
            <span class="stat-label">需求</span>
          </div>
          <div class="stat-value-row">
            <span class="stat-value">{{ animatedStories }}</span>
            <span class="stat-suffix">总数</span>
          </div>
          <div class="stat-dist">
            <div class="dist-row" v-for="(seg, idx) in storyDist" :key="idx">
              <span class="dist-label">{{ seg.label }}</span>
              <div class="dist-bar">
                <div class="dist-fill dist-fill--story" :style="{ width: seg.pct + '%' }"></div>
              </div>
              <span class="dist-num">{{ seg.count }}</span>
            </div>
          </div>
        </div>

        <div class="stat-card stat-card--task">
          <div class="stat-accent"></div>
          <div class="stat-head">
            <div class="stat-icon stat-icon--task">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="3" y="4" width="18" height="16" rx="2" />
                <path d="M8 2v4M16 2v4M3 10h18" />
                <path d="M9 16l2 2 4-4" />
              </svg>
            </div>
            <span class="stat-label">任务</span>
          </div>
          <div class="stat-value-row">
            <span class="stat-value">{{ animatedTasks }}</span>
            <span class="stat-suffix">总数</span>
          </div>
          <div v-if="data?.tasksUnattributed" class="stat-unattributed-hint">
            该产品无关联执行，任务统计不可用（受限实例的产品-执行关联链可能不完整）
          </div>
          <div v-else class="stat-dist">
            <div class="dist-row" v-for="(seg, idx) in taskDist" :key="idx">
              <span class="dist-label">{{ seg.label }}</span>
              <div class="dist-bar">
                <div class="dist-fill" :class="`dist-fill--task-${seg.kind}`" :style="{ width: seg.pct + '%' }"></div>
              </div>
              <span class="dist-num">{{ seg.count }}</span>
            </div>
          </div>
        </div>

        <div class="stat-card stat-card--time">
          <div class="stat-accent"></div>
          <div class="stat-head">
            <div class="stat-icon stat-icon--time">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10" />
                <polyline points="12 6 12 12 16 14" />
              </svg>
            </div>
            <span class="stat-label">工时</span>
          </div>
          <div class="stat-value-row">
            <span class="stat-value">{{ (data.timelog?.totalHours ?? 0).toFixed(1) }}</span>
            <span class="stat-suffix">h · 累计</span>
          </div>
          <div class="stat-dist">
            <div class="dist-row">
              <span class="dist-label">本周</span>
              <div class="dist-bar">
                <div class="dist-fill dist-fill--time" :style="{ width: weekPct + '%' }"></div>
              </div>
              <span class="dist-num">{{ (data.timelog?.thisWeekHours ?? 0).toFixed(1) }}h</span>
            </div>
            <div class="dist-row">
              <span class="dist-label">日均</span>
              <div class="dist-bar">
                <div class="dist-fill dist-fill--time-light" :style="{ width: dailyPct + '%' }"></div>
              </div>
              <span class="dist-num">{{ dailyAvg }}h</span>
            </div>
            <div class="dist-row">
              <span class="dist-label">累计</span>
              <div class="dist-bar">
                <div class="dist-fill dist-fill--time-dark" :style="{ width: '100%' }"></div>
              </div>
              <span class="dist-num">{{ (data.timelog?.totalHours ?? 0).toFixed(1) }}h</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 我的待办：只显示总数 > 0 的，否则不显示 -->
      <div v-if="myTodoTotal > 0" class="my-todo">
        <div class="my-todo-head">
          <span class="my-todo-icon">📋</span>
          <span class="my-todo-title">我的待办</span>
          <span class="my-todo-account">{{ currentAccount }}</span>
        </div>
        <div class="my-todo-chips">
          <router-link
            v-if="myTodo.bugs > 0"
            :to="`/bugs?assignedTo=${currentAccount}`"
            class="todo-chip todo-chip--bug"
          >
            <span class="chip-num">{{ myTodo.bugs }}</span>
            <span class="chip-label">指派给我的 Bug</span>
          </router-link>
          <router-link
            v-if="myTodo.tasks > 0"
            :to="`/tasks?assignedTo=${currentAccount}`"
            class="todo-chip todo-chip--task"
          >
            <span class="chip-num">{{ myTodo.tasks }}</span>
            <span class="chip-label">指派给我的任务</span>
          </router-link>
          <router-link
            v-if="myTodo.stories > 0"
            :to="`/stories?assignedTo=${currentAccount}`"
            class="todo-chip todo-chip--story"
          >
            <span class="chip-num">{{ myTodo.stories }}</span>
            <span class="chip-label">指派给我的需求</span>
          </router-link>
        </div>
      </div>

      <!-- Charts -->
      <div class="charts-grid">
        <div class="chart-card">
          <div class="chart-card-header">
            <h3>Bug 严重程度</h3>
            <el-button v-if="hasSeverityData" link size="small" @click="downloadChart(0, 'bug-severity')">导出</el-button>
          </div>
          <div v-if="hasSeverityData" class="chart-wrapper"><canvas :ref="el => { severityChartRef = el as HTMLCanvasElement | null }" /></div>
          <div v-else class="chart-empty">暂无数据</div>
        </div>
        <div class="chart-card">
          <div class="chart-card-header">
            <h3>Bug 类型分布</h3>
            <el-button v-if="hasTypeData" link size="small" @click="downloadChart(1, 'bug-type')">导出</el-button>
          </div>
          <div v-if="hasTypeData" class="chart-wrapper"><canvas :ref="el => { typeChartRef = el as HTMLCanvasElement | null }" /></div>
          <div v-else class="chart-empty">暂无数据</div>
        </div>
        <div class="chart-card">
          <div class="chart-card-header">
            <h3>任务状态</h3>
            <el-button v-if="hasTaskData" link size="small" @click="downloadChart(2, 'task-status')">导出</el-button>
          </div>
          <div v-if="hasTaskData" class="chart-wrapper"><canvas :ref="el => { taskChartRef = el as HTMLCanvasElement | null }" /></div>
          <div v-else class="chart-empty">暂无数据</div>
        </div>
      </div>

      <!-- 最近列表 -->
      <div class="lists-grid">
        <div class="list-card">
          <div class="list-header">
            <h3>最近 Bug</h3>
            <router-link to="/bugs" class="list-link">查看全部 →</router-link>
          </div>
          <div v-if="!data.recentBugs || data.recentBugs.length === 0" class="list-empty">暂无数据</div>
          <ul v-else class="list-body">
            <li
              v-for="bug in data.recentBugs"
              :key="bug.id"
              class="list-item"
              @click="navigateTo('bugs', bug.id)"
            >
              <span class="item-status-dot" :class="'dot--' + bug.status"></span>
              <span class="item-id">#{{ bug.id }}</span>
              <span class="item-title" :title="bug.title">{{ bug.title }}</span>
              <span class="item-severity" v-if="bug.severity">{{ severityLabel[bug.severity] || `L${bug.severity}` }}</span>
              <span class="item-status-tag" :class="'tag--' + bug.status">{{ getBugStatusLabel(bug.status) }}</span>
            </li>
          </ul>
        </div>

        <div class="list-card">
          <div class="list-header">
            <h3>最近任务</h3>
            <router-link to="/tasks" class="list-link">查看全部 →</router-link>
          </div>
          <div v-if="!data.recentTasks || data.recentTasks.length === 0" class="list-empty">暂无数据</div>
          <ul v-else class="list-body">
            <li
              v-for="task in data.recentTasks"
              :key="task.id"
              class="list-item"
              @click="navigateTo('tasks', task.id)"
            >
              <span class="item-status-dot" :class="'dot--' + task.status"></span>
              <span class="item-id">#{{ task.id }}</span>
              <span class="item-title" :title="task.name">{{ task.name }}</span>
              <span class="item-status-tag" :class="'tag--' + task.status">{{ getTaskStatusLabel(task.status) }}</span>
            </li>
          </ul>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="48" height="48">
        <path d="M3 13h2v-2H3v2zm0 4h2v-2H3v2zm0-8h2V7H3v2zm4 4h14v-2H7v2zm0 4h14v-2H7v2zM7 7v2h14V7H7z" />
      </svg>
      <p>请先在顶部选择产品以查看仪表盘数据</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, inject, watch, computed } from 'vue'
import { useRouter } from 'vue-router'
import { getDashboard, getBugs, getStories } from '@/api/zentao'
import api from '@/api/api'
import { useDashboardCharts } from '@/composables/useDashboardCharts'
import type { ApiResponse } from '@/types/api'
import { ElMessage } from 'element-plus'
import type { DashboardData } from '@/types/api'

interface GlobalSelection {
  product: number | null
  project: number | null
  currentAccount: string | null
}

const router = useRouter()
const globalSelection = inject<GlobalSelection>('globalSelection')!
const loading = ref(false)
const error = ref('')
const data = ref<DashboardData | null>(null)

const currentAccount = ref<string>('')
const myTodo = reactive<{ bugs: number; tasks: number; stories: number }>({ bugs: 0, tasks: 0, stories: 0 })
const myTodoTotal = computed(() => myTodo.bugs + myTodo.tasks + myTodo.stories)

type TimeRangeKey = '7d' | '30d' | '90d' | 'all' | 'custom'
const timeRange = ref<TimeRangeKey>('all')
const customDateRange = ref<[string, string] | null>(null)

const formatDate = (d: Date): string => {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

const timeRangeParams = computed<{ startDate?: string; endDate?: string }>(() => {
  if (timeRange.value === 'all') return {}
  if (timeRange.value === 'custom') {
    if (!customDateRange.value || !customDateRange.value[0] || !customDateRange.value[1]) return {}
    return { startDate: customDateRange.value[0], endDate: customDateRange.value[1] }
  }
  const days = timeRange.value === '7d' ? 7 : timeRange.value === '30d' ? 30 : 90
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - days + 1)
  return { startDate: formatDate(start), endDate: formatDate(end) }
})

// ============ 数字 count-up 动画 ============
function animateNumber(target: number, duration = 700): { current: number; done: boolean } {
  const state = { current: 0, done: false }
  if (target === 0) {
    state.current = 0
    state.done = true
    return state
  }
  const startTime = performance.now()
  const step = (now: number) => {
    const elapsed = now - startTime
    const t = Math.min(elapsed / duration, 1)
    // easeOutCubic
    const eased = 1 - Math.pow(1 - t, 3)
    state.current = Math.round(target * eased)
    if (t < 1) requestAnimationFrame(step)
    else { state.current = target; state.done = true }
  }
  requestAnimationFrame(step)
  return state
}

const animatedBugs = ref(0)
const animatedStories = ref(0)
const animatedTasks = ref(0)

// ============ 分布计算（mini bar） ============
const severityLabel: Record<string, string> = { '1': '致命', '2': '严重', '3': '一般', '4': '轻微', '5': '建议' }

// ============ Charts（issue #7-2：渲染逻辑提取 useDashboardCharts） ============
const {
  severityChartRef,
  typeChartRef,
  taskChartRef,
  hasSeverityData,
  hasTypeData,
  hasTaskData,
  destroyCharts,
  downloadChart
} = useDashboardCharts(data)

const bugDist = computed(() => {
  const b = data.value?.bugs
  const total = b?.total || 0
  if (!b) return []
  return [
    { label: '激活', count: b.active ?? 0, level: 'high', pct: total ? Math.round((b.active / total) * 100) : 0 },
    { label: '已解决', count: b.resolved ?? 0, level: 'mid', pct: total ? Math.round((b.resolved / total) * 100) : 0 },
    { label: '已关闭', count: b.closed ?? 0, level: 'low', pct: total ? Math.round((b.closed / total) * 100) : 0 }
  ]
})

const storyDist = computed(() => {
  const s = data.value?.stories
  const total = s?.total || 0
  if (!s) return []
  return [
    { label: '激活', count: s.active ?? 0, pct: total ? Math.round((s.active / total) * 100) : 0 },
    { label: '草稿', count: s.draft ?? 0, pct: total ? Math.round((s.draft / total) * 100) : 0 },
    { label: '已关闭', count: s.closed ?? 0, pct: total ? Math.round((s.closed / total) * 100) : 0 }
  ]
})

const taskDist = computed(() => {
  const t = data.value?.tasks
  const total = t?.total || 0
  if (!t) return []
  return [
    { label: '进行中', count: t.doing ?? 0, kind: 'doing', pct: total ? Math.round((t.doing / total) * 100) : 0 },
    { label: '待开始', count: t.wait ?? 0, kind: 'wait', pct: total ? Math.round((t.wait / total) * 100) : 0 },
    { label: '已完成', count: t.done ?? 0, kind: 'done', pct: total ? Math.round((t.done / total) * 100) : 0 },
    { label: '已关闭', count: t.closed ?? 0, kind: 'closed', pct: total ? Math.round((t.closed / total) * 100) : 0 }
  ].filter(s => s.count > 0)
})

const weekPct = computed(() => {
  const total = data.value?.timelog?.totalHours || 0
  const week = data.value?.timelog?.thisWeekHours || 0
  return total > 0 ? Math.min(100, Math.round((week / total) * 100)) : 0
})

const dailyAvg = computed(() => {
  const total = data.value?.timelog?.totalHours || 0
  if (!total) return '0.0'
  return (total / 30).toFixed(1)
})

const dailyPct = computed(() => {
  const total = data.value?.timelog?.totalHours || 0
  const daily = parseFloat(dailyAvg.value) || 0
  if (!total || !daily) return 0
  return Math.min(100, Math.round((daily / Math.max(total, 1)) * 100 * 30))
})
const refreshing = ref(false)

const fetchData = async (opts?: { refresh?: boolean }): Promise<void> => {
  const pid = globalSelection.product
  if (!pid) {
    data.value = null
    return
  }
  if (opts?.refresh) refreshing.value = true
  else loading.value = true
  error.value = ''
  try {
    const res = await getDashboard(pid, timeRangeParams.value, { refresh: opts?.refresh })
    if (opts?.refresh) ElMessage.success('已刷新为最新统计')
    const newData = res.data
    data.value = newData
    // count-up 动画
    const bugsTotal = newData?.bugs?.total ?? 0
    const storiesTotal = newData?.stories?.total ?? 0
    const tasksTotal = newData?.tasks?.total ?? 0
    const a = animateNumber(bugsTotal)
    const b = animateNumber(storiesTotal)
    const c = animateNumber(tasksTotal)
    const sync = () => {
      animatedBugs.value = a.current
      animatedStories.value = b.current
      animatedTasks.value = c.current
      if (!a.done || !b.done || !c.done) requestAnimationFrame(sync)
    }
    requestAnimationFrame(sync)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : '未知错误'
    console.error('获取仪表盘数据失败:', e)
    error.value = msg
    ElMessage.error('获取仪表盘数据失败')
    if (!opts?.refresh) data.value = null
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

watch([timeRange, customDateRange], () => {
  if (globalSelection.product) fetchData()
})

const fetchMyTodo = async (): Promise<void> => {
  const account = globalSelection.currentAccount
  const pid = globalSelection.product
  if (!account || !pid) {
    currentAccount.value = ''
    myTodo.bugs = 0
    myTodo.tasks = 0
    myTodo.stories = 0
    return
  }
  currentAccount.value = account
  const [bugRes, taskRes, storyRes] = await Promise.allSettled([
    getBugs({ productId: pid, assignedTo: account, page: 1, pageSize: 1 }),
    api.get('/tasks', { params: { productId: pid, assignedTo: account, page: 1, pageSize: 1 } }) as Promise<ApiResponse<{ total?: number }>>,
    getStories({ productId: pid, assignedTo: account, page: 1, pageSize: 1 })
  ])
  myTodo.bugs = bugRes.status === 'fulfilled' ? (bugRes.value.data as { total?: number })?.total ?? 0 : 0
  myTodo.tasks = taskRes.status === 'fulfilled' ? taskRes.value?.data?.total ?? 0 : 0
  myTodo.stories = storyRes.status === 'fulfilled' ? (storyRes.value.data as { total?: number })?.total ?? 0 : 0
}

watch([() => globalSelection.currentAccount, () => globalSelection.product], () => {
  fetchMyTodo()
})

const navigateTo = (type: 'bugs' | 'tasks' | 'stories', id: number | string): void => {
  const map = { bugs: '/bugs', tasks: '/tasks', stories: '/stories' } as const
  router.push({ path: map[type], query: { id: String(id) } })
}

watch(() => globalSelection.product, (val) => {
  destroyCharts()
  if (val) fetchData()
  else data.value = null
}, { immediate: true })

const getBugStatusLabel = (status: string): string => {
  const map: Record<string, string> = { active: '激活', resolved: '已解决', closed: '已关闭' }
  return map[status] || status
}

const getTaskStatusLabel = (status: string): string => {
  const map: Record<string, string> = { wait: '未开始', doing: '进行中', done: '已完成', pause: '已暂停', cancel: '已取消', closed: '已关闭' }
  return map[status] || status
}
</script>

<style scoped>
.dashboard-container {
  max-width: 1400px;
  margin: 0 auto;
}

.cold-load-hint {
  margin-bottom: 12px;
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

/* ============ Toolbar ============ */
.dashboard-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  gap: 16px;
  flex-wrap: wrap;
}
.toolbar-left,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.cache-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  background: linear-gradient(135deg, rgba(79, 107, 246, 0.08) 0%, rgba(79, 107, 246, 0.04) 100%);
  border: 1px solid rgba(79, 107, 246, 0.15);
  border-radius: 100px;
  transition: all var(--transition-fast);
}
.cache-chip .cache-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(79, 107, 246, 0.2);
  animation: pulse 2s ease-in-out infinite;
}
.cache-chip.is-stale {
  color: #92400E;
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.12) 0%, rgba(245, 158, 11, 0.06) 100%);
  border-color: rgba(245, 158, 11, 0.2);
}
.cache-chip.is-stale .cache-dot {
  background: #F59E0B;
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.2);
}
.cache-chip.is-live {
  color: #166534;
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.10) 0%, rgba(34, 197, 94, 0.04) 100%);
  border-color: rgba(34, 197, 94, 0.2);
}
.cache-chip.is-live .cache-dot {
  background: #22C55E;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.2);
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.time-range-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-secondary);
}
.time-range-picker {
  margin-left: 8px;
}

/* ============ Skeleton ============ */
.skeleton-wrap {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.skeleton-row {
  display: grid;
  gap: 16px;
}
.skeleton-row.stats-skeleton { grid-template-columns: repeat(4, 1fr); }
.skeleton-row.charts-skeleton { grid-template-columns: repeat(3, 1fr); }
.skeleton-card {
  height: 168px;
  background: linear-gradient(90deg, #f1f5f9 0%, #e2e8f0 50%, #f1f5f9 100%);
  background-size: 200% 100%;
  border-radius: 14px;
  animation: shimmer 1.4s ease-in-out infinite;
}
.skeleton-card--tall { height: 240px; }
@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* ============ Error Banner ============ */
.error-banner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 16px 20px;
  margin-bottom: 16px;
  background: linear-gradient(135deg, #fef2f2 0%, #fee2e2 100%);
  color: #b91c1c;
  border: 1px solid #fecaca;
  border-radius: 10px;
  font-size: 14px;
}

/* ============ Stats Grid ============ */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  position: relative;
  background: var(--color-bg-card);
  border-radius: 14px;
  padding: 18px 20px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04), 0 1px 3px rgba(15, 23, 42, 0.02);
  border: 1px solid var(--color-border-light);
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.25s ease;
  overflow: hidden;
}
.stat-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 12px 24px -8px rgba(15, 23, 42, 0.12), 0 4px 8px rgba(15, 23, 42, 0.04);
}
.stat-card::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0;
  height: 80px;
  background: linear-gradient(180deg, var(--card-bg-tint, transparent) 0%, transparent 100%);
  pointer-events: none;
  opacity: 0.6;
}

.stat-accent {
  position: absolute;
  top: 0; left: 0;
  width: 100%;
  height: 3px;
  background: var(--accent-color, var(--color-primary));
}

.stat-card--bug {
  --accent-color: linear-gradient(90deg, #EF4444 0%, #F87171 100%);
  --card-bg-tint: rgba(239, 68, 68, 0.04);
}
.stat-card--story {
  --accent-color: linear-gradient(90deg, #4F6BF6 0%, #7C8FF8 100%);
  --card-bg-tint: rgba(79, 107, 246, 0.04);
}
.stat-card--task {
  --accent-color: linear-gradient(90deg, #F59E0B 0%, #FBBF24 100%);
  --card-bg-tint: rgba(245, 158, 11, 0.04);
}
.stat-card--time {
  --accent-color: linear-gradient(90deg, #22C55E 0%, #4ADE80 100%);
  --card-bg-tint: rgba(34, 197, 94, 0.04);
}

.stat-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.stat-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.stat-icon svg {
  width: 18px;
  height: 18px;
}
.stat-icon--bug {
  background: linear-gradient(135deg, #FEE2E2 0%, #FECACA 100%);
  color: #DC2626;
  box-shadow: inset 0 0 0 1px rgba(239, 68, 68, 0.1);
}
.stat-icon--story {
  background: linear-gradient(135deg, #E8ECFF 0%, #C7D2FE 100%);
  color: #4F6BF6;
  box-shadow: inset 0 0 0 1px rgba(79, 107, 246, 0.1);
}
.stat-icon--task {
  background: linear-gradient(135deg, #FEF3C7 0%, #FDE68A 100%);
  color: #D97706;
  box-shadow: inset 0 0 0 1px rgba(245, 158, 11, 0.1);
}
.stat-icon--time {
  background: linear-gradient(135deg, #DCFCE7 0%, #BBF7D0 100%);
  color: #16A34A;
  box-shadow: inset 0 0 0 1px rgba(34, 197, 94, 0.1);
}
.stat-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-secondary);
  letter-spacing: 0.02em;
}

.stat-value-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-top: 2px;
}
.stat-value {
  font-size: 30px;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1;
  font-variant-numeric: tabular-nums;
  background: linear-gradient(180deg, #1E293B 0%, #334155 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.stat-suffix {
  font-size: 12px;
  color: var(--color-text-tertiary);
  font-weight: 500;
}

.stat-unattributed-hint {
  margin-top: auto;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--color-bg-secondary, #F8FAFC);
  border: 1px dashed var(--color-border-light);
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-tertiary);
}
.stat-dist {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: auto;
}
.dist-row {
  display: grid;
  grid-template-columns: 50px 1fr 32px;
  align-items: center;
  gap: 8px;
  font-size: 11px;
}
.dist-label {
  color: var(--color-text-tertiary);
  font-weight: 500;
}
.dist-bar {
  height: 6px;
  background: var(--color-border-light);
  border-radius: 100px;
  overflow: hidden;
}
.dist-fill {
  height: 100%;
  border-radius: 100px;
  transition: width 0.7s cubic-bezier(0.4, 0, 0.2, 1);
}
.dist-fill--bug-high { background: linear-gradient(90deg, #EF4444 0%, #F87171 100%); }
.dist-fill--bug-mid { background: linear-gradient(90deg, #22C55E 0%, #4ADE80 100%); }
.dist-fill--bug-low { background: linear-gradient(90deg, #94A3B8 0%, #CBD5E1 100%); }
.dist-fill--story { background: linear-gradient(90deg, #4F6BF6 0%, #818CF8 100%); }
.dist-fill--task-doing { background: linear-gradient(90deg, #3B82F6 0%, #60A5FA 100%); }
.dist-fill--task-wait { background: linear-gradient(90deg, #F59E0B 0%, #FBBF24 100%); }
.dist-fill--task-done { background: linear-gradient(90deg, #22C55E 0%, #4ADE80 100%); }
.dist-fill--task-closed { background: linear-gradient(90deg, #94A3B8 0%, #CBD5E1 100%); }
.dist-fill--time { background: linear-gradient(90deg, #22C55E 0%, #4ADE80 100%); }
.dist-fill--time-light { background: linear-gradient(90deg, #86EFAC 0%, #BBF7D0 100%); }
.dist-fill--time-dark { background: linear-gradient(90deg, #16A34A 0%, #22C55E 100%); }
.dist-num {
  color: var(--color-text-secondary);
  font-weight: 600;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

/* ============ My Todo ============ */
.my-todo {
  background: linear-gradient(135deg, #FFFFFF 0%, #F8FAFC 100%);
  border-radius: 14px;
  padding: 14px 20px;
  margin-bottom: 20px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
  border: 1px solid var(--color-border-light);
  border-left: 3px solid var(--color-primary);
  display: flex;
  align-items: center;
  gap: 20px;
  transition: box-shadow var(--transition-normal);
}
.my-todo:hover {
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.06);
}
.my-todo-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.my-todo-icon {
  font-size: 16px;
}
.my-todo-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}
.my-todo-account {
  font-size: 11px;
  color: var(--color-text-tertiary);
  background: var(--color-bg-hover);
  padding: 2px 8px;
  border-radius: 100px;
  font-weight: 500;
  font-family: monospace;
}
.my-todo-chips {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  flex: 1;
}
.todo-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 10px;
  text-decoration: none;
  font-size: 12px;
  font-weight: 500;
  transition: all 0.2s ease;
  cursor: pointer;
  border: 1px solid transparent;
}
.todo-chip .chip-num {
  font-size: 16px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.todo-chip--bug {
  background: linear-gradient(135deg, #FEE2E2 0%, #FECACA 100%);
  color: #B91C1C;
  border-color: rgba(239, 68, 68, 0.2);
}
.todo-chip--bug:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 10px rgba(239, 68, 68, 0.18);
}
.todo-chip--task {
  background: linear-gradient(135deg, #FEF3C7 0%, #FDE68A 100%);
  color: #92400E;
  border-color: rgba(245, 158, 11, 0.2);
}
.todo-chip--task:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 10px rgba(245, 158, 11, 0.18);
}
.todo-chip--story {
  background: linear-gradient(135deg, #E8ECFF 0%, #C7D2FE 100%);
  color: #3730A3;
  border-color: rgba(79, 107, 246, 0.2);
}
.todo-chip--story:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 10px rgba(79, 107, 246, 0.18);
}

/* ============ Charts Grid ============ */
.charts-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}
.chart-card {
  background: var(--color-bg-card);
  border-radius: 14px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
  padding: 18px 20px 14px;
  border: 1px solid var(--color-border-light);
  transition: box-shadow var(--transition-normal);
  /* 网格子项默认 min-width:auto，canvas 初始位图宽度会把 1fr 轨道最小尺寸撑破——
     窄窗口（1025-1280px）下第三张卡被顶出视口裁掉（右侧空白条+图表截断）。
     min-width:0 允许轨道收缩，Chart.js 的 ResizeObserver 接管宽度适配 */
  min-width: 0;
}
.chart-card:hover {
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.06);
}
.chart-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.chart-card-header h3 {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
  letter-spacing: 0.01em;
}
.chart-card-header .el-button {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.chart-card-header .el-button:hover {
  color: var(--color-primary);
}
.chart-wrapper {
  position: relative;
  height: 200px;
}
.chart-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--color-text-tertiary);
  font-size: 12px;
}

/* ============ Lists Grid ============ */
.lists-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.list-card {
  background: var(--color-bg-card);
  border-radius: 14px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
  border: 1px solid var(--color-border-light);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid var(--color-border-light);
  background: linear-gradient(180deg, #FAFBFC 0%, transparent 100%);
}
.list-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}
.list-link {
  font-size: 12px;
  color: var(--color-primary);
  text-decoration: none;
  font-weight: 500;
  transition: opacity 0.2s;
}
.list-link:hover {
  opacity: 0.7;
}
.list-body {
  list-style: none;
  padding: 0;
  margin: 0;
}
.list-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 20px;
  border-bottom: 1px solid var(--color-border-light);
  transition: background 0.15s ease;
  cursor: pointer;
}
.list-item:last-child {
  border-bottom: none;
}
.list-item:hover {
  background: linear-gradient(90deg, rgba(79, 107, 246, 0.04) 0%, transparent 100%);
}
.list-item:active {
  background: rgba(79, 107, 246, 0.08);
}
.item-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  position: relative;
}
.item-status-dot::after {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: 50%;
  background: inherit;
  opacity: 0.2;
}
.dot--active, .dot--doing { background: var(--color-danger); }
.dot--resolved, .dot--done { background: var(--color-success); }
.dot--closed { background: var(--color-info); }
.dot--wait { background: var(--color-warning); }
.dot--draft, .dot--pause { background: var(--color-text-tertiary); }

.item-id {
  font-size: 11px;
  color: var(--color-text-tertiary);
  font-family: ui-monospace, SFMono-Regular, monospace;
  flex-shrink: 0;
  min-width: 50px;
  font-weight: 500;
}
.item-title {
  flex: 1;
  font-size: 13px;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 0.15s ease;
}
.list-item:hover .item-title {
  color: var(--color-primary);
}
.item-severity {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--color-warning-light);
  color: #92400E;
  flex-shrink: 0;
}
.item-status-tag {
  font-size: 10px;
  padding: 2px 8px;
  border-radius: 100px;
  font-weight: 600;
  flex-shrink: 0;
  white-space: nowrap;
  letter-spacing: 0.02em;
}
.tag--active, .tag--doing { background: var(--color-danger-light); color: var(--color-danger); }
.tag--resolved, .tag--done { background: var(--color-success-light); color: var(--color-success); }
.tag--closed { background: var(--color-info-light); color: var(--color-info); }
.tag--wait { background: var(--color-warning-light); color: var(--color-warning); }
.tag--draft, .tag--pause { background: var(--color-info-light); color: var(--color-text-tertiary); }

.list-empty {
  padding: 32px 20px;
  text-align: center;
  color: var(--color-text-tertiary);
  font-size: 12px;
}

/* ============ Empty State ============ */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 0;
  gap: 12px;
  color: var(--color-text-tertiary);
}
.empty-state p {
  font-size: 14px;
}

/* ============ Responsive ============ */
@media screen and (max-width: 1280px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  /* 与统计卡断点对齐：1025-1280px 区间 3 列图表会挤爆内容区
     （第三张卡被顶出视口，右侧空白条+图例截断），降为 2 列换行 */
  .charts-grid {
    grid-template-columns: 1fr 1fr;
  }
}
@media screen and (max-width: 1024px) {
  .charts-grid {
    grid-template-columns: 1fr 1fr;
  }
  .dashboard-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }
  .toolbar-right {
    flex-wrap: wrap;
  }
}
@media screen and (max-width: 768px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
  .charts-grid {
    grid-template-columns: 1fr;
  }
  .lists-grid {
    grid-template-columns: 1fr;
  }
  .my-todo {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  .my-todo-chips {
    width: 100%;
  }
}
</style>
