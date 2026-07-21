/**
 * 通用数据导出工具：支持 Excel / CSV / PDF
 * - Excel: 用动态 import 的 xlsx 库（已装）
 * - CSV: 纯字符串拼接，自带 UTF-8 BOM 防 Excel 乱码
 * - PDF: 用 jspdf + jspdf-autotable，支持中文需要嵌入字体或回退到英文
 *
 * 用法：
 *   const cols: ExportColumn<Bug>[] = [
 *     { header: 'ID', access: row => row.id },
 *     { header: '标题', access: row => row.title }
 *   ]
 *   await exportData('bugs', bugs, cols, 'excel')
 */

export interface ExportColumn<T> {
  /** 列标题 */
  header: string
  /** 取值函数（支持嵌套字段） */
  access: (row: T) => string | number | null | undefined
  /** 列宽（仅 PDF 生效，单位 pt；可选） */
  width?: number
}

export type ExportFormat = 'excel' | 'csv' | 'pdf'

/** 触发浏览器下载 */
function triggerDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.style.display = 'none'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  // 延迟释放，避免下载未完成
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

/** 把列值规范化为字符串 */
function normalize(value: string | number | null | undefined): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'number') return String(value)
  return String(value)
}

/** CSV 单元格转义：含逗号、引号、换行需用双引号包裹 */
function escapeCsvCell(s: string): string {
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`
  }
  return s
}

/**
 * 导出 Excel
 */
export async function exportToExcel<T>(
  rows: T[],
  columns: ExportColumn<T>[],
  filename: string
): Promise<void> {
  if (rows.length === 0) {
    throw new Error('没有可导出的数据')
  }
  const XLSX = await import('xlsx')
  const data = rows.map(row => {
    const obj: Record<string, string | number> = {}
    columns.forEach(col => {
      obj[col.header] = normalize(col.access(row))
    })
    return obj
  })
  const ws = XLSX.utils.json_to_sheet(data)
  const wb = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(wb, ws, 'Sheet1')
  XLSX.writeFile(wb, filename.endsWith('.xlsx') ? filename : `${filename}.xlsx`)
}

/**
 * 导出 CSV（含 UTF-8 BOM 防 Excel 乱码）
 */
export function exportToCSV<T>(
  rows: T[],
  columns: ExportColumn<T>[],
  filename: string
): void {
  if (rows.length === 0) {
    throw new Error('没有可导出的数据')
  }
  const header = columns.map(c => escapeCsvCell(c.header)).join(',')
  const body = rows.map(row =>
    columns.map(col => escapeCsvCell(normalize(col.access(row)))).join(',')
  )
  // BOM + 内容
  const csv = '\uFEFF' + [header, ...body].join('\r\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const finalName = filename.endsWith('.csv') ? filename : `${filename}.csv`
  triggerDownload(blob, finalName)
}

/**
 * 导出 PDF（用 autotable 渲染表格）
 *
 * 注意：jspdf 默认内置字体不支持中文。中文字符会被忽略或显示为方块。
 * 如需中文，需要嵌入自定义字体（如 NotoSansSC）。
 * 此处采用 autotable + 默认字体，英文/数字正常显示。
 */
export async function exportToPDF<T>(
  rows: T[],
  columns: ExportColumn<T>[],
  filename: string,
  title?: string
): Promise<void> {
  if (rows.length === 0) {
    throw new Error('没有可导出的数据')
  }
  const { jsPDF } = await import('jspdf')
  const autoTable = (await import('jspdf-autotable')).default

  const doc = new jsPDF({ orientation: 'landscape', unit: 'pt', format: 'a4' })

  if (title) {
    doc.setFontSize(14)
    doc.text(title, 40, 30)
  }

  const head = [columns.map(c => c.header)]
  const body = rows.map(row => columns.map(col => normalize(col.access(row))))

  autoTable(doc, {
    head,
    body,
    startY: title ? 40 : 20,
    styles: { fontSize: 8, cellPadding: 3, overflow: 'linebreak' },
    headStyles: { fillColor: [79, 107, 246], textColor: 255 },
    alternateRowStyles: { fillColor: [245, 247, 250] },
    margin: { left: 40, right: 40 }
  })

  const finalName = filename.endsWith('.pdf') ? filename : `${filename}.pdf`
  doc.save(finalName)
}

/**
 * 统一导出入口：根据格式自动选择实现
 */
export async function exportData<T>(
  filename: string,
  rows: T[],
  columns: ExportColumn<T>[],
  format: ExportFormat,
  options?: { title?: string }
): Promise<void> {
  switch (format) {
    case 'excel':
      return exportToExcel(rows, columns, filename)
    case 'csv':
      return exportToCSV(rows, columns, filename)
    case 'pdf':
      return exportToPDF(rows, columns, filename, options?.title)
  }
}

/**
 * 生成带时间戳的文件名，如 "bugs-2024-06-15"
 */
export function timestampedFilename(base: string): string {
  const now = new Date()
  const y = now.getFullYear()
  const m = String(now.getMonth() + 1).padStart(2, '0')
  const d = String(now.getDate()).padStart(2, '0')
  return `${base}-${y}${m}${d}`
}
