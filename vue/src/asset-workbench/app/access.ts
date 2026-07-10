import type { AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'

export interface AssetWorkbenchRouteAccess {
  label: string
  subtitle?: string
  aliases?: readonly string[]
  simple?: boolean
  requiresAnyCapability?: readonly string[]
  anyAuthenticated?: boolean
}

const SETTINGS_CHILD_PATHS = [
  '/settings/pricing',
  '/settings/people',
  '/settings/members',
  '/settings/events',
] as const

export type AssetWorkbenchSettingsPath = (typeof SETTINGS_CHILD_PATHS)[number]

const DAILY_OPERATION_CAPABILITIES = [
  'asset.workbench.submit',
  'asset.workbench.manage',
  'asset.workbench.settlement',
  'asset.workbench.material.download',
  'asset.workbench.system_search',
] as const

const SETTINGS_CAPABILITIES = [
  'asset.workbench.cost_center.manage',
  'asset.workbench.profile',
  'asset.workbench.profile.manage',
  'asset.workbench.member.identity',
  'asset.workbench.manage',
] as const

export const assetWorkbenchRouteAccess = {
  '/': {
    label: '今日待办',
    subtitle: '待质检清单与本月预估',
    aliases: ['总览', '首页'],
    simple: true,
  },
  '/upload': {
    label: '上传成品',
    subtitle: '按上传目录交稿计件',
    aliases: ['交作品'],
    simple: true,
    requiresAnyCapability: ['asset.workbench.submit'],
  },
  '/my-settlement': {
    label: '看收入',
    subtitle: '查看自己的结算明细',
    simple: true,
  },
  '/account': {
    label: '个人中心',
    subtitle: '资料与账号',
    simple: true,
    anyAuthenticated: true,
  },
  '/notifications': {
    label: '消息',
    subtitle: '上传、资料、作品检查与结算提醒',
    aliases: ['通知'],
    simple: true,
    anyAuthenticated: true,
  },
  '/submissions': {
    label: '上传记录',
    subtitle: '跳转到上传总览',
    aliases: ['维护区', '维护专区', '上传记录', '我的上传记录'],
    simple: true,
    requiresAnyCapability: ['asset.workbench.manage', 'asset.workbench.settlement'],
  },
  '/upload-overview': {
    label: '上传总览',
    subtitle: '全站上传台账与批量管理',
    aliases: ['上传台账', '上传记录', '作品总览', '作品台账'],
    requiresAnyCapability: ['asset.workbench.manage', 'asset.workbench.settlement'],
  },
  '/materials': {
    label: '素材库',
    subtitle: '已并入素材网盘',
    simple: true,
    requiresAnyCapability: ['asset.workbench.material.download', 'asset.workbench.system_search'],
  },
  '/drive': {
    label: '素材网盘',
    subtitle: '运营素材区 + 上传目录区',
    aliases: ['网盘', '素材网盘', '文件夹', '素材库', '上传记录', 'drive'],
    simple: true,
    requiresAnyCapability: [
      'asset.workbench.submit',
      'asset.workbench.manage',
      'asset.workbench.settlement',
      'asset.workbench.material.download',
      'asset.workbench.system_search',
    ],
  },
  '/overview': {
    label: '全站搜索',
    subtitle: '已并入素材网盘',
    aliases: ['总盘查询'],
    simple: true,
    requiresAnyCapability: [
      'asset.workbench.submit',
      'asset.workbench.manage',
      'asset.workbench.settlement',
      'asset.workbench.material.download',
      'asset.workbench.system_search',
    ],
  },
  '/settlement': {
    label: '本月结算',
    subtitle: '预览并出工资批次',
    aliases: ['结算工资', '结算'],
    requiresAnyCapability: ['asset.workbench.settlement'],
  },
  '/reports': {
    label: '计件统计',
    subtitle: '按月导出明细',
    aliases: ['计件明细'],
    requiresAnyCapability: ['asset.workbench.settlement'],
  },
  '/settings': {
    label: '设置',
    subtitle: '计价、人事与审计配置',
    requiresAnyCapability: [...SETTINGS_CAPABILITIES],
  },
  '/settings/pricing': {
    label: '计价设置',
    subtitle: '单价、质检扣款、补贴、活动价',
    aliases: ['计价设置', '单价设置', '质检扣款'],
    requiresAnyCapability: ['asset.workbench.cost_center.manage'],
  },
  '/settings/people': {
    label: '人员定级',
    subtitle: '建档、定级、收款资料',
    aliases: ['员工档案', '人员资料', '人员', '定级', '待定级'],
    requiresAnyCapability: ['asset.workbench.profile', 'asset.workbench.profile.manage'],
  },
  '/settings/members': {
    label: '账号权限',
    subtitle: '开通登录与角色',
    aliases: ['成员管理'],
    requiresAnyCapability: ['asset.workbench.member.identity'],
  },
  '/settings/events': {
    label: '操作记录',
    subtitle: '谁在何时改了什么',
    aliases: ['操作日志', '日志'],
    requiresAnyCapability: ['asset.workbench.manage', 'asset.workbench.cost_center.manage', 'asset.workbench.settlement'],
  },
  '/cost-center': {
    label: '计价设置',
    subtitle: '单价、质检扣款、补贴、活动价',
    aliases: ['计价设置', '单价设置'],
    requiresAnyCapability: ['asset.workbench.cost_center.manage'],
  },
  '/people': {
    label: '人员定级',
    subtitle: '建档、定级、收款资料',
    aliases: ['员工档案', '人员资料', '定级', '待定级'],
    requiresAnyCapability: ['asset.workbench.profile', 'asset.workbench.profile.manage'],
  },
  '/members': {
    label: '账号权限',
    subtitle: '开通登录与角色',
    aliases: ['成员管理'],
    requiresAnyCapability: ['asset.workbench.member.identity'],
  },
  '/events': {
    label: '操作记录',
    subtitle: '谁在何时改了什么',
    aliases: ['操作日志'],
    requiresAnyCapability: ['asset.workbench.manage', 'asset.workbench.cost_center.manage', 'asset.workbench.settlement'],
  },
  '/403': { label: '无权访问', anyAuthenticated: true },
} as const satisfies Record<string, AssetWorkbenchRouteAccess>

export type AssetWorkbenchPath = keyof typeof assetWorkbenchRouteAccess

export const assetWorkbenchSettingsPaths = SETTINGS_CHILD_PATHS

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

export function canAccessPath(bootstrap: AssetWorkbenchBootstrap | null, path: string): boolean {
  return canAccessAssetWorkbenchRoute(bootstrap, routeAccessForPath(path))
}

export function accessibleSettingsPaths(bootstrap: AssetWorkbenchBootstrap | null): AssetWorkbenchSettingsPath[] {
  if (!bootstrap) return []
  return SETTINGS_CHILD_PATHS.filter((path) => canAccessPath(bootstrap, path))
}

export function firstAccessibleSettingsPath(bootstrap: AssetWorkbenchBootstrap | null): AssetWorkbenchSettingsPath | null {
  return accessibleSettingsPaths(bootstrap)[0] ?? null
}

export function hasSettingsAccess(bootstrap: AssetWorkbenchBootstrap | null): boolean {
  return accessibleSettingsPaths(bootstrap).length > 0
}

export function hasDailyOperationAccess(bootstrap: AssetWorkbenchBootstrap | null): boolean {
  if (!bootstrap) return false
  return hasAnyCapability(bootstrap, DAILY_OPERATION_CAPABILITIES)
}

export function isConfigOnlyAdmin(bootstrap: AssetWorkbenchBootstrap | null): boolean {
  if (!bootstrap?.is_admin) return false
  if (!hasSettingsAccess(bootstrap)) return false
  return !hasDailyOperationAccess(bootstrap)
}

export function settlementHubPaths(): readonly string[] {
  return ['/settlement', '/reports']
}

export function isSettlementHubPath(path: string): boolean {
  return settlementHubPaths().includes(path)
}
