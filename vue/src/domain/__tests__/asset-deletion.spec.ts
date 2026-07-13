import { describe, expect, it } from 'vitest'
import {
  assetDeletionSuccessMessage,
  assetDeletionUnavailableReason,
  canDeleteAssetResource,
} from '@/domain/asset-deletion'

const completedAsset = {
  isExternal: false,
  taskId: 2219,
  assetId: 9881,
  assetKind: 'delivery',
  usableState: 'ready_for_use',
  taskStatus: 'Completed',
}

describe('asset deletion gate', () => {
  it.each(['CustomizationReviewer', 'Audit_A', 'Audit_B', 'AssetManager'])(
    'allows %s to delete a completed current resource',
    (role) => {
      expect(canDeleteAssetResource(completedAsset, [role])).toBe(true)
    },
  )

  it('keeps maintenance roles out of active tasks while SuperAdmin retains lifecycle authority', () => {
    const activeAsset = { ...completedAsset, taskStatus: 'PendingAuditA' }
    expect(canDeleteAssetResource(activeAsset, ['Audit_A'])).toBe(false)
    expect(assetDeletionUnavailableReason(activeAsset, ['Audit_A'])).toContain('只能删除已结单任务')
    expect(canDeleteAssetResource(activeAsset, ['super_admin'])).toBe(true)
  })

  it('blocks unrelated roles and non-current resources', () => {
    expect(canDeleteAssetResource(completedAsset, ['Designer'])).toBe(false)
    expect(canDeleteAssetResource({ ...completedAsset, isExternal: true }, ['SuperAdmin'])).toBe(false)
    expect(canDeleteAssetResource({ ...completedAsset, usableState: 'history' }, ['SuperAdmin'])).toBe(false)
    expect(canDeleteAssetResource({ ...completedAsset, assetKind: 'preview' }, ['SuperAdmin'])).toBe(false)
  })

  it('explains that deleting a completed resource does not reopen the task', () => {
    expect(assetDeletionSuccessMessage('Completed')).toContain('任务状态未改变')
  })
})
