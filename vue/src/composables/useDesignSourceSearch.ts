import { ref } from 'vue'
import { designSourcesApi } from '@/services/api/designSourcesApi'

export function useDesignSourceSearch() {
  const loading = ref(false)
  const items = ref<Array<Record<string, unknown>>>([])

  async function search(keyword: string): Promise<void> {
    loading.value = true
    try {
      const res = await designSourcesApi.search({ keyword })
      const body = (res.data ?? {}) as {
        data?: Array<Record<string, unknown>>
        items?: Array<Record<string, unknown>>
      }
      items.value = body.data ?? body.items ?? []
    } finally {
      loading.value = false
    }
  }

  return { loading, items, search }
}
