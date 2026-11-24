import { computed, onBeforeUnmount, ref, watch, type Ref } from 'vue'

import { assetWorkbenchApi, type PersonnelSuggestion, type PersonnelSuggestionScope } from '@aw/shared/api/assetWorkbenchApi'

export function usePersonnelSuggestions(
  scope: PersonnelSuggestionScope,
  query: Readonly<Ref<string>>,
  enabled: Readonly<Ref<boolean>>,
  memberStatus?: Readonly<Ref<string | undefined>>,
) {
  const items = ref<PersonnelSuggestion[]>([])
  const total = ref(0)
  const error = ref('')
  const loading = ref(false)
  const activeIndex = ref(-1)
  const active = computed(() => items.value[activeIndex.value])
  let request: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined

  function cancel() {
    clearTimeout(timer)
    request?.abort()
  }

  async function search() {
    cancel()
    if (!enabled.value) return
    const current = new AbortController()
    request = current
    loading.value = true
    error.value = ''
    try {
      const text = query.value.trim()
      const status = memberStatus?.value
      const result = status
        ? await assetWorkbenchApi.suggestPersonnel(scope, text, current.signal, status)
        : await assetWorkbenchApi.suggestPersonnel(scope, text, current.signal)
      if (current.signal.aborted) return
      items.value = result.items
      total.value = result.total
      activeIndex.value = -1
    } catch (err) {
      if (current.signal.aborted) return
      items.value = []
      total.value = 0
      error.value = err instanceof Error ? err.message : '人员联想加载失败，请重试。'
    } finally {
      if (!current.signal.aborted) loading.value = false
    }
  }

  function step(delta: number) {
    if (!items.value.length) return
    activeIndex.value = activeIndex.value < 0
      ? delta > 0 ? 0 : items.value.length - 1
      : (activeIndex.value + delta + items.value.length) % items.value.length
  }

  watch([query, enabled, () => memberStatus?.value], () => {
    cancel()
    items.value = []
    total.value = 0
    error.value = ''
    activeIndex.value = -1
    loading.value = enabled.value
    if (!enabled.value) return
    if (!query.value.trim()) {
      void search()
    } else {
      timer = setTimeout(search, 220)
    }
  }, { flush: 'sync', immediate: true })

  onBeforeUnmount(cancel)
  return { items, total, error, loading, activeIndex, active, search, step }
}
