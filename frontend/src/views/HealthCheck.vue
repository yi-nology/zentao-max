<template>
  <div class="health-page">
    <div class="health-header">
      <div class="header-left">
        <h2>系统监控</h2>
        <span class="check-time" v-if="lastCheckTime">最近检测：{{ lastCheckTime }}</span>
      </div>
      <div class="header-right">
        <label class="auto-toggle">
          <input type="checkbox" v-model="autoRefresh" />
          <span class="toggle-track" :class="{ on: autoRefresh }">
            <span class="toggle-thumb"></span>
          </span>
          <span>自动刷新</span>
        </label>
        <button class="refresh-btn" @click="runCheck" :disabled="loading">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :class="{ spinning: loading }" style="width:14px;height:14px"><path d="M23 4v6h-6M1 20v-6h6"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/></svg>
          {{ loading ? '检测中' : '刷新' }}
        </button>
      </div>
    </div>

    <div v-if="loading && !data" class="loading-state">
      <div class="loader"></div>
      <span>正在采集数据...</span>
    </div>

    <template v-else-if="data">
      <div class="overview-row">
        <div class="overview-main" :class="data.summary.healthy ? 'main-ok' : 'main-err'">
          <div class="main-left">
            <div class="main-icon" :class="data.summary.healthy ? 'icon-ok' : 'icon-err'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="width:28px;height:28px">
                <path v-if="data.summary.healthy" d="M22 11.08V12a10 10 0 11-5.93-9.14"/><path v-if="data.summary.healthy" d="M22 4L12 14.01l-3-3"/>
                <path v-if="!data.summary.healthy" d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line v-if="!data.summary.healthy" x1="12" y1="9" x2="12" y2="13"/><line v-if="!data.summary.healthy" x1="12" y1="17" x2="12.01" y2="17"/>
              </svg>
            </div>
            <div class="main-text">
              <div class="main-title">{{ data.summary.healthy ? '系统运行正常' : '存在异常服务' }}</div>
              <div class="main-desc">{{ data.zentao?.status === 'ok' ? data.zentao.message : data.zentao?.message || '未连接禅道服务器' }}</div>
            </div>
          </div>
          <div class="main-right">
            <div class="main-count" :class="data.summary.healthy ? 'cnt-ok' : 'cnt-err'">{{ data.summary.ok }}<span class="cnt-total">/{{ data.summary.total }}</span></div>
            <div class="main-label">服务正常</div>
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-num ok-num">{{ data.summary.ok }}</div>
          <div class="stat-label">正常</div>
          <div class="stat-bar bar-ok"></div>
        </div>
        <div class="stat-card" :class="{ 'stat-alert': data.summary.fail > 0 }">
          <div class="stat-num" :class="data.summary.fail > 0 ? 'err-num' : 'muted-num'">{{ data.summary.fail }}</div>
          <div class="stat-label">异常</div>
          <div class="stat-bar" :class="data.summary.fail > 0 ? 'bar-err' : 'bar-muted'"></div>
        </div>
      </div>

      <div class="section" v-if="data.zentao">
        <div class="section-header">
          <span class="section-dot"></span>
          <span class="section-title">服务连接</span>
        </div>
        <div class="conn-row" :class="data.zentao.status === 'ok' ? 'conn-ok' : 'conn-fail'">
          <div class="conn-left">
            <div class="conn-indicator">
              <span class="conn-dot" :class="data.zentao.status === 'ok' ? 'dot-on' : 'dot-off'"></span>
            </div>
            <span class="conn-icon">🔗</span>
            <div class="conn-info">
              <div class="conn-name">禅道服务器</div>
              <div class="conn-detail">{{ data.zentao.message }}</div>
            </div>
          </div>
          <div class="conn-right">
            <span class="conn-latency" v-if="data.zentao.latencyMs">{{ data.zentao.latencyMs }}ms</span>
            <span class="conn-tag" :class="data.zentao.status === 'ok' ? 'tag-ok' : 'tag-err'">
              {{ data.zentao.status === 'ok' ? '已连接' : '连接失败' }}
            </span>
          </div>
        </div>
      </div>

      <div class="section" v-if="data.checks && data.checks.length > 0">
        <div class="section-header">
          <span class="section-dot"></span>
          <span class="section-title">数据检测</span>
        </div>
        <div class="data-grid">
          <div v-for="check in data.checks" :key="check.name" class="data-card" :class="check.status === 'ok' ? 'dc-ok' : 'dc-err'">
            <div class="dc-top">
              <div class="dc-icon">{{ iconMap[check.name] }}</div>
              <div class="dc-info">
                <div class="dc-name">{{ labelMap[check.name] || check.name }}</div>
              </div>
              <div class="dc-count">{{ check.count }}</div>
            </div>
            <div class="dc-progress">
              <div class="dc-bar-track">
                <div class="dc-bar-fill" :class="check.status === 'ok' ? 'fill-ok' : 'fill-err'" :style="{ width: barWidth(check) }"></div>
              </div>
              <span class="dc-ms">{{ check.latencyMs }}ms</span>
            </div>
            <div class="dc-bottom">
              <span class="dc-status" :class="check.status === 'ok' ? 'st-ok' : 'st-err'">
                {{ check.status === 'ok' ? (check.count > 0 ? '有数据' : '暂无数据') : '异常' }}
              </span>
            </div>
            <div class="dc-error" v-if="check.status === 'fail'">{{ check.message }}</div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import api from '@/api/api'
