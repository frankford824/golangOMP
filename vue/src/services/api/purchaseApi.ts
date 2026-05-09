import type { Task } from '@/domain/types/task'
import type { PurchaseInfo } from '@/domain/types/purchase'

export interface UpdatePurchaseInfoPayload {
  taskId: string
  purchaseInfo: Partial<PurchaseInfo>
  action_id: string
}

export interface MarkPurchasedPayload {
  taskId: string
  action_id: string
}

/** 更新采购信息 */
export async function updatePurchaseInfo(payload: UpdatePurchaseInfoPayload): Promise<Task | null> {
  void payload
  return Promise.resolve(null)
}

/** 标记已采购 */
export async function markPurchased(payload: MarkPurchasedPayload): Promise<Task | null> {
  void payload
  return Promise.resolve(null)
}

/** 查询采购任务列表 */
export async function fetchPurchaseTasks(): Promise<Task[]> {
  const { fetchTaskList } = await import('./tasksApi')
  const all = await fetchTaskList()
  return all.filter((t: Task) => t.businessType === 'PURCHASE_TASK')
}
