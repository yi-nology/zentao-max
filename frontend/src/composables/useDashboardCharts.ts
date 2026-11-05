/**
 * 仪表盘图表渲染（issue #7-2：从 Dashboard.vue 提取）。
 * bar chart 替代 donut：严重程度/类型分布横向条 + 任务分布堆叠条。
 * data 变化后 nextTick 重渲染；卸载自动销毁（Chart.js 实例不主动释放会泄漏
 * ResizeObserver）。
 */
import { computed, nextTick, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import Chart from 'chart.js/auto'
import type { Chart as ChartType } from 'chart.js/auto'
import type { DashboardData } from '@/types/api'

const severityLabel: Record<string, string> = { '1': '致命', '2': '严重', '3': '一般', '4': '轻微', '5': '建议' }

export function useDashboardCharts(data: Ref<DashboardData | null>) {

// ============ Charts（bar chart 替代 dougnut） ============
  const severityChartRef = ref<HTMLCanvasElement | null>(null)
  const typeChartRef = ref<HTMLCanvasElement | null>(null)
  const taskChartRef = ref<HTMLCanvasElement | null>(null)
let charts: ChartType[] = []

const hasSeverityData = computed(() => {
  const bySeverity = data.value?.bugs?.bySeverity
  return bySeverity && Object.keys(bySeverity).length > 0
})

const hasTypeData = computed(() => {
  const byType = data.value?.bugs?.byType
  return byType && Object.keys(byType).length > 0
})

const hasTaskData = computed(() => {
  const tasks = data.value?.tasks
  if (!tasks) return false
  return (tasks.wait || 0) + (tasks.doing || 0) + (tasks.done || 0) + (tasks.closed || 0) > 0
})


const setupChartDefaults = () => {
  Chart.defaults.font.family = 'Inter, "Nunito", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
  Chart.defaults.font.size = 11
  Chart.defaults.color = '#94A3B8'
}


const renderCharts = () => {
  destroyCharts()
  if (!data.value) return
  setupChartDefaults()

  const bugs = data.value.bugs
  const tasks = data.value.tasks

  // Bug severity → horizontal bar
  if (severityChartRef.value && bugs?.bySeverity && Object.keys(bugs.bySeverity).length > 0) {
    const entries = Object.entries(bugs.bySeverity)
      .map(([k, v]) => ({ key: k, label: severityLabel[k] || `L${k}`, value: v as number }))
      .sort((a, b) => a.value - b.value) // 致命在上
    charts.push(new Chart(severityChartRef.value, {
      type: 'bar',
      data: {
        labels: entries.map(e => e.label),
        datasets: [{
          data: entries.map(e => e.value),
          backgroundColor: entries.map(e => {
            const palette: Record<string, string> = { '1': '#DC2626', '2': '#EF4444', '3': '#F59E0B', '4': '#3B82F6', '5': '#94A3B8' }
            return palette[e.key] || '#94A3B8'
          }),
          borderRadius: 6,
          barPercentage: 0.65,
          categoryPercentage: 0.8
        }]
      },
      options: {
        indexAxis: 'y',
        responsive: true,
        maintainAspectRatio: false,
        animation: { duration: 700, easing: 'easeOutCubic' },
        plugins: {
          legend: { display: false },
          tooltip: { backgroundColor: 'rgba(15, 23, 42, 0.92)', padding: 10, cornerRadius: 8, displayColors: false }
        },
        scales: {
          x: { grid: { display: false }, ticks: { maxTicksLimit: 6 } },
          y: { grid: { display: false }, ticks: { font: { size: 12, weight: 500 }, color: '#475569', callback: function(this: any, value: any) { return this.chart?.data?.labels?.[value] ?? '' } } }
        }
      }
    }))
  }

  // Bug type → vertical bar (top 8)
  if (typeChartRef.value && bugs?.byType && Object.keys(bugs.byType).length > 0) {
    const entries = Object.entries(bugs.byType)
      .map(([k, v]) => ({ label: k, value: v as number }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 8)
    charts.push(new Chart(typeChartRef.value, {
      type: 'bar',
      data: {
        labels: entries.map(e => e.label),
        datasets: [{
          data: entries.map(e => e.value),
          backgroundColor: ['#EF4444', '#F59E0B', '#3B82F6', '#22C55E', '#8B5CF6', '#EC4899', '#06B6D4', '#F97316'].slice(0, entries.length),
          borderRadius: 6,
          barPercentage: 0.6,
          categoryPercentage: 0.8
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: { duration: 700, easing: 'easeOutCubic' },
        plugins: {
          legend: { display: false },
          tooltip: { backgroundColor: 'rgba(15, 23, 42, 0.92)', padding: 10, cornerRadius: 8, displayColors: false }
        },
        scales: {
          x: { grid: { display: false }, ticks: { font: { size: 11 }, color: '#64748B', maxRotation: 30, minRotation: 0 } },
          y: { grid: { color: 'rgba(148, 163, 184, 0.12)' }, ticks: { color: '#94A3B8', stepSize: 1, callback: (v: any) => Number.isInteger(v) ? v : '' }, beginAtZero: true }
        }
      }
    }))
  }

  // Task status → horizontal stacked bar
  if (taskChartRef.value && tasks) {
    const entries = [
      { key: 'wait', label: '待开始', color: '#94A3B8' },
      { key: 'doing', label: '进行中', color: '#3B82F6' },
      { key: 'done', label: '已完成', color: '#22C55E' },
      { key: 'closed', label: '已关闭', color: '#64748B' }
    ].map(e => ({ ...e, value: (tasks as unknown as Record<string, number>)[e.key] || 0 }))
      .filter(e => e.value > 0)

    if (entries.length > 0) {
      charts.push(new Chart(taskChartRef.value, {
        type: 'bar',
        data: {
          labels: ['任务分布'],
          datasets: entries.map(e => ({
            label: e.label,
            data: [e.value],
            backgroundColor: e.color,
            borderWidth: 0,
            borderRadius: 4,
            barPercentage: 0.5
          }))
        },
        options: {
          indexAxis: 'y',
          responsive: true,
          maintainAspectRatio: false,
          animation: { duration: 700, easing: 'easeOutCubic' },
          plugins: {
            legend: { position: 'bottom', labels: { padding: 12, usePointStyle: true, pointStyle: 'circle', font: { size: 11 } } },
            tooltip: { backgroundColor: 'rgba(15, 23, 42, 0.92)', padding: 10, cornerRadius: 8 }
          },
          scales: {
            x: { stacked: true, grid: { display: false }, ticks: { color: '#94A3B8' } },
            y: { stacked: true, grid: { display: false }, ticks: { display: false } }
          }
        }
      }))
    }
  }
}

const destroyCharts = () => {
  charts.forEach(c => c.destroy())
  charts = []
}

const downloadChart = (index: number, name: string): void => {
  const chart = charts[index]
  if (!chart) return
  const base64 = (chart as any).toBase64Image('image/png', 1)
  const link = document.createElement('a')
  link.download = `${name}-${Date.now()}.png`
  link.href = base64
  link.click()
}

  // data 变化 → 重渲染；产品切换/卸载 → 销毁
  watch(data, async (newData) => {
    if (!newData) return
    await nextTick()
    renderCharts()
  }, { flush: 'post' })

  onBeforeUnmount(() => { destroyCharts() })

  return {
    severityChartRef,
    typeChartRef,
    taskChartRef,
    hasSeverityData,
    hasTypeData,
    hasTaskData,
    destroyCharts,
    downloadChart
  }
}
