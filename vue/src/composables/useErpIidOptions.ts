import { computed, ref } from 'vue'
import { erpApi } from '@/services/api/erpApi'
import { ERP_IID_PRESETS, type ErpIidOption } from '@/domain/erp-iid-presets'

type SourceMode = 'api' | 'fallback' | 'mixed'

const optionCache = new Map<string, ErpIidOption[]>()
const PAGE_SIZE = 200

function unwrapErpIidsPayload(payloadRaw: unknown): {
  rows: unknown[]
  total?: number
  page?: number
  page_size?: number
} {
  const payload = (payloadRaw ?? {}) as { data?: unknown; pagination?: unknown }
  const nested = (payload.data ?? {}) as { data?: unknown; pagination?: unknown }
  const rows = Array.isArray(payload.data)
    ? payload.data
    : Array.isArray(nested.data)
      ? nested.data
      : []
  const paginationRaw = (payload.pagination ?? nested.pagination ?? {}) as Record<string, unknown>
  const totalRaw = Number(paginationRaw.total)
  const pageRaw = Number(paginationRaw.page)
  const pageSizeRaw = Number(paginationRaw.page_size)
  return {
    rows,
    total: Number.isFinite(totalRaw) && totalRaw >= 0 ? totalRaw : undefined,
    page: Number.isFinite(pageRaw) && pageRaw > 0 ? pageRaw : undefined,
    page_size: Number.isFinite(pageSizeRaw) && pageSizeRaw > 0 ? pageSizeRaw : undefined,
  }
}

function normalizeErpIidItem(raw: Record<string, unknown>): ErpIidOption | null {
  const i_id = String(raw.i_id ?? raw.label ?? '').trim()
  if (!i_id) return null
  const label = String(raw.label ?? i_id).trim() || i_id
  const category = String(raw.category ?? i_id).trim() || i_id
  const category_name = String(raw.category_name ?? category).trim() || category
  const product_count_raw = Number(raw.product_count)
  const product_count =
    Number.isFinite(product_count_raw) && product_count_raw >= 0
      ? Math.floor(product_count_raw)
      : undefined
  return { i_id, label, category, category_name, product_count }
}

function filterPresets(keyword: string): ErpIidOption[] {
  const q = keyword.trim().toLowerCase()
  if (!q) return ERP_IID_PRESETS.slice()
  return ERP_IID_PRESETS.filter((item) => {
    return (
      item.i_id.toLowerCase().includes(q) ||
      item.label.toLowerCase().includes(q) ||
      item.category.toLowerCase().includes(q) ||
      item.category_name.toLowerCase().includes(q)
    )
  })
}

export function useErpIidOptions() {
  const loading = ref(false)
  const items = ref<ErpIidOption[]>(ERP_IID_PRESETS.slice())
  const lastSourceMode = ref<SourceMode>('fallback')

  async function loadIids(keyword = ''): Promise<void> {
    const cacheKey = keyword.trim().toLowerCase()
    if (!cacheKey) {
      const presets = ERP_IID_PRESETS.slice()
      items.value = presets
      lastSourceMode.value = 'fallback'
      optionCache.set(cacheKey, presets)
      return
    }
    if (optionCache.has(cacheKey)) {
      items.value = optionCache.get(cacheKey) ?? []
      return
    }
    loading.value = true
    try {
      const q = keyword.trim()
      const first = await erpApi.getIids({
        q,
        page: 1,
        page_size: PAGE_SIZE,
      })
      const firstPayload = unwrapErpIidsPayload(first.data)
      const normalized = firstPayload.rows
        .map((row) => normalizeErpIidItem((row ?? {}) as Record<string, unknown>))
        .filter((row): row is ErpIidOption => row != null)
      if (normalized.length > 0) {
        lastSourceMode.value = 'api'
        items.value = normalized
        optionCache.set(cacheKey, normalized)
        return
      }
      const fallback = filterPresets(keyword)
      lastSourceMode.value = 'mixed'
      items.value = fallback
      optionCache.set(cacheKey, fallback)
    } catch (error) {
      // eslint-disable-next-line no-console
      console.warn('[useErpIidOptions] GET /v1/erp/iids failed, fallback to local presets', error)
      const fallback = filterPresets(keyword)
      lastSourceMode.value = 'fallback'
      items.value = fallback
      optionCache.set(cacheKey, fallback)
    } finally {
      loading.value = false
    }
  }

  const selectOptions = computed(() =>
    items.value.map((item) => ({
      value: item.i_id,
      label:
        item.category_name && item.category_name !== item.i_id
          ? `${item.i_id}（${item.category_name}）`
          : item.i_id,
    })),
  )

  return {
    loading,
    items,
    lastSourceMode,
    selectOptions,
    loadIids,
  }
}
