/**
 * 从 register-options 获取后端认可的 team 枚举值，供任务创建等场景使用。
 * 返回的 option.value 即为后端 owner_team 所需的有效值（非 g1/g2 等本地占位 id）。
 */
import { ref, onMounted } from 'vue'
import { authApi } from '@/services/api/authApi'
import { fetchOrgOwnershipOptions } from '@/services/api/orgApi'
import type { BaseSelectOption } from '@/components/base/BaseSelect.vue'

export function useTeamOptions() {
  const teamOptions = ref<BaseSelectOption[]>([])
  const teamDepartmentMap = ref<Record<string, string>>({})
  const loading = ref(false)
  const loadError = ref('')

  onMounted(async () => {
    loading.value = true
    loadError.value = ''
    try {
      const org = await fetchOrgOwnershipOptions()
      if (org.teamOptions.length > 0) {
        const unique = [...new Set(org.teamOptions.map((t) => t.value).filter(Boolean))]
        teamOptions.value = unique.map((t) => ({ value: t, label: t }))
        const map: Record<string, string> = {}
        for (const row of org.teamOptions) {
          const key = String(row.value ?? '').trim()
          const department = String(row.department ?? '').trim()
          if (key && department && !map[key]) map[key] = department
        }
        teamDepartmentMap.value = map
        return
      }

      // fallback：兼容旧接口 register-options（无 department 映射）
      const res = await authApi.registerOptions()
      let raw = res.data
      if (Array.isArray(raw) && raw.length > 0) raw = raw[0]
      const data = (raw?.data ?? raw) as Record<string, unknown>

      // 支持两种结构：1) 顶层 teams 数组  2) departments[].teams 嵌套
      let teams: string[] = []
      if (Array.isArray(data?.teams)) {
        teams = data.teams as string[]
      } else if (Array.isArray(data?.departments)) {
        const depts = data.departments as Array<{ name?: string; teams?: string[] }>
        teams = depts.flatMap((d) => d.teams ?? [])
      }
      const unique = [...new Set(teams.filter(Boolean))]
      teamOptions.value = unique.map((t) => ({ value: t, label: t }))
      teamDepartmentMap.value = {}
    } catch {
      loadError.value = '加载运营组失败，请稍后重试'
      teamOptions.value = []
      teamDepartmentMap.value = {}
    } finally {
      loading.value = false
    }
  })

  function resolveDepartmentByTeam(team: string | null | undefined): string | undefined {
    const key = String(team ?? '').trim()
    if (!key) return undefined
    const hit = teamDepartmentMap.value[key]
    return hit && hit.trim() ? hit.trim() : undefined
  }

  return { teamOptions, loading, loadError, teamDepartmentMap, resolveDepartmentByTeam }
}
