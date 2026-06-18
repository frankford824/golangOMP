import { ref } from 'vue'
import { erpApi } from '@/services/api/erpApi'

export function useErpProduct() {
  const loading = ref(false)
  const items = ref<Array<Record<string, unknown>>>([])
  let seq = 0
  let activeAbort: AbortController | null = null

  async function searchByCode(code: string): Promise<void> {
    activeAbort?.abort()
    const requestSeq = ++seq
    if (!code.trim()) {
      items.value = []
      loading.value = false
      return
    }
    const abortController = new AbortController()
    activeAbort = abortController
    loading.value = true
    try {
      const res = await erpApi.getProductByCode(code, abortController.signal)
      if (abortController.signal.aborted || requestSeq !== seq) return
      items.value = (res.data as { items?: Array<Record<string, unknown>> }).items ?? []
    } catch (error) {
      if (abortController.signal.aborted || requestSeq !== seq) return
      throw error
    } finally {
      if (activeAbort === abortController) {
        activeAbort = null
      }
      if (requestSeq === seq) {
        loading.value = false
      }
    }
  }

  return { loading, items, searchByCode }
}
