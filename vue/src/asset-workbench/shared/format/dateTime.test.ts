import { describe, expect, it } from 'vitest'

import { formatShanghaiDateTime } from './dateTime'

describe('formatShanghaiDateTime', () => {
  it('formats UTC timestamps in Shanghai time', () => {
    expect(formatShanghaiDateTime('2026-07-03T01:58:30Z')).toBe('2026-07-03 09:58')
  })

  it('treats timezone-less persisted timestamps as UTC instead of browser local time', () => {
    expect(formatShanghaiDateTime('2026-07-03 01:58:30')).toBe('2026-07-03 09:58')
  })
})
