import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw, Router } from 'vue-router'
import type { AppRoute } from '@/types/router'
import Layout from '../views/Layout.vue'

const routes: AppRoute[] = [
  {
    path: '/',
    component: Layout,
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('../views/Dashboard.vue'),
        meta: { title: '仪表盘' }
      },
      {
        path: 'bugs',
        name: 'Bugs',
        component: () => import('../views/Bugs.vue'),
        meta: { title: 'Bug 查询' }
      },
      {
        path: 'stories',
        name: 'Stories',
        component: () => import('../views/Stories.vue'),
        meta: { title: '需求查询' }
      },
      {
        path: 'tasks',
        name: 'Tasks',
        component: () => import('../views/Tasks.vue'),
        meta: { title: '任务查询' }
      },
      {
        path: 'cases',
        name: 'Cases',
        component: () => import('../views/Cases.vue'),
        meta: { title: '测试用例' }
      },
      {
        path: 'timelog',
        name: 'Timelog',
        component: () => import('../views/Timelog.vue'),
        meta: { title: '工时统计' }
      },
      {
        path: 'effort-calendar',
        name: 'EffortCalendar',
        component: () => import('../views/EffortCalendar.vue'),
        meta: { title: '工时日历' }
      },
      {
        path: 'mcp-guide',
        name: 'MCPGuide',
        component: () => import('../views/MCPGuide.vue'),
        meta: { title: 'MCP对接指南' }
      },
      {
        path: 'scheduler',
        name: 'Scheduler',
        component: () => import('../views/Scheduler.vue'),
        meta: { title: '定时任务' }
      },
      {
        // 不用 /health：生产模式下该路径被后端健康检查端点占用（Hertz 路由先于静态兜底），
        // 整页加载 /health 会拿到 JSON 而非本页
        path: 'health-check',
        name: 'HealthCheck',
        component: () => import('../views/HealthCheck.vue'),
        meta: { title: '心跳检测' }
      },
      {
        path: 'logs',
        name: 'Logs',
        component: () => import('../views/Logs.vue'),
        meta: { title: '系统日志' }
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('../views/Settings.vue'),
        meta: { title: '设置' }
      }
    ]
  },
  {
    path: '/init-status',
    name: 'InitStatus',
    component: () => import('../views/InitStatus.vue'),
    meta: { title: '初始化状态' }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
    meta: { title: '管理员登录' }
  },
  // 未知路径兜底（含旧书签 /health 等），避免深链 404 白屏
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' }
]

const router: Router = createRouter({
  history: createWebHistory(),
  routes: routes as RouteRecordRaw[]
})

export default router
