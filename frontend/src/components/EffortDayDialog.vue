<template>
    <!-- 当日明细弹窗 -->
    <el-dialog
      v-model="visible"
      :title="title"
      width="680px"
      append-to-body
    >
      <!-- 这天我干了什么（自由文本） -->
      <div class="note-section">
        <div class="note-header">
          <span class="note-title">📝 这天我干了什么</span>
          <span v-if="dirty" class="note-status note-status-dirty">未保存</span>
          <span v-else-if="savedFlag" class="note-status note-status-saved">已保存 ✓</span>
          <span v-if="currentNote" class="note-updated">
            {{ formatNoteTime(currentNote.updatedAt) }}
          </span>
        </div>
        <el-input
          v-model="draft"
          type="textarea"
          :rows="4"
          placeholder="今天做 X / 会议 / 明日计划…（留空也可保存）"
          maxlength="2000"
          show-word-limit
          @input="onInput"
        />
        <!-- 同步为工时：备注内容挂到指定任务（禅道日志必须挂对象） -->
        <div class="note-effort-sync">
          <el-checkbox v-model="syncEnabled">同步为工时（挂到任务）</el-checkbox>
          <template v-if="syncEnabled">
            <el-select
              v-model="syncTask"
              placeholder="选择任务"
              filterable
              size="small"
              style="width: 100%; margin-top: 8px"
            >
              <el-option-group label="我负责的任务">
                <el-option
                  v-for="t in taskOptions"
                  :key="`note-${t.id}`"
                  :label="`#${t.id} ${t.name} (剩 ${t.left}h)`"
                  :value="`task_${t.id}`"
                />
              </el-option-group>
            </el-select>
            <div style="margin-top: 8px; display: flex; align-items: center; gap: 8px">
              <span class="note-sync-label">消耗(h)</span>
              <el-input-number v-model="syncConsumed" :min="0.5" :max="24" :step="0.5" :precision="1" size="small" />
              <span class="note-sync-label" v-if="syncTask">{{ currentDay }}</span>
            </div>
          </template>
        </div>
        <div class="note-actions">
          <el-button size="small" @click="onClear" :disabled="!draft">清空</el-button>
          <el-button size="small" type="primary" :loading="saving" @click="onSave">
            {{ syncEnabled && syncTask ? '保存并挂工时' : '保存' }}
          </el-button>
        </div>
      </div>

      <el-divider />

      <!-- 当日工时记录（禅道 effort） -->
      <div v-if="entries.length === 0" class="empty-day">该日无禅道工时记录</div>
      <div v-else class="day-entries">
        <div v-for="e in entries" :key="e.id" class="day-entry">
          <span class="entry-type" :class="`type-${e.objectType}`">{{ typeLabel(e.objectType) }}</span>
          <span class="entry-title">{{ e.title }}</span>
          <span class="entry-hours">{{ e.consumed.toFixed(1) }}h</span>
          <a v-if="e.url" :href="originBase + e.url" target="_blank" rel="noopener" class="entry-link">查看</a>
        </div>
        <div class="day-total">
          合计: <strong>{{ total.toFixed(1) }}</strong> 小时 · {{ entries.length }} 条
        </div>
      </div>
    </el-dialog>
</template>

<script setup lang="ts">
/**
 * 当日明细弹窗：禅道 effort 记录 + daily note 自由文本（issue #7-4 从
 * EffortCalendar.vue 拆出）。「同步为工时」把备注内容挂到指定任务。
 * 打开方式：dayDialogRef.open(day, entries, note)；保存/清空结果经
 * saved/cleared 事件回传父组件维护缓存，挂工时成功后经 synced 通知父组件刷新日历。
 */
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { putDailyNote, deleteDailyNote, batchCreateEffort, type DailyNote } from '@/api/zentao'
import type { EffortCalendarEntry } from '@/types/api'

const props = defineProps<{
  /** 目标用户账号（保存 note 必填；父组件解析后传入） */
  user: string
  taskOptions: { id: number; name: string; execution: number; left: number }[]
}>()

