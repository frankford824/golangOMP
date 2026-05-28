import { describe, expect, it } from 'vitest'
import { mapRetouchRequirementsFromApi } from '@/domain/mappers/retouch-requirements-from-api'

describe('mapRetouchRequirementsFromApi', () => {
  it('maps reference_file_refs and source_assets on each requirement', () => {
    const rows = mapRetouchRequirementsFromApi([
      {
        id: 10,
        task_id: 1,
        description: '需求一',
        sort_order: 1,
        reference_file_refs: [
          {
            asset_id: 'ref-1',
            download_url: 'https://cdn.example/ref.png',
            filename: 'ref.png',
          },
        ],
        source_assets: [
          {
            id: 99,
            asset_type: 'source',
            current_version: {
              id: 100,
              file_name: 'pack.psd',
              download_url: 'https://cdn.example/pack.psd',
              file_size: 2048,
              mime_type: 'application/octet-stream',
            },
          },
        ],
      },
    ])

    expect(rows).toHaveLength(1)
    expect(rows[0].referenceFileRefs).toHaveLength(1)
    expect(rows[0].referenceFileRefs?.[0].filename).toBe('ref.png')
    expect(rows[0].sourceAssets).toHaveLength(1)
    expect(rows[0].sourceAssets?.[0].id).toBe('99')
    expect(rows[0].sourceAssets?.[0].versions?.[0].file_name).toBe('pack.psd')
  })

  it('returns empty asset arrays when nested assets are absent', () => {
    const rows = mapRetouchRequirementsFromApi([
      {
        id: 2,
        task_id: 1,
        description: '仅文字',
        sort_order: 1,
      },
    ])
    expect(rows[0].referenceFileRefs).toEqual([])
    expect(rows[0].sourceAssets).toEqual([])
  })

  it('returns empty list for non-array input', () => {
    expect(mapRetouchRequirementsFromApi(null)).toEqual([])
    expect(mapRetouchRequirementsFromApi(undefined)).toEqual([])
  })
})
