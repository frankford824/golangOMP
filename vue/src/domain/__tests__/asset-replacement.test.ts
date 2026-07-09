import { describe, expect, it } from 'vitest'
import {
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
    expect(canReplaceAssetResource({ ...baseAsset, taskStatus: 'pending_customization_production' })).toBe(true)
  })

  it('blocks completed and cancelled tasks before opening the file picker', () => {
    expect(canReplaceAssetResource({ ...baseAsset, taskStatus: 'Completed' })).toBe(false)
    expect(assetReplacementUnavailableReason({ ...baseAsset, taskStatus: 'Completed' })).toContain('已结单')
    expect(assetReplacementUnavailableReason({ ...baseAsset, taskStatus: 'Cancelled' })).toContain('已取消')
  })

  it('keeps legacy responses without task_status on the old path', () => {
    expect(canReplaceAssetResource(baseAsset)).toBe(true)
    expect(taskStatusBlocksAssetReplacement(undefined)).toBe('')
  })

  it('blocks external, historical, cleaned, and non-replaceable asset kinds', () => {
    expect(canReplaceAssetResource({ ...baseAsset, isExternal: true })).toBe(false)
    expect(canReplaceAssetResource({ ...baseAsset, usableState: 'history' })).toBe(false)
    expect(canReplaceAssetResource({ ...baseAsset, usableState: 'cleaned' })).toBe(false)
    expect(canReplaceAssetResource({ ...baseAsset, assetKind: 'preview' })).toBe(false)
  })
})
