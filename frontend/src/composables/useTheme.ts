/**
 * 主题管理 composable
 * 支持三种模式：light / dark / auto（跟随系统）
 * 持久化到 localStorage 键 zentao-max-theme
 */

export type ThemeMode = 'light' | 'dark' | 'auto'

const STORAGE_KEY = 'zentao-max-theme'
const DARK_CLASS = 'dark'

/** 系统暗色偏好查询句柄（懒初始化） */
let systemMql: MediaQueryList | null = null

/** 获取存储的模式（默认 auto） */
export function getStoredThemeMode(): ThemeMode {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'light' || saved === 'dark' || saved === 'auto') return saved
  return 'auto'
}

/** 兼容旧版 key zentao-max-dark：旧值 '1' = dark, '0' = light */
export function migrateOldThemeKey(): void {
  const oldKey = 'zentao-max-dark'
  const oldVal = localStorage.getItem(oldKey)
  if (oldVal !== null && localStorage.getItem(STORAGE_KEY) === null) {
    localStorage.setItem(STORAGE_KEY, oldVal === '1' ? 'dark' : 'light')
    localStorage.removeItem(oldKey)
  }
}

/** 系统当前是否偏好暗色 */
export function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

/** 根据 mode 计算最终是否为暗色 */
export function resolveDark(mode: ThemeMode): boolean {
  if (mode === 'auto') return systemPrefersDark()
  return mode === 'dark'
}

/** 实际切换 html.dark 类 */
export function applyDark(isDark: boolean): void {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle(DARK_CLASS, isDark)
}

/** 设置主题模式 + 立即应用 */
export function setThemeMode(mode: ThemeMode): void {
  localStorage.setItem(STORAGE_KEY, mode)
  applyDark(resolveDark(mode))
}

/**
 * 初始化主题（应用启动时调用）：
 * 1. 迁移旧 key
 * 2. 立即应用，避免白闪
 * 3. 监听系统主题变化（auto 模式下自动切换）
 */
export function initTheme(): void {
  migrateOldThemeKey()
  const mode = getStoredThemeMode()
  applyDark(resolveDark(mode))

  if (typeof window !== 'undefined' && window.matchMedia) {
    systemMql = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
      if (getStoredThemeMode() === 'auto') {
        applyDark(systemPrefersDark())
      }
    }
    // addEventListener 在旧 Safari 不可用，回退到 addListener
    if (systemMql.addEventListener) {
      systemMql.addEventListener('change', handler)
    } else if ((systemMql as any).addListener) {
      ;(systemMql as any).addListener(handler)
    }
  }
}
