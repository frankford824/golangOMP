import { describe, expect, it } from 'vitest'

import {
  selectedSettlementReportMonths,
  settlementReportRangeHint,
} from './settlementReportRange'

describe('asset workbench settlement report range', () => {
  it('anchors rolling ranges to the selected month rather than the system month', () => {
    expect(selectedSettlementReportMonths('last3', '2025-02', [])).toEqual([
      '2025-02',
      '2025-01',
      '2024-12',
    ])
  })

  it('deduplicates and sorts all known months while retaining the selected month', () => {
    expect(selectedSettlementReportMonths('available', '2025-02', ['2025-01', '2025-03', '2025-01'], '2025-03')).toEqual([
      '2025-03',
      '2025-02',
      '2025-01',
    ])
  })

  it('states that a single export does not merge the current system month', () => {
    expect(settlementReportRangeHint('single', '2025-02', ['2025-02'])).toBe(
      '仅导出 2025-02，不会合并系统当前月份',
    )
  })
})
