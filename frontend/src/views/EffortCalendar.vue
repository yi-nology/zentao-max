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

    <!-- 当日明细弹窗 -->
    <el-dialog
      v-model="dayDialogVisible"
      :title="dayDialogTitle"
      width="680px"
      append-to-body
    >
      <!-- 这天我干了什么（自由文本） -->
      <div class="note-section">
        <div class="note-header">
          <span class="note-title">📝 这天我干了什么</span>
          <span v-if="noteDirty" class="note-status note-status-dirty">未保存</span>
          <span v-else-if="noteSaved" class="note-status note-status-saved">已保存 ✓</span>
          <span v-if="currentNote" class="note-updated">
            {{ formatNoteTime(currentNote.updatedAt) }}
          </span>
        </div>
        <el-input
          v-model="noteDraft"
          type="textarea"
          :rows="4"
          placeholder="今天做 X / 会议 / 明日计划…（留空也可保存）"
          maxlength="2000"
          show-word-limit
          @input="onNoteInput"
        />
        <div class="note-actions">
          <el-button size="small" @click="onClearNote" :disabled="!noteDraft">清空</el-button>
          <el-button size="small" type="primary" :loading="noteSaving" @click="onSaveNote">保存</el-button>
        </div>
      </div>

      <el-divider />

      <!-- 当日工时记录（禅道 effort） -->
      <div v-if="dayEntries.length === 0" class="empty-day">该日无禅道工时记录</div>
      <div v-else class="day-entries">
        <div v-for="e in dayEntries" :key="e.id" class="day-entry">
          <span class="entry-type" :class="`type-${e.objectType}`">{{ typeLabel(e.objectType) }}</span>
          <span class="entry-title">{{ e.title }}</span>
          <span class="entry-hours">{{ e.consumed.toFixed(1) }}h</span>
          <a v-if="e.url" :href="originBase + e.url" target="_blank" rel="noopener" class="entry-link">查看</a>
        </div>
        <div class="day-total">
          合计: <strong>{{ dayTotal.toFixed(1) }}</strong> 小时 · {{ dayEntries.length }} 条
        </div>
      </div>
    </el-dialog>

    <!-- 批量添加工时弹窗（受限实例 batchCreate 端点转发） -->
    <el-dialog
      v-model="batchDialogVisible"
      title="批量添加工时"
      width="820px"
      append-to-body
      :close-on-click-modal="false"
    >
      <div class="batch-form">
        <div class="batch-row batch-row-header">
          <div class="batch-cell batch-cell-date">
            <span class="batch-label">日期</span>
            <el-date-picker
              v-model="batchDate"
              type="date"
              placeholder="选择日期"
              value-format="YYYY-MM-DD"
              style="width: 160px"
            />
          </div>
          <div class="batch-cell batch-cell-stats">
            <span class="batch-stat">
              共 <strong>{{ batchItems.length }}</strong> 行 ·
              <strong>{{ validBatchCount }}</strong> 条有效 ·
              合计 <strong>{{ batchTotalConsumed.toFixed(1) }}</strong> h
            </span>
          </div>
        </div>

        <el-table :data="batchItems" style="width: 100%" size="small" empty-text="暂无数据，点击下方 + 添加行">
          <el-table-column label="任务 / Bug" min-width="280">
            <template #default="{ row }">
              <el-select
                v-model="row.objectType"
                placeholder="选择任务或Bug"
                filterable
                style="width: 100%"
                @change="(v: string) => onTaskSelectChange(row, v)"
              >
                <el-option-group label="我负责的任务">
                  <el-option
                    v-for="t in taskOptions"
                    :key="`task-${t.id}`"
                    :label="`#${t.id} ${t.name} (剩 ${t.left}h)`"
                    :value="`task_${t.id}`"
                  />
                </el-option-group>
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="工作内容" min-width="200">
            <template #default="{ row }">
              <el-input v-model="row.work" placeholder="做了什么" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="消耗(h)" width="100">
            <template #default="{ row }">
              <el-input-number
                v-model="row.consumed"
                :min="0"
                :max="24"
                :step="0.5"
                :precision="1"
                size="small"
                controls-position="right"
                style="width: 100%"
              />
            </template>
          </el-table-column>
          <el-table-column label="剩余(h)" width="100">
            <template #default="{ row }">
              <el-input-number
                v-model="row.left"
                :min="0"
                :max="999"
                :step="0.5"
                :precision="1"
                size="small"
                controls-position="right"
                style="width: 100%"
              />
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="removeBatchItem($index)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="batch-toolbar">
          <el-button size="small" @click="addBatchItem">+ 添加行</el-button>
          <el-button size="small" @click="resetBatchItems">清空</el-button>
          <span class="batch-tip">提示：受限实例端点要求 objectType 格式为 task_xxx 或 bug_xxx</span>
        </div>
      </div>

      <template #footer>
        <el-button @click="batchDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchSubmitting" @click="onBatchSubmit">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getEffortCalendar,
  getUsers,
  listDailyNotes,
  putDailyNote,
  deleteDailyNote,
  getTasks,
  batchCreateEffort,
  type DailyNote
} from '@/api/zentao'
import type {
  EffortCalendarResult,
  EffortCalendarEntry,
  EffortCalendarSummary
} from '@/types/api'