import type { ApiResponse } from '@/types/api'

interface CheckItem {
  name: string
  status: string
  count: number
  message: string
  latencyMs: number
}

interface HealthData {
  timestamp: string
  zentao: CheckItem
  checks: CheckItem[]
  summary: { total: number; ok: number; fail: number; healthy: boolean }
}

const data = ref<HealthData | null>(null)
const loading = ref(false)
const lastCheckTime = ref('')
const autoRefresh = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const iconMap: Record<string, string> = {
  products: '📦', projects: '📁', bugs: '🐛',
  stories: '📋', tasks: '✅', users: '👥', scheduler: '⏰',
}

const labelMap: Record<string, string> = {
  products: '产品', projects: '项目', bugs: 'Bug',
  stories: '需求', tasks: '任务', users: '用户', scheduler: '定时任务',
}

const barWidth = (c: CheckItem) => {
  const ms = c.latencyMs || 0
  if (ms <= 100) return '15%'
  if (ms <= 500) return '35%'
  if (ms <= 1000) return '55%'
  if (ms <= 3000) return '75%'
  return '95%'
}

const runCheck = async () => {
  loading.value = true
  try {
    const res = await api.get('/healthz') as ApiResponse<HealthData>
    data.value = res.data
    lastCheckTime.value = new Date().toLocaleString('zh-CN')
  } catch (e) { console.error('健康检查失败', e) }
  finally { loading.value = false }
}

onMounted(() => { runCheck() })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })

watch(autoRefresh, (v) => {
  if (timer) clearInterval(timer)
  if (v) timer = setInterval(runCheck, 30000)
  else timer = null
})
</script>

<style scoped>
.health-page { max-width: 1100px; }

.health-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.header-left { display: flex; align-items: baseline; gap: 12px; }
.header-left h2 { margin: 0; font-size: 20px; font-weight: 700; }
.check-time { font-size: 12px; color: var(--color-text-tertiary); }

.header-right { display: flex; align-items: center; gap: 12px; }

.auto-toggle {
  display: flex; align-items: center; gap: 8px;
  font-size: 12px; color: var(--color-text-secondary); cursor: pointer; user-select: none;
}

