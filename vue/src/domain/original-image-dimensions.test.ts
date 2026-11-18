import { describe, expect, it } from 'vitest'
import { originalImageDimensions } from './original-image-dimensions'

describe('originalImageDimensions', () => {
  it('uses the JPEG frame and skips the embedded EXIF thumbnail', () => {
    const bytes = Uint8Array.from([
      0xff, 0xd8,
      0xff, 0xe1, 0, 12, 0xff, 0xc0, 0, 8, 8, 0, 24, 0, 36, 1,
      0xff, 0xc2, 0, 11, 8, 0x09, 0x60, 0x0c, 0x80, 1, 1, 0x11, 0,
    ])
    expect(originalImageDimensions(bytes.slice(0, 22))).toBeUndefined()
    expect(originalImageDimensions(bytes)).toEqual({ width: 3200, height: 2400 })
  })

  it('returns the extended WebP canvas dimensions', () => {
    const bytes = Uint8Array.from([
      ...Buffer.from('RIFF'), 22, 0, 0, 0,
      ...Buffer.from('WEBPVP8X'), 10, 0, 0, 0,
      0, 0, 0, 0, 0xff, 0x01, 0, 0xff, 0x02, 0,
    ])
    expect(originalImageDimensions(bytes)).toEqual({ width: 512, height: 768 })
  })

  it('never derives dimensions from a vector view box or PDF page size', () => {
    expect(originalImageDimensions(Buffer.from('<svg viewBox="0 0 400 300"></svg>'))).toBeUndefined()
    expect(originalImageDimensions(Buffer.from('%PDF-1.4 /MediaBox [0 0 400 300]'))).toBeUndefined()
  })
})
