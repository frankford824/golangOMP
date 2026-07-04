import { describe, expect, it } from 'vitest'

import {
  accessibleSettingsPaths,
  canAccessAssetWorkbenchRoute,
  canAccessPath,
  firstAccessibleSettingsPath,
  hasAnyCapability,
  hasSettingsAccess,
  isConfigOnlyAdmin,
  routeAccessForPath,
} from './access'
import type { AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'

function bootstrap(overrides: Partial<AssetWorkbenchBootstrap>): AssetWorkbenchBootstrap {
  return {
    app: 'asset-workbench',
    version: 'test',
    timezone: 'Asia/Shanghai',
    oss_prefix: '',
    upload_session_ttl_seconds: 0,
    is_admin: false,
    capabilities: [],
    settlement_item_types: [],
    deferred_business_items: [],
    architecture_guardrails: [],
    ...overrides,
  }
}

describe('asset workbench access rules', () => {
  it('keeps simple user routes available to non-admin accounts', () => {
    const user = bootstrap({ is_admin: false, capabilities: ['asset.workbench.submit', 'asset.workbench.material.download'] })

    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/'))).toBe(true)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/upload'))).toBe(true)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/submissions'))).toBe(false)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/upload-overview'))).toBe(false)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/materials'))).toBe(true)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/my-settlement'))).toBe(true)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/account'))).toBe(true)
  })

  it('keeps capability-gated simple routes closed when the capability is missing', () => {
    const user = bootstrap({ is_admin: false, capabilities: [] })

    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/'))).toBe(true)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/upload'))).toBe(false)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/materials'))).toBe(false)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/account'))).toBe(true)
  })

  it('blocks non-admin accounts from maintenance-only routes', () => {
    const user = bootstrap({ is_admin: false, capabilities: ['asset.workbench.manage'] })

    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/settlement'))).toBe(false)
  })

  it('allows admin routes when any declared capability matches', () => {
    const admin = bootstrap({ is_admin: true, capabilities: ['asset.workbench.settlement'] })

    expect(hasAnyCapability(admin, ['asset.workbench.manage', 'asset.workbench.settlement'])).toBe(true)
    expect(canAccessAssetWorkbenchRoute(admin, routeAccessForPath('/settlement'))).toBe(true)
    expect(canAccessAssetWorkbenchRoute(admin, routeAccessForPath('/upload-overview'))).toBe(true)
    expect(canAccessAssetWorkbenchRoute(admin, routeAccessForPath('/materials'))).toBe(false)
  })

  it('registers settings child routes with explicit capabilities', () => {
    const pricingOnly = bootstrap({ is_admin: true, capabilities: ['asset.workbench.cost_center.manage'] })

    expect(canAccessPath(pricingOnly, '/settings/pricing')).toBe(true)
    expect(canAccessPath(pricingOnly, '/settings/members')).toBe(false)
    expect(canAccessPath(pricingOnly, '/settings/events')).toBe(true)
    expect(hasSettingsAccess(pricingOnly)).toBe(true)
    expect(firstAccessibleSettingsPath(pricingOnly)).toBe('/settings/pricing')
    expect(accessibleSettingsPaths(pricingOnly)).toEqual(['/settings/pricing', '/settings/events'])
  })

  it('does not resolve a default settings route before bootstrap is known', () => {
    expect(accessibleSettingsPaths(null)).toEqual([])
    expect(firstAccessibleSettingsPath(null)).toBeNull()
    expect(hasSettingsAccess(null)).toBe(false)
  })

  it('blocks settings routes when capability is missing', () => {
    const settlementOnly = bootstrap({ is_admin: true, capabilities: ['asset.workbench.settlement'] })

    expect(canAccessPath(settlementOnly, '/settings/pricing')).toBe(false)
    expect(canAccessPath(settlementOnly, '/settings/events')).toBe(true)
    expect(hasSettingsAccess(settlementOnly)).toBe(true)
  })

  it('detects config-only admins without daily operation access', () => {
    const hrAdmin = bootstrap({
      is_admin: true,
      capabilities: ['asset.workbench.profile.manage', 'asset.workbench.member.identity'],
    })

    expect(isConfigOnlyAdmin(hrAdmin)).toBe(true)
    expect(canAccessPath(hrAdmin, '/upload')).toBe(false)
    expect(firstAccessibleSettingsPath(hrAdmin)).toBe('/settings/people')
  })

  it('uses renamed labels for daily routes', () => {
    expect(routeAccessForPath('/')?.label).toBe('今日待办')
    expect(routeAccessForPath('/submissions')?.aliases).toContain('维护区')
    expect(routeAccessForPath('/upload-overview')?.label).toBe('上传总览')
    expect(routeAccessForPath('/settings/pricing')?.label).toBe('计价设置')
    expect(routeAccessForPath('/overview')?.aliases).toContain('总盘查询')
  })
})
