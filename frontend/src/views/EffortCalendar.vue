<template>
  <div class="effort-calendar-page">
    <!-- 顶部统计卡片 -->
    <div class="summary-row">
      <div class="summary-card primary">
        <div class="summary-value">{{ (summary?.totalHours ?? 0).toFixed(1) }}</div>
        <div class="summary-label">总工时 (小时)</div>
      </div>
      <div class="summary-card">
        <div class="summary-value">{{ summary?.totalRecords ?? 0 }}</div>
        <div class="summary-label">工时记录数</div>
      </div>
      <div class="summary-card">
        <div class="summary-value">{{ summary?.workDays ?? 0 }}</div>
        <div class="summary-label">填工时天数</div>
      </div>
      <div class="summary-card success">
        <div class="summary-value">{{ (summary?.avgDailyHours ?? 0).toFixed(1) }}</div>
        <div class="summary-label">日均工时</div>
      </div>
      <div class="summary-card warning">
        <div class="summary-value">{{ (summary?.maxDailyHours ?? 0).toFixed(1) }}</div>
        <div class="summary-label">单日最高{{ summary?.maxDailyDate ? ' · ' + summary.maxDailyDate : '' }}</div>
      </div>
    </div>

    <!-- 顶部控件 -->
    <div class="control-row">
      <div class="control-group">
        <span class="control-label">年份</span>
        <el-date-picker
          v-model="year"
          type="year"
          placeholder="选择年份"
          value-format="YYYY"
          style="width: 110px"
          @change="onFilterChange"
        />
      </div>
      <div class="control-group">
        <span class="control-label">月份</span>
        <el-date-picker
          v-model="currentMonth"
          type="month"
          placeholder="选择月份"
          value-format="YYYY-MM"
          style="width: 130px"
          @change="onFilterChange"
        />
      </div>
      <div class="control-group">
        <span class="control-label">用户</span>
        <el-select
          v-model="selectedUser"
          placeholder="当前用户"
          clearable
          filterable
          style="width: 160px"
          @change="onFilterChange"
        >
          <el-option v-for="u in userOptions" :key="u.value" :label="u.label" :value="u.value" />
        </el-select>
      </div>
      <el-button type="primary" :loading="loading" @click="loadData">刷新</el-button>
      <el-button type="success" @click="openBatchCreateDialog">+ 添加工时</el-button>
      <span v-if="result?.userName" class="user-tag">查看: {{ result.userName }}</span>
    </div>

    <!-- 月历主体 -->
    <el-calendar v-model="currentDate">
      <template #date-cell="{ data }">
        <div
          class="day-cell"
          :class="{ 'is-today': data.isSelected, 'is-other-month': !data.day.startsWith(currentMonth) }"
          @click="onDayClick(data.day)"
        >
          <div class="day-num">
            <span>{{ Number(data.day.split('-')[2]) }}</span>
            <span
              v-if="dayNote(data.day)"
              class="note-indicator"
              :title="dayNote(data.day)!.content.slice(0, 80)"
            >📝</span>
          </div>
          <div v-if="dayStat(data.day).hours > 0" class="day-stat">
            <div class="day-hours">{{ dayStat(data.day).hours.toFixed(1) }}h</div>
            <div class="day-bars">
              <span
                v-for="(seg, idx) in dayStat(data.day).segments"
                :key="idx"
                class="day-bar"
                :class="`type-${seg.type}`"
                :style="{ width: seg.width + '%' }"
                :title="`${seg.type}: ${seg.hours.toFixed(1)}h`"
              />
            </div>
          </div>
        </div>
      </template>
    </el-calendar>

    <!-- 类型图例 -->
    <div class="legend-row">
      <span class="legend-label">类型:</span>
      <span class="legend-item"><span class="legend-swatch type-task" />任务</span>
      <span class="legend-item"><span class="legend-swatch type-bug" />Bug</span>
      <span class="legend-item"><span class="legend-swatch type-story" />需求</span>
      <span class="legend-item"><span class="legend-swatch type-project" />项目</span>
      <span class="legend-item"><span class="legend-swatch type-other" />其他</span>
    </div>

    <EffortDayDialog
      ref="dayDialogRef"
      :user="noteUser"
      :task-options="taskOptions"
      @saved="onNoteSaved"
      @cleared="onNoteCleared"
      @synced="loadData"
    />

    <EffortBatchDialog
      ref="batchDialogRef"
      :task-options="taskOptions"
      :existing-entries="result?.entries ?? []"
      @submitted="loadData"
    />
  </div>
</template>

