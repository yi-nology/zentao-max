import { ref, watch, type Ref } from 'vue'

/**
 * 表格列配置接口
 * - key: 列唯一标识
 * - label: 列标题
 * - visible: 是否显示
 * - sortable: 是否可排序（true=本地排序，'custom'=后端排序）
 * - width: 列宽（可选）
 * - minWidth: 最小列宽（可选）
 * - fixed: 固定列（'left' | 'right' | false）
 */
export interface ColumnConfig {
  key: string
  label: string
  visible: boolean
  sortable?: boolean | string
  width?: number | string
  minWidth?: number | string
  fixed?: 'left' | 'right' | false
}

/**
 * 通用表格列管理 composable
 * - 自动从 localStorage 恢复用户上次配置
 * - 修改后自动持久化
 *
 * @param storageKey localStorage 键名
 * @param defaultColumns 默认列配置
 */
export function useTableColumns<T extends ColumnConfig>(
  storageKey: string,
  defaultColumns: T[]
): {
  columns: Ref<T[]>
  visibleColumns: Ref<T[]>
  toggleColumn: (key: string, visible?: boolean) => void
  showAllColumns: () => void
  hideAllColumns: () => void
  resetColumns: () => void
} {
  const columns = ref<T[]>(loadColumns()) as Ref<T[]>

  // 仅返回可见列
  const visibleColumns = ref<T[]>(columns.value.filter(c => c.visible)) as Ref<T[]>

  // 监听变化，自动持久化和刷新可见列
  watch(columns, (newCols) => {
    saveColumns(storageKey, newCols)
    visibleColumns.value = newCols.filter(c => c.visible)
  }, { deep: true })

  function loadColumns(): T[] {
    try {
      const saved = localStorage.getItem(storageKey)
      if (!saved) return JSON.parse(JSON.stringify(defaultColumns))
      const savedCols = JSON.parse(saved) as Record<string, { visible?: boolean; sortable?: boolean | string }>
      // 合并默认配置和保存的覆盖项（默认列优先，saved 中只覆盖 visible/sortable）
      return defaultColumns.map(def => {
        const override = savedCols[def.key]
        if (!override) return { ...def }
        return { ...def, visible: override.visible ?? def.visible, sortable: override.sortable ?? def.sortable }
      })
    } catch {
      return JSON.parse(JSON.stringify(defaultColumns))
    }
  }

  function saveColumns(key: string, cols: T[]): void {
    try {
      const obj: Record<string, { visible: boolean; sortable?: boolean | string }> = {}
      cols.forEach(c => {
        obj[c.key] = { visible: c.visible, sortable: c.sortable }
      })
      localStorage.setItem(key, JSON.stringify(obj))
    } catch {
      /* localStorage 可能被禁用，忽略 */
    }
  }

  function toggleColumn(key: string, visible?: boolean): void {
    const col = columns.value.find(c => c.key === key)
    if (col) {
      col.visible = visible ?? !col.visible
    }
  }

  function showAllColumns(): void {
    columns.value.forEach(c => { c.visible = true })
  }

  function hideAllColumns(): void {
    columns.value.forEach(c => {
      // 保留固定必显示列（如 selection）
      if (c.fixed !== 'left' && c.fixed !== 'right' && c.key !== '__selection__') {
        c.visible = false
      }
    })
  }

  function resetColumns(): void {
    columns.value = JSON.parse(JSON.stringify(defaultColumns))
  }

  return {
    columns,
    visibleColumns,
    toggleColumn,
    showAllColumns,
    hideAllColumns,
    resetColumns
  }
}
