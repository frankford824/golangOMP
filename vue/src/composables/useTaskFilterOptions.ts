import { onMounted, ref } from 'vue'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'
import { tasksApi, type TaskFilterActorOption } from '@/services/api/tasksApi'

function extractFilterOptions(payload: unknown): {
  creators?: TaskFilterActorOption[]
  designers?: TaskFilterActorOption[]
} {
  const body =
    payload && typeof payload === 'object' && 'data' in payload
      ? (payload as { data?: unknown }).data
      : payload
  if (!body || typeof body !== 'object') return {}
  return body as {
    creators?: TaskFilterActorOption[]
    designers?: TaskFilterActorOption[]
  }
}

function actorOptions(list: TaskFilterActorOption[] | undefined): BaseSelectOption[] {
  const seen = new Set<string>()
  const out: BaseSelectOption[] = []
  for (const row of list ?? []) {
    const value = String(row.id ?? '').trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    const label = String(row.name || row.display_name || row.username || value).trim()
    out.push({ value, label: label || value })
  }
  return out
}

function withEmpty(options: BaseSelectOption[], includeEmpty: boolean, emptyLabel: string) {
  return includeEmpty ? [{ value: '', label: emptyLabel }, ...options] : options
}

export function useTaskFilterOptions(includeEmpty = true, emptyLabel = '全部') {
  const creatorOptions = ref<BaseSelectOption[]>(withEmpty([], includeEmpty, emptyLabel))
  const assigneeOptions = ref<BaseSelectOption[]>(withEmpty([], includeEmpty, emptyLabel))
  const loading = ref(false)
  const loadError = ref('')

  async function loadFilterOptions() {
    loading.value = true
    loadError.value = ''
    try {
      const res = await tasksApi.filterOptions()
      const options = extractFilterOptions(res?.data)
      creatorOptions.value = withEmpty(actorOptions(options.creators), includeEmpty, emptyLabel)
      assigneeOptions.value = withEmpty(actorOptions(options.designers), includeEmpty, emptyLabel)
    } catch {
      loadError.value = '加载任务筛选候选失败，请稍后重试'
      creatorOptions.value = withEmpty([], includeEmpty, emptyLabel)
      assigneeOptions.value = withEmpty([], includeEmpty, emptyLabel)
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void loadFilterOptions()
  })

  return { creatorOptions, assigneeOptions, loadFilterOptions, loadError, loading }
}
