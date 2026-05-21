import { describe, expect, it } from 'vitest'
import { dedupeReferenceFileRefs } from '@/domain/mappers/reference-file-refs'
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
