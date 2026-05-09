/**
 * useApi — 通用 API 请求封装 composable
 *
 * 提供统一的 loading / error / data 状态管理，支持 AbortController 取消请求。
 *
 * @example
 * ```vue
 * <script setup lang="ts">
 * import { useApi } from '@/composables/useApi'
 * import { tasksApi } from '@/services/api'
 *
 * const { data, loading, error, execute } = useApi(() => tasksApi.list({ page: 1 }))
 *
 * onMounted(() => execute())
 * </script>
 * ```
 */

import { ref, type Ref } from 'vue'
import type { AxiosResponse } from 'axios'

interface UseApiOptions {
  /** 是否在创建时立即执行（默认 false，需手动调用 execute） */
  immediate?: boolean
}

interface UseApiReturn<T> {
  /** 请求返回的数据（axios response.data） */
  data: Ref<T | null>
  /** 加载状态 */
  loading: Ref<boolean>
  /** 错误信息（Error 对象或 null） */
  error: Ref<Error | null>
  /** 手动触发请求，支持传入新的参数 */
  execute: () => Promise<void>
  /** 取消当前请求 */
  abort: () => void
}

export function useApi<T>(
  apiFn: (signal: AbortSignal) => Promise<AxiosResponse<T>>,
  options: UseApiOptions = {},
): UseApiReturn<T> {
  const data = ref<T | null>(null) as Ref<T | null>
  const loading = ref(false)
  const error = ref<Error | null>(null)

  let controller: AbortController | null = null

  function abort() {
    if (controller) {
      controller.abort()
      controller = null
    }
  }

  async function execute() {
    abort()
    controller = new AbortController()
    const signal = controller.signal

    loading.value = true
    error.value = null

    try {
      const response = await apiFn(signal)
      data.value = response.data
    } catch (err: unknown) {
      if ((err as { name?: string })?.name === 'CanceledError' || (err as { name?: string })?.name === 'AbortError') {
        // 请求被主动取消，不更新 error 状态
        return
      }
      error.value = err instanceof Error ? err : new Error(String(err))
    } finally {
      loading.value = false
    }
  }

  if (options.immediate) {
    execute()
  }

  return { data, loading, error, execute, abort }
}
