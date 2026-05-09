import { ref, computed, onMounted } from 'vue'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'
import {
  departmentsAndGroupsFromOrgOptions,
  fetchOrgOwnershipOptions,
} from '@/services/api/orgApi'
import { usePermissionsStore } from '@/stores/permissions'

/**
 * 任务列表筛选：归属部门 / 归属团队下拉数据（GET /v1/org/options）。
 * 选中部门后团队列表按 department 联动过滤；无 department 的团队始终展示。
 */
export function useOrgOwnershipFilterOptions(selectedDepartment: () => string) {
  const permissionsStore = usePermissionsStore()
  const departmentOptions = ref<BaseSelectOption[]>([])
  const teamRows = ref<Array<{ value: string; label: string; department?: string }>>([])
  const loadError = ref('')

  const teamOptions = computed<BaseSelectOption[]>(() => {
    const dept = selectedDepartment()?.trim()
    if (!dept) {
      return teamRows.value.map((t) => ({ value: t.value, label: t.label }))
    }
    const filtered = teamRows.value.filter((t) => !t.department || t.department === dept)
    return filtered.map((t) => ({ value: t.value, label: t.label }))
  })

  onMounted(async () => {
    loadError.value = ''
    if (!permissionsStore.hasMenu('org_admin') && !permissionsStore.hasMenu('user_admin')) {
      departmentOptions.value = []
      teamRows.value = []
      return
    }
    try {
      const parsed = await fetchOrgOwnershipOptions()
      departmentOptions.value = parsed.departmentOptions
      teamRows.value = parsed.teamOptions
      const hydrated = departmentsAndGroupsFromOrgOptions(parsed)
      if (hydrated) {
        permissionsStore.hydrateOrgFromServer(hydrated.departments, hydrated.groups)
      }
    } catch {
      loadError.value = '加载组织选项失败'
    }
  })

  return { departmentOptions, teamOptions, loadError }
}