<script setup lang="ts">
/**
 * 工时日历（issue #7-4：当日明细弹窗拆 EffortDayDialog、批量添加/Excel 导入
 * 拆 EffortBatchDialog）。本文件保留：年月用户控件、日历格子聚合渲染、
 * daily note 缓存（📝 指示）、任务选项加载。
 */
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getEffortCalendar,
  getUsers,
  getTasks,
  listDailyNotes,
  type DailyNote
} from '@/api/zentao'
import { formatDate } from '@/utils/date'
import EffortDayDialog from '@/components/EffortDayDialog.vue'
import EffortBatchDialog from '@/components/EffortBatchDialog.vue'
import type {
  EffortCalendarResult,
  EffortCalendarSummary
} from '@/types/api'

// === state ===
const year = ref(new Date().getFullYear().toString())
const currentMonth = ref(formatDate(new Date()).slice(0, 7)) // YYYY-MM（本地时区）
const currentDate = ref(new Date())
const selectedUser = ref<string>('')
const loading = ref(false)
const result = ref<EffortCalendarResult | null>(null)
const userOptions = ref<{ value: string; label: string }[]>([])

const noteByDate = ref<Record<string, DailyNote>>({})

// 「同步为工时」与批量添加工时共用的任务下拉选项
const taskOptions = ref<{ id: number; name: string; execution: number; left: number }[]>([])

async function loadTaskOptions() {
  // 拉当前查看用户的待办任务。selectedUser 是账号；未显式选择时日历接口返回的
  // userName 是中文姓名（如"张三"），必须经 users 列表反查成账号再筛选
  let user = selectedUser.value
  if (!user) {
    const name = result.value?.userName || ''
    const hit = userOptions.value.find((u) => u.value === name || u.label.startsWith(`${name} (`))
    user = hit?.value || ''
  }
  if (!user) {
    taskOptions.value = []
    return
  }
  try {
    const res = await getTasks({ assignedTo: user, status: 'wait,doing', pageSize: 100 })
    if (res.code === 200 && res.data?.list) {
      taskOptions.value = res.data.list.map((t) => ({
        id: t.id,
        name: t.name,
        execution: t.execution,
        left: t.left,
      }))
    } else {
      taskOptions.value = []
    }
  } catch {
    taskOptions.value = []
  }
}

function dayNote(day: string): DailyNote | null {
  return noteByDate.value[day] ?? null
}



const summary = computed<EffortCalendarSummary | null>(() => result.value?.summary ?? null)

// === 按日期聚合(给日历格子用) ===
interface DaySeg { type: string; hours: number; width: number }

// 缓存 byDate 避免每次重算
const byDateCache = ref<Record<string, { hours: number; segments: DaySeg[] }>>({})

function dayStat(day: string): { hours: number; segments: DaySeg[] } {
  return byDateCache.value[day] ?? { hours: 0, segments: [] }
}

function rebuildByDateCache() {
  const map: Record<string, { hours: number; segments: DaySeg[] }> = {}
  if (!result.value) {
    byDateCache.value = map
    return
  }
  // 先按日聚合 hours
  const dayHours: Record<string, number> = {}
  for (const e of result.value.entries) {
    dayHours[e.start] = (dayHours[e.start] ?? 0) + e.consumed
  }
  // 再按 (日 × 类型) 算 segments
  const dayTypeHours: Record<string, Record<string, number>> = {}
  for (const e of result.value.entries) {
    if (!dayTypeHours[e.start]) dayTypeHours[e.start] = {}
    const t = e.objectType || 'other'
    dayTypeHours[e.start][t] = (dayTypeHours[e.start][t] ?? 0) + e.consumed
  }
  for (const day of Object.keys(dayHours)) {
    const typeMap = dayTypeHours[day] ?? {}
    const total = dayHours[day]
    const segs: DaySeg[] = []
    // 固定类型顺序保证颜色块稳定
    const order = ['task', 'bug', 'story', 'project', 'other']
    for (const t of order) {
      const h = typeMap[t] ?? 0
      if (h <= 0) continue
      segs.push({ type: t, hours: h, width: (h / total) * 100 })
    }
    map[day] = { hours: total, segments: segs }
  }
  byDateCache.value = map
}

// === 数据加载 ===
async function loadData() {
  loading.value = true
  try {
    const params: { user?: string; year?: number } = { year: Number(year.value) }
    if (selectedUser.value) params.user = selectedUser.value
    const res = await getEffortCalendar(params)
    if (res.code !== 200) {
      ElMessage.error(res.message || '拉取工时数据失败')
      return
    }
    result.value = res.data
    rebuildByDateCache()
    // 并行拉 daily note（即便失败也不影响日历）
    loadNotes()
  } catch (err) {
    ElMessage.error(`请求失败: ${(err as Error).message}`)
  } finally {
    loading.value = false
  }
}

