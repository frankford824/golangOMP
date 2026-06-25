import { describe, expect, it } from 'vitest'
import {
  dedupeReferenceFileRefs,
  filterTaskLevelBackendReferenceAssets,
  mergeReferenceFileRefsPreferBackend,
  referenceFileRefsFromBackendReferenceAssets,
} from '@/domain/mappers/reference-file-refs'
import type { BackendAsset } from '@/services/apiTypes'
import type { ReferenceFileRef } from '@/services/api/assetsApi'

describe('dedupeReferenceFileRefs', () => {
  it('collapses task and sku refs that share the same asset_id', () => {
    const shared: ReferenceFileRef = {
      asset_id: 'ref-a',
      ref_id: 'ref-a',
      download_url: 'https://cdn.example/a.png',
      filename: 'a.png',
    }
    const merged = dedupeReferenceFileRefs([
      shared,
      { ...shared, download_url: 'https://cdn.example/a-signed.png' },
    ])
    expect(merged).toHaveLength(1)
  })

  it('mother-task batch union: task 2 + sku 2 with same asset_ids counts as 2', () => {
    const refA: ReferenceFileRef = {
      asset_id: 'asset-a',
      ref_id: 'asset-a',
      download_url: 'https://cdn.example/a.png',
    }
    const refB: ReferenceFileRef = {
      asset_id: 'asset-b',
      ref_id: 'asset-b',
      download_url: 'https://cdn.example/b.png',
    }
    const taskUnion = dedupeReferenceFileRefs([refA, refB])
    const skuMerged = dedupeReferenceFileRefs([refA, refB])
    expect(taskUnion).toHaveLength(2)
    expect(skuMerged).toHaveLength(2)
    expect(taskUnion.length).toBe(skuMerged.length)
  })

  it('does not collapse different SKUs when filename matches but asset_id differs', () => {
    const merged = dedupeReferenceFileRefs([
      {
        asset_id: 'sku-1-ref',
        ref_id: 'sku-1-ref',
        download_url: 'https://cdn.example/same-name.png',
        filename: 'sample.png',
        file_size: 100,
      },
      {
        asset_id: 'sku-2-ref',
        ref_id: 'sku-2-ref',
        download_url: 'https://cdn.example/other.png',
        filename: 'sample.png',
        file_size: 100,
      },
    ])
    expect(merged).toHaveLength(2)
  })
})

describe('filterTaskLevelBackendReferenceAssets', () => {
  it('drops assets scoped to a retouch requirement', () => {
    const assets: BackendAsset[] = [
      { id: '1', file_role: 'reference', retouch_requirement_id: 9 } as BackendAsset,
      { id: '2', file_role: 'reference' } as BackendAsset,
    ]
    const filtered = filterTaskLevelBackendReferenceAssets(assets)
    expect(filtered).toHaveLength(1)
    expect(filtered[0]?.id).toBe('2')
  })
})

describe('referenceFileRefsFromBackendReferenceAssets', () => {
  it('maps reference assets and dedupes against legacy refs by asset_id', () => {
    const assets: BackendAsset[] = [
      {
        id: '10',
        file_role: 'reference',
        asset_kind: 'reference',
        versions: [
          {
            id: 'v1',
            file_role: 'reference',
            asset_id: 'asset-new',
            ref_id: 'asset-new',
            download_url: 'https://cdn.example/new.png',
            file_name: 'new.png',
          },
        ],
      } as BackendAsset,
    ]
    const fromAssets = referenceFileRefsFromBackendReferenceAssets(assets)
    expect(fromAssets).toHaveLength(1)
    expect(fromAssets[0]?.asset_id).toBe('asset-new')
    const legacy: ReferenceFileRef = {
      asset_id: 'asset-new',
      ref_id: 'asset-new',
      download_url: 'https://cdn.example/new-signed.png',
      filename: 'new.png',
    }
    const merged = dedupeReferenceFileRefs([legacy, ...fromAssets])
    expect(merged).toHaveLength(1)
  })

  it('ignores requirement-scoped reference assets', () => {
    const assets: BackendAsset[] = [
      {
        id: 'req-ref',
        file_role: 'reference',
        asset_kind: 'reference',
        retouch_requirement_id: 12,
        versions: [
          {
            id: 'v-req',
            file_role: 'reference',
            download_url: 'https://cdn.example/req.png',
            file_name: 'req.png',
          },
        ],
      } as BackendAsset,
      {
        id: 'task-ref',
        file_role: 'reference',
        asset_kind: 'reference',
        versions: [
          {
            id: 'v-task',
            file_role: 'reference',
            download_url: 'https://cdn.example/task.png',
            file_name: 'task.png',
          },
        ],
      } as BackendAsset,
    ]
    const fromAssets = referenceFileRefsFromBackendReferenceAssets(assets)
    expect(fromAssets).toHaveLength(1)
    expect(fromAssets[0]?.filename).toBe('task.png')
  })
})

describe('mergeReferenceFileRefsPreferBackend', () => {
  it('uses backend current reference assets as authoritative when present', () => {
    const legacy: ReferenceFileRef = {
      asset_id: 'precreate-ref',
      ref_id: 'precreate-ref',
      download_url: '/v1/assets/files/tasks/task-create-reference/old.png',
      filename: 'old.png',
    }
    const current: ReferenceFileRef = {
      asset_id: '4198',
      download_url: '/v1/assets/files/tasks/RW-1/assets/AST-0001/v2/reference/new.png',
      filename: 'new.png',
    }

    const merged = mergeReferenceFileRefsPreferBackend([legacy], [current])

    expect(merged).toHaveLength(1)
    expect(merged[0]?.filename).toBe('new.png')
  })

  it('falls back to legacy refs when no backend reference asset exists', () => {
    const legacy: ReferenceFileRef = {
      asset_id: 'precreate-ref',
      download_url: '/v1/assets/files/tasks/task-create-reference/old.png',
      filename: 'old.png',
    }

    const merged = mergeReferenceFileRefsPreferBackend([legacy], [])

    expect(merged).toHaveLength(1)
    expect(merged[0]?.filename).toBe('old.png')
  })
})
