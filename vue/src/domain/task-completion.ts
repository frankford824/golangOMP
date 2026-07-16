import type { Task } from './types/task'

export interface TaskCompletionResult {
  canComplete: boolean
  reasons: string[]
}

/** v8 has no client-side manual-close gate; completion is performed by TaskFinalizer. */
export function checkTaskCompletion(task: Task): TaskCompletionResult {
  if (task.status === 'Completed' || task.status === 'Archived') {
    return { canComplete: true, reasons: [] }
  }
  return {
    canComplete: false,
    reasons: ['任务会在设计审核、修图提交或策划 SKU 创建成功后由系统自动结单'],
  }
}
