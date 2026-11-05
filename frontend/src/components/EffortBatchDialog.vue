<template>
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
              :disabled-date="(d: Date) => d.getTime() > Date.now()"
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

        <el-table :data="batchItems" style="width: 100%" size="small" empty-text="暂无数据，点击下方 + 添加行 或 导入 Excel">
          <el-table-column label="日期" width="115">
            <template #default="{ row }">
              <el-input v-model="row.date" :placeholder="batchDate" size="small" style="width: 100%" />
            </template>
          </el-table-column>
          <el-table-column label="任务 / Bug" min-width="260">
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
          <el-button size="small" type="success" plain :loading="importing" @click="importFileRef?.click()">导入 Excel</el-button>
          <input
            ref="importFileRef"
            type="file"
            accept=".xlsx,.xls"
            style="display: none"
            @change="onImportFile"
          />
          <span class="batch-tip">提示：受限实例端点要求 objectType 格式为 task_xxx 或 bug_xxx；剩余留空将按任务当前剩余自动扣减</span>
        </div>
      </div>

      <template #footer>
        <el-button @click="batchDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchSubmitting" @click="onBatchSubmit">提交</el-button>
      </template>
    </el-dialog>
</template>

<script setup lang="ts">
/**
 * 批量添加工时弹窗（issue #7-4 从 EffortCalendar.vue 拆出）：行编辑 +
 * Excel 导入（复刻 skills/zentao-timelog zt.py 解析语义）。红线：先预览
 * （本表格即 dry-run）再提交；剩余留空 = 任务当前剩余按行累计扣减；已提交过的行
 * （同日同对象同消耗）自动跳过并提示。打开方式：batchDialogRef.open(date)。
 */
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchCreateEffort } from '@/api/zentao'
import { formatDate } from '@/utils/date'
import type { EffortCalendarEntry } from '@/types/api'

const props = defineProps<{
  taskOptions: { id: number; name: string; execution: number; left: number }[]
  /** 日历已有条目（台账式去重基准：同日+同对象+同消耗视为已提交） */
  existingEntries: EffortCalendarEntry[]
}>()

const emit = defineEmits<{
  /** 提交成功（父组件回读校验重拉日历） */
  submitted: []
}>()

const batchDialogVisible = ref(false)
const batchSubmitting = ref(false)
const batchDate = ref(formatDate(new Date())) // YYYY-MM-DD（本地时区）

/** 批量表单行（Excel 导入可跨天，空 date 用顶部 batchDate） */
interface BatchFormItem {
  date: string
  work: string
  objectType: string  // 'task_xxx' / 'bug_xxx'
  execution: number
  consumed: number
  left: number
}
const batchItems = ref<BatchFormItem[]>([])
const importing = ref(false)
const importFileRef = ref<HTMLInputElement | null>(null)

const validBatchCount = computed(() => batchItems.value.filter(i =>
  i.objectType && i.execution > 0 && i.consumed >= 0
).length)
const batchTotalConsumed = computed(() => batchItems.value.reduce((s, i) => s + (i.consumed || 0), 0))

function emptyBatchItem(): BatchFormItem {
  return { date: '', work: '', objectType: '', execution: 0, consumed: 0, left: 0 }
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
  if (!row.date) row.date = batchDate.value
  const [kind, idStr] = v.split('_')
  const id = Number(idStr)
  if (kind === 'task') {
    const t = props.taskOptions.find((x) => x.id === id)
    if (t) {
      row.execution = t.execution
      // 默认 left 用 task 当前剩余
      if (row.left === 0) row.left = t.left
    }
  }
}

// 已提交过的行（日历已有同日同对象同消耗）自动跳过并在导入时提示。

/** Excel 日期序列号 → YYYY-MM-DD（1899-12-30 起算） */
function excelSerialToDate(n: number): Date {
  return new Date(Math.round((n - 25569) * 86400000))
}

/** 归一日期：YYYY-MM-DD / YYYY/M/D / M.D / 今天 / 昨天 / Excel 序列号 / Date 对象 */
function normDate(v: unknown, base = new Date()): string | null {
  const fmt = (d: Date): string => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  if (v == null || v === '') return null
  if (v instanceof Date && !Number.isNaN(v.getTime())) return fmt(v)
  if (typeof v === 'number' && Number.isFinite(v)) {
    // Excel 日期序列号（常见 > 20000）；M.D 形态（如 9.16）按"今年月.日"解释
    if (v > 59) return fmt(excelSerialToDate(v))
    const m = String(v).match(/^(\d{1,2})\.(\d{1,2})$/)
    if (m) return fmt(new Date(base.getFullYear(), Number(m[1]) - 1, Number(m[2])))
    return null
  }
  const s = String(v).trim()
  if (s === '今天') return fmt(base)
  if (s === '昨天') { const d = new Date(base); d.setDate(d.getDate() - 1); return fmt(d) }
  let m = s.match(/^(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})日?$/)
  if (m) return `${m[1]}-${m[2].padStart(2, '0')}-${m[3].padStart(2, '0')}`
  m = s.match(/^(\d{1,2})[.月](\d{1,2})[.]?日?$/)
  if (m) return fmt(new Date(base.getFullYear(), Number(m[1]) - 1, Number(m[2])))
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? null : fmt(d)
}

