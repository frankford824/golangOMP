import type { AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'

export interface AssetWorkbenchRouteAccess {
  label: string
  simple?: boolean
  requiresAnyCapability?: readonly string[]
  anyAuthenticated?: boolean
}

export const assetWorkbenchRouteAccess = {
  '/': { label: '首页', simple: true },
  '/upload': {
    label: '交作品',
    simple: true,
    requiresAnyCapability: ['asset.workbench.submit'],
  },
  '/my-settlement': { label: '看收入', simple: true },
  '/account': { label: '个人中心', simple: true, anyAuthenticated: true },
  '/submissions': {
    label: '维护专区',
    requiresAnyCapability: ['asset.workbench.submit', 'asset.workbench.manage', 'asset.workbench.settlement'],
  },
  '/materials': {
    label: '素材库',
    simple: true,
    requiresAnyCapability: ['asset.workbench.material.download', 'asset.workbench.system_search'],
  },
  '/cost-center': {
    label: '成本中心',
    requiresAnyCapability: ['asset.workbench.cost_center.manage'],
  },
  '/template-assignments': {
    label: '模板下发',
    requiresAnyCapability: ['asset.workbench.template.assign'],
  },
  '/settlement': {
    label: '结算',
    requiresAnyCapability: ['asset.workbench.settlement'],
  },
  '/people': {
    label: '人员',
    requiresAnyCapability: ['asset.workbench.profile', 'asset.workbench.profile.manage'],
  },
  '/members': {
    label: '成员管理',
    requiresAnyCapability: ['asset.workbench.member.identity'],
  },
  '/events': {
    label: '日志',
    requiresAnyCapability: ['asset.workbench.manage', 'asset.workbench.cost_center.manage', 'asset.workbench.settlement'],
  },
  '/403': { label: '无权访问', anyAuthenticated: true },
} as const satisfies Record<string, AssetWorkbenchRouteAccess>

export type AssetWorkbenchPath = keyof typeof assetWorkbenchRouteAccess

export const assetWorkbenchSimplePaths = new Set(
  Object.entries(assetWorkbenchRouteAccess)
    .filter(([, access]) => 'simple' in access && access.simple)
    .map(([path]) => path),
)

export function routeAccessForPath(path: string): AssetWorkbenchRouteAccess | undefined {
  return assetWorkbenchRouteAccess[path as AssetWorkbenchPath]
}

export function hasAnyCapability(bootstrap: AssetWorkbenchBootstrap | null, required: readonly string[] = []): boolean {
  if (required.length === 0) return true
  const capabilities = new Set(bootstrap?.capabilities ?? [])
  return required.some((item) => capabilities.has(item))
}

export function canAccessAssetWorkbenchRoute(
  bootstrap: AssetWorkbenchBootstrap | null,
  access: AssetWorkbenchRouteAccess | undefined,
): boolean {
  if (!access || access.anyAuthenticated) return true
  if (!bootstrap) return true
  if (!bootstrap.is_admin) {
    if (access.simple !== true) return false
    return hasAnyCapability(bootstrap, access.requiresAnyCapability)
  }
  return hasAnyCapability(bootstrap, access.requiresAnyCapability)
}
