import type { Task } from './types/task'
import { isRetouchTask } from './task-actions'
import { getDesignSubStatusLabel, DesignSubStatusEnum } from './enums/task-status'

const RETOUCH_STATE_LABELS: Record<string, string> = {
  pending_claim: '待领取',
  in_progress: '精修中',
  submitted: '已提交',
  closed: '已完成',
  completed: '已完成',
}

/** 设计与资产区：P 图任务状态行（优先 retouch 模块，避免脏 legacy 的「待审核」文案）。 */
export function retouchDesignAssetStatusDisplay(task: Task): {
  text: string
  dotClass: string
} | null {
  if (!isRetouchTask(task)) return null

  const mod = task.moduleSummaries?.find((m) => m.module_key === 'retouch')
  const modState = String(mod?.state ?? '')
    .trim()
    .toLowerCase()
    .replace(/-/g, '_')

  if (modState === 'in_progress') {
    return { text: RETOUCH_STATE_LABELS.in_progress, dotClass: 'dot-blue' }
  }
  if (modState === 'submitted' || modState === 'closed' || modState === 'completed') {
    const text = RETOUCH_STATE_LABELS[modState] ?? '已完成'
    return { text, dotClass: 'dot-green' }
  }
  if (modState === 'pending_claim') {
    return { text: RETOUCH_STATE_LABELS.pending_claim, dotClass: 'dot-grey' }
  }

  if (task.status === 'Completed' || task.status === 'Archived') {
    return { text: '已完成', dotClass: 'dot-green' }
  }

  if (
    (task.status === 'PendingAuditA' || task.status === 'PendingAuditB') &&
    task.designSubStatus !== DesignSubStatusEnum.FINALIZED &&
    task.designSubStatus !== DesignSubStatusEnum.APPROVED
  ) {
    return { text: '精修处理中', dotClass: 'dot-blue' }
  }

  if ((task.status === 'InProgress' || task.status === 'PendingAssign') && task.designerId) {
    return { text: '精修中', dotClass: 'dot-blue' }
  }

  if (task.designSubStatus) {
    const text = getDesignSubStatusLabel(task.designSubStatus)
    if (task.designSubStatus === DesignSubStatusEnum.FINALIZED || task.designSubStatus === DesignSubStatusEnum.APPROVED) {
      return { text, dotClass: 'dot-green' }
    }
    if (task.designSubStatus === DesignSubStatusEnum.PENDING_AUDIT) {
      return { text, dotClass: 'dot-blue' }
    }
    if (task.designSubStatus === DesignSubStatusEnum.REJECTED) {
      return { text, dotClass: 'dot-red' }
    }
    return { text, dotClass: 'dot-grey' }
  }

  return { text: '精修待处理', dotClass: 'dot-grey' }
}
