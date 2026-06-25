import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { enrichTaskDomainFields } from '@/domain/mappers/task-mappers'
import {
  canClaimCustomizationOnTaskDetail,
  canClaimCustomizationTask,
  canClaimRegularDesignTask,
  canClaimTaskFromCenter,
  userCanActAsCustomizationClaimActor,
  userIsPureDesignerForCustomizationClaim,
} from '@/domain/task-center-claim'

function makeTask(partial: Partial<Task>): Task {
  return enrichTaskDomainFields({
    id: '778',
    taskNo: 'RW-TEST',
    sku: 'DZK000003',
    productId: null,
    productName: '测试商品',
    productSource: 'new',
    taskType: 'NEW_PRODUCT_DEV',
    status: 'PendingAssign',
    referenceFileRefs: [],
    assetVersions: [],
    needOutsource: false,
    groupId: '',
    groupName: '未分配池',
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

const customizationActorGate = {
  canActAsCustomizationClaimActor: true,
  canClaimFromDesignerPool: false,
  activeTabIsPool: false,
}

const designerPoolGate = {
  canActAsCustomizationClaimActor: false,
  canClaimFromDesignerPool: true,
  activeTabIsPool: true,
}

describe('task-center-claim', () => {
  it('allows customization PendingCustomizationProduction when actor is CustomizationOperator and unassigned', () => {
    const task = makeTask({
      businessLane: 'customization',
      customizationRequired: true,
      status: 'PendingCustomizationProduction',
    })
    expect(canClaimCustomizationTask(task, customizationActorGate)).toBe(true)
    expect(canClaimTaskFromCenter(task, customizationActorGate)).toBe(true)
  })

  it('denies customization claim for pure Designer', () => {
    const hasRole = (roles: readonly string[]) => roles.includes('Designer')
    expect(userIsPureDesignerForCustomizationClaim(hasRole)).toBe(true)
    expect(userCanActAsCustomizationClaimActor(hasRole, false)).toBe(false)

    const task = makeTask({
      businessLane: 'customization',
      status: 'PendingCustomizationProduction',
    })
    expect(
      canClaimCustomizationTask(task, {
        canActAsCustomizationClaimActor: userCanActAsCustomizationClaimActor(hasRole, false),
      }),
    ).toBe(false)
  })

  it('hides customization claim when designer or handler already set', () => {
    const task = makeTask({
      businessLane: 'customization',
      status: 'PendingCustomizationProduction',
      designerId: '99',
    })
    expect(canClaimCustomizationTask(task, customizationActorGate)).toBe(false)
  })

  it('keeps regular PendingAssign pool claim unchanged', () => {
    const task = makeTask({ status: 'PendingAssign', businessLane: 'normal' })
    expect(canClaimRegularDesignTask(task, designerPoolGate)).toBe(true)
    expect(canClaimCustomizationTask(task, customizationActorGate)).toBe(false)
    expect(canClaimTaskFromCenter(task, designerPoolGate)).toBe(true)
  })
})

describe('canClaimCustomizationOnTaskDetail', () => {
  const hasCustomizationActorRole = (roles: readonly string[]) =>
    roles.includes('CustomizationOperator')

  it('allows claim for CustomizationOperator on unassigned customization task', () => {
    const task = makeTask({
      businessLane: 'customization',
      customizationRequired: true,
      status: 'PendingCustomizationProduction',
    })
    expect(
      canClaimCustomizationOnTaskDetail(task, hasCustomizationActorRole, true),
    ).toBe(true)
  })

  it('denies claim for pure Designer', () => {
    const hasDesignerOnly = (roles: readonly string[]) => roles.includes('Designer')
    const task = makeTask({
      businessLane: 'customization',
      status: 'PendingCustomizationProduction',
    })
    expect(canClaimCustomizationOnTaskDetail(task, hasDesignerOnly, false)).toBe(false)
  })

  it('hides claim when designer or handler already set', () => {
    const task = makeTask({
      businessLane: 'customization',
      status: 'PendingCustomizationProduction',
      currentHandlerId: '42',
    })
    expect(
      canClaimCustomizationOnTaskDetail(task, hasCustomizationActorRole, true),
    ).toBe(false)
  })

  it('does not affect regular tasks', () => {
    const task = makeTask({ status: 'PendingAssign', businessLane: 'normal' })
    expect(
      canClaimCustomizationOnTaskDetail(task, hasCustomizationActorRole, true),
    ).toBe(false)
  })
})