// === state ===
const year = ref(new Date().getFullYear().toString())
const currentMonth = ref(new Date().toISOString().slice(0, 7)) // YYYY-MM
const currentDate = ref(new Date())
const selectedUser = ref<string>('')
const loading = ref(false)
const result = ref<EffortCalendarResult | null>(null)
const userOptions = ref<{ value: string; label: string }[]>([])

// 弹窗
const dayDialogVisible = ref(false)
const dayDialogTitle = ref('')
const dayEntries = ref<EffortCalendarEntry[]>([])
const dayTotal = computed(() => dayEntries.value.reduce((s, e) => s + e.consumed, 0))

// Daily note 状态
const noteByDate = ref<Record<string, DailyNote>>({})
const noteDraft = ref('')
const noteSaving = ref(false)
const noteDirty = ref(false)  // 草稿未保存
const noteSaved = ref(false)  // 刚保存成功（短暂显示 "已保存"）
const currentNote = ref<DailyNote | null>(null) // 弹窗当前日期的 note（来自 noteByDate 或 null）
const currentNoteDay = ref<string>('') // 弹窗正在编辑的日期

// === 批量添加工时弹窗 ===
const batchDialogVisible = ref(false)
const batchSubmitting = ref(false)
const batchDate = ref(new Date().toISOString().slice(0, 10)) // YYYY-MM-DD
interface BatchFormItem {
  work: string
  objectType: string  // 'task_xxx' / 'bug_xxx'
  execution: number
  consumed: number
  left: number
}
const batchItems = ref<BatchFormItem[]>([])
const taskOptions = ref<{ id: number; name: string; execution: number; left: number }[]>([])

const validBatchCount = computed(() => batchItems.value.filter(i =>
  i.objectType && i.execution > 0 && i.consumed >= 0
).length)
const batchTotalConsumed = computed(() => batchItems.value.reduce((s, i) => s + (i.consumed || 0), 0))

function emptyBatchItem(): BatchFormItem {
  return { work: '', objectType: '', execution: 0, consumed: 0, left: 0 }
}

function addBatchItem() {
  batchItems.value.push(emptyBatchItem())
}

function removeBatchItem(idx: number) {
  batchItems.value.splice(idx, 1)
}

function resetBatchItems() {
  batchItems.value = [emptyBatchItem()]
}

function onTaskSelectChange(row: BatchFormItem, v: string) {
  // v 格式: "task_{id}" 或 "bug_{id}"
  if (!v) return
  const [kind, idStr] = v.split('_')
  const id = Number(idStr)
  if (kind === 'task') {
    const t = taskOptions.value.find((x) => x.id === id)
    if (t) {
      row.execution = t.execution
      // 默认 left 用 task 当前剩余
      if (row.left === 0) row.left = t.left
    }
  }
}

