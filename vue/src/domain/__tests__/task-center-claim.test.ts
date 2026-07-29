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
  it('shows claim only when task assignment and design submission are both allowed', () => {
    expect(canClaimTaskFromCenter(task({ allowedActions: ['task.assign'] }), true)).toBe(true)
    expect(canClaimTaskFromCenter(task({ allowedActions: [] }), true)).toBe(false)
  })

  it('does not treat an operations assignment capability as self-claim permission', () => {
    expect(canClaimTaskFromCenter(task({ allowedActions: ['task.assign'] }), false)).toBe(false)
  })

  it('does not infer claim from business lane or task state', () => {
    expect(canClaimTaskFromCenter(task({ businessLane: 'customization' }), true)).toBe(false)
    expect(canClaimTaskFromCenter(task({ status: 'InProgress' }), true)).toBe(false)
  })

  it('hides claim when a handler is already assigned', () => {
    expect(
      canClaimTaskFromCenter(task({ allowedActions: ['task.assign'], currentHandlerId: '42' }), true),
    ).toBe(false)
  })

  it('uses one neutral claim label for every v8 lane', () => {
    expect(taskCenterClaimButtonLabel(task({ businessLane: 'customization' }), false)).toBe('接单')
    expect(taskCenterClaimButtonLabel(task(), true)).toBe('接单中...')
  })
})
