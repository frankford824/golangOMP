import { describe, expect, it } from 'vitest'
import { expandTaskListStatusFilter } from '@/domain/task-list-status-filter'

describe('expandTaskListStatusFilter', () => {
  it('keeps the unified activity status unchanged', () => {
    expect(expandTaskListStatusFilter(['InProgress'])).toEqual(['InProgress'])
  })

  it('deduplicates repeated v8 statuses without adding aliases', () => {
    expect(expandTaskListStatusFilter(['PendingAudit', 'PendingAudit'])).toEqual(['PendingAudit'])
  })
})
