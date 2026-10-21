import { computed, ref } from 'vue'
import type { ErpIidOption } from '@/domain/erp-iid-presets'
import { fetchErpIidOptions } from '@/services/erpIidCatalog'

export function useErpIidOptions() {
  const loading = ref(false)
  const items = ref<ErpIidOption[]>([])
  const error = ref('')
  const hasLoaded = ref(false)
  let seq = 0
  let activeAbort: AbortController | null = null

  function reset() {
    seq += 1
    activeAbort?.abort()
    activeAbort = null
    items.value = []
    error.value = ''
    hasLoaded.value = false
    loading.value = false
  }

  async function loadIids(keyword = ''): Promise<void> {
    reset()
    const requestSeq = seq
    const controller = new AbortController()
    activeAbort = controller
    loading.value = true
    try {
      const options = await fetchErpIidOptions(keyword, controller.signal)
      if (controller.signal.aborted || requestSeq !== seq) return
      items.value = options
      hasLoaded.value = true
    } catch {
      if (controller.signal.aborted || requestSeq !== seq) return
      error.value = 'ERP 款式查询失败，请重试；当前未提供可选项'
    } finally {
      if (requestSeq === seq) {
        loading.value = false
        activeAbort = null
      }
    }
  }

  const selectOptions = computed(() => items.value.map((item) => ({
    value: item.i_id,
    label: item.category_name && item.category_name !== item.i_id
      ? `${item.i_id}（${item.category_name}）` : item.i_id,
  })))

  return { loading, items, error, hasLoaded, selectOptions, loadIids, reset }
}
