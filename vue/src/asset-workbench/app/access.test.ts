import { describe, expect, it } from 'vitest'

import { canAccessAssetWorkbenchRoute, hasAnyCapability, routeAccessForPath } from './access'
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

    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/submissions'))).toBe(false)
    expect(canAccessAssetWorkbenchRoute(user, routeAccessForPath('/settlement'))).toBe(false)
  })

  it('allows admin routes when any declared capability matches', () => {
    const admin = bootstrap({ is_admin: true, capabilities: ['asset.workbench.settlement'] })

    expect(hasAnyCapability(admin, ['asset.workbench.manage', 'asset.workbench.settlement'])).toBe(true)
    expect(canAccessAssetWorkbenchRoute(admin, routeAccessForPath('/settlement'))).toBe(true)
    expect(canAccessAssetWorkbenchRoute(admin, routeAccessForPath('/materials'))).toBe(false)
  })
})
