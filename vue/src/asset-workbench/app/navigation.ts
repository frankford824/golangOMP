import type { Component } from 'vue'
import {
  BarChart3,
  Calculator,
  ClipboardCheck,
  FileUp,
  HardDrive,
  LayoutDashboard,
  ReceiptText,
  ScrollText,
  UserRound,
  UsersRound,
} from 'lucide-vue-next'

import {
  assetWorkbenchRouteAccess,
  assetWorkbenchSettingsPaths,
  canAccessPath,
  routeAccessForPath,
  type AssetWorkbenchSettingsPath,
} from './access'
import type { AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'

export interface WorkbenchNavItem {
  to: string
  label: string
  subtitle?: string
  aliases?: readonly string[]
  icon: Component
  requires: readonly string[]
  hub?: 'settlement'
}

export interface WorkbenchCommandItem {
  id: string
  kind: 'navigate' | 'action' | 'search'
  group: string
  label: string
  subtitle?: string
  aliases?: readonly string[]
  to?: string
  icon?: Component
  requires?: readonly string[]
  run?: () => void | Promise<void>
}

const DAILY_NAV_DEFS: Array<{ path: string; icon: Component; requires?: readonly string[]; hub?: 'settlement' }> = [
  { path: '/', icon: LayoutDashboard },
  { path: '/upload', icon: FileUp, requires: assetWorkbenchRouteAccess['/upload'].requiresAnyCapability ?? [] },
  { path: '/drive', icon: HardDrive, requires: assetWorkbenchRouteAccess['/drive'].requiresAnyCapability ?? [] },
  { path: '/upload-overview', icon: ScrollText, requires: assetWorkbenchRouteAccess['/upload-overview'].requiresAnyCapability ?? [] },
  { path: '/quality-errors', icon: ClipboardCheck, requires: assetWorkbenchRouteAccess['/quality-errors'].requiresAnyCapability ?? [] },
  { path: '/settlement', icon: ReceiptText, requires: assetWorkbenchRouteAccess['/settlement'].requiresAnyCapability ?? [], hub: 'settlement' },
]

const SETTINGS_NAV_DEFS: Array<{ path: AssetWorkbenchSettingsPath; icon: Component }> = [
  { path: '/settings/pricing', icon: Calculator },
  { path: '/settings/people', icon: UsersRound },
  { path: '/settings/members', icon: UserRound },
  { path: '/settings/events', icon: ScrollText },
]

const SETTLEMENT_HUB_TABS = [
  { path: '/settlement', icon: ReceiptText },
  { path: '/reports', icon: BarChart3 },
] as const

function accessFor(path: string) {
  return routeAccessForPath(path)
}

export function buildDailyNavItems(): WorkbenchNavItem[] {
  return DAILY_NAV_DEFS.map(({ path, icon, requires, hub }) => {
    const access = accessFor(path)
    const hubLabel = hub === 'settlement' ? '结算' : access?.label ?? path
    const hubSubtitle = hub === 'settlement' ? '工资预览与统计' : access?.subtitle
    return {
      to: path,
      label: hub === 'settlement' ? hubLabel : access?.label ?? path,
      subtitle: hubSubtitle,
      aliases: access?.aliases,
      icon,
      requires: requires ?? [],
      hub,
    }
  })
}

export function buildSettingsNavItems(): WorkbenchNavItem[] {
  return SETTINGS_NAV_DEFS.map(({ path, icon }) => {
    const access = accessFor(path)
    return {
      to: path,
      label: access?.label ?? path,
      subtitle: access?.subtitle,
      aliases: access?.aliases,
      icon,
      requires: access?.requiresAnyCapability ?? [],
    }
  })
}

export function visibleDailyNavItems(bootstrap: AssetWorkbenchBootstrap | null): WorkbenchNavItem[] {
  return buildDailyNavItems().filter((item) => canAccessPath(bootstrap, item.to))
}

export function visibleSettingsNavItems(bootstrap: AssetWorkbenchBootstrap | null): WorkbenchNavItem[] {
  return buildSettingsNavItems().filter((item) => canAccessPath(bootstrap, item.to))
}

export function settlementHubTabs(bootstrap: AssetWorkbenchBootstrap | null) {
  return SETTLEMENT_HUB_TABS.filter(({ path }) => canAccessPath(bootstrap, path)).map(({ path, icon }) => {
    const access = accessFor(path)
    return {
      path,
      icon,
      label: access?.label ?? path,
      subtitle: access?.subtitle,
    }
  })
}

export function commandHaystack(item: Pick<WorkbenchCommandItem, 'label' | 'subtitle' | 'aliases' | 'to'>): string {
  return [item.label, item.subtitle, item.to, ...(item.aliases ?? [])].filter(Boolean).join(' ').toLowerCase()
}

export function buildCommandItems(
  bootstrap: AssetWorkbenchBootstrap | null,
  handlers: {
    navigate: (to: string) => void | Promise<void>
    markAllRead?: () => void | Promise<void>
    searchOverview?: (query: string) => void | Promise<void>
    generateSettlement?: () => void | Promise<void>
    exportSettlement?: () => void | Promise<void>
    exportReport?: () => void | Promise<void>
  },
): WorkbenchCommandItem[] {
  const items: WorkbenchCommandItem[] = []

  for (const nav of visibleDailyNavItems(bootstrap)) {
    items.push({
      id: `nav:${nav.to}`,
      kind: 'navigate',
      group: '日常导航',
      label: nav.label,
      subtitle: nav.subtitle,
      aliases: nav.aliases,
      to: nav.to,
      icon: nav.icon,
      requires: nav.requires,
      run: () => handlers.navigate(nav.to),
    })
  }

  for (const tab of settlementHubTabs(bootstrap)) {
    if (tab.path === '/settlement') continue
    items.push({
      id: `nav:${tab.path}`,
      kind: 'navigate',
      group: '结算与查询',
      label: tab.label,
      subtitle: tab.subtitle,
      to: tab.path,
      icon: tab.icon,
      requires: assetWorkbenchRouteAccess[tab.path as '/reports'].requiresAnyCapability,
      run: () => handlers.navigate(tab.path),
    })
  }

  for (const nav of visibleSettingsNavItems(bootstrap)) {
    items.push({
      id: `nav:${nav.to}`,
      kind: 'navigate',
      group: '设置',
      label: nav.label,
      subtitle: nav.subtitle,
      aliases: nav.aliases,
      to: nav.to,
      icon: nav.icon,
      requires: nav.requires,
      run: () => handlers.navigate(nav.to),
    })
  }

  if (handlers.markAllRead) {
    items.push({
      id: 'action:mark-all-read',
      kind: 'action',
      group: '动作',
      label: '全部标为已读',
      subtitle: '消息收件箱',
      aliases: ['通知'],
      run: handlers.markAllRead,
    })
  }

  if (handlers.generateSettlement && canAccessPath(bootstrap, '/settlement')) {
    items.push({
      id: 'action:generate-settlement',
      kind: 'action',
      group: '动作',
      label: '生成结算批次',
      subtitle: '前往本月结算页确认后操作',
      aliases: ['结算工资'],
      run: handlers.generateSettlement,
    })
  }

  if (handlers.exportSettlement && canAccessPath(bootstrap, '/settlement')) {
    items.push({
      id: 'action:export-settlement',
      kind: 'action',
      group: '动作',
      label: '导出工资',
      subtitle: '请在本页选择批次后导出',
      run: handlers.exportSettlement,
    })
  }

  if (handlers.exportReport && canAccessPath(bootstrap, '/reports')) {
    items.push({
      id: 'action:export-report',
      kind: 'action',
      group: '动作',
      label: '导出计件统计',
      subtitle: '请在本页确认业务月后导出',
      run: handlers.exportReport,
    })
  }

  return items
}

export function filterCommandItems(items: WorkbenchCommandItem[], query: string): WorkbenchCommandItem[] {
  const normalized = query.trim().toLowerCase()
  if (!normalized) return items
  const searchMatches = items.filter((item) => item.kind === 'search' || commandHaystack(item).includes(normalized))
  if (searchMatches.length > 0) return searchMatches
  return items.filter((item) => commandHaystack(item).includes(normalized))
}

export function appendSearchCommand(
  items: WorkbenchCommandItem[],
  query: string,
  bootstrap: AssetWorkbenchBootstrap | null,
  handler: (query: string) => void | Promise<void>,
): WorkbenchCommandItem[] {
  const trimmed = query.trim()
  if (trimmed.length < 2 || !canAccessPath(bootstrap, '/drive')) return items
  return [
    ...items,
    {
      id: `search:${trimmed}`,
      kind: 'search',
      group: '搜索',
      label: `全站搜索「${trimmed}」`,
      subtitle: '素材网盘内搜索运营素材、交稿文件与订单',
      aliases: ['总盘查询'],
      run: () => handler(trimmed),
    },
  ]
}

export { assetWorkbenchSettingsPaths }
