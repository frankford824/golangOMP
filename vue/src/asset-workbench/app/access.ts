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
  '/settings/dispatch',
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
  'asset.workbench.template.assign',
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
    subtitle: '按模板交稿计件',
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
    subtitle: '审核、分配与系统提醒',
    aliases: ['通知'],
    simple: true,
    anyAuthenticated: true,
  },
  '/submissions': {
    label: '查改作品',
    subtitle: '质检、改量、作废',
    aliases: ['维护区', '维护专区', '上传记录', '我的上传记录'],
    simple: true,
    requiresAnyCapability: ['asset.workbench.submit', 'asset.workbench.manage', 'asset.workbench.settlement'],
  },
  '/materials': {
    label: '素材库',
    subtitle: '搜运营素材并下载',
    simple: true,
    requiresAnyCapability: ['asset.workbench.material.download', 'asset.workbench.system_search'],
  },
  '/overview': {
    label: '全站搜索',
    subtitle: '素材/交稿/计件一条搜',
    aliases: ['总盘查询'],
    requiresAnyCapability: ['asset.workbench.manage', 'asset.workbench.settlement'],
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
    aliases: ['计件报表'],
    requiresAnyCapability: ['asset.workbench.settlement'],
  },
  '/settings': {
    label: '设置',
    subtitle: '计价、分配与人事配置',
    requiresAnyCapability: [...SETTINGS_CAPABILITIES],
  },
  '/settings/pricing': {
    label: '计价设置',
    subtitle: '改单价、扣款、补贴、大促',
    aliases: ['成本中心', '价格规则'],
    requiresAnyCapability: ['asset.workbench.cost_center.manage'],
  },
  '/settings/dispatch': {
    label: '分配任务',
    subtitle: '指定谁能交哪类稿',
    aliases: ['作品下发', '模板下发'],
    requiresAnyCapability: ['asset.workbench.template.assign'],
  },
  '/settings/people': {
    label: '员工档案',
    subtitle: '建档：姓名岗位收款',
    aliases: ['人员资料', '人员'],
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
    subtitle: '改单价、扣款、补贴、大促',
    aliases: ['成本中心'],
    requiresAnyCapability: ['asset.workbench.cost_center.manage'],
  },
  '/template-assignments': {
    label: '分配任务',
    subtitle: '指定谁能交哪类稿',
    aliases: ['作品下发'],
    requiresAnyCapability: ['asset.workbench.template.assign'],
  },
  '/people': {
    label: '员工档案',
    subtitle: '建档：姓名岗位收款',
    aliases: ['人员资料'],
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
