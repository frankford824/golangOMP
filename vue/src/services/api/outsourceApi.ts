import http from '@/services/http'
import type { OutsourceOrderListQuery } from '@/services/apiTypes'

/**
 * @deprecated 新逻辑请改用 customizationApi.listCustomizationJobs (GET /v1/customization-jobs)。
 * 兼容路由：GET /v1/outsource-orders 仍由后端提供但标记为 deprecated。
 */
export async function listOutsourceOrders(params?: OutsourceOrderListQuery, signal?: AbortSignal) {
  const res = await http.get<unknown>('/v1/outsource-orders', { params, signal })
  return res.data
}
