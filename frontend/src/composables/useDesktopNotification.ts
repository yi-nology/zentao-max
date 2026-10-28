/**
 * 桌面通知 composable（纯 Web 版本）
 * 定时任务执行结束后通过浏览器 Notification API 提醒（需用户授权），
 * 同时用 Element Plus 的 ElNotification 在应用内显示（不依赖授权）。
 *
 * 数据源：轮询 /api/scheduler/tasks 的 lastRunAt/lastRunStatus 变化
 * （web 应用模式下后端无 Wails 事件通道，改为轮询）。
 */
import { onMounted, onBeforeUnmount } from 'vue'
import { ElNotification } from 'element-plus'
import { listTasks } from '@/api/scheduler'

const STORAGE_KEY = 'zentao-max-notification'
const POLL_INTERVAL_MS = 60_000

export interface NotificationPayload {
  type?: string
  level?: 'info' | 'warning' | 'error'
  title: string
  body: string
  taskId?: string
  taskName?: string
  status?: string
  bugTotal?: number
}

export function isNotificationEnabled(): boolean {
  return localStorage.getItem(STORAGE_KEY) !== '0'
}

export function setNotificationEnabled(enabled: boolean): void {
  localStorage.setItem(STORAGE_KEY, enabled ? '1' : '0')
}

/** 请求 Notification API 权限（如果尚未授权） */
export async function ensurePermission(): Promise<boolean> {
  if (typeof Notification === 'undefined') return false
  if (Notification.permission === 'granted') return true
  if (Notification.permission === 'denied') return false
  try {
    const result = await Notification.requestPermission()
    return result === 'granted'
  } catch {
    return false
  }
}

function showOSNotification(payload: NotificationPayload): void {
  if (typeof Notification === 'undefined' || Notification.permission !== 'granted') {
    return
  }
  try {
    const n = new Notification(payload.title, {
      body: payload.body
    })
    n.onclick = () => {
      window.focus()
      n.close()
    }
    setTimeout(() => n.close(), 8000)
  } catch {
    /* Safari 等 iframe 内可能不可用 */
  }
}

function showAppNotification(payload: NotificationPayload): void {
  const type = payload.level === 'error' ? 'error'
    : payload.level === 'warning' ? 'warning' : 'success'
  ElNotification({
    title: payload.title,
    message: payload.body,
    type,
    duration: 6000,
    position: 'bottom-right'
  })
}

/**
 * 注册桌面通知：轮询定时任务执行状态，变化时弹浏览器+应用内通知。
 * 返回取消轮询函数。
 */
export function useDesktopNotification(): { cancel: () => void } {
  let timer: ReturnType<typeof setInterval> | null = null
  // 基线：首次轮询建立各任务的 lastRunAt 快照，之后只对新增执行弹通知
  let baseline: Map<string, string> | null = null

  const handler = (payload: NotificationPayload): void => {
    if (!isNotificationEnabled()) return
    showAppNotification(payload)
    showOSNotification(payload)
  }

  const poll = async (): Promise<void> => {
    try {
      const res = await listTasks()
      const tasks = res.data || []
      const snapshot = new Map<string, string>()
      for (const t of tasks) {
        snapshot.set(t.id, `${t.lastRunAt ?? ''}|${t.lastRunStatus ?? ''}`)
        if (baseline && baseline.get(t.id) !== snapshot.get(t.id) && t.lastRunAt) {
          const ok = t.lastRunStatus === 'success'
          handler({
            level: ok ? 'info' : 'error',
            title: `定时任务：${t.name}`,
            body: ok ? '执行成功' : `执行${t.lastRunStatus}`,
            taskId: t.id,
            taskName: t.name,
            status: t.lastRunStatus
          })
        }
      }
      baseline = snapshot
    } catch {
      /* 后端不可达时静默，下轮重试 */
    }
  }

  onMounted(() => {
    timer = setInterval(() => {
      void poll()
    }, POLL_INTERVAL_MS)
    // 首轮延迟一点，避免与页面初始数据请求挤在一起
    setTimeout(() => {
      void poll()
    }, 3000)
  })

  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
  })

  return {
    cancel: () => {
      if (timer) clearInterval(timer)
    }
  }
}
