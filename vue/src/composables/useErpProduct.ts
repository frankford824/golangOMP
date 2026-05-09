import { ref } from 'vue'
import { erpApi } from '@/services/api/erpApi'

export function useErpProduct() {
  const loading = ref(false)
  const items = ref<Array<Record<string, unknown>>>([])

  async function searchByCode(code: string): Promise<void> {
    if (!code.trim()) {
      items.value = []
      return
    }
    loading.value = true
    try {
      const res = await erpApi.getProductByCode(code)
      items.value = (res.data as { items?: Array<Record<string, unknown>> }).items ?? []
    } finally {
      loading.value = false
    }
  }

  return { loading, items, searchByCode }
}
