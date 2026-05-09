import http from '@/services/http'
import type { Task } from '@/domain/types/task'
import {
  extractWarehouseReceiptList,
  type ReceiptRow,
  warehouseReceiptRawToReceiptRow,
} from '@/domain/mappers/warehouse-receipts-from-api'

export interface ReceiveTaskPayload {
  taskId: string
  warehouseLocationCode?: string
  action_id: string
}

export interface ReturnTaskPayload {
  taskId: string
  reason: string
  action_id: string
}

export interface BatchReceivePayload {
  taskIds: string[]
  action_id: string
}

export interface WarehouseReceiptListParams {
  page?: number
  page_size?: number
  /** 后端：normal | customization */
  workflow_lane?: 'normal' | 'customization' | string
  task_id?: number | string
  status?: string
  receiver_id?: number | string
}

/** 仓库接收（占位，真实流转见 tasksStore.receiveInWarehouse） */
export async function receiveTask(payload: ReceiveTaskPayload): Promise<Task | null> {
  void payload
  return Promise.resolve(null)
}

/** 仓库退回（占位） */
export async function returnTask(payload: ReturnTaskPayload): Promise<Task | null> {
  void payload
  return Promise.resolve(null)
}

/** 批量接收（占位） */
export async function batchReceiveTasks(payload: BatchReceivePayload): Promise<Task[]> {
  void payload
  return Promise.resolve([])
}

/**
 * 仓库回执列表（主入口）
 * GET /v1/warehouse/receipts
 */
export async function listWarehouseReceipts(
  params?: WarehouseReceiptListParams,
  signal?: AbortSignal,
): Promise<ReceiptRow[]> {
  const q: Record<string, string | number | undefined> = {}
  if (params?.page != null) q.page = params.page
  if (params?.page_size != null) q.page_size = params.page_size
  if (params?.task_id != null && String(params.task_id).trim() !== '') q.task_id = params.task_id
  if (params?.status != null && String(params.status).trim() !== '') q.status = params.status
  if (params?.receiver_id != null && String(params.receiver_id).trim() !== '') q.receiver_id = params.receiver_id
  const lane = params?.workflow_lane != null ? String(params.workflow_lane).trim() : ''
  if (lane === 'normal' || lane === 'customization') q.workflow_lane = lane

  const res = await http.get<unknown>('/v1/warehouse/receipts', { params: q, signal })
  const rawList = extractWarehouseReceiptList(res.data)
  return rawList.map((row) => warehouseReceiptRawToReceiptRow(row))
}

/** @deprecated 请优先使用 listWarehouseReceipts */
export async function fetchWarehousePendingTasks(): Promise<Task[]> {
  const { fetchTaskList } = await import('./tasksApi')
  const all = await fetchTaskList()
  return all.filter(
    (t: Task) =>
      t.warehouseSubStatus === 'PENDING_RECEIVE' ||
      t.warehouseReceiveStatus === 'pending',
  )
}
