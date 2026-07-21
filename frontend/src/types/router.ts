import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    requiresAuth?: boolean
    icon?: string
    hidden?: boolean
  }
}

export interface RouteMeta {
  title?: string
  requiresAuth?: boolean
  icon?: string
  hidden?: boolean
}

export interface AppRoute {
  path: string
  name?: string
  component?: unknown
  meta?: RouteMeta
  children?: AppRoute[]
  redirect?: string
}
