import { describe, expect, it } from 'vitest'

import { currentBusinessMonth } from './businessMonth'

describe('currentBusinessMonth', () => {
  it('returns YYYY-MM with a zero-padded month', () => {
    expect(currentBusinessMonth(new Date('2026-07-03T00:00:00Z'))).toBe('2026-07')
  })

  it('keeps two-digit months unchanged', () => {
    expect(currentBusinessMonth(new Date('2026-11-03T00:00:00Z'))).toBe('2026-11')
  })
})