async function loadUsers() {
  try {
    const users = await getUsers({})
    userOptions.value = (users ?? []).map((u) => ({ value: u.account, label: `${u.realname} (${u.account})` }))
  } catch {
    // 拉不到用户列表时降级:只能查自己
    userOptions.value = []
  }
}

// === Daily note 数据加载 ===
async function loadNotes() {
  // 拉当前用户当年的所有 note（前端 byDate 缓存）。
  // user 留空让后端用当前登录用户（后端暂未强制解析 user；前端用 selectedUser）。
  const user = selectedUser.value || undefined
  const [y] = currentMonth.value.split('-')
  try {
    const res = await listDailyNotes({ user, year: Number(y) })
    if (res.code === 200 && res.data?.notes) {
      const map: Record<string, DailyNote> = {}
      for (const n of res.data.notes) {
        map[n.date] = n
      }
      noteByDate.value = map
    } else {
      noteByDate.value = {}
    }
  } catch {
    // daily note 失败不影响日历显示
    noteByDate.value = {}
  }
}

function onFilterChange() {
  // 切换月份 / 年份 / 用户 都跳到对应日,让 el-calendar 跟着切视图
  if (currentMonth.value) {
    const [y, m] = currentMonth.value.split('-')
    currentDate.value = new Date(Number(y), Number(m) - 1, 1)
  }
  loadData()
}

// 目标用户账号（daily note 保存必填；未选时回退日历接口返回的 userName）
const noteUser = computed(() => selectedUser.value || result.value?.userName || '')

const dayDialogRef = ref<InstanceType<typeof EffortDayDialog> | null>(null)
const batchDialogRef = ref<InstanceType<typeof EffortBatchDialog> | null>(null)

function onDayClick(day: string) {
  const entries = (result.value?.entries ?? []).filter((e) => e.start === day)
  // "同步为工时"需要我负责的任务下拉，打开时预加载
  loadTaskOptions()
  dayDialogRef.value?.open(day, entries, noteByDate.value[day] ?? null)
}

function onNoteSaved(note: DailyNote): void {
  noteByDate.value = { ...noteByDate.value, [note.date]: note }
}

function onNoteCleared(day: string): void {
  const next = { ...noteByDate.value }
  delete next[day]
  noteByDate.value = next
}

// 批量添加：默认日期 = 当前日历选中日
function openBatchCreateDialog(): void {
  const d = currentDate.value
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  loadTaskOptions()
  batchDialogRef.value?.open(`${y}-${m}-${day}`)
}


// === 监听 selectedMonth 跟 el-calendar 同步 ===
watch(currentMonth, (val) => {
  if (!val) return
  const [y, m] = val.split('-')
  currentDate.value = new Date(Number(y), Number(m) - 1, 1)
})

onMounted(() => {
  loadUsers()
  loadData()
})
</script>

<style scoped>
.effort-calendar-page {
  padding: 16px;
}

.summary-row {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.summary-card {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  padding: 14px 20px;
  min-width: 140px;
  flex: 1;
}

.summary-card.primary { border-left: 3px solid var(--el-color-primary); }
.summary-card.success { border-left: 3px solid #67c23a; }
.summary-card.warning { border-left: 3px solid #e6a23c; }

.summary-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.summary-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.control-row {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.control-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.control-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.user-tag {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-left: auto;
}

:deep(.el-calendar) {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  padding: 8px;
}

:deep(.el-calendar__header) {
  padding: 8px 16px;
}

:deep(.el-calendar-day) {
  padding: 4px;
  min-height: 78px;
}

.day-cell {
  display: flex;
  flex-direction: column;
  height: 100%;
  cursor: pointer;
  padding: 2px 4px;
}

.day-cell.is-other-month {
  opacity: 0.4;
}

.day-num {
  font-size: 13px;
  color: var(--el-text-color-regular);
  margin-bottom: 4px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.day-num .note-indicator {
  font-size: 12px;
  line-height: 1;
  cursor: help;
  flex-shrink: 0;
}

.day-cell.is-today .day-num {
  color: var(--el-color-primary);
  font-weight: 600;
}

.day-stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
}

.day-hours {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-bottom: 4px;
}

.day-bars {
  display: flex;
  height: 6px;
  border-radius: 3px;
  overflow: hidden;
  gap: 1px;
  background: var(--el-fill-color-light);
}

.day-bar {
  height: 100%;
}

.type-task    { background: #409eff; }
.type-bug     { background: #f56c6c; }
.type-story   { background: #67c23a; }
.type-project { background: #e6a23c; }
.type-other   { background: #909399; }

.legend-row {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 12px;
  padding: 8px 12px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.legend-swatch {
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 2px;
}
</style>
