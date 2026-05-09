import type { Task, TaskAssetVersion } from '@/domain/types/task'

export interface SubmitDesignPayload {
  taskId: string
  assetVersion: Omit<TaskAssetVersion, 'id' | 'uploadedAt'>
  action_id: string
}

export interface AssignDesignerPayload {
  taskId: string
  assigneeId: string
  assigneeName: string
  action_id: string
}

/** 指派设计师 */
export async function assignDesigner(payload: AssignDesignerPayload): Promise<Task | null> {
  void payload
  return Promise.resolve(null)
}

/** 提交设计版本 */
export async function submitDesignVersion(payload: SubmitDesignPayload): Promise<Task | null> {
  void payload
  return Promise.resolve(null)
}

/** 提交审核（设计师提交） */
export async function submitToAudit(taskId: string, action_id: string): Promise<Task | null> {
  void taskId
  void action_id
  return Promise.resolve(null)
}
