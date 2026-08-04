// @vitest-environment jsdom

import { afterEach, describe, expect, it } from 'vitest'

import { resolveCssRgbToken } from './color-tokens'

describe('resolveCssRgbToken', () => {
  afterEach(() => {
    document.documentElement.style.removeProperty('--test-brand')
  })

  it('resolves space-delimited CSS channels into a concrete Naive UI color', () => {
    document.documentElement.style.setProperty('--test-brand', '37 99 235')

    expect(resolveCssRgbToken('--test-brand', '0 0 0')).toBe('rgb(37, 99, 235)')
  })

  it('falls back to a concrete color when the token is unavailable', () => {
    expect(resolveCssRgbToken('--missing-brand', '29 78 216')).toBe('rgb(29, 78, 216)')
  })
})
