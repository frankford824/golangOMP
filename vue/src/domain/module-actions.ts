import type { ModuleSummary } from '@/services/apiTypes'

export type ModuleAllowedActionsRaw =
  | string[]
  | {
      actions?: unknown
    }
  | null
  | undefined

function normalizeActionList(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  const out = new Set<string>()
  for (const item of raw) {
    const action = String(item ?? '').trim()
    if (action) out.add(action)
  }
  return Array.from(out)
}

export function getModuleAllowedActions(module?: ModuleSummary | null): string[] {
  const raw = module?.allowed_actions as ModuleAllowedActionsRaw
  if (Array.isArray(raw)) return normalizeActionList(raw)
  if (raw && typeof raw === 'object') return normalizeActionList(raw.actions)
  return []
}

export function hasModuleActionProjection(module?: ModuleSummary | null): boolean {
  if (module?.allowed_actions == null) return false
  // 后端若下发空数组，应视为「未投影可执行动作」，继续走任务状态等兜底（与 null 同义）
  return getModuleAllowedActions(module).length > 0
}

export function hasModuleAction(
  module: ModuleSummary | undefined | null,
  candidates: string | string[],
): boolean {
  const wanted = Array.isArray(candidates) ? candidates : [candidates]
  if (wanted.length === 0) return false
  const actions = new Set(getModuleAllowedActions(module))
  return wanted.some((action) => actions.has(action))
}
