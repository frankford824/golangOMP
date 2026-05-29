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
        path: 'tasks/excel-assist',
        name: 'TaskExcelAssistCreate',
        component: () => import('@/views/TaskExcelAssistCreateView.vue'),
        meta: {
          requiresAuth: true,
          requiredPermissions: ['task:create'],
        },
      },
      {
        path: 'tasks/:id',
        name: 'TaskDetail',
        component: () => import('@/views/TaskDetailView.vue'),
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
        component: () => import('@/views/AssetsIndexView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['resource_management', 'task_list'] },
      },
      {
        path: 'asset-center/:id',
        name: 'AssetDetail',
        component: () => import('@/views/AssetDetailView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: ['resource_management', 'task_list'] },
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
        component: () => import('@/views/org-permission/OrgPermissionView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'org_admin' },
      },
      {
        path: 'users',
        name: 'UserManagement',
        component: () => import('@/views/org-permission/UserManagementView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'user_admin' },
      },
      {
        path: 'export-center',
        name: 'ExportCenter',
        component: () => import('@/views/export/ExportCenterView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'export_center' },
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
        component: () => import('@/views/logs/LogsManagementView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'logs_center' },
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
        component: () => import('@/views/kpi/KpiView.vue'),
        meta: { requiresAuth: true, requiredMenuKey: 'kpi' },
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
