import { describe, expect, it } from 'vitest'
import {
  assetReplacementScopeSKUCode,
  assetReplacementSuccessMessage,
  assetReplacementUnavailableReason,
  canReplaceAssetResource,
  taskStatusBlocksAssetReplacement,
} from '@/domain/asset-replacement'

const baseAsset = {
  isExternal: false,
  taskId: 2219,
  assetId: 9881,
  assetKind: 'delivery',
  usableState: 'ready_for_use',
}

describe('asset replacement gate', () => {
  it('allows replacement while backend upload-session status gate allows the task', () => {
    expect(canReplaceAssetResource({ ...baseAsset, taskStatus: 'PendingAuditA' })).toBe(true)
    expect(canReplaceAssetResource({ ...baseAsset, taskStatus: 'PendingCustomizationReview' })).toBe(true)
    expect(canReplaceAssetResource({ ...baseAsset, taskStatus: 'pending_customization_production' })).toBe(true)
  })

  it('allows replacing current resources after close but still blocks cancelled tasks', () => {
    expect(canReplaceAssetResource({ ...baseAsset, taskStatus: 'Completed' })).toBe(true)
    expect(assetReplacementUnavailableReason({ ...baseAsset, taskStatus: 'Completed' })).toBe('')
    expect(assetReplacementSuccessMessage({ ...baseAsset, taskStatus: 'Completed' })).toBe(
      '资源已修改，新版本已生效，任务状态未改变',
    )
    expect(assetReplacementUnavailableReason({ ...baseAsset, taskStatus: 'Cancelled' })).toContain('已取消')
  })

  it('keeps legacy responses without task_status on the old path', () => {
    expect(canReplaceAssetResource(baseAsset)).toBe(true)
    expect(taskStatusBlocksAssetReplacement(undefined)).toBe('')
  })

  it('blocks external, archived, historical, cleaned, and non-replaceable asset kinds', () => {
    expect(canReplaceAssetResource({ ...baseAsset, isExternal: true })).toBe(false)
    expect(canReplaceAssetResource({ ...baseAsset, isArchived: true })).toBe(false)
    expect(canReplaceAssetResource({ ...baseAsset, archiveStatus: 'archived' })).toBe(false)
    expect(canReplaceAssetResource({ ...baseAsset, usableState: 'history' })).toBe(false)
    expect(canReplaceAssetResource({ ...baseAsset, usableState: 'cleaned' })).toBe(false)
    expect(canReplaceAssetResource({ ...baseAsset, assetKind: 'preview' })).toBe(false)
  })

  it('submits only the asset scope when replacing and never falls back to the product SKU', () => {
    expect(assetReplacementScopeSKUCode({ scope_sku_code: 'SKU-SCOPE', sku_code: 'SKU-PRODUCT' })).toBe('SKU-SCOPE')
    expect(assetReplacementScopeSKUCode({ targetSkuCode: 'SKU-TARGET', primary_sku_code: 'SKU-PRIMARY' })).toBe('SKU-TARGET')
    expect(assetReplacementScopeSKUCode({ sku_code: 'DZK000142', primary_sku_code: 'DZK000142' })).toBe('')
  })
})
