<template>
  <div class="page-container">
    <!-- 过滤栏 -->
    <div class="filter-card">
      <el-form :inline="true" class="filter-form">
        <el-form-item label="级别">
          <el-select v-model="level" placeholder="全部" clearable style="width: 120px" @change="fetchLogs">
            <el-option label="Debug" value="debug" />
            <el-option label="Info" value="info" />
            <el-option label="Warn" value="warn" />
            <el-option label="Error" value="error" />
            <el-option label="Fatal" value="fatal" />
          </el-select>
        </el-form-item>
        <el-form-item label="关键字">
          <el-input
            v-model="keyword"
            placeholder="搜索消息/调用方..."
            clearable
            style="width: 240px"
            @keyup.enter="fetchLogs"
            @clear="fetchLogs"
          />
        </el-form-item>
        <el-form-item label="行数">
          <el-select v-model="limit" style="width: 100px" @change="fetchLogs">
            <el-option :value="100" label="100" />
            <el-option :value="200" label="200" />
            <el-option :value="500" label="500" />
            <el-option :value="1000" label="1000" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchLogs">
            <el-icon><Search /></el-icon>刷新
          </el-button>
          <el-button @click="clearLogs" :disabled="loading">
            清空缓冲
          </el-button>
          <el-tooltip :content="autoRefresh ? '停止自动刷新' : '开始自动刷新（3秒）'" placement="top">
            <el-button :type="autoRefresh ? 'success' : 'default'" @click="toggleAutoRefresh">
              <el-icon><Refresh /></el-icon>
              {{ autoRefresh ? '自动中' : '自动刷新' }}
            </el-button>
          </el-tooltip>
        </el-form-item>
      </el-form>
    </div>

    <!-- 缓冲状态 -->
    <div class="status-bar">
      <span>缓冲: <strong>{{ bufferSize }}</strong> / 1000</span>
      <span>当前查询返回: <strong>{{ logs.length }}</strong> 条</span>
    </div>

    <!-- 日志表格 -->
    <div class="table-card">
      <el-table
        v-loading="loading"
        :data="logs"
        border
        stripe
        style="width: 100%"
        :default-sort="{ prop: 'time', order: 'descending' }"
        :row-class-name="rowClassName"
        max-height="70vh"
      >
        <el-table-column prop="time" label="时间" width="200" sortable>
          <template #default="{ row }">
            <span class="log-time">{{ formatTime(row.time) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="level" label="级别" width="90" align="center" :filters="levelFilters" :filter-method="filterLevel">
          <template #default="{ row }">
            <el-tag :type="levelTagType(row.level)" size="small" effect="dark">{{ row.level.toUpperCase() }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="caller" label="调用方" width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="log-caller">{{ row.caller || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="message" label="消息" min-width="400" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="log-message">{{ row.message }}</span>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="logs.length === 0 && !loading" class="empty-state">暂无日志</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { Search, Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api/api'
import type { ApiResponse } from '@/types/api'

interface LogEntry {
  time: string
  level: string
  message: string
  caller: string
}

interface LogsResponse {
  entries: LogEntry[]
  total: number
  buffer_size: number
}

const loading = ref(false)
const logs = ref<LogEntry[]>([])
const level = ref('')
const keyword = ref('')
const limit = ref(200)
const bufferSize = ref(0)
const autoRefresh = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const levelFilters = [
  { text: 'Error', value: 'error' },
  { text: 'Warn', value: 'warn' },
  { text: 'Info', value: 'info' },
  { text: 'Debug', value: 'debug' }
]

const fetchLogs = async (): Promise<void> => {
  loading.value = true
  try {
    const params: Record<string, string | number> = { limit: limit.value }
    if (level.value) params.level = level.value
    if (keyword.value.trim()) params.q = keyword.value.trim()
    const res = await api.get('/logs', { params }) as ApiResponse<LogsResponse>
    if (res?.data) {
      logs.value = res.data.entries || []
      bufferSize.value = res.data.buffer_size || 0
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : '获取日志失败'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

const clearLogs = async (): Promise<void> => {
  try {
    await ElMessageBox.confirm('确定要清空日志缓冲吗？此操作不可恢复。', '清空确认', {
      type: 'warning'
    })
  } catch {
    return // 用户取消
  }
  try {
    await api.delete('/logs')
    ElMessage.success('日志缓冲已清空')
    fetchLogs()
  } catch (e) {
    const msg = e instanceof Error ? e.message : '清空失败'
    ElMessage.error(msg)
  }
}

const toggleAutoRefresh = (): void => {
  if (autoRefresh.value) {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    autoRefresh.value = false
  } else {
    autoRefresh.value = true
    timer = setInterval(fetchLogs, 3000)
  }
}

const levelTagType = (lvl: string): 'danger' | 'warning' | 'success' | 'info' | 'primary' => {
  switch (lvl) {
    case 'error':
    case 'fatal':
    case 'dpanic':
      return 'danger'
    case 'warn':
      return 'warning'
    case 'info':
      return 'success'
    case 'debug':
      return 'info'
    default:
      return 'primary'
  }
}

const filterLevel = (value: string, row: LogEntry): boolean => row.level === value

const rowClassName = ({ row }: { row: LogEntry }): string => {
  if (row.level === 'error' || row.level === 'fatal') return 'log-row-error'
  if (row.level === 'warn') return 'log-row-warn'
  return ''
}

const formatTime = (t: string): string => {
  if (!t) return ''
  // ISO8601 已经可读，直接返回（前端按本地时区显示）
  return t
}

onMounted(() => {
  fetchLogs()
})

onBeforeUnmount(() => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
})
</script>

<style scoped>
.filter-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  padding: var(--space-md) var(--space-lg);
  margin-bottom: var(--space-md);
  box-shadow: var(--shadow-sm);
}
.status-bar {
  display: flex;
  gap: 24px;
  padding: 0 8px 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
}
.status-bar strong {
  color: var(--color-text-primary);
  font-weight: 600;
}
.table-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  padding: var(--space-md);
  box-shadow: var(--shadow-sm);
}
.log-time {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 12px;
  color: var(--color-text-secondary);
}
.log-caller {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.log-message {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 12px;
  word-break: break-all;
}
.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: var(--color-text-tertiary);
}

:deep(.log-row-error) {
  background-color: var(--color-danger-bg) !important;
}
:deep(.log-row-warn) {
  background-color: var(--color-warning-light) !important;
}
</style>
