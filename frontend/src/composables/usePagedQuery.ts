import { reactive } from 'vue'

/** 列表页分页状态（issue #6：Bugs/Stories/Tasks/Timelog 四份逐字复制收敛）。 */
export interface Pagination {
  page: number
  pageSize: number
  total: number
}

/**
 * 分页 + 翻页处理器。onFetch 由调用方提供，内部做路由同步（syncRoute 白名单
 * 各页过滤器语义不同，留在各 view，不进 composable——与 WriteHandler 评估同理）。
 * size/page 变化统一：pageSize 变化回第 1 页，然后走 onFetch。
 */
export function usePagedQuery(onFetch: () => void, opts?: { guard?: () => boolean }) {
  const pagination = reactive<Pagination>({ page: 1, pageSize: 20, total: 0 })

  const handleSizeChange = (size: number): void => {
    if (opts?.guard && !opts.guard()) return
    pagination.pageSize = size
    pagination.page = 1
    onFetch()
  }
  const handlePageChange = (page: number): void => {
    if (opts?.guard && !opts.guard()) return
    pagination.page = page
    onFetch()
  }

  return { pagination, handleSizeChange, handlePageChange }
}
