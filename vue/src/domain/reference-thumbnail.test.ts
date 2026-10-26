import { describe, expect, it } from 'vitest'
import { referenceThumbnailURL } from './reference-thumbnail'

describe('server reference thumbnail selection', () => {
  it('uses the explicit thumbnail for task and SKU UUID references', () => {
    expect(referenceThumbnailURL({ asset_id: 'uuid', thumbnail_url: ' https://objects.example/thumb.webp ' })).toBe('https://objects.example/thumb.webp')
  })
  it('does not substitute an original download, label, or legacy URL', () => {
    expect(referenceThumbnailURL({ download_url: 'https://objects.example/original.jpg', preview_url: '/legacy.jpg' })).toBeUndefined()
    for (const input of [null, undefined, {}, { thumbnail_url: '' }, { thumbnail_url: 123 }]) expect(referenceThumbnailURL(input)).toBeUndefined()
  })
  it('preserves the controlled path for bound revision references', () => {
    expect(referenceThumbnailURL({ formal_task_asset_id: 201, thumbnail_url: 'https://objects.example/stale.webp' })).toBeUndefined()
  })
})
