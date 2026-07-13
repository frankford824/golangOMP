import { describe, expect, it } from 'vitest'

import { canAttemptSystemAssetPreview, isPdfMimeOrFilename, materialAssetKey, resolvedSystemAssetThumbnailUrl } from './systemAssetPreview'

describe('systemAssetPreview helpers', () => {
  it('requests derived previews for design files before metadata is ready', () => {
    expect(canAttemptSystemAssetPreview({ id: 7, file_name: 'poster.psd', mime_type: 'image/vnd.adobe.photoshop' })).toBe(true)
    expect(canAttemptSystemAssetPreview({ id: 8, file_name: 'poster.ai', mime_type: 'application/octet-stream' })).toBe(true)
  })

  it('requests a published client material preview even when its old snapshot is empty', () => {
    expect(canAttemptSystemAssetPreview({
      id: 14354,
      material_id: 982,
      file_name: '',
      mime_type: '',
      preview_available: false,
    })).toBe(true)
  })

  it('detects pdf by filename when mime is missing', () => {
    expect(isPdfMimeOrFilename('', 'catalog.pdf')).toBe(true)
    expect(isPdfMimeOrFilename('', 'catalog.PDF')).toBe(true)
  })

  it('does not use download_url for gallery thumbnails', () => {
    expect(
      resolvedSystemAssetThumbnailUrl({
        id: 1,
        download_url: 'https://example.com/full.jpg',
      } as never),
    ).toBe('')
  })

  it('uses resource_id as stable key for external assets', () => {
    expect(
      materialAssetKey({
        id: 42,
        source_type: 'external',
        resource_id: 'ext-42',
      } as never),
    ).toBe('ext-42')
  })
})
