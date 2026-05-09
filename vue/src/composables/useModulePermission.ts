import { computed } from 'vue'
import type { ModuleSummary } from '@/services/apiTypes'
import { getModuleAllowedActions } from '@/domain/module-actions'

export function useModulePermission(moduleRef: () => ModuleSummary | undefined) {
  const inScope = computed(() => Boolean(moduleRef()?.scope?.in_scope ?? true))
  const actions = computed(() => getModuleAllowedActions(moduleRef()))
  const can = (action: string): boolean => actions.value.includes(action)
  return {
    inScope,
    actions,
    can,
  }
}
