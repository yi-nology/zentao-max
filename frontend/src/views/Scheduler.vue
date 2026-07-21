<template>
  <div class="scheduler-page">
    <!-- Stats Cards -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon stat-icon--primary">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
        </div>
        <div class="stat-info">
          <span class="stat-label">定时任务</span>
          <span class="stat-value">{{ tasks.length }}</span>
          <span class="stat-sub">启用 {{ enabledCount }} / 禁用 {{ tasks.length - enabledCount }}</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon--success">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
        </div>
        <div class="stat-info">
          <span class="stat-label">执行成功</span>
          <span class="stat-value">{{ successCount }}</span>
          <span class="stat-sub">最近 {{ logs.length }} 次执行</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon--warning">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" /></svg>
        </div>
        <div class="stat-info">
          <span class="stat-label">执行失败</span>
          <span class="stat-value">{{ failedCount }}</span>
          <span class="stat-sub">失败率 {{ failRate }}%</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon--info">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 20H5a2 2 0 01-2-2V6a2 2 0 012-2h10a2 2 0 012 2v1m2 13a2 2 0 01-2-2V7m2 13a2 2 0 002-2V9a2 2 0 00-2-2h-2m-4-3H9M7 16h6M7 8h6v4H7V8z" /></svg>
        </div>
        <div class="stat-info">
          <span class="stat-label">执行日志</span>
          <span class="stat-value">{{ logs.length }}</span>
          <span class="stat-sub">最近 100 条</span>
        </div>
      </div>
    </div>

    <!-- Tabs -->
    <div class="page-tabs">
      <button class="tab-btn" :class="{ active: activeTab === 'tasks' }" @click="activeTab = 'tasks'">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:16px;height:16px"><path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" /></svg>
        任务列表
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'logs' }" @click="activeTab = 'logs'">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:16px;height:16px"><path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
        执行日志
      </button>
      <div style="flex:1" />
      <button v-if="activeTab === 'tasks'" class="btn btn-primary" @click="openCreateDialog">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:16px;height:16px"><path d="M12 5v14M5 12h14" /></svg>
        新建任务
      </button>
    </div>

    <!-- Task List -->
    <div v-if="activeTab === 'tasks'" class="card">
      <div v-if="loading" class="empty-state">加载中...</div>
      <div v-else-if="tasks.length === 0" class="empty-state">暂无定时任务，点击右上角「新建任务」创建</div>
      <table v-else class="data-table">
        <thead>
          <tr>
            <th>任务名称</th>
            <th>报告类型</th>
            <th>Cron 表达式</th>
            <th>项目</th>
            <th>Webhooks</th>
            <th>状态</th>
            <th>上次执行</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="task in tasks" :key="task.id">
            <td class="task-name">{{ task.name }}</td>
            <td>
              <span class="report-type-badge" :class="'report-' + (task.reportType || 'bug')">
                {{ reportTypeLabel(task.reportType || 'bug') }}
              </span>
            </td>
            <td><code class="cron-code">{{ task.cronExpr }}</code></td>
            <td>{{ task.projectName || task.productName || task.projectId || '-' }}</td>
            <td>
              <span v-for="wh in task.webhooks" :key="wh.id" class="webhook-tag" :class="{ disabled: !wh.enabled }">
                {{ wh.name || wh.platform || 'webhook' }}
              </span>
              <span v-if="(task.webhooks || []).length === 0" class="text-muted">无</span>
            </td>
            <td>
              <span class="status-badge" :class="task.enabled ? 'status-on' : 'status-off'" @click="handleToggle(task)">
                {{ task.enabled ? '已启用' : '已禁用' }}
              </span>
            </td>
            <td>
              <template v-if="task.lastRunAt">
                <div class="text-sm">{{ formatTime(task.lastRunAt) }}</div>
                <span class="run-status" :class="'run-' + task.lastRunStatus">{{ runStatusLabel(task.lastRunStatus) }}</span>
              </template>
              <span v-else class="text-muted">未执行</span>
            </td>
            <td class="actions-cell">
              <button class="btn btn-sm" @click="handleRunNow(task)" title="立即执行">▶</button>
              <button class="btn btn-sm" @click="openEditDialog(task)" title="编辑">✎</button>
              <button class="btn btn-sm btn-danger" @click="handleDelete(task)" title="删除">✕</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Logs -->
    <div v-if="activeTab === 'logs'" class="card">
      <div v-if="logsLoading" class="empty-state">加载中...</div>
      <div v-else-if="logs.length === 0" class="empty-state">暂无执行日志</div>
      <table v-else class="data-table">
        <thead>
          <tr>
            <th>任务名称</th>
            <th>执行时间</th>
            <th>耗时</th>
            <th>状态</th>
            <th>数量</th>
            <th>高级别</th>
            <th>指派人</th>
            <th>Webhook结果</th>
            <th>错误信息</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in logs" :key="log.id">
            <td>{{ log.taskName }}</td>
            <td class="text-sm">{{ formatTime(log.startedAt) }}</td>
            <td class="text-sm">{{ log.finishedAt ? calcDuration(log.startedAt, log.finishedAt) : '-' }}</td>
            <td><span class="run-status" :class="'run-' + log.status">{{ runStatusLabel(log.status) }}</span></td>
            <td>{{ log.bugTotal || 0 }}</td>
            <td>{{ log.highSeverity || 0 }}</td>
            <td>{{ log.assigneeCount || 0 }}</td>
            <td>
              <span v-for="wr in log.webhookResults || []" :key="wr.webhookId" class="wh-result" :class="wr.success ? 'wh-ok' : 'wh-fail'" :title="wr.webhookName + ': ' + (wr.error || 'OK')">
                {{ wr.success ? '✓' : '✗' }} {{ wr.webhookName }}
              </span>
              <span v-if="(log.webhookResults || []).length === 0" class="text-muted">-</span>
            </td>
            <td class="error-cell">{{ log.error || '-' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create/Edit Dialog -->
    <div v-if="dialogVisible" class="dialog-overlay" @click.self="dialogVisible = false">
      <div class="dialog dialog-lg">
        <div class="dialog-header">
          <h3>{{ isEdit ? '编辑任务' : '新建任务' }}</h3>
          <button class="dialog-close" @click="dialogVisible = false">✕</button>
        </div>
        <div class="dialog-body">
          <!-- Report Type Selection -->
          <div class="form-group">
            <label>报告类型 <span class="required">*</span></label>
            <div class="report-type-grid">
              <div
                v-for="rt in REPORT_TYPE_OPTIONS"
                :key="rt.value"
                class="report-type-card"
                :class="{ active: form.reportType === rt.value }"
                @click="form.reportType = rt.value"
              >
                <span class="rtc-icon">{{ rt.icon }}</span>
                <span class="rtc-label">{{ rt.label }}</span>
                <span class="rtc-desc">{{ reportTypeDesc(rt.value) }}</span>
              </div>
            </div>
          </div>

          <div class="form-group">
            <label>任务名称 <span class="required">*</span></label>
            <input v-model="form.name" class="form-input" :placeholder="namePlaceholder" />
          </div>

          <div class="form-row">
            <div class="form-group form-half">
              <label>产品 <span class="required">*</span></label>
              <select v-model="form.productId" class="form-input" @change="onProductChange">
                <option value="">请选择产品</option>
                <option v-for="p in products" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select>
            </div>
            <div class="form-group form-half">
              <label>项目</label>
              <select v-model="form.projectId" class="form-input" @change="onProjectChange">
                <option value="">请选择项目（可选）</option>
                <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select>
            </div>
          </div>

          <div class="form-row">
            <div class="form-group form-half">
              <label>Cron 表达式 <span class="required">*</span></label>
              <input v-model="form.cronExpr" class="form-input" placeholder="0 9 * * 1-5" />
              <div class="cron-presets">
                <button v-for="preset in CRON_PRESETS" :key="preset.expr" class="preset-btn" @click="form.cronExpr = preset.expr">
                  {{ preset.label }}
                </button>
              </div>
            </div>
            <div class="form-group form-half" v-if="form.reportType === 'bug' || form.reportType === 'bug-aging'">
              <label>Bug 状态过滤</label>
              <select v-model="form.statusFilter" class="form-input">
                <option v-for="opt in STATUS_OPTIONS" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
              </select>
            </div>
            <div class="form-group form-half" v-if="form.reportType === 'bug-aging'">
              <label>超时阈值</label>
              <select v-model="form.agingDays" class="form-input">
                <option v-for="opt in AGING_DAYS_OPTIONS" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
              </select>
            </div>
          </div>

          <div class="form-row">
            <div class="form-group form-half">
              <label>消息关键词</label>
              <input v-model="form.keyword" class="form-input" placeholder="如：提醒" />
              <div class="hint-text">填写后消息开头会自动加上【关键词】</div>
            </div>
            <div class="form-group form-half">
              <label>消息头备注</label>
              <input v-model="form.messageHeader" class="form-input" placeholder="如：详情查看 https://..." />
              <div class="hint-text">会显示在消息标题下方，所有报告类型通用</div>
            </div>
          </div>

          <div class="form-row">
            <div class="form-group">
              <label>优先人员</label>
              <div class="priority-hint">输入账号，按回车添加，优先展示在消息前面</div>
              <div class="priority-tags">
                <span v-for="(name, idx) in form.priorityAssignees" :key="idx" class="priority-tag">
                  {{ name }}
                  <button class="tag-remove" @click="form.priorityAssignees.splice(idx, 1)">✕</button>
                </span>
              </div>
              <input
                class="form-input"
                placeholder="输入账号后按回车添加"
                @keydown.enter.prevent="(e) => {
                  const val = (e.target as HTMLInputElement).value.trim()
                  if (val && !form.priorityAssignees.includes(val)) {
                    form.priorityAssignees.push(val)
                  }
                  (e.target as HTMLInputElement).value = ''
                }"
              />
            </div>
          </div>

          <div class="form-row">
            <div class="form-group">
              <label>外部信息</label>
              <textarea v-model="form.externalInfo" class="form-input form-textarea" placeholder="附加信息，会展示在消息中" rows="2"></textarea>
            </div>
          </div>

          <!-- Preview Section -->
          <div class="preview-section">
            <div class="preview-header">
              <label>报告预览</label>
              <button class="btn btn-sm" @click="handlePreview" :disabled="previewLoading || !form.productId">
                {{ previewLoading ? '生成中...' : '预览报告' }}
              </button>
            </div>
            <div v-if="previewResult" class="preview-content">
              <pre class="preview-message">{{ previewResult.message }}</pre>
            </div>
            <div v-else-if="previewError" class="preview-error">{{ previewError }}</div>
            <div v-else class="preview-placeholder">点击「预览报告」查看播报内容</div>
          </div>

          <!-- Webhook Section -->
          <div class="form-group">
            <label>Webhook 列表</label>
            <div v-for="(wh, idx) in form.webhooks" :key="idx" class="webhook-block">
              <div class="webhook-row">
                <input v-model="wh.name" class="form-input wh-name" :placeholder="wh.platform === 'lanxin' ? '蓝信群' : 'Webhook'" />
                <select v-model="wh.platform" class="form-input wh-platform">
                  <option value="generic">通用</option>
                  <option value="lanxin">蓝信</option>
                </select>
                <label class="wh-enable">
                  <input type="checkbox" v-model="wh.enabled" />
                  启用
                </label>
                <button class="btn btn-sm btn-danger" @click="form.webhooks.splice(idx, 1)">✕</button>
              </div>
              <div class="webhook-row" style="margin-top:6px">
                <input v-model="wh.url" class="form-input wh-url-full" placeholder="Webhook URL (https://...)" />
              </div>
              <div v-if="wh.platform === 'lanxin'" class="webhook-row" style="margin-top:6px">
                <input v-model="wh.secret" class="form-input wh-url-full" placeholder="蓝信签名密钥 (可选)" />
              </div>
              <div class="webhook-row" style="margin-top:4px">
                <label class="wh-enable">
                  <input type="checkbox" v-model="wh.skipSSL" />
                  跳过SSL验证
                </label>
                <button class="btn btn-sm" @click="handleTestWebhook(wh.url)" :disabled="!wh.url" style="margin-left:auto">测试</button>
              </div>
              <div v-if="testResult && testResult.webhookUrl === wh.url" class="test-result" :class="testResult.success ? 'test-ok' : 'test-fail'">
                {{ testResult.success ? '连接成功' : '连接失败: ' + testResult.error }}
              </div>
            </div>
            <button class="btn btn-sm" @click="form.webhooks.push({ id: '', name: 'Webhook ' + (form.webhooks.length + 1), url: '', enabled: true, platform: 'generic', secret: '', skipSSL: false })">+ 添加 Webhook</button>
          </div>
        </div>
        <div class="dialog-footer">
          <button class="btn" @click="dialogVisible = false">取消</button>
          <button class="btn btn-primary" @click="handleSubmit" :disabled="submitting">
            {{ submitting ? '提交中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Run Result Dialog -->
    <div v-if="runResultVisible" class="dialog-overlay" @click.self="runResultVisible = false">
      <div class="dialog">
        <div class="dialog-header">
          <h3>手动执行结果</h3>
          <button class="dialog-close" @click="runResultVisible = false">✕</button>
        </div>
        <div class="dialog-body">
          <div v-if="runResult" class="run-result-info">
            <div class="result-row">
              <span class="result-label">状态：</span>
              <span class="run-status" :class="'run-' + runResult.status">{{ runStatusLabel(runResult.status) }}</span>
            </div>
            <div class="result-row">
              <span class="result-label">总数：</span>
              <span>{{ runResult.bugTotal }}</span>
            </div>
            <div v-if="runResult.highSeverity > 0" class="result-row">
              <span class="result-label">高级别：</span>
              <span>{{ runResult.highSeverity }}</span>
            </div>
            <div v-if="runResult.webhookResults?.length > 0" class="result-webhooks">
              <div class="result-label">Webhook 结果：</div>
              <div v-for="wr in runResult.webhookResults" :key="wr.webhookId" class="wh-result-item">
                <span :class="wr.success ? 'wh-ok' : 'wh-fail'">{{ wr.success ? '✓' : '✗' }}</span>
                {{ wr.webhookName }} ({{ wr.webhookUrl }})
                <span v-if="wr.error" class="text-muted">- {{ wr.error }}</span>
              </div>
            </div>
            <div v-if="runResult.error" class="result-error">{{ runResult.error }}</div>
          </div>
        </div>
        <div class="dialog-footer">
          <button class="btn" @click="runResultVisible = false">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, watch, computed } from 'vue'
import {
  listTasks, createTask, updateTask, deleteTask, toggleTask,
  runTaskNow, getAllLogs, testWebhook, previewReport
} from '@/api/scheduler'
import type { PreviewParams } from '@/api/scheduler'
import { getProducts, getProjects } from '@/api/zentao'
import { ElMessageBox } from 'element-plus'
import type { SchedulerTask, TaskExecutionLog, WebhookResult, WebhookConfig, RequirementReport, TaskProgressReport, BugReport, BugAgingReport } from '@/types/scheduler'
import { CRON_PRESETS, STATUS_OPTIONS, REPORT_TYPE_OPTIONS, AGING_DAYS_OPTIONS } from '@/types/scheduler'

interface Product { id: number; name: string }
interface Project { id: number; name: string }

const activeTab = ref<'tasks' | 'logs'>('tasks')
const tasks = ref<SchedulerTask[]>([])
const logs = ref<TaskExecutionLog[]>([])
const loading = ref(false)
const logsLoading = ref(false)
const products = ref<Product[]>([])
const projects = ref<Project[]>([])

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitting = ref(false)
const testResult = ref<WebhookResult | null>(null)
const runResultVisible = ref(false)
const runResult = ref<TaskExecutionLog | null>(null)

const previewLoading = ref(false)
const previewResult = ref<RequirementReport | TaskProgressReport | BugReport | BugAgingReport | null>(null)
const previewError = ref('')

const form = reactive<{
  id: string
  name: string
  productId: number | string
  projectId: number | string
  projectName: string
  productName: string
  cronExpr: string
  statusFilter: string
  reportType: string
  agingDays: number
  priorityAssignees: string[]
  messageHeader: string
  keyword: string
  externalInfo: string
  enabled: boolean
  webhooks: WebhookConfig[]
}>({
  id: '',
  name: '',
  productId: '',
  projectId: '',
  projectName: '',
  productName: '',
  cronExpr: '0 9 * * 1-5',
  statusFilter: 'active',
  reportType: 'bug',
  agingDays: 7,
  priorityAssignees: [],
  messageHeader: '',
  keyword: '提醒',
  externalInfo: '',
  enabled: true,
  webhooks: [{ id: '', name: '', url: '', enabled: true, platform: 'generic', secret: '', skipSSL: false }],
})

const enabledCount = computed(() => tasks.value.filter(t => t.enabled).length)
const successCount = computed(() => logs.value.filter(l => l.status === 'success').length)
const failedCount = computed(() => logs.value.filter(l => l.status === 'failed').length)
const failRate = computed(() => {
  if (logs.value.length === 0) return 0
  return Math.round((failedCount.value / logs.value.length) * 100)
})

const namePlaceholder = computed(() => {
  const m: Record<string, string> = {
    bug: '如：每日Bug分布报告',
    requirement: '如：需求进度播报',
    task: '如：任务进度播报',
    'bug-aging': '如：Bug超时提醒',
  }
  return m[form.reportType] || '任务名称'
})

onMounted(() => {
  loadTasks()
  loadProducts()
})

const loadTasks = async () => {
  loading.value = true
  try {
    const res = await listTasks()
    tasks.value = res.data || []
  } catch { tasks.value = [] }
  finally { loading.value = false }
}

const loadLogs = async () => {
  logsLoading.value = true
  try {
    const res = await getAllLogs()
    logs.value = res.data || []
  } catch { logs.value = [] }
  finally { logsLoading.value = false }
}

const loadProducts = async () => {
  try {
    const res = await getProducts()
    products.value = res.data || []
  } catch { products.value = [] }
}

const onProductChange = async () => {
  form.projectId = ''
  projects.value = []
  if (!form.productId) return
  try {
    const res = await getProjects({ productId: Number(form.productId) })
    projects.value = res.data || []
  } catch { projects.value = [] }
}

const onProjectChange = () => {
  const proj = projects.value.find(p => p.id === Number(form.projectId))
  if (proj) form.projectName = proj.name
  const prod = products.value.find(p => p.id === Number(form.productId))
  if (prod) form.productName = prod.name
}

const resetForm = () => {
  form.id = ''
  form.name = ''
  form.productId = ''
  form.projectId = ''
  form.projectName = ''
  form.productName = ''
  form.cronExpr = '0 9 * * 1-5'
  form.statusFilter = 'active'
  form.reportType = 'bug'
  form.agingDays = 7
  form.priorityAssignees = []
  form.messageHeader = ''
  form.keyword = '提醒'
  form.externalInfo = ''
  form.enabled = true
  form.webhooks = [{ id: '', name: '', url: '', enabled: true, platform: 'generic', secret: '', skipSSL: false }]
  testResult.value = null
  previewResult.value = null
  previewError.value = ''
}

const openCreateDialog = () => {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

const openEditDialog = (task: SchedulerTask) => {
  isEdit.value = true
  form.id = task.id
  form.name = task.name
  form.productId = task.productId
  form.projectId = task.projectId
  form.projectName = task.projectName
  form.productName = task.productName
  form.cronExpr = task.cronExpr
  form.statusFilter = task.statusFilter
  form.reportType = task.reportType || 'bug'
  form.agingDays = task.agingDays || 7
  form.priorityAssignees = task.priorityAssignees || []
  form.messageHeader = task.messageHeader || ''
  form.externalInfo = task.externalInfo || ''
  form.keyword = task.keyword || '提醒'
  form.enabled = task.enabled
  form.webhooks = task.webhooks.map(w => ({ ...w }))
  testResult.value = null
  previewResult.value = null
  previewError.value = ''
  dialogVisible.value = true
  onProductChange()
}

const handleSubmit = async () => {
  if (!form.name || !form.cronExpr || !form.productId) return
  submitting.value = true
  try {
    const payload = {
      name: form.name,
      cronExpr: form.cronExpr,
      enabled: form.enabled,
      productId: Number(form.productId) || 0,
      projectId: Number(form.projectId) || 0,
      projectName: form.projectName,
      productName: form.productName,
      statusFilter: form.statusFilter,
      reportType: form.reportType || 'bug',
      agingDays: form.agingDays || 7,
      priorityAssignees: form.priorityAssignees || [],
      messageHeader: form.messageHeader || '',
      keyword: form.keyword,
      externalInfo: form.externalInfo,
      webhooks: form.webhooks.filter(w => w.url).map(w => ({
        id: w.id || '',
        name: w.name,
        url: w.url,
        enabled: w.enabled,
        platform: w.platform || 'generic',
        secret: w.secret || '',
        skipSSL: w.skipSSL || false,
      })),
    }
    if (isEdit.value) {
      await updateTask(form.id, payload)
    } else {
      await createTask(payload)
    }
    dialogVisible.value = false
    await loadTasks()
  } catch (e) {
    console.error('保存失败', e)
  } finally {
    submitting.value = false
  }
}

const handleToggle = async (task: SchedulerTask) => {
  try {
    await toggleTask(task.id)
    await loadTasks()
  } catch (e) { console.error(e) }
}

const handleRunNow = async (task: SchedulerTask) => {
  try {
    const res = await runTaskNow(task.id)
    runResult.value = res.data
    runResultVisible.value = true
    await loadTasks()
    if (activeTab.value === 'logs') await loadLogs()
  } catch (e) { console.error(e) }
}

const handleDelete = async (task: SchedulerTask) => {
  try {
    await ElMessageBox.confirm(`确定删除任务「${task.name}」？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await deleteTask(task.id)
    await loadTasks()
  } catch (e) { console.error(e) }
}

const handleTestWebhook = async (url: string) => {
  if (!url) return
  try {
    const res = await testWebhook(url)
    testResult.value = res.data
  } catch (e: any) {
    testResult.value = { success: false, error: e.message || '请求失败', webhookId: '', webhookName: '', webhookUrl: url, statusCode: 0 }
  }
}

const handlePreview = async () => {
  if (!form.productId) return
  previewLoading.value = true
  previewResult.value = null
  previewError.value = ''
  try {
    const params: PreviewParams = {
      reportType: form.reportType,
      productId: Number(form.productId) || 0,
      projectId: Number(form.projectId) || 0,
      projectName: form.projectName,
      productName: form.productName,
      statusFilter: form.statusFilter,
      agingDays: form.agingDays || 7,
      keyword: form.keyword,
      externalInfo: form.externalInfo,
      messageHeader: form.messageHeader || '',
      priorityAssignees: form.priorityAssignees || [],
    }
    const res = await previewReport(params)
    previewResult.value = res.data
  } catch (e: any) {
    previewError.value = e.message || '预览失败'
  } finally {
    previewLoading.value = false
  }
}

const formatTime = (t: string | null) => {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

const calcDuration = (start: string, end: string) => {
  const ms = new Date(end).getTime() - new Date(start).getTime()
  if (ms < 1000) return ms + 'ms'
  return (ms / 1000).toFixed(1) + 's'
}

const runStatusLabel = (s: string) => {
  const m: Record<string, string> = { running: '执行中', success: '成功', partial: '部分成功', failed: '失败' }
  return m[s] || s
}

const reportTypeLabel = (t: string) => {
  const m: Record<string, string> = { bug: 'Bug报告', requirement: '需求播报', task: '任务播报', 'bug-aging': '超时提醒' }
  return m[t] || t
}

const reportTypeDesc = (t: string) => {
  const m: Record<string, string> = {
    bug: '按指派人分组的Bug分布报告，含严重级别统计',
    requirement: '需求状态分布与指派人进度播报',
    task: '任务进度与工时消耗播报，含完成率统计',
    'bug-aging': '停留超时的Bug按责任人分组强提醒，含具体停留天数',
  }
  return m[t] || ''
}

watch(() => activeTab.value, (tab) => {
  if (tab === 'logs') loadLogs()
})
</script>

<style scoped>
.scheduler-page {
  max-width: 1200px;
}

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-md);
  margin-bottom: var(--space-lg);
}

.stat-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  padding: var(--space-lg);
  display: flex;
  align-items: flex-start;
  gap: var(--space-md);
  box-shadow: var(--shadow-sm);
  transition: box-shadow var(--transition-normal);
}

.stat-card:hover {
  box-shadow: var(--shadow-md);
}

.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-icon svg {
  width: 22px;
  height: 22px;
}

.stat-icon--primary { background: var(--color-primary-light); color: var(--color-primary); }
.stat-icon--success { background: var(--color-success-light); color: var(--color-success); }
.stat-icon--warning { background: var(--color-warning-light); color: var(--color-warning); }
.stat-icon--info { background: var(--color-info-light); color: var(--color-info); }

.stat-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stat-label {
  font-size: 12px;
  color: var(--color-text-tertiary);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1.2;
}

.stat-sub {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

/* Tabs */
.page-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
}

.tab-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 20px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.tab-btn:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.tab-btn.active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.card {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

.empty-state {
  padding: 60px 20px;
  text-align: center;
  color: var(--color-text-tertiary);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.data-table th {
  padding: 12px 16px;
  text-align: left;
  font-weight: 600;
  color: var(--color-text-secondary);
  background: var(--color-bg-hover);
  border-bottom: 1px solid var(--color-border-light);
  white-space: nowrap;
}

.data-table td {
  padding: 10px 16px;
  border-bottom: 1px solid var(--color-border-light);
  color: var(--color-text-primary);
  vertical-align: middle;
}

.data-table tr:last-child td {
  border-bottom: none;
}

.task-name {
  font-weight: 500;
}

.report-type-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 100px;
  font-size: 11px;
  font-weight: 500;
}

.report-bug { background: var(--color-danger-light); color: var(--color-danger); }
.report-requirement { background: var(--color-primary-light); color: var(--color-primary); }
.report-task { background: var(--color-success-light); color: var(--color-success); }

.cron-code {
  background: var(--color-bg-hover);
  padding: 2px 8px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
}

.webhook-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  margin-right: 4px;
  background: var(--color-primary-light);
  color: var(--color-primary);
}

.webhook-tag.disabled {
  background: var(--color-bg-hover);
  color: var(--color-text-tertiary);
  opacity: 0.6;
}

.status-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 100px;
  font-size: 12px;
  cursor: pointer;
  transition: opacity 0.2s;
}

.status-badge:hover { opacity: 0.8; }
.status-on { background: #dcfce7; color: #16a34a; }
.status-off { background: #fee2e2; color: #dc2626; }

.run-status {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
}

.run-success { background: #dcfce7; color: #16a34a; }
.run-failed { background: #fee2e2; color: #dc2626; }
.run-partial { background: #fef3c7; color: #d97706; }
.run-running { background: #dbeafe; color: #2563eb; }

.actions-cell { white-space: nowrap; }
.text-sm { font-size: 12px; }
.text-muted { color: var(--color-text-tertiary); font-size: 12px; }
.wh-result { display: inline-block; margin-right: 6px; font-size: 11px; }
.wh-ok { color: #16a34a; }
.wh-fail { color: #dc2626; }
.error-cell { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--color-text-tertiary); font-size: 12px; }

/* Dialog */
.dialog-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog {
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  width: 640px;
  max-width: 95vw;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-lg);
}

.dialog-lg {
  width: 780px;
}

.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border-light);
}

.dialog-header h3 { font-size: 16px; font-weight: 600; margin: 0; }
.dialog-close { background: none; border: none; font-size: 18px; cursor: pointer; color: var(--color-text-tertiary); }
.dialog-body { padding: 20px; overflow-y: auto; flex: 1; }
.dialog-footer { padding: 12px 20px; border-top: 1px solid var(--color-border-light); display: flex; justify-content: flex-end; gap: 8px; }

.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-size: 13px; font-weight: 500; color: var(--color-text-secondary); margin-bottom: 6px; }
.required { color: #dc2626; }

.form-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-size: 13px;
  color: var(--color-text-primary);
  background: var(--color-bg);
  outline: none;
  transition: border-color 0.2s;
}

.form-input:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(79, 107, 246, 0.12);
}

.form-textarea { resize: vertical; min-height: 60px; font-family: inherit; }
.hint-text { margin-top: 4px; font-size: 11px; color: var(--color-text-tertiary); }
.form-row { display: flex; gap: 12px; }
.form-half { flex: 1; }

/* Report Type Grid */
.report-type-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.report-type-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 14px 10px;
  border: 2px solid var(--color-border);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
}

.report-type-card:hover {
  border-color: var(--color-primary);
}

.report-type-card.active {
  border-color: var(--color-primary);
  background: var(--color-primary-light);
}

.rtc-icon { font-size: 24px; }
.rtc-label { font-size: 13px; font-weight: 600; color: var(--color-text-primary); }
.rtc-desc { font-size: 11px; color: var(--color-text-tertiary); line-height: 1.3; }

/* Cron Presets */
.cron-presets { display: flex; gap: 6px; margin-top: 6px; flex-wrap: wrap; }

.preset-btn {
  padding: 3px 10px;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  background: var(--color-bg);
  color: var(--color-text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s;
}

.preset-btn:hover { border-color: var(--color-primary); color: var(--color-primary); }

/* Webhook */
.webhook-row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.webhook-block { border: 1px solid var(--color-border-light); border-radius: var(--radius-sm); padding: 10px; margin-bottom: 10px; background: var(--color-bg); }
.wh-name { width: 120px !important; flex-shrink: 0; }
.wh-platform { width: 100px !important; flex-shrink: 0; }
.wh-url-full { width: 100% !important; }
.wh-enable { display: flex; align-items: center; gap: 4px; font-size: 12px; color: var(--color-text-secondary); white-space: nowrap; cursor: pointer; }
.test-result { margin-top: 8px; padding: 6px 12px; border-radius: 4px; font-size: 12px; }
.test-ok { background: #dcfce7; color: #16a34a; }
.test-fail { background: #fee2e2; color: #dc2626; }

/* Preview */
.preview-section {
  margin-bottom: 16px;
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.preview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  background: var(--color-bg-hover);
  border-bottom: 1px solid var(--color-border-light);
}

.preview-header label { font-size: 13px; font-weight: 500; color: var(--color-text-secondary); }

.preview-content {
  padding: 12px 14px;
  max-height: 300px;
  overflow-y: auto;
}

.preview-message {
  font-family: 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--color-text-primary);
  margin: 0;
}

.preview-error {
  padding: 12px 14px;
  color: var(--color-danger);
  font-size: 13px;
}

.preview-placeholder {
  padding: 24px 14px;
  text-align: center;
  color: var(--color-text-tertiary);
  font-size: 13px;
}

/* Buttons */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn:hover { border-color: var(--color-primary); color: var(--color-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover { opacity: 0.9; }
.btn-sm { padding: 4px 10px; font-size: 12px; }
.btn-danger { color: #dc2626; }
.btn-danger:hover { background: #fee2e2; }
.btn:disabled { opacity: 0.4; cursor: not-allowed; }

/* Run result */
.run-result-info { display: flex; flex-direction: column; gap: 8px; }
.result-row { font-size: 14px; }
.result-label { font-weight: 500; color: var(--color-text-secondary); }
.result-webhooks { margin-top: 8px; }
.wh-result-item { font-size: 13px; padding: 2px 0; }
.result-error { margin-top: 8px; padding: 8px 12px; background: #fee2e2; color: #dc2626; border-radius: 4px; font-size: 13px; }

/* Responsive */
@media screen and (max-width: 1024px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .report-type-grid { grid-template-columns: 1fr; }
}

@media screen and (max-width: 768px) {
  .stats-grid { grid-template-columns: 1fr; }
  .dialog-lg { width: 95vw; }
  .form-row { flex-direction: column; gap: 0; }
  .data-table { font-size: 12px; }
  .data-table th, .data-table td { padding: 8px 10px; }
}

/* Priority tags */
.priority-hint { font-size: 12px; color: var(--color-text-tertiary); margin-bottom: 6px; }
.priority-tags { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
.priority-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  background: #dbeafe;
  color: #2563eb;
  border-radius: 4px;
  font-size: 13px;
}
.tag-remove {
  background: none;
  border: none;
  color: #2563eb;
  cursor: pointer;
  padding: 0;
  font-size: 12px;
  line-height: 1;
}
.tag-remove:hover { color: #dc2626; }
</style>
