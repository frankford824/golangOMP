import { describe, expect, it } from 'vitest'

import { computeGalleryVirtualScroll } from './materialGalleryVirtualScroll'

describe('computeGalleryVirtualScroll', () => {
  it('keeps visible items when scrollTop exceeds gallery content height (H2)', () => {
    const range = computeGalleryVirtualScroll({
      itemCount: 10,
      scrollTop: 5000,
      viewportHeight: 800,
      containerWidth: 960,
    })

    expect(range.endIndex).toBeGreaterThan(range.startIndex)
    expect(range.startIndex).toBeLessThan(10)
    expect(range.endIndex).toBeLessThanOrEqual(10)
  })

  it('resets to first row when item count shrinks below prior start index (H11)', () => {
    const range = computeGalleryVirtualScroll({
      itemCount: 3,
      scrollTop: 1800,
      viewportHeight: 800,
      containerWidth: 960,
    })

    expect(range.startIndex).toBe(0)
    expect(range.endIndex).toBeGreaterThan(0)
  })
})
