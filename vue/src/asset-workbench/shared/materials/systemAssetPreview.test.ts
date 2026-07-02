import { describe, expect, it } from 'vitest'

import { isPdfMimeOrFilename, resolvedSystemAssetThumbnailUrl } from './systemAssetPreview'

describe('systemAssetPreview helpers', () => {
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
})
