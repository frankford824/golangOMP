import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { auditActionForRow, isInAuditQueue } from '@/domain/task-actions'

function makeRetouchTask(partial: Partial<Task>): Task {
  return {
    id: '1',
    taskNo: 'T1',
    sku: null,
    productId: null,
    productName: '',
    productSource: 'new',
    taskType: 'RETOUCH_TASK',
    status: 'PendingAuditA',
    referenceFileRefs: [],
    assetVersions: [],
    needOutsource: false,
    groupId: '',
    groupName: '',
    requesterId: '1',
    requesterName: '',
    creatorId: '1',
    creatorName: '',
    designerId: '2',
    designerName: '',
    currentHandlerId: null,
    currentHandlerName: null,
    assigneeId: null,
    assigneeName: null,
    dueAt: null,
    priority: 'normal',
    ...partial,
  } as Task
}

describe('retouch_task audit gates (dirty legacy status)', () => {
  it('isInAuditQueue is false for retouch even when status is PendingAuditA', () => {
    const t = makeRetouchTask({ status: 'PendingAuditA' })
    expect(isInAuditQueue(t)).toBe(false)
  })

  it('auditActionForRow is null for retouch even when status is PendingAuditA', () => {
    const t = makeRetouchTask({ status: 'PendingAuditA' })
    expect(auditActionForRow(t)).toBeNull()
  })
})
