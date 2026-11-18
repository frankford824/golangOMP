import type { LocationQuery } from 'vue-router'
import { oaEnabled } from './access'
import { send } from './api'

export function orderTaskSource(query: LocationQuery) {
  if (!oaEnabled) return undefined
  const [shop_id, sku, from, to] = [query.oa_shop, query.oa_sku, query.oa_from, query.oa_to]
  if (typeof shop_id !== 'string' || typeof sku !== 'string' || typeof from !== 'string' || typeof to !== 'string') return undefined
  return shop_id && sku && from && to ? { shop_id, sku, from, to } : undefined
}

export async function linkOrderTask(source: NonNullable<ReturnType<typeof orderTaskSource>>, taskId: string | number) {
  const task_id = Number(taskId)
  if (!Number.isSafeInteger(task_id) || task_id < 1) throw new Error('未能读取已创建任务的编号')
  await send('/oa-api/orders/task-links', { ...source, task_id })
}
