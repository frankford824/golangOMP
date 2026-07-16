import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { auditActionForRow, isInAuditQueue } from '@/domain/task-actions'

describe('retouch task v8 audit gates', () => {
  const task = {
    taskType: 'RETOUCH_TASK',
    status: 'Completed',
    allowedActions: [],
  } as unknown as Task

  it('does not enter the audit queue after direct completion', () => {
    expect(isInAuditQueue(task)).toBe(false)
  })

  it('does not expose an audit action after direct completion', () => {
    expect(auditActionForRow(task)).toBeNull()
  })
})
