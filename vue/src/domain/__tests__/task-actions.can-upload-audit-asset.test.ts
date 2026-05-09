import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { canUploadAuditAsset } from '@/domain/task-actions'

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    status: 'InProgress',
    designSubStatus: 'IN_PROGRESS',
    taskType: 'NEW_PRODUCT_DEV',
    businessType: 'NEW_PRODUCT_DEV',
    ...overrides,
  } as Task
}

describe('canUploadAuditAsset', () => {
  it('blocks PendingAuditA so UI does not expose a guaranteed 403 entry', () => {
    expect(canUploadAuditAsset(makeTask({ status: 'PendingAuditA' }))).toBe(false)
  })

  it('blocks PendingAuditB for the same reason as PendingAuditA', () => {
    expect(canUploadAuditAsset(makeTask({ status: 'PendingAuditB' }))).toBe(false)
  })

  it('allows RejectedByAuditA so reviewer-initiated rework upload stays possible', () => {
    expect(canUploadAuditAsset(makeTask({ status: 'RejectedByAuditA' }))).toBe(true)
  })

  it('allows InProgress (designer editable phase)', () => {
    expect(canUploadAuditAsset(makeTask({ status: 'InProgress' }))).toBe(true)
  })

  it('blocks purchase tasks regardless of status', () => {
    const task = makeTask({
      status: 'InProgress',
      taskType: 'PURCHASE_TASK',
      businessType: 'PURCHASE_TASK',
    })
    expect(canUploadAuditAsset(task)).toBe(false)
  })

  it('blocks terminal states like Completed / Cancelled', () => {
    expect(canUploadAuditAsset(makeTask({ status: 'Completed' }))).toBe(false)
    expect(canUploadAuditAsset(makeTask({ status: 'Cancelled' }))).toBe(false)
  })
})
