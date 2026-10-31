import DOMPurify from 'dompurify'

/**
 * 最小 Markdown 渲染，用于版本发布说明（note_md）。
 * 支持范围（应用对接指南约定的最低集合）：
 *   #/##/### 标题、-/* 列表、**粗体**、`行内代码`、[文本](http链接)、段落换行。
 * 安全策略：先整体 HTML 转义、再按白名单生成标签（发布内容里的原始 HTML
 * 只会以文本呈现），最后经 DOMPurify 二次消毒；链接仅接受 http(s)。
 */
export function renderMarkdown(md: string): string {
  if (!md || !md.trim()) return ''
  const escapeHtml = (s: string): string =>
    s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;')
  const inline = (s: string): string =>
    escapeHtml(s)
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(
        /\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g,
        '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>'
      )

  const out: string[] = []
  let inList = false
  const closeList = (): void => {
    if (inList) {
      out.push('</ul>')
      inList = false
    }
  }
  for (const raw of md.replace(/\r\n/g, '\n').split('\n')) {
    const line = raw.trim()
    if (line === '') {
      closeList()
      continue
    }
    const heading = /^(#{1,3})\s*(\S.*)$/.exec(line)
    if (heading) {
      closeList()
      const level = heading[1].length
      out.push(`<h${level}>${inline(heading[2])}</h${level}>`)
      continue
    }
    const item = /^[-*]\s+(.+)$/.exec(line)
    if (item) {
      if (!inList) {
        out.push('<ul>')
        inList = true
      }
      out.push(`<li>${inline(item[1])}</li>`)
      continue
    }
    closeList()
    out.push(`<p>${inline(line)}</p>`)
  }
  closeList()
  return DOMPurify.sanitize(out.join('\n'))
}
