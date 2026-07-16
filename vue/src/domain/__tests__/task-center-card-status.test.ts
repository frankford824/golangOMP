import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { enrichTaskDomainFields } from '@/domain/mappers/task-mappers'
import { getTaskCenterCardStatusDisplayLabel } from '@/domain/task-center-card-status'

function task(status: Task['status'], businessLane: Task['businessLane'] = 'normal'): Task {
  return enrichTaskDomainFields({
    status,
    businessLane,
    taskType: 'NEW_PRODUCT_DEV',
    productSource: 'new',
  } as Task)
}

describe('task-center-card-status v8 states', () => {
  it('uses the unified audit label for normal and customization tasks', () => {
    expect(getTaskCenterCardStatusDisplayLabel(task('PendingAudit'))).toBe('待审核')
    expect(getTaskCenterCardStatusDisplayLabel(task('PendingAudit', 'customization'))).toBe('待审核')
  })

  it('uses the completed label without deriving removed workflow nodes', () => {
    expect(getTaskCenterCardStatusDisplayLabel(task('Completed'))).toBe('已结单')
  })
})
