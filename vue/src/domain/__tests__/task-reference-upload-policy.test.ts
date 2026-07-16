import { describe, expect, it } from 'vitest'
import { PermissionEnum, type PermissionEnumValue, type PermissionUser } from '@/types'
import type { Task } from '@/domain/types/task'
import { canUserSupplementReferenceOnTaskDetail } from '@/domain/task-reference-upload-policy'

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    status: 'InProgress',
    designSubStatus: 'IN_PROGRESS',
    taskType: 'NEW_PRODUCT_DEV',
    businessType: 'NEW_PRODUCT_DEV',
    creatorId: 'u-creator',
    requesterId: 'u-requester',
    ...overrides,
  } as Task
}

function makeUser(id: string): PermissionUser {
  return {
    id,
    name: id,
    role: 'ops',
    departmentId: 'dep-1',
    groupId: 'grp-1',
    dataScope: 'self',
    permissions: [],
  }
}

function makeCtx(options?: {
  permissions?: PermissionEnumValue[]
  roleCodes?: string[]
}) {
  const permissionSet = new Set(options?.permissions ?? [])
  const roleSet = new Set(options?.roleCodes ?? [])
  return {
    hasPermission: (p: PermissionEnumValue) => permissionSet.has(p),
    hasAnyRole: (codes: readonly string[]) => codes.some((code) => roleSet.has(code)),
  }
}

describe('canUserSupplementReferenceOnTaskDetail', () => {
  it('returns false outside the editable design phase', () => {
    const task = makeTask({ status: 'PendingAudit', designSubStatus: 'PENDING_AUDIT' })
    const user = makeUser('u-creator')
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE] })
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(false)
  })

  it('does not infer access from a legacy admin role', () => {
    const task = makeTask()
    const user = makeUser('someone')
    const ctx = makeCtx({ roleCodes: ['super_admin'] })
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(false)
  })

  it('allows creator when task.create is granted', () => {
    const task = makeTask({ creatorId: 'u-creator' })
    const user = makeUser('u-creator')
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE] })
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(true)
  })

  it('allows requester when creator is empty and task.create is granted', () => {
    const task = makeTask({ creatorId: null, requesterId: 'u-requester' })
    const user = makeUser('u-requester')
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE] })
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(true)
  })

  it('rejects other ops users even with task.create', () => {
    const task = makeTask()
    const user = makeUser('u-other-ops')
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE] })
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(false)
  })

  it('rejects creator when task.create is missing', () => {
    const task = makeTask({ creatorId: 'u-creator' })
    const user = makeUser('u-creator')
    const ctx = makeCtx()
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(false)
  })

  it('rejects department admin style user when not creator or top admin', () => {
    const task = makeTask()
    const user = makeUser('u-dept-admin')
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE], roleCodes: ['dept_admin'] })
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(false)
  })

  it('rejects null user', () => {
    const task = makeTask()
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE] })
    expect(canUserSupplementReferenceOnTaskDetail(task, null, ctx)).toBe(false)
  })

  it('rejects when design has entered pending audit', () => {
    const task = makeTask({ designSubStatus: 'PENDING_AUDIT' })
    const user = makeUser('u-creator')
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE], roleCodes: ['super_admin'] })
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(false)
  })

  it('rejects approved/finalized sub statuses', () => {
    const user = makeUser('u-creator')
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE], roleCodes: ['hr_admin'] })
    expect(
      canUserSupplementReferenceOnTaskDetail(makeTask({ designSubStatus: 'APPROVED' }), user, ctx),
    ).toBe(false)
    expect(
      canUserSupplementReferenceOnTaskDetail(makeTask({ designSubStatus: 'FINALIZED' }), user, ctx),
    ).toBe(false)
  })

  it('rejects non InProgress status', () => {
    const task = makeTask({ status: 'PendingAssign' })
    const user = makeUser('u-creator')
    const ctx = makeCtx({ permissions: [PermissionEnum.TASK_CREATE], roleCodes: ['super_admin'] })
    expect(canUserSupplementReferenceOnTaskDetail(task, user, ctx)).toBe(false)
  })
})