const emit = defineEmits<{
  saved: [note: DailyNote]
  cleared: [day: string]
  synced: []
}>()

const visible = ref(false)
const title = ref('')
const entries = ref<EffortCalendarEntry[]>([])
const total = computed(() => entries.value.reduce((s, e) => s + e.consumed, 0))

// Daily note 状态
const draft = ref('')
const saving = ref(false)
const dirty = ref(false)   // 草稿未保存
const savedFlag = ref(false) // 刚保存成功（短暂显示 "已保存"）
const currentNote = ref<DailyNote | null>(null)
const currentDay = ref('')

// 同步为工时：备注内容挂到指定任务（禅道日志必须挂对象，纯文本不算日志）
const syncEnabled = ref(false)
const syncTask = ref('')
const syncConsumed = ref(0.5)

// 禅道 base（从当前 location 推断，给 entry url 拼前缀用）
const originBase = computed(() => window.location.origin)

function formatNoteTime(s?: string): string {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
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

function open(day: string, dayEntries: EffortCalendarEntry[], note: DailyNote | null): void {
  title.value = day
  entries.value = dayEntries
  currentNote.value = note
  currentDay.value = day
  draft.value = note?.content ?? ''
  dirty.value = false
  savedFlag.value = false
  visible.value = true
}

function onInput(): void {
  dirty.value = draft.value !== (currentNote.value?.content ?? '')
  savedFlag.value = false
}

async function onSave(): Promise<void> {
  if (!currentDay.value) return
  if (!props.user) {
    ElMessage.error('请先选择用户')
    return
  }
  saving.value = true
  try {
    const res = await putDailyNote(props.user, currentDay.value, draft.value)
    if (res.code === 200 && res.data) {
      const savedNote: DailyNote = res.data
      currentNote.value = savedNote
      draft.value = savedNote.content
      dirty.value = false
      savedFlag.value = true
      emit('saved', savedNote)
      // 同步为工时：备注内容作为 work 挂到选中任务
      let syncMsg = ''
      if (syncEnabled.value && syncTask.value) {
        const taskId = Number(syncTask.value.split('_')[1])
        const t = props.taskOptions.find((x) => x.id === taskId)
        try {
          const er = await batchCreateEffort({
            date: currentDay.value,
            items: [{
              work: draft.value.trim(),
              objectType: syncTask.value,
              execution: t?.execution || 0,
              consumed: syncConsumed.value,
              left: t ? Math.max(0, t.left - syncConsumed.value) : 0
            }]
          })
          if (er.code === 200 && er.data && er.data.created > 0) {
            syncMsg = `，已挂工时到 #${taskId}（${syncConsumed.value}h）`
            emit('synced')
          } else {
            syncMsg = '，但挂工时失败：' + (er.message || '未知错误')
          }
        } catch (syncErr) {
          syncMsg = '，但挂工时失败：' + (syncErr as Error).message
        }
      }
      if (syncMsg.includes('失败')) {
        ElMessage.warning('备注已保存' + syncMsg)
      } else {
        ElMessage.success('已保存' + syncMsg)
      }
      setTimeout(() => {
        if (!dirty.value) savedFlag.value = false
      }, 1500)
    } else {
      ElMessage.error(res.message || '保存失败')
    }
  } catch (err) {
    ElMessage.error(`保存失败: ${(err as Error).message}`)
  } finally {
    saving.value = false
  }
}

async function onClear(): Promise<void> {
  if (!currentDay.value || !props.user) return
  try {
    await deleteDailyNote(props.user, currentDay.value)
    currentNote.value = null
    draft.value = ''
    dirty.value = false
    emit('cleared', currentDay.value)
    ElMessage.success('已清空')
  } catch (err) {
    ElMessage.error(`清空失败: ${(err as Error).message}`)
  }
}

defineExpose({ open })
</script>

<style scoped>
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

.note-effort-sync {
  margin-top: 10px;
  padding: 8px 10px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}

.note-sync-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
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
</style>