.toggle-track {
  width: 32px; height: 18px; border-radius: 9px; background: var(--color-border);
  position: relative; transition: background 0.2s;
}
.toggle-track.on { background: var(--color-primary); }
.toggle-thumb {
  width: 14px; height: 14px; border-radius: 50%; background: #fff;
  position: absolute; top: 2px; left: 2px; transition: transform 0.2s;
  box-shadow: 0 1px 3px rgba(0,0,0,0.15);
}
.toggle-track.on .toggle-thumb { transform: translateX(14px); }

.refresh-btn {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 7px 16px; border: 1px solid var(--color-border); border-radius: var(--radius-sm);
  background: var(--color-bg-card); color: var(--color-text-secondary);
  font-size: 13px; cursor: pointer; transition: all 0.2s;
}
.refresh-btn:hover { border-color: var(--color-primary); color: var(--color-primary); }
.refresh-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.spinning { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.loading-state {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 80px 20px; color: var(--color-text-tertiary); gap: 16px;
}
.loader {
  width: 36px; height: 36px; border: 3px solid var(--color-border);
  border-top-color: var(--color-primary); border-radius: 50%; animation: spin 0.8s linear infinite;
}

/* Overview */
.overview-row {
  display: grid; grid-template-columns: 1fr 140px 140px; gap: 16px; margin-bottom: 24px;
}

.overview-main {
  border-radius: var(--radius-md); padding: 20px 24px;
  display: flex; align-items: center; justify-content: space-between;
}
.main-ok { background: linear-gradient(135deg, var(--color-success-bg), var(--color-success-light)); border: 1px solid var(--color-success-border); }
.main-err { background: linear-gradient(135deg, var(--color-danger-bg), var(--color-danger-light)); border: 1px solid var(--color-danger-border); }

.main-left { display: flex; align-items: center; gap: 16px; }

.main-icon {
  width: 48px; height: 48px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.icon-ok { background: var(--color-success-light); color: var(--color-success); }
.icon-err { background: var(--color-danger-light); color: var(--color-danger); }

.main-title { font-size: 16px; font-weight: 700; color: var(--color-text-primary); margin-bottom: 2px; }
.main-desc { font-size: 12px; color: var(--color-text-secondary); }

.main-right { text-align: right; }
.main-count { font-size: 36px; font-weight: 800; line-height: 1; font-family: 'Inter', sans-serif; }
.cnt-ok { color: var(--color-success); }
.cnt-err { color: var(--color-danger); }
.cnt-total { font-size: 18px; color: var(--color-text-tertiary); font-weight: 400; }
.main-label { font-size: 11px; color: var(--color-text-tertiary); margin-top: 2px; }

.stat-card {
  background: var(--color-bg-card); border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md); padding: 20px; text-align: center;
  position: relative; overflow: hidden;
}
.stat-alert { border-color: var(--color-danger-border); }

.stat-num { font-size: 32px; font-weight: 800; line-height: 1; font-family: 'Inter', sans-serif; }
.ok-num { color: var(--color-success); }
.err-num { color: var(--color-danger); }
.muted-num { color: var(--color-text-tertiary); }

.stat-label { font-size: 12px; color: var(--color-text-secondary); margin-top: 4px; margin-bottom: 12px; }

.stat-bar {
  position: absolute; bottom: 0; left: 0; right: 0; height: 3px;
}
.bar-ok { background: linear-gradient(90deg, var(--color-success), var(--color-success-light)); }
.bar-err { background: linear-gradient(90deg, var(--color-danger), var(--color-danger-light)); }
.bar-muted { background: var(--color-border); }

/* Section */
.section {
  background: var(--color-bg-card); border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md); padding: 20px; margin-bottom: 16px;
}

.section-header { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.section-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--color-primary); }
.section-title { font-size: 13px; font-weight: 600; color: var(--color-text-primary); text-transform: uppercase; letter-spacing: 0.04em; }

/* Connection */
.conn-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px; border-radius: var(--radius-sm); border: 1px solid var(--color-border-light);
}
.conn-ok { background: var(--color-success-bg); border-color: var(--color-success-border); }
.conn-fail { background: var(--color-danger-bg); border-color: var(--color-danger-border); }

