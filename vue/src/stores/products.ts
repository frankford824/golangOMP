import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Product } from '@/types'
import { erpApi } from '@/services/api/erpApi'

function normalizeErpProduct(raw: Record<string, unknown>): Product {
  // 兼容 product_id 为 string 或 number（后端可能返回 int64）
  const facadeProductId =
    raw.product_id != null && raw.product_id !== ''
      ? String(raw.product_id)
      : raw.id != null && raw.id !== ''
        ? String(raw.id)
        : ''

  return {
    // 始终优先使用 ERP facade product_id（字符串），与后端 /v1/erp/products 契约对齐
    id: facadeProductId,
    sku: String(raw.sku ?? raw.sku_code ?? ''),
    name: String(raw.name ?? raw.product_name ?? ''),
    category: String(raw.category ?? raw.category_name ?? ''),
    spec: String(raw.spec ?? raw.specification ?? ''),
    designHistorySummary: raw.design_history_summary != null ? String(raw.design_history_summary) : undefined,
    imageUrl: typeof raw.image_url === 'string' && raw.image_url ? raw.image_url : undefined,
    categoryCode:
      raw.category_code != null && String(raw.category_code).trim() !== ''
        ? String(raw.category_code).trim()
        : undefined,
    shortName: typeof raw.product_short_name === 'string' && raw.product_short_name ? raw.product_short_name : undefined,
  }
}

export const useProductsStore = defineStore('products', () => {
  const items = ref<Product[]>([])
  const loading = ref(false)
  const searchError = ref<string | null>(null)
  const page = ref(1)
  const pageSize = ref(20)
  const total = ref(0)
  let loadSeq = 0
  let loadAbort: AbortController | null = null

  const list = computed(() => items.value)
  const totalPages = computed(() => (pageSize.value > 0 ? Math.max(1, Math.ceil(total.value / pageSize.value)) : 1))
  const getById = (id: string) => items.value.find((p) => p.id === id)

  /** 调用 GET /v1/erp/products 查询产品，支持 keyword、category 等筛选与分页 */
  async function loadProducts(params?: { keyword?: string; category?: string; sku_code?: string; page?: number }) {
    loadAbort?.abort()
    const seq = ++loadSeq
    const abortController = new AbortController()
    loadAbort = abortController
    loading.value = true
    searchError.value = null
    try {
      const mergedParams = {
        page: params?.page ?? page.value,
        page_size: pageSize.value,
        keyword: params?.keyword,
        category: params?.category,
        sku_code: params?.sku_code,
      }
      const res = await erpApi.getProducts(mergedParams, abortController.signal)
      if (abortController.signal.aborted || seq !== loadSeq) return
      const data = res?.data
      const body = data?.data ?? data
      const rawList = Array.isArray(body) ? body : (body?.items ?? body?.products ?? [])
      items.value = rawList.map((raw: Record<string, unknown>) => normalizeErpProduct(raw))
      if (body && typeof body === 'object' && !Array.isArray(body)) {
        total.value = typeof body.total === 'number' ? body.total : items.value.length
        page.value = typeof body.page === 'number' ? body.page : mergedParams.page ?? 1
        pageSize.value = typeof body.page_size === 'number' ? body.page_size : pageSize.value
      } else {
        total.value = items.value.length
      }
    } catch (e) {
      if (abortController.signal.aborted || seq !== loadSeq) return
      searchError.value = e instanceof Error ? e.message : '加载产品列表失败'
      items.value = []
      total.value = 0
    } finally {
      if (loadAbort === abortController) {
        loadAbort = null
      }
      if (seq === loadSeq) {
        loading.value = false
      }
    }
  }

  function setProducts(products: Product[]) {
    items.value = products
    total.value = products.length
  }

  return { list, getById, setProducts, loadProducts, loading, searchError, page, pageSize, total, totalPages }
})
