import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { canUploadAuditAsset } from '@/domain/task-actions'

function task(allowedActions: string[], status: Task['status'] = 'PendingAudit'): Task {
  return { status, taskType: 'NEW_PRODUCT_DEV', allowedActions } as Task
}

describe('canUploadAuditAsset v8 action contract', () => {
  it('allows replacement upload only when approve is explicitly allowed', () => {
    expect(canUploadAuditAsset(task(['task.audit.approve']))).toBe(true)
  })

  it('does not infer upload from audit state or unrelated actions', () => {
    expect(canUploadAuditAsset(task([]))).toBe(false)
    expect(canUploadAuditAsset(task(['task.audit.return_to_design']))).toBe(false)
  })

  it('keeps terminal tasks closed when no action is returned', () => {
    expect(canUploadAuditAsset(task([], 'Completed'))).toBe(false)
  })
})