.conn-left { display: flex; align-items: center; gap: 12px; }
.conn-indicator { position: relative; }

.conn-dot { width: 10px; height: 10px; border-radius: 50%; display: block; }
.dot-on {
  background: var(--color-success); box-shadow: 0 0 0 3px rgba(34,197,94,0.2);
  animation: pulse-dot 2s ease-in-out infinite;
}
.dot-off {
  background: var(--color-danger); box-shadow: 0 0 0 3px rgba(239,68,68,0.2);
  animation: pulse-dot 1s ease-in-out infinite;
}
@keyframes pulse-dot {
  0%, 100% { box-shadow: 0 0 0 3px rgba(34,197,94,0.2); }
  50% { box-shadow: 0 0 0 6px rgba(34,197,94,0.05); }
}
.dot-off { animation-name: pulse-dot-err; }
@keyframes pulse-dot-err {
  0%, 100% { box-shadow: 0 0 0 3px rgba(239,68,68,0.2); }
  50% { box-shadow: 0 0 0 6px rgba(239,68,68,0.05); }
}

.conn-icon { font-size: 18px; }
.conn-name { font-size: 14px; font-weight: 600; color: var(--color-text-primary); }
.conn-detail { font-size: 12px; color: var(--color-text-secondary); margin-top: 1px; }

.conn-right { display: flex; align-items: center; gap: 10px; }
.conn-latency { font-size: 11px; color: var(--color-text-tertiary); font-family: monospace; }

.conn-tag {
  font-size: 11px; font-weight: 600; padding: 3px 10px; border-radius: 4px;
}
.tag-ok { background: var(--color-success-light); color: var(--color-success); }
.tag-err { background: var(--color-danger-light); color: var(--color-danger); }

/* Data grid */
.data-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px;
}

.data-card {
  border: 1px solid var(--color-border-light); border-radius: var(--radius-sm);
  padding: 16px; position: relative; transition: box-shadow 0.2s;
}
.data-card:hover { box-shadow: var(--shadow-md); }
.dc-ok { border-top: 3px solid #22c55e; }
.dc-err { border-top: 3px solid #ef4444; }

.dc-top { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }

.dc-icon {
  width: 36px; height: 36px; border-radius: 8px;
  background: var(--color-bg-hover); display: flex; align-items: center;
  justify-content: center; font-size: 18px; flex-shrink: 0;
}

.dc-info { flex: 1; }
.dc-name { font-size: 13px; font-weight: 600; color: var(--color-text-primary); }

.dc-count {
  font-size: 24px; font-weight: 800; color: var(--color-text-primary);
  font-family: 'Inter', sans-serif; line-height: 1;
}

.dc-progress { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }

.dc-bar-track {
  flex: 1; height: 4px; background: var(--color-bg-hover);
  border-radius: 2px; overflow: hidden;
}
.dc-bar-fill { height: 100%; border-radius: 2px; transition: width 0.6s ease; }
.fill-ok { background: linear-gradient(90deg, #22c55e, #86efac); }
.fill-err { background: linear-gradient(90deg, #ef4444, #fca5a5); }

.dc-ms { font-size: 10px; color: var(--color-text-tertiary); font-family: monospace; min-width: 42px; text-align: right; }

.dc-bottom { display: flex; align-items: center; justify-content: space-between; }

.dc-status { font-size: 11px; font-weight: 500; }
.st-ok { color: #16a34a; }
.st-err { color: #dc2626; }

.dc-error {
  margin-top: 8px; padding: 8px 10px; background: #fef2f2; border-radius: 4px;
  font-size: 11px; color: #dc2626; line-height: 1.4; word-break: break-all;
}

@media (max-width: 768px) {
  .overview-row { grid-template-columns: 1fr; }
  .overview-main { flex-direction: column; gap: 12px; text-align: center; }
  .main-left { flex-direction: column; }
  .main-right { text-align: center; }
}
</style>
