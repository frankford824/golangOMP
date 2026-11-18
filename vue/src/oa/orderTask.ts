import http from '@/services/http'

export function getOrderTaskContext() {
  const [path, search] = window.location.hash.slice(1).split('?')
  if (path !== '/tasks/create') return undefined
  const params = new URLSearchParams(search)
  const shop_id = params.get('oa_shop') || ''
  const sku = params.get('oa_sku') || ''
  const from = params.get('oa_from') || ''
  const to = params.get('oa_to') || ''
  return shop_id && sku && from && to ? { shop_id, sku, from, to } : undefined
}

export async function createOAOrderTask(payload: Record<string, unknown>, signal?: AbortSignal, idempotencyKey?: string) {
  const context = getOrderTaskContext()
  if (!context) {
    return http.post('/v1/tasks', payload, {
      signal,
      headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined,
    })
  }
  if (!idempotencyKey) throw new Error('缺少任务请求编号，请重新打开创建表单')
  const result = await http.post<{ data: { task_id: number } }>('/oa-api/orders/tasks', {
    ...context,
    request_id: idempotencyKey,
    task: payload,
  }, { signal })
  const id = result.data.data.task_id
  // A successful create stays successful if the follow-up read temporarily fails.
  return http.get(`/v1/tasks/${id}`, { signal }).catch(() => ({
    ...result,
    data: { data: { id, task_type: payload.task_type } },
  }))
}