async function loadTaskOptions() {
  // 拉当前用户（selectedUser 优先,否则用 result.userName）assigned 的未完成任务
  const user = selectedUser.value || result.value?.userName || ''
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

function openBatchCreateDialog() {
  // 默认日期 = 当前日历选中日
  if (currentDate.value) {
    const d = new Date(currentDate.value)
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    batchDate.value = `${y}-${m}-${day}`
  }
  if (batchItems.value.length === 0) {
    batchItems.value = [emptyBatchItem()]
  }
  batchDialogVisible.value = true
  loadTaskOptions()
}

async function onBatchSubmit() {
  if (!batchDate.value) {
    ElMessage.error('请选择日期')
    return
  }
  const valid = batchItems.value.filter((i) => i.objectType && i.execution > 0)
  if (valid.length === 0) {
    ElMessage.error('至少需要一条有效记录（选择任务/Bug 且 execution 不为空）')
    return
  }
  batchSubmitting.value = true
  try {
    const payload = {
      date: batchDate.value,
      items: batchItems.value.map((i) => ({
        work: i.work,
        objectType: i.objectType,
        execution: i.execution,
        consumed: i.consumed,
        left: i.left,
      })),
    }
    const res = await batchCreateEffort(payload)
    if (res.code === 200 && res.data) {
      ElMessage.success(`已写入 ${res.data.created} 条，跳过 ${res.data.skipped} 条空行`)
      batchDialogVisible.value = false
      // 重新拉当前 year 的日历
      await loadData()
    } else {
      ElMessage.error(res.message || '提交失败')
    }
  } catch (err) {
    ElMessage.error(`提交失败: ${(err as Error).message}`)
  } finally {
    batchSubmitting.value = false
  }
}

function dayNote(day: string): DailyNote | null {
  return noteByDate.value[day] ?? null
}

function formatNoteTime(s?: string): string {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// 禅道 base（从当前 location 推断，给 url 拼前缀用）
const originBase = computed(() => window.location.origin)

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

// === 控件联动 ===
function onFilterChange() {
  // 切换月份 / 年份 / 用户 都跳到对应日,让 el-calendar 跟着切视图
  if (currentMonth.value) {
    const [y, m] = currentMonth.value.split('-')
    currentDate.value = new Date(Number(y), Number(m) - 1, 1)
  }
  loadData()
}

function onDayClick(day: string) {
  dayDialogTitle.value = day
  dayEntries.value = (result.value?.entries ?? []).filter((e) => e.start === day)
  // 加载当天的 note 进草稿
  const existing = noteByDate.value[day] ?? null
  currentNote.value = existing
  currentNoteDay.value = day
  noteDraft.value = existing?.content ?? ''
  noteDirty.value = false
  noteSaved.value = false
  dayDialogVisible.value = true
}

function onNoteInput() {
  noteDirty.value = noteDraft.value !== (currentNote.value?.content ?? '')
  noteSaved.value = false
}

async function onSaveNote() {
  if (!currentNoteDay.value) return
  // user 没选时用当前登录用户（后端 user 留空时无法定位；这里强制传 user）
  const user = selectedUser.value || (result.value?.userName ?? '')
  if (!user) {
    ElMessage.error('请先选择用户')
    return
  }
  noteSaving.value = true
  try {
    const res = await putDailyNote(user, currentNoteDay.value, noteDraft.value)
    if (res.code === 200 && res.data) {
      const saved: DailyNote = res.data
      noteByDate.value = { ...noteByDate.value, [saved.date]: saved }
      currentNote.value = saved
      noteDraft.value = saved.content
      noteDirty.value = false
      noteSaved.value = true
      ElMessage.success('已保存')
      // 1.5s 后清掉 "已保存" 标志
      setTimeout(() => {
        if (!noteDirty.value) noteSaved.value = false
      }, 1500)
    } else {
      ElMessage.error(res.message || '保存失败')
    }
  } catch (err) {
    ElMessage.error(`保存失败: ${(err as Error).message}`)
  } finally {
    noteSaving.value = false
  }
}

async function onClearNote() {
  if (!currentNoteDay.value) return
  const user = selectedUser.value || (result.value?.userName ?? '')
  if (!user) return
  try {
    await deleteDailyNote(user, currentNoteDay.value)
    // 清理本地缓存
    const next = { ...noteByDate.value }
    delete next[currentNoteDay.value]
    noteByDate.value = next
    currentNote.value = null
    noteDraft.value = ''
    noteDirty.value = false
    ElMessage.success('已清空')
  } catch (err) {
    ElMessage.error(`清空失败: ${(err as Error).message}`)
  }
}

function typeLabel(t: string): string {
  switch (t) {
    case 'task': return '任务'
    case 'bug': return 'Bug'
    case 'story': return '需求'
    case 'project': return '项目'
    default: return '其他'
  }
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

.empty-day {
  text-align: center;
  color: var(--el-text-color-secondary);
  padding: 24px 0;
}

/* === daily note 弹窗 === */
.note-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.note-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.note-title {
  font-weight: 600;
}

.note-status {
  font-size: 12px;
  padding: 1px 6px;
  border-radius: 3px;
}
.note-status-dirty {
  color: #d97706;
  background: #fef3c7;
}
.note-status-saved {
  color: #059669;
  background: #d1fae5;
}

.note-updated {
  margin-left: auto;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.note-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.day-entries {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.day-entry {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}

.entry-type {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 3px;
  font-size: 12px;
  color: #fff;
  min-width: 48px;
  text-align: center;
}

.entry-title {
  flex: 1;
  font-size: 13px;
}

.entry-hours {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-color-primary);
  min-width: 50px;
  text-align: right;
}

.entry-link {
  font-size: 12px;
  color: var(--el-color-primary);
  text-decoration: none;
}

.entry-link:hover {
  text-decoration: underline;
}

.day-total {
  margin-top: 12px;
  text-align: right;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

/* === 批量添加工时弹窗 === */
.batch-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.batch-row {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 0;
}

.batch-cell-date {
  display: flex;
  align-items: center;
  gap: 6px;
}

.batch-label {
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.batch-cell-stats {
  margin-left: auto;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.batch-stat strong {
  color: var(--el-color-primary);
  margin: 0 2px;
}

.batch-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 4px;
}

.batch-tip {
  margin-left: auto;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
