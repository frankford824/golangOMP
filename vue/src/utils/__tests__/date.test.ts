import { describe, expect, it } from 'vitest'
import {
  formatDateBeijing,
  formatTaskRecordDateBeijing,
} from '@/utils/date'

describe('task record timestamp formatting', () => {
  it('treats misleading Z-suffixed MySQL DATETIME digits as Beijing wall clock', () => {
    expect(formatTaskRecordDateBeijing('2026-08-04T10:20:25Z')).toBe('2026/08/04 10:20')
  })

  it('accepts MySQL-style local record timestamps without a suffix', () => {
    expect(formatTaskRecordDateBeijing('2026-08-04 10:20:25')).toBe('2026/08/04 10:20')
  })

  it('keeps UTC business timestamps on the existing conversion path', () => {
    expect(formatDateBeijing('2026-08-05T02:00:00Z')).toBe('2026/08/05 10:00')
  })
})
