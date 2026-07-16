import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import AppShell from '@/layouts/AppShell.vue'
import { usePermissionsStore } from '@/stores/permissions'
import type { PermissionEnumValue } from '@/types'
import { getToken } from '@/services/http'
import {
  isDashboardEntryRoute,
  resolveFirstAccessibleHomeRoute,
} from '@/router/home-fallback'
import { logsApi } from '@/services/api/logsApi'

/**
 * 路由门禁：以后端 `frontend_access.menus` 为 SoT（Single Source of Truth）。
 *
 * 每条受保护路由在 `meta.requiredMenuKey` 声明所需菜单 key。
 * - 字符串：必须命中该菜单。
 * - 字符串数组：命中任意一项即放行（用于跨入口可达的子页，例如
 *   `/tasks/:id` 由任务中心入口跳入）。
 *
 * 组件层（按钮级）仍保留 `can(...)` / `hasPage(...)` / `hasAction(...)`，
 * 不在此层做二次门禁。
 */

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/AuthView.vue'),
    meta: { layout: 'blank' },
  },
  {
    path: '/',
    component: AppShell,
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: () => import('@/views/DashboardView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'dashboard' },
      },
      {
        path: '403',
        name: 'Forbidden',
        component: () => import('@/views/error/403.vue'),
      },
      {
        path: 'tasks',
        name: 'TaskList',
        component: () => import('@/views/TaskListView.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: 'tasks/create',
        name: 'TaskCreate',
        component: () => import('@/views/TaskListView.vue'),
        meta: {
          requiresAuth: true,
          requiredPermissions: ['task:create'],
          openCreateModal: true,
        },
      },
      {
        path: 'tasks/sku-planning',
        name: 'PlanningSKUCreate',
        component: () => import('@/views/PlanningSKUCreateView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['planning_sku', 'task_list'] },
      },
      {
        path: 'tasks/:id',
        name: 'TaskDetail',
        component: () => import('@/views/TaskDetailV8View.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: 'tasks/:id/assets',
        name: 'TaskAssets',
        component: () => import('@/views/TaskAssetsView.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: 'me/task-drafts',
        name: 'MeTaskDrafts',
        component: () => import('@/views/profile/MyDraftsView.vue'),
        meta: { requiresAuth: true, emptyTitle: '任务草稿' },
      },
      {
        path: 'me',
        name: 'Me',
        component: () => import('@/views/profile/AccountView.vue'),
        meta: { requiresAuth: true, emptyTitle: '个人中心' },
      },
      {
        path: 'me/security',
        name: 'MeSecurity',
        component: () => import('@/views/profile/SecurityView.vue'),
        meta: { requiresAuth: true, emptyTitle: '安全设置' },
      },
      {
        path: 'me/org',
        name: 'MeOrg',
        component: () => import('@/views/profile/MyOrgView.vue'),
        meta: { requiresAuth: true, emptyTitle: '我的组织信息' },
      },
      {
        path: 'me/notifications',
        name: 'MeNotifications',
        component: () => import('@/views/profile/MyNotificationsView.vue'),
        meta: { requiresAuth: true, emptyTitle: '通知中心' },
      },
      {
        path: 'asset-center',
        name: 'AssetsIndex',
        component: () => import('@/views/ResourceGroupsView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['resource_management', 'task_list'] },
      },
      {
        path: 'asset-center/:id(ext-[^/]+)',
        name: 'ExternalAssetDetail',
        component: () => import('@/views/AssetDetailView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['resource_management', 'task_list'] },
      },
      {
        path: 'asset-center/:id(\\d+)',
        name: 'AssetDetail',
        component: () => import('@/views/ResourceGroupDetailView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['resource_management', 'task_list'] },
      },
      {
        path: 'products',
        name: 'ProductManagement',
        component: () => import('@/views/ProductManagementView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['product_management', 'resource_management'] },
      },
      {
        path: 'org',
        name: 'OrgIndex',
        component: () => import('@/views/org/OrgIndexView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['org_admin'], emptyTitle: '组织管理' },
      },
      {
        path: 'org/users',
        name: 'OrgUsers',
        component: () => import('@/views/org/UsersView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['org_admin'], emptyTitle: '用户管理' },
      },
      {
        path: 'org/departments',
        name: 'OrgDepartments',
        component: () => import('@/views/org/DepartmentsView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['org_admin'], emptyTitle: '部门管理' },
      },
      {
        path: 'org/teams',
        name: 'OrgTeams',
        component: () => import('@/views/org/TeamsView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['org_admin'], emptyTitle: '团队管理' },
      },
      {
        path: 'org/move-requests',
        name: 'OrgMoveRequests',
        component: () => import('@/views/org/MoveRequestsView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['org_admin'], emptyTitle: '异动申请' },
      },
      {
        path: 'data-center',
        name: 'DataCenter',
        component: () => import('@/views/data-center/DataCenterView.vue'),
        meta: {
          requiresAuth: true,
          requiredMenuKey: ['report_center', 'export_center', 'logs_center', 'kpi', 'finance'],
          emptyTitle: '数据中心',
        },
      },
      {
        path: 'reports',
        name: 'Reports',
        component: () => import('@/views/reports/ReportsHomeView.vue'),
        meta: {
          requiresAuth: true,
          requiredMenuKey: ['kpi', 'finance', 'report_center'],
          emptyTitle: '报表中心',
        },
      },
      {
        path: 'reports/task-throughput',
        name: 'ReportTaskThroughput',
        component: () => import('@/views/reports/TaskThroughputReportView.vue'),
        meta: {
          requiresAuth: true,
          requiredMenuKey: ['kpi', 'finance', 'report_center'],
          emptyTitle: '任务吞吐报表',
        },
      },
      {
        path: 'reports/module-dwell',
        name: 'ReportModuleDwell',
        component: () => import('@/views/reports/AgingReportView.vue'),
        meta: {
          requiresAuth: true,
          requiredMenuKey: ['kpi', 'finance', 'report_center'],
          emptyTitle: '模块停留报表',
        },
      },
      {
        path: 'reports/aging',
        redirect: { name: 'ReportModuleDwell' },
      },
      {
        path: 'rules',
        name: 'RuleConfig',
        component: () => import('@/views/RuleConfigView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'rules' },
      },
      {
        path: 'org-permission',
        name: 'OrgPermission',
        redirect: { name: 'UserManagement' },
        meta: { requiresAuth: true, requiredMenuKey: 'user_admin' },
      },
      {
        path: 'users',
        name: 'UserManagement',
        component: () => import('@/views/org-permission/UserManagementView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'user_admin' },
      },
      {
        path: 'access-policy',
        name: 'AccessPolicy',
        component: () => import('@/views/AccessPolicyView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['access_policy', 'user_admin'] },
      },
      {
        path: 'export-center',
        name: 'ExportCenter',
        redirect: { name: 'DataCenter', query: { tab: 'export' } },
      },
      {
        path: 'audit-log',
        name: 'AuditLog',
        component: () => import('@/views/audit/AuditLogView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'audit_log' },
      },
      {
        path: 'logs',
        name: 'LogsManagement',
        redirect: { name: 'DataCenter', query: { tab: 'business' } },
      },
      {
        path: 'finance',
        name: 'Finance',
        component: () => import('@/views/finance/FinanceView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'finance' },
      },
      {
        path: 'kpi',
        name: 'Kpi',
        redirect: { name: 'DataCenter', query: { tab: 'kpi' } },
      },
    ],
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

function resolveRequiredMenuKeys(meta: unknown): string[] {
  if (!meta || typeof meta !== 'object') return []
  const raw = (meta as { requiredMenuKey?: unknown }).requiredMenuKey
  if (typeof raw === 'string' && raw.trim()) return [raw.trim()]
  if (Array.isArray(raw)) {
    return raw
      .map((k) => (typeof k === 'string' ? k.trim() : ''))
      .filter((k) => k.length > 0)
  }
  return []
}

router.beforeEach(async (to, _from, next) => {
  const permissionsStore = usePermissionsStore()
  const requiresAuth = to.meta?.requiresAuth === true

  // ─── Token 会话恢复 ────────────────────────────────────────────────────────
  // 若本地有 token 但 store 中尚无用户信息（如硬刷新），尝试通过 /me 恢复会话
  if (getToken() && !permissionsStore.currentUser) {
    try {
      await permissionsStore.restoreSession()
    } catch {
      // 若 /me 失败（401），http 拦截器已清除 token，继续往下走即跳登录
    }
  }

  const currentUser = permissionsStore.currentUser

  if (requiresAuth && !currentUser) {
    next({ path: '/login', query: { redirect: to.fullPath } })
    return
  }

  const requiredRoles = Array.isArray((to.meta as { requiredRoles?: unknown })?.requiredRoles)
    ? ((to.meta as { requiredRoles: unknown[] }).requiredRoles as unknown[])
        .filter((role): role is string => typeof role === 'string' && role.trim().length > 0)
    : []
  if (requiredRoles.length > 0) {
    const actorRole = currentUser?.role ?? ''
    if (!requiredRoles.includes(actorRole)) {
      next('/403')
      return
    }
  }

  const requiredPermissions = (to.meta?.requiredPermissions ?? []) as PermissionEnumValue[]
  if (requiredPermissions.length > 0) {
    const has = permissionsStore.hasPermission(requiredPermissions)
    if (!has) {
      // eslint-disable-next-line no-console
      console.warn('[router] permission denied for route', to.fullPath, requiredPermissions)
      next('/403')
      return
    }
  }

  const requiredMenuKeys = resolveRequiredMenuKeys(to.meta)
  if (requiredMenuKeys.length > 0) {
    const hasAnyMenu = requiredMenuKeys.some((key) => permissionsStore.hasMenu(key))
    if (!hasAnyMenu) {
      if (isDashboardEntryRoute(to)) {
        const fallbackRoute = resolveFirstAccessibleHomeRoute(permissionsStore)
        if (fallbackRoute) {
          const resolvedFallback = router.resolve(fallbackRoute)
          if (resolvedFallback.name && resolvedFallback.name !== to.name) {
            next(fallbackRoute)
            return
          }
        }
      }
      next('/403')
      return
    }
  }

  next()
})

router.afterEach((to) => {
  if (typeof window === 'undefined') return
  if (!getToken()) return
  if (to.path === '/login') return

  const pageName = routePageName(to.meta, to.name, to.path)
  const componentID = typeof to.name === 'string' ? to.name : String(to.name ?? '')
  const taskID = numericRouteParam(to.params.id)

  window.setTimeout(() => {
    logsApi
      .recordTraceEvent({
        event_type: 'page_view',
        action: '打开页面',
        page_url: window.location.href,
        page_name: pageName,
        component_id: componentID,
        task_id: taskID,
        outcome: 'succeeded',
        payload: {
          route_path: to.path,
          route_name: componentID,
          query_keys: Object.keys(to.query),
        },
      })
      .catch(() => {
        // 前端行为追踪不能影响用户主流程。
      })
  }, 0)
})

function routePageName(meta: unknown, routeName: unknown, path: string): string {
  if (meta && typeof meta === 'object') {
    const title = (meta as { emptyTitle?: unknown }).emptyTitle
    if (typeof title === 'string' && title.trim()) return title.trim()
  }
  if (typeof routeName === 'string' && routeName.trim()) return routeName.trim()
  return path
}

function numericRouteParam(raw: unknown): number | undefined {
  const value = Array.isArray(raw) ? raw[0] : raw
  if (typeof value !== 'string' || !/^\d+$/.test(value)) return undefined
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined
}