/** 对象号归一：123456 / 任务#123456 / BUG#123456 / task_123456 → { type, id } */
function parseObjectNo(v: unknown, typeHint: string): { objectType: string; id: number } | null {
  if (v == null) return null
  const s = String(v).trim()
  if (!s) return null
  let m = s.match(/^(task|bug|任务|BUG|Bug)[#_:\-\s]*(\d+)$/i)
  if (m) {
    const kind = /^(bug|BUG|Bug)$/.test(m[1]) ? 'bug' : 'task'
    return { objectType: `${kind}_${m[2]}`, id: Number(m[2]) }
  }
  m = s.match(/^(\d+)$/)
  if (m) {
    const kind = typeHint.toLowerCase().startsWith('bug') ? 'bug' : 'task'
    return { objectType: `${kind}_${m[1]}`, id: Number(m[1]) }
  }
  return null
}

// 台账去重键：日期|工作内容|消耗（日历 entry.url 是 effort-view 不含对象 ID，
// 与 zt.py 台账语义一致按"内容+日期+消耗"判重）

const HEADER_HINTS = ['日期', '类型', '对象号', '工作内容', '消耗', '剩余', '测试包版本']

async function onImportFile(ev: Event): Promise<void> {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  importing.value = true
  try {
    const XLSX = await import('xlsx')
    const buf = await file.arrayBuffer()
    // cellDates: 日期单元格直接给 Date，避免序列号歧义
    const wb = XLSX.read(buf, { cellDates: true })
    const sheet = wb.Sheets[wb.SheetNames[0]]
    const rows = XLSX.utils.sheet_to_json<Record<string, unknown>>(sheet, { defval: '', raw: false })

    // 前 10 行内定位表头行（sheet_to_json 已按第一个非空行做表头，这里按列名做别名映射）
    const pick = (row: Record<string, unknown>, keys: string[]): unknown => {
      for (const k of keys) {
        for (const col of Object.keys(row)) {
          if (col.trim() === k) return row[col]
        }
      }
      return undefined
    }
    // 校验表头：至少命中 3 个提示词，否则提示模板不符
    const cols = rows.length > 0 ? Object.keys(rows[0]) : []
    const hit = cols.filter((c) => HEADER_HINTS.some((h) => c.includes(h))).length
    if (rows.length === 0 || hit < 3) {
      ElMessage.error('未识别到表头（需要包含：日期/类型/对象号/工作内容/消耗工时 列）。请使用模板格式，表头前允许有说明行。')
      return
    }

    const items: BatchFormItem[] = []
    const invalidRows: string[] = []
    let skippedComment = 0
    const fileSeen = new Set<string>() // 文件内去重
    let skippedDupInFile = 0
    // 台账式去重：日历已有 同日 + 同对象 + 同消耗 的记录视为已提交
    const existing = new Set<string>()
    for (const e of props.existingEntries) {
      existing.add(`${e.start}|${String(e.title || '').trim()}|${e.consumed}`)
    }
    let skippedAlreadySubmitted = 0

    // 剩余工时自动计算基准：任务当前剩余，同一任务多行按提交顺序累计扣减
    const taskLeftBase = new Map<number, number>()
    const taskExec = new Map<number, number>()

    for (const row of rows) {
      const objNo = pick(row, ['对象号', '任务/Bug', '任务号', 'Bug号', '对象'])
      const work = String(pick(row, ['工作内容', '内容', '工作']) ?? '').trim()
      // 注释行：对象号或内容以 # 开头
      if (String(objNo ?? '').trim().startsWith('#') || work.startsWith('#')) { skippedComment++; continue }
      const typeHint = String(pick(row, ['类型']) ?? '')
      const parsed = parseObjectNo(objNo, typeHint)
      const consumed = Number(pick(row, ['消耗工时', '消耗', '工时']) ?? 0) || 0
      const leftRaw = pick(row, ['剩余工时', '剩余'])
      const dateStr = normDate(pick(row, ['日期']), new Date(batchDate.value))
      // 说明行/空行：对象号与内容都空
      if (!parsed && !work) { skippedComment++; continue }
      if (!parsed || consumed <= 0 || !dateStr) {
        invalidRows.push(`第 ${rows.indexOf(row) + 2} 行：${!parsed ? '对象号无法识别' : consumed <= 0 ? '消耗工时必须大于 0' : '日期无法识别'}`)
        continue
      }

      // 台账去重（禅道侧已有）与文件内去重
      const key = `${dateStr}|${work}|${consumed}`
      if (existing.has(key)) { skippedAlreadySubmitted++; continue }
      if (fileSeen.has(key)) { skippedDupInFile++; continue }
      fileSeen.add(key)

      const taskId = parsed.objectType.startsWith('task_') ? parsed.id : 0
      const known = props.taskOptions.find((t) => t.id === parsed.id)
      if (taskId && known) {
        if (!taskLeftBase.has(parsed.id)) taskLeftBase.set(parsed.id, known.left)
        if (!taskExec.has(parsed.id)) taskExec.set(parsed.id, known.execution)
      }
      let left: number
      const leftManual = String(leftRaw ?? '').trim()
      if (leftManual !== '' && leftManual !== undefined && !Number.isNaN(Number(leftManual))) {
        // 手填 = 绝对值（覆盖任务当前剩余）
        left = Number(leftManual)
      } else if (taskId && taskLeftBase.has(taskId)) {
        // 自动：当前剩余 − 本次消耗，同任务多行累计扣减
        left = Math.max(0, (taskLeftBase.get(taskId) ?? 0) - consumed)
        taskLeftBase.set(taskId, left)
      } else {
        // Bug 工时或找不到的任务：剩余无基准，置 0（弹窗内可手改）
        left = 0
      }
      items.push({
        date: dateStr,
        work,
        objectType: parsed.objectType,
        execution: taskExec.get(parsed.id) ?? known?.execution ?? 0,
        consumed,
        left
      })
    }

    if (items.length === 0) {
      ElMessage.error(`没有可导入的行。${invalidRows.length ? '问题：' + invalidRows.slice(0, 3).join('；') : ''}`)
      return
    }
    batchItems.value = items
    const parts = [`导入 ${items.length} 行`]
    if (skippedAlreadySubmitted) parts.push(`禅道已有 ${skippedAlreadySubmitted} 行（跳过）`)
    if (skippedDupInFile) parts.push(`文件内重复 ${skippedDupInFile} 行（跳过）`)
    if (skippedComment) parts.push(`注释/空行 ${skippedComment} 行`)
    if (invalidRows.length) parts.push(`无效 ${invalidRows.length} 行`)
    const warn = [
      invalidRows.length ? '无效行：\n' + invalidRows.slice(0, 5).join('\n') : '',
      '剩余为 0 的行会把任务置为完成（受限实例还要求测试包版本），如非交付请手动改剩余。'
    ].filter(Boolean).join('\n\n')
    await ElMessageBox.alert(`${parts.join('，')}。请核对下方列表后再提交。${warn ? '\n\n' + warn : ''}`, '导入完成', { type: 'success', customStyle: { whiteSpace: 'pre-line' } }).catch(() => {})
  } catch (err) {
    console.error('Excel 解析失败:', err)
    ElMessage.error(`Excel 解析失败: ${(err as Error).message}`)
  } finally {
    importing.value = false
    input.value = ''
  }
}

function open(date: string): void {
  batchDate.value = date
  if (batchItems.value.length === 0) {
    batchItems.value = [emptyBatchItem()]
  }
  batchDialogVisible.value = true
}

async function onBatchSubmit() {
  const valid = batchItems.value.filter((i) => i.objectType && i.execution > 0)
  if (valid.length === 0) {
    ElMessage.error('至少需要一条有效记录（选择任务/Bug 且 execution 不为空）')
    return
  }
  // 按行日期分组（Excel 导入可跨天），空日期回落到顶部 batchDate
  const byDate = new Map<string, BatchFormItem[]>()
  for (const i of valid) {
    const d = i.date || batchDate.value
    if (!byDate.has(d)) byDate.set(d, [])
    byDate.get(d)!.push(i)
  }
  for (const d of byDate.keys()) {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(d)) {
      ElMessage.error(`日期格式不正确：${d}（应为 YYYY-MM-DD）`)
      return
    }
  }
  batchSubmitting.value = true
  try {
    let created = 0
    let skipped = 0
    // 逐天提交（batchCreate 端点按日期组织表单），任一天失败即中止并提示已完成部分
    for (const [date, items] of byDate) {
      const payload = {
        date,
        items: items.map((i) => ({
          work: i.work,
          objectType: i.objectType,
          execution: i.execution,
          consumed: i.consumed,
          left: i.left,
        })),
      }
      const res = await batchCreateEffort(payload)
      if (res.code === 200 && res.data) {
        created += res.data.created
        skipped += res.data.skipped
      } else {
        ElMessage.error(`${date} 提交失败: ${res.message || '未知错误'}（此前日期已写入的记录保留）`)
        return
      }
    }
    ElMessage.success(`已写入 ${created} 条，跳过 ${skipped} 条空行`)
    batchDialogVisible.value = false
    // 回读校验由父组件负责：重新拉当前 year 的日历核对数值
    emit('submitted')
  } catch (err) {
    ElMessage.error(`提交失败: ${(err as Error).message}`)
  } finally {
    batchSubmitting.value = false
  }
}

defineExpose({ open })
</script>

<style scoped>
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
