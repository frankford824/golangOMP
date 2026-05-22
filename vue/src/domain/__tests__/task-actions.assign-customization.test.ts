import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { enrichTaskDomainFields } from '@/domain/mappers/task-mappers'
import {
  canAssign,
  canAssignCustomizationArtOperator,
  canReassignDesigner,
  isInCustomizationArtReassignmentPhase,
} from '@/domain/task-actions'
import { getTaskActionAvailability } from '@/domain/task-action-availability'

function makeTask(partial: Partial<Task>): Task {
  return enrichTaskDomainFields({
    id: '900',
    taskNo: 'RW-900',
    sku: 'DZK000001',
    productId: null,
    productName: '测试',
    productSource: 'new',
    taskType: 'NEW_PRODUCT_DEV',
    status: 'PendingAssign',
    referenceFileRefs: [],
    assetVersions: [],
    needOutsource: false,
    groupId: '',
    groupName: '组',
    requesterId: '1',
    requesterName: '运营',
    creatorId: '1',
    creatorName: '运营',
    designerId: null,
    designerName: null,
    currentHandlerId: null,
    currentHandlerName: null,
    assigneeId: null,
    assigneeName: null,
    dueAt: null,
    priority: 'normal',
    ...partial,
  } as Task)
}

describe('canAssignCustomizationArtOperator', () => {
  it('allows assign for unassigned customization production task', () => {
    const task = makeTask({
      businessLane: 'customization',
      customizationRequired: true,
      status: 'PendingCustomizationProduction',
    })
    expect(canAssignCustomizationArtOperator(task)).toBe(true)
    expect(canAssign(task)).toBe(true)
    expect(getTaskActionAvailability(task).canShowAssign).toBe(true)
  })

  it('hides assign when handler already set', () => {
    const task = makeTask({
      businessLane: 'customization',
      status: 'PendingCustomizationProduction',
      currentHandlerId: '42',
    })
    expect(canAssignCustomizationArtOperator(task)).toBe(false)
    expect(getTaskActionAvailability(task).canShowAssign).toBe(false)
  })

  it('keeps regular PendingAssign assign unchanged', () => {
    const task = makeTask({ status: 'PendingAssign', businessLane: 'normal' })
    expect(canAssignCustomizationArtOperator(task)).toBe(false)
    expect(canAssign(task)).toBe(true)
    expect(getTaskActionAvailability(task).canShowAssign).toBe(true)
  })

  it('does not treat regular task as customization operator assign', () => {
    const task = makeTask({ status: 'PendingAssign' })
    expect(canAssign(task)).toBe(true)
    const customizationOnly = makeTask({
      status: 'PendingCustomizationProduction',
      customizationRequired: true,
    })
    expect(canAssign(customizationOnly)).toBe(true)
    expect(canAssign(task)).toBe(true)
  })
})

describe('customization art reassignment phase', () => {
  it('allows reassign when PendingCustomizationProduction and handler assigned', () => {
    const task = makeTask({
      businessLane: 'customization',
      customizationRequired: true,
      status: 'PendingCustomizationProduction',
      designerId: '228',
      assigneeId: '228',
      currentHandlerId: '228',
    })
    expect(isInCustomizationArtReassignmentPhase(task)).toBe(true)
    expect(canReassignDesigner(task)).toBe(true)
    expect(getTaskActionAvailability(task).canShowReassign).toBe(true)
  })

  it('hides reassign when customization production is unassigned', () => {
    const task = makeTask({
      businessLane: 'customization',
      status: 'PendingCustomizationProduction',
    })
    expect(isInCustomizationArtReassignmentPhase(task)).toBe(false)
    expect(canReassignDesigner(task)).toBe(false)
    expect(getTaskActionAvailability(task).canShowReassign).toBe(false)
  })

  it('keeps regular InProgress reassign unchanged', () => {
    const task = makeTask({
      status: 'InProgress',
      designerId: '10',
      assigneeId: '10',
    })
    expect(isInCustomizationArtReassignmentPhase(task)).toBe(false)
    expect(canReassignDesigner(task)).toBe(true)
    expect(getTaskActionAvailability(task).canShowReassign).toBe(true)
  })
})
