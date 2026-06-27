import { onBeforeUnmount, ref } from 'vue'

import { assetWorkbenchApi, type AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'

export function useAssetWorkbenchBootstrap() {
  const bootstrap = ref<AssetWorkbenchBootstrap | null>(null)
  const loading = ref(false)
  const error = ref('')
  let controller: AbortController | null = null

  async function refresh() {
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    error.value = ''
    try {
      bootstrap.value = await assetWorkbenchApi.bootstrap(controller.signal)
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载工作台信息失败'
    } finally {
      loading.value = false
    }
  }

  onBeforeUnmount(() => controller?.abort())

  return { bootstrap, loading, error, refresh }
}
