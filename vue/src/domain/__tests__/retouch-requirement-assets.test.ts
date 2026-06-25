import { describe, expect, it } from 'vitest'
import {
  collectRetouchRequirementBatchAssetIds,
  retouchRequirementReferenceRefsToDisplayItems,
  retouchSourceAssetsToDisplayItems,
} from '@/domain/retouch-requirement-assets'
import type { ReferenceFileRef } from '@/services/api/assetsApi'

describe('retouchRequirementReferenceRefsToDisplayItems', () => {
  it('maps filename and numeric asset id for download', () => {
    const refs: ReferenceFileRef[] = [
      {
        asset_id: '601',
        download_url: 'https://cdn.example/photo.jpg',
        filename: '运营参考.jpg',
        mime_type: 'image/jpeg',
        file_size: 1024,
      },
    ]
    const items = retouchRequirementReferenceRefsToDisplayItems(refs, 'req-1')
    expect(items).toHaveLength(1)
    expect(items[0].fileName).toBe('运营参考.jpg')
    expect(items[0].assetId).toBe('601')
    expect(items[0].previewSrc).toContain('photo.jpg')
  })
})

describe('retouchSourceAssetsToDisplayItems', () => {
  it('exposes asset id and original filename', () => {
    const items = retouchSourceAssetsToDisplayItems([
      {
        id: '99',
        file_role: 'source',
        current_version: {
          id: '100',
          file_name: 'pack.psd',
          download_url: 'https://cdn.example/pack.psd',
          file_size: 2048,
          mime_type: 'application/octet-stream',
        },
      } as never,
    ])
    expect(items[0].assetId).toBe('99')
    expect(items[0].fileName).toBe('pack.psd')
  })
})

describe('collectRetouchRequirementBatchAssetIds', () => {
  it('collects unique numeric ids for future batch download', () => {
    const ids = collectRetouchRequirementBatchAssetIds(
      [{ asset_id: '10', download_url: 'https://x/a.png', filename: 'a.png' }],
      [{ id: '20', file_role: 'source' } as never],
    )
    expect(ids.referenceAssetIds).toEqual([10])
    expect(ids.sourceAssetIds).toEqual([20])
  })
})
