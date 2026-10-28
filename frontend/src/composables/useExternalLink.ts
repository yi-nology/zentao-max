/**
 * 外部链接打开（web 应用模式：新标签页打开系统默认浏览器）
 */
export async function openExternalLink(url: string): Promise<void> {
  if (!url) return
  window.open(url, '_blank', 'noopener,noreferrer')
}
