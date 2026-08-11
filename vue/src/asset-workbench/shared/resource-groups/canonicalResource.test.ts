import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  resourceGroupsApi,
  type FlatResourceItem,
  type ResourceGroup,
} from '@/services/api/resourceGroupsApi'
import {
  canonicalFileFromGroup,
  resolveCanonicalDownload,
  resolveCanonicalPreview,
} from './canonicalResource'

const group = {
  id: 8,
  task_id: 3,
  scope_kind: 'sku',
  lock_version: 1,
  migration_incomplete: false,
  finalized_revision_id: 70,
  finalized_revision: {
    id: 70,
    group_id: 8,
    revision_no: 2,
    status: 'finalized',
    mode: 'set',
    source_stage: 'audit',
    source_task_asset_id: 1001,
    created_by: 9,
    legacy_migration: false,
    created_at: '2026-08-10T00:00:00Z',
    source_file: { task_asset_id: 1001, file_name: 'source.psd', preview_url: '/v1/task-assets/1001/preview', download_url: '/v1/task-assets/1001/download' },
    references: [{ id: 501, revision_id: 70, reference_file_ref_id: 41, sort_order: 1, ref_id: 'ref-41', file_name: 'reference.png', preview_url: 'https://oss.example/reference.png', download_url: 'https://oss.example/reference.png' }],
    items: [{ id: 701, revision_id: 70, task_asset_id: 1002, sort_order: 1, file: { task_asset_id: 1002, file_name: 'final.png' } }],
  },
} as ResourceGroup

function item(role: FlatResourceItem['resource_role'], resourceItemID: number, taskAssetID?: number): FlatResourceItem {
  return {
    group_id: 8,
    task_id: 3,
    revision_id: 70,
    resource_item_id: resourceItemID,
    task_asset_id: taskAssetID,
    sort_order: 1,
    task_type: 'new_product_development',
    resource_role: role,
    file_name: role === 'reference' ? 'reference.png' : role === 'source' ? 'source.psd' : 'final.png',
    resource_created_at: '2026-08-10T00:00:00Z',
  }
}

describe('canonical resource resolver', () => {
  afterEach(() => vi.restoreAllMocks())

  it('resolves all three roles from the exact current revision member identity', () => {
    expect(canonicalFileFromGroup(group, item('reference', 501)).filename).toBe('reference.png')
    expect(canonicalFileFromGroup(group, item('source', 1001, 1001)).filename).toBe('source.psd')
    expect(canonicalFileFromGroup(group, item('final', 701, 1002)).filename).toBe('final.png')
  })

  it('uses the immutable task asset route for formalized preview and download', async () => {
    const preview = vi.spyOn(resourceGroupsApi, 'previewTaskAsset').mockResolvedValue({ download_mode: 'direct', download_url: 'https://oss.example/preview.png', filename: 'final.png', file_size: 10 })
    const download = vi.spyOn(resourceGroupsApi, 'downloadTaskAsset').mockResolvedValue({ download_mode: 'direct', download_url: 'https://oss.example/final.png', filename: 'final.png', file_size: 10 })
    const target = item('final', 701, 1002)

    await expect(resolveCanonicalPreview(target)).resolves.toMatchObject({ download_url: 'https://oss.example/preview.png' })
    await expect(resolveCanonicalDownload(target)).resolves.toMatchObject({ download_url: 'https://oss.example/final.png' })
    expect(preview).toHaveBeenCalledWith(1002, undefined)
    expect(download).toHaveBeenCalledWith(1002, undefined)
  })

  it('keeps a legacy reference on the exact resource-group snapshot instead of guessing by filename', async () => {
    const getGroup = vi.fn().mockResolvedValue(group)
    const target = item('reference', 501)

    await expect(resolveCanonicalPreview(target, getGroup)).resolves.toMatchObject({ download_url: 'https://oss.example/reference.png' })
    await expect(resolveCanonicalDownload(target, getGroup)).resolves.toMatchObject({ download_url: 'https://oss.example/reference.png' })
    expect(getGroup).toHaveBeenCalledWith(8)
  })

  it('fails closed when the listed revision is no longer current', () => {
    expect(() => canonicalFileFromGroup(group, { ...item('final', 701, 1002), revision_id: 69 })).toThrow('资源版本已更新')
  })
})
