import { computed, onMounted, ref } from 'vue'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'
import {
  tasksApi,
  type TaskFilterActorOption,
  type TaskFilterOrgOption,
} from '@/services/api/tasksApi'

function extractFilterOptions(payload: unknown): {
  creators?: TaskFilterActorOption[]
  designers?: TaskFilterActorOption[]
  owner_departments?: TaskFilterOrgOption[]
  owner_teams?: TaskFilterOrgOption[]
} {
  const body =
    payload && typeof payload === 'object' && 'data' in payload
      ? (payload as { data?: unknown }).data
      : payload
  if (!body || typeof body !== 'object') return {}
  return body as {
    creators?: TaskFilterActorOption[]
    designers?: TaskFilterActorOption[]
    owner_departments?: TaskFilterOrgOption[]
    owner_teams?: TaskFilterOrgOption[]
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

function orgOptions(list: TaskFilterOrgOption[] | undefined): BaseSelectOption[] {
  const seen = new Set<string>()
  const out: BaseSelectOption[] = []
  for (const row of list ?? []) {
    const value = String(row.name ?? '').trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    out.push({ value, label: value })
  }
  return out
}

export function useTaskFilterOptions(
  includeEmpty = true,
  emptyLabel = '全部',
  selectedDepartment: () => string = () => '',
) {
  const creatorOptions = ref<BaseSelectOption[]>(withEmpty([], includeEmpty, emptyLabel))
  const assigneeOptions = ref<BaseSelectOption[]>(withEmpty([], includeEmpty, emptyLabel))
  const ownerDepartmentOptions = ref<BaseSelectOption[]>([])
  const ownerTeamRows = ref<TaskFilterOrgOption[]>([])
  const ownerTeamOptions = computed<BaseSelectOption[]>(() => {
    const department = selectedDepartment().trim()
    const rows = department
      ? ownerTeamRows.value.filter((row) => !row.department_name || row.department_name === department)
      : ownerTeamRows.value
    return orgOptions(rows)
  })
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
      ownerDepartmentOptions.value = orgOptions(options.owner_departments)
      ownerTeamRows.value = options.owner_teams ?? []
    } catch {
      loadError.value = '加载任务筛选候选失败，请稍后重试'
      creatorOptions.value = withEmpty([], includeEmpty, emptyLabel)
      assigneeOptions.value = withEmpty([], includeEmpty, emptyLabel)
      ownerDepartmentOptions.value = []
      ownerTeamRows.value = []
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void loadFilterOptions()
  })

  return {
    creatorOptions,
    assigneeOptions,
    ownerDepartmentOptions,
    ownerTeamOptions,
    loadFilterOptions,
    loadError,
    loading,
  }
}
