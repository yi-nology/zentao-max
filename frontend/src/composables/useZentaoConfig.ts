import { ElMessage } from 'element-plus'
import { ref } from 'vue'
import { getAccountInfo } from '@/api/zentao'
import { openExternalLink } from './useExternalLink'

let cachedDomain: string | null = null
let fetchPromise: Promise<string> | null = null

const domain = ref('')

async function fetchDomain(): Promise<string> {
  if (cachedDomain !== null) return cachedDomain
  if (fetchPromise) return fetchPromise

  fetchPromise = (async () => {
    try {
      const res = await getAccountInfo()
      if (res.code === 200 && res.data?.domain) {
        cachedDomain = res.data.domain.replace(/\/$/, '')
        domain.value = cachedDomain
        return cachedDomain
      }
    } catch { /* ignore */ }
    cachedDomain = ''
    domain.value = ''
    return ''
  })()

  return fetchPromise
}

export function useZentaoConfig() {
  fetchDomain()

  const buildUrl = (path: string): string => {
    const d = domain.value
    return d ? `${d}/${path}` : ''
  }

  // 打开禅道实体 view 页（issue #6：Bugs/Tasks/Stories 三处
  // buildZentaoUrl(`x-view-N.html`) + openZentaoLink 收敛为一句 openEntity）
  const openEntity = (type: 'bug' | 'task' | 'story', id: number): void => {
    const url = buildUrl(`${type}-view-${id}.html`)
    if (!url) {
      ElMessage.warning('禅道地址未配置，请检查系统设置')
      return
    }
    void openExternalLink(url)
  }

  return { domain, buildUrl, fetchDomain, openEntity }
}
