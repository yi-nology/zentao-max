import { ref } from 'vue'
import { getAccountInfo } from '@/api/zentao'

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

  return { domain, buildUrl, fetchDomain }
}
