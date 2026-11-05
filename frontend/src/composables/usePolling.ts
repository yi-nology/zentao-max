import { onBeforeUnmount, ref } from 'vue'

/**
 * 统一轮询原语（issue #6：Logs/HealthCheck 手搓 setInterval + in-flight 锁 +
 * 卸载清理三件套收敛）。
 *
 * - intervalMs：基础间隔；fn 仍在执行时跳过一次（防重入，不叠加）
 * - 卸载自动 stop
 * - 连续失败指数退避（×2 封顶 maxBackoff 倍），成功即恢复基础间隔
 *   （注意：fn 内部自行吞错时退避不触发——退避只看 fn 是否抛异常）
 */
export function usePolling(fn: () => Promise<void> | void, intervalMs: number, maxBackoff = 8) {
  const running = ref(false)
  const inFlight = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null
  let failures = 0

  const tick = async (): Promise<void> => {
    if (inFlight.value) return
    inFlight.value = true
    try {
      await fn()
      failures = 0
    } catch (e) {
      failures++
      console.warn('轮询执行失败:', e)
    } finally {
      inFlight.value = false
    }
  }

  const schedule = (): void => {
    if (!running.value) return
    const delay = intervalMs * Math.min(Math.max(2 ** failures, 1), maxBackoff)
    timer = setTimeout(async () => {
      await tick()
      schedule()
    }, delay)
  }

  const start = (): void => {
    if (running.value) return
    running.value = true
    void tick()
    schedule()
  }

  const stop = (): void => {
    running.value = false
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
  }

  onBeforeUnmount(stop)

  return { running, inFlight, start, stop }
}
