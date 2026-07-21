/**
 * 桌面通知 composable (Wails v3 版本)
 * 1. 监听 @wailsio/runtime 事件 'notification'（后端 app.Event.Emit 推送）
 * 2. 通过 Notification API 显示桌面通知（需要用户授权）
 * 3. 同时通过 Element Plus 的 ElNotification 在应用内显示（不依赖授权）
 *
 * 仅在 Wails 桌面环境下生效（其他模式后端不会发 Emit）
 */
import { onMounted, onBeforeUnmount } from 'vue'
import { ElNotification } from 'element-plus'
import { Events } from '@wailsio/runtime'

const STORAGE_KEY = 'zentao-max-notification'

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
    // 点击通知聚焦窗口（v3 通过 ShowWindow service binding）
    n.onclick = async () => {
      try {
        // 相对路径引用 wails3 generate 生成的 bindings
        // @ts-ignore - 由 wails3 generate 生成，无类型声明
        const mod = await import('../../bindings/github.com/yi-nology/zentao-max/app.js')
        if (mod.ShowWindow) mod.ShowWindow()
      } catch {
        /* 非 wails 环境忽略 */
      }
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

const NOTIFICATION_EVENT = 'notification'

/**
 * 注册桌面通知监听，返回取消监听函数
 */
export function useDesktopNotification(): { cancel: () => void } {
  let cancelFn: (() => void) | null = null

  const handler = (ev: { data?: NotificationPayload }): void => {
    const payload = ev?.data
    if (!payload) return
    if (!isNotificationEnabled()) return
    showAppNotification(payload)
    showOSNotification(payload)
  }

  onMounted(() => {
    // v3 用 Events.On 返回取消函数
    cancelFn = Events.On(NOTIFICATION_EVENT, handler as any)
  })

  onBeforeUnmount(() => {
    if (cancelFn) cancelFn()
    else Events.Off(NOTIFICATION_EVENT)
  })

  return {
    cancel: () => cancelFn?.()
  }
}
