import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { canClaimTaskFromCenter, taskCenterClaimButtonLabel } from '@/domain/task-center-claim'

function task(overrides: Partial<Task> = {}): Task {
  return {
    status: 'PendingAssign',
    taskType: 'NEW_PRODUCT_DEV',
    designerId: null,
    assigneeId: null,
    currentHandlerId: null,
    allowedActions: [],
    ...overrides,
  } as Task
}

describe('task-center-claim v8 action contract', () => {
  it('shows claim only for the exact backend action', () => {
    expect(canClaimTaskFromCenter(task({ allowedActions: ['task.assign'] }))).toBe(true)
    expect(canClaimTaskFromCenter(task({ allowedActions: [] }))).toBe(false)
  })

  it('does not infer claim from business lane or task state', () => {
    expect(canClaimTaskFromCenter(task({ businessLane: 'customization' }))).toBe(false)
    expect(canClaimTaskFromCenter(task({ status: 'InProgress' }))).toBe(false)
  })

  it('hides claim when a handler is already assigned', () => {
    expect(
      canClaimTaskFromCenter(task({ allowedActions: ['task.assign'], currentHandlerId: '42' })),
    ).toBe(false)
  })

  it('uses one neutral claim label for every v8 lane', () => {
    expect(taskCenterClaimButtonLabel(task({ businessLane: 'customization' }), false)).toBe('接单')
    expect(taskCenterClaimButtonLabel(task(), true)).toBe('接单中...')
  })
})
