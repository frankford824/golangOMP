/**
 * @deprecated 请改用 customizationApi + CustomizationJobsView。
 * 保留仅供历史兼容，不接入新功能。
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { OutsourceOrder } from '@/types'
import type { BackendOutsourceOrderRaw } from '@/services/apiTypes'
import { listOutsourceOrders } from '@/services/api/outsourceApi'
import { tasksApi } from '@/services/api/tasksApi'
import {
  parseOutsourceListBody,
  mapBackendOutsourceOrderToDomain,
  extractTaskSummaryFromTaskResponse,
} from '@/domain/mappers/outsource-from-api'

function isAbortError(e: unknown): boolean {
  return (
    e !== null &&
    typeof e === 'object' &&
    'code' in e &&
    (e as { code?: string }).code === 'ERR_CANCELED'
  )
}

function rowNeedsTaskFetch(raw: BackendOutsourceOrderRaw): boolean {
  const hasTaskNo = typeof raw.task_no === 'string' && raw.task_no.trim() !== ''
  const hasSku = typeof raw.sku === 'string' && raw.sku.trim() !== ''
  const hasPn = typeof raw.product_name === 'string' && raw.product_name.trim() !== ''
  return !(hasTaskNo && hasSku && hasPn)
}

export const useOutsourceStore = defineStore('outsource', () => {
  const orders = ref<OutsourceOrder[]>([])
  const loading = ref(false)
  const loadError = ref<string | null>(null)
  const listTotal = ref(0)
  const page = ref(1)
  const pageSize = ref(50)

  const list = computed(() => orders.value)
  const getById = (id: string) => orders.value.find((o) => o.id === id)

  async function fetchList(opts?: { signal?: AbortSignal }) {
    loading.value = true
    loadError.value = null
    try {
      const rawBody = await listOutsourceOrders(
        { page: page.value, page_size: pageSize.value },
        opts?.signal,
      )
      const { items: rawItems, pagination } = parseOutsourceListBody(rawBody ?? {})
      listTotal.value = pagination.total ?? rawItems.length

      const idsToFetch = new Set<number>()
      for (const raw of rawItems) {
        if (rowNeedsTaskFetch(raw)) idsToFetch.add(raw.task_id)
      }

      const summaryByTaskId = new Map<
        number,
        ReturnType<typeof extractTaskSummaryFromTaskResponse>
      >()
      await Promise.all(
        [...idsToFetch].map(async (tid) => {
          try {
            const res = await tasksApi.getById(String(tid), opts?.signal)
            const payload = (res?.data as { data?: unknown } | undefined)?.data ?? res?.data
            summaryByTaskId.set(tid, extractTaskSummaryFromTaskResponse(payload))
          } catch {
            summaryByTaskId.set(tid, { taskNo: '—', sku: '—', productName: '—' })
          }
        }),
      )

      orders.value = rawItems.map((raw) =>
        mapBackendOutsourceOrderToDomain(raw, summaryByTaskId.get(raw.task_id)),
      )
    } catch (e) {
      if (isAbortError(e)) return
      loadError.value = e instanceof Error ? e.message : '加载定制单列表失败'
      orders.value = []
      listTotal.value = 0
    } finally {
      loading.value = false
    }
  }

  return {
    list,
    getById,
    loading,
    loadError,
    listTotal,
    page,
    pageSize,
    fetchList,
  }
})
