import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import {
  canAssign,
  canAssignCustomizationArtOperator,
  canReassignDesigner,
  isInCustomizationArtReassignmentPhase,
} from '@/domain/task-actions'
import { getTaskActionAvailability } from '@/domain/task-action-availability'

function task(overrides: Partial<Task> = {}): Task {
  return {
    status: 'InProgress',
    taskType: 'NEW_PRODUCT_DEV',
    businessLane: 'customization',
    allowedActions: [],
    ...overrides,
  } as Task
}

describe('customization task actions use the v8 backend contract', () => {
  it('allows assignment only when task.assign is present', () => {
    const allowed = task({ allowedActions: ['task.assign'] })
    expect(canAssignCustomizationArtOperator(allowed)).toBe(true)
    expect(canAssign(allowed)).toBe(true)
    expect(getTaskActionAvailability(allowed).canShowAssign).toBe(true)
    expect(canAssignCustomizationArtOperator(task())).toBe(false)
  })

  it('allows reassignment only when task.reassign is present', () => {
    const allowed = task({ allowedActions: ['task.reassign'] })
    expect(isInCustomizationArtReassignmentPhase(allowed)).toBe(true)
    expect(canReassignDesigner(allowed)).toBe(true)
    expect(getTaskActionAvailability(allowed).canShowReassign).toBe(true)
    expect(canReassignDesigner(task())).toBe(false)
  })

  it('does not infer customization actions from lane or handler fields', () => {
    expect(
      canAssignCustomizationArtOperator(task({ currentHandlerId: null, customizationRequired: true })),
    ).toBe(false)
    expect(canReassignDesigner(task({ currentHandlerId: '42' }))).toBe(false)
  })
})
