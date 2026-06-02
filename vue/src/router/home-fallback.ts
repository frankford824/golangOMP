import type {
  Router,
  RouteLocationNormalizedLoaded,
  RouteLocationRaw,
  RouteMeta,
  RouteRecordName,
} from 'vue-router'
import type { PermissionEnumValue } from '@/types'

interface PermissionStoreLike {
  currentUser: unknown
  hasMenu: (key: string) => boolean
  hasPermission: (perms: PermissionEnumValue | PermissionEnumValue[]) => boolean
  hasAction?: (key: string) => boolean
}

interface HomeRouteCandidate {
  name: RouteRecordName
  menuKey: string
}

// 登录后/受限回退时的首选落地页次序。与后端 `frontend_access.menus` 对齐：
// 命中哪个菜单就跳到对应首屏，让用户落在自己确实拥有权限的页面，避免 403。
const HOME_ROUTE_CANDIDATES: HomeRouteCandidate[] = [
  { name: 'Dashboard', menuKey: 'dashboard' },
  { name: 'TaskList', menuKey: 'task_list' },
  { name: 'AssetsIndex', menuKey: 'resource_management' },
  { name: 'DataCenter', menuKey: 'report_center' },
  { name: 'DataCenter', menuKey: 'export_center' },
  { name: 'AuditLog', menuKey: 'audit_log' },
  { name: 'DataCenter', menuKey: 'logs_center' },
  { name: 'Finance', menuKey: 'finance' },
  { name: 'DataCenter', menuKey: 'kpi' },
  { name: 'RuleConfig', menuKey: 'rules' },
  { name: 'OrgIndex', menuKey: 'org_admin' },
  { name: 'UserManagement', menuKey: 'user_admin' },
]

export function isDashboardEntryRoute(to: RouteLocationNormalizedLoaded): boolean {
  return to.name === 'Dashboard' || to.path === '/'
}

export function resolveFirstAccessibleHomeRoute(
  permissionsStore: PermissionStoreLike,
): RouteLocationRaw | null {
  for (const candidate of HOME_ROUTE_CANDIDATES) {
    if (candidate.name === 'TaskList') {
      const canReviewCustomization =
        permissionsStore.hasAction?.('task.customization.review') === true ||
        permissionsStore.hasAction?.('task.customization.effect_review') === true ||
        permissionsStore.hasMenu('customization_management')
      const canReviewNormal =
        permissionsStore.hasAction?.('task.audit.review') === true ||
        permissionsStore.hasAction?.('task.audit.claim') === true
      if (canReviewCustomization && !canReviewNormal) {
        return {
          name: candidate.name,
          query: {
            task_category: 'customization',
            status: 'PendingCustomizationReview,PendingEffectReview',
          },
        }
      }
      if (canReviewNormal && !canReviewCustomization) {
        return {
          name: candidate.name,
          query: {
            task_category: 'normal',
            status: 'PendingAuditA,PendingAuditB',
          },
        }
      }
      if (canReviewCustomization && canReviewNormal) {
        return {
          name: candidate.name,
          query: {
            status: 'PendingAuditA,PendingAuditB,PendingCustomizationReview,PendingEffectReview',
          },
        }
      }
      return { name: candidate.name }
    }
    if (permissionsStore.hasMenu(candidate.menuKey)) {
      return { name: candidate.name }
    }
  }
  return null
}

function resolveRequiredMenuKeys(meta: RouteMeta | undefined): string[] {
  if (!meta) return []
  const raw = (meta as { requiredMenuKey?: unknown }).requiredMenuKey
  if (typeof raw === 'string' && raw.trim()) return [raw.trim()]
  if (Array.isArray(raw)) {
    return raw
      .map((k) => (typeof k === 'string' ? k.trim() : ''))
      .filter((k) => k.length > 0)
  }
  return []
}

export function canAccessRouteByMeta(
  to: { meta: RouteMeta; path: string; name?: RouteRecordName | null },
  permissionsStore: PermissionStoreLike,
): boolean {
  if (to.meta?.requiresAuth === true && !permissionsStore.currentUser) return false

  const requiredPermissions = (to.meta?.requiredPermissions ?? []) as PermissionEnumValue[]
  if (requiredPermissions.length > 0 && !permissionsStore.hasPermission(requiredPermissions)) {
    return false
  }

  const requiredMenuKeys = resolveRequiredMenuKeys(to.meta)
  if (requiredMenuKeys.length > 0) {
    return requiredMenuKeys.some((key) => permissionsStore.hasMenu(key))
  }

  return true
}

export function resolvePostLoginLandingRoute(
  router: Router,
  permissionsStore: PermissionStoreLike,
  redirectPath?: string,
): RouteLocationRaw {
  const redirect = (redirectPath ?? '').trim()
  if (redirect) {
    const resolvedRedirect = router.resolve(redirect)
    if (resolvedRedirect.matched.length > 0 && canAccessRouteByMeta(resolvedRedirect, permissionsStore)) {
      return redirect
    }
  }

  return resolveFirstAccessibleHomeRoute(permissionsStore) ?? { name: 'Forbidden' }
}
