import { describe, expect, it } from 'vitest'
import { expandTaskListStatusFilter } from '@/domain/task-list-status-filter'

describe('expandTaskListStatusFilter', () => {
  it('includes assigned customization work in the business in-progress filter', () => {
    expect(expandTaskListStatusFilter(['InProgress'])).toEqual([
      'InProgress',
      'Assigned',
      'PendingCustomizationProduction',
      'PendingEffectRevision',
    ])
  })

  it('deduplicates statuses when multiple filter aliases overlap', () => {
    expect(expandTaskListStatusFilter(['InProgress', 'PendingCustomizationProduction'])).toEqual([
      'InProgress',
      'Assigned',
      'PendingCustomizationProduction',
      'PendingEffectRevision',
    ])
  })
})
