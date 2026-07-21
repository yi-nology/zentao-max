/**
 * 外部链接打开 composable
 * 优先用 Wails v3 的 browser.OpenURL（系统浏览器），
 * 在非 Wails 环境下 fallback 到 window.open
 */
import { Browser } from '@wailsio/runtime'

export async function openExternalLink(url: string): Promise<void> {
  if (!url) return
  try {
    await Browser.OpenURL(url)
  } catch (err) {
    // fallback：浏览器环境或调用失败
    console.warn('Wails OpenURL failed, fallback to window.open:', err)
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}
