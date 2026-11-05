/**
 * 本地时区日期工具。
 *
 * 背景：`new Date().toISOString()` 返回 UTC——东八区早上 8 点前会得到「昨天」，
 * 曾导致记工时默认日期错一天（见 Bugs.vue 记工时弹窗）。全项目统一用这里
 * 的本地时区实现，不要再手搓 `toISOString().slice(0, 10)`。
 */

/** Date → 本地时区 'YYYY-MM-DD' */
export function formatDate(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

/** Date → 本地时区 'YYYY-MM-DD HH:mm:ss' */
export function formatDateTime(d: Date): string {
  const h = String(d.getHours()).padStart(2, '0')
  const mi = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${formatDate(d)} ${h}:${mi}:${s}`
}

/**
 * 日期范围选择器的「近 N 天」快捷选项（element-plus shortcuts 形状，
 * issue #6：Bugs/Stories/Tasks 三处逐字复制收敛至此）。
 */
export const dateShortcuts = [
  { text: '近7天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 6); return [start, end] } },
  { text: '近30天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 29); return [start, end] } },
  { text: '近90天', value: (): [Date, Date] => { const end = new Date(); const start = new Date(); start.setDate(start.getDate() - 89); return [start, end] } }
]
