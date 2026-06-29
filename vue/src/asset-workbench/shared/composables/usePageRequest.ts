import { computed, onBeforeUnmount, ref, shallowRef } from 'vue'

type PageRequest<T> = (signal: AbortSignal) => Promise<T>

export function usePageRequest<T>(
  request: PageRequest<T>,
  initialData: T | null = null,
  fallbackError = '页面数据加载失败',
) {
  const data = shallowRef<T | null>(initialData)
  const loading = ref(false)
  const error = ref('')
  let controller: AbortController | null = null
  let requestId = 0

  const empty = computed(() => !loading.value && !error.value && data.value === null)

  async function run() {
    const id = requestId + 1
    requestId = id
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    error.value = ''
    try {
      data.value = await request(controller.signal)
      return data.value
    } catch (err) {
      if ((err as DOMException)?.name !== 'AbortError') {
        error.value = err instanceof Error ? err.message : fallbackError
      }
      return data.value
    } finally {
      if (id === requestId) {
        loading.value = false
      }
    }
  }

  function reset(next: T | null = initialData) {
    requestId += 1
    controller?.abort()
    data.value = next
    loading.value = false
    error.value = ''
  }

  onBeforeUnmount(() => controller?.abort())

  return { data, loading, error, empty, run, reset }
}
