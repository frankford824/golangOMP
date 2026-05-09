import { describe, expect, it } from 'vitest'
import { shouldWarnForMissingBitmapDelivery } from '../design-submit-validation'

describe('shouldWarnForMissingBitmapDelivery', () => {
  it('returns true when staged files have no bitmap delivery', () => {
    expect(shouldWarnForMissingBitmapDelivery(['source.psd', 'vector.ai'])).toBe(true)
  })

  it('returns false when at least one bitmap delivery exists', () => {
    expect(shouldWarnForMissingBitmapDelivery(['source.psd', 'preview.jpeg'])).toBe(false)
  })
})
