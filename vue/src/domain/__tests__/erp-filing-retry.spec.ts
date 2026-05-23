import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { canUserRetryErpFiling, taskNeedsErpFilingRetry } from '@/domain/erp-filing-retry'

function task(partial: Partial<Task>): Task {
  return {
    id: '1',
    taskNo: 'T-1',
    sku: null,
    productId: null,
    productName: '',
    productSource: 'manual',
    taskType: 'NEW_PRODUCT_DEV',
    createdAt: '',
    updatedAt: '',
    ...partial,
  } as Task
}

describe('taskNeedsErpFilingRetry', () => {
  it('returns true when task filing_status is filing_failed', () => {
    expect(taskNeedsErpFilingRetry(task({ filing_status: 'filing_failed' }))).toBe(true)
  })

  it('returns true when erp_sync_required is true', () => {
    expect(taskNeedsErpFilingRetry(task({ erp_sync_required: true }))).toBe(true)
  })

  it('checks batch sku_items erp_sync_status', () => {
    expect(
      taskNeedsErpFilingRetry(
        task({
          filing_status: 'filed',
          skuItems: [{ id: 1, erp_sync_status: 'filing_failed' }],
        }),
      ),
    ).toBe(true)
  })

  it('returns false when no retry signals', () => {
    expect(taskNeedsErpFilingRetry(task({ filing_status: 'filed' }))).toBe(false)
  })
})

describe('canUserRetryErpFiling', () => {
  it('allows ops and admin roles', () => {
    expect(canUserRetryErpFiling((roles) => roles.includes('ops'))).toBe(true)
    expect(canUserRetryErpFiling((roles) => roles.includes('admin'))).toBe(true)
    expect(canUserRetryErpFiling((roles) => roles.includes('designer'))).toBe(false)
  })
})
