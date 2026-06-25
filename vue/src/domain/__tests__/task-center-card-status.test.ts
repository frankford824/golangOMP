import { describe, expect, it } from 'vitest'
import type { Task } from '@/domain/types/task'
import { enrichTaskDomainFields } from '@/domain/mappers/task-mappers'
import { getMainTaskStatusLabel } from '@/domain/enums/task-status'
import {
  getTaskCenterCardStatusDisplayLabel,
  getTaskCenterCardStatusLabel,
} from '@/domain/task-center-card-status'

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

describe('task-center-card-status (customization lane)', () => {
  it('PendingCustomizationProduction does not display as 待仓库接收', () => {
    const task = makeTask({
      businessLane: 'customization',
      customizationRequired: true,
      status: 'PendingCustomizationProduction',
    })
    expect(task.mainStatus).toBe('WAREHOUSE_PENDING')
    expect(getMainTaskStatusLabel(task.mainStatus!)).toBe('待仓库接收')
    expect(getTaskCenterCardStatusLabel(task)).toBe('待美工处理')
    expect(getTaskCenterCardStatusDisplayLabel(task)).toBe('待美工处理')
    expect(getTaskCenterCardStatusDisplayLabel(task)).not.toBe('待仓库接收')
  })

  it('PendingCustomizationReview does not display as 待仓库接收', () => {
    const task = makeTask({
      businessLane: 'customization',
      status: 'PendingCustomizationReview',
    })
    expect(getTaskCenterCardStatusLabel(task)).toBe('待定制审核')
    expect(getTaskCenterCardStatusDisplayLabel(task)).toBe('待定制审核')
    expect(getTaskCenterCardStatusDisplayLabel(task)).not.toBe('待仓库接收')
  })

  it('PendingWarehouseReceive still displays 待仓库接收', () => {
    const task = makeTask({
      businessLane: 'customization',
      status: 'PendingWarehouseReceive',
    })
    expect(getTaskCenterCardStatusLabel(task)).toBe('待仓库接收')
    expect(getTaskCenterCardStatusDisplayLabel(task)).toBe('待仓库接收')
  })

  it('normal task InProgress is unchanged', () => {
    const task = makeTask({
      businessLane: 'normal',
      status: 'InProgress',
    })
    expect(getTaskCenterCardStatusLabel(task)).toBeNull()
    expect(getTaskCenterCardStatusDisplayLabel(task)).toBe('信息待完善')
    expect(getTaskCenterCardStatusDisplayLabel(task)).not.toBe('待美工处理')
  })
})
