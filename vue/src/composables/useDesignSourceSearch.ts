import { ref } from 'vue'
import { designSourcesApi } from '@/services/api/designSourcesApi'

export function useDesignSourceSearch() {
  const loading = ref(false)
  const items = ref<Array<Record<string, unknown>>>([])
  let seq = 0
  let activeAbort: AbortController | null = null

  async function search(keyword: string): Promise<void> {
    activeAbort?.abort()
    const requestSeq = ++seq
    const abortController = new AbortController()
    activeAbort = abortController
    loading.value = true
    try {
      const res = await designSourcesApi.search({ keyword }, abortController.signal)
      if (abortController.signal.aborted || requestSeq !== seq) return
      const body = (res.data ?? {}) as {
        data?: Array<Record<string, unknown>>
        items?: Array<Record<string, unknown>>
      }
      items.value = body.data ?? body.items ?? []
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

  return { loading, items, search }
}
