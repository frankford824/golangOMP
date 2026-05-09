import { mockBlueprints, type MockBlueprint } from './blueprints'
import { mockTaskModules, type MockModuleState, type MockTaskModule } from './taskModules'
import { nowISO } from '@/utils/date'

function defaultBlueprint(): MockBlueprint {
  return mockBlueprints[0]!
}

export function findBlueprint(taskType: string): MockBlueprint {
  return mockBlueprints.find((b) => b.task_type === taskType) ?? defaultBlueprint()
}

const stateMap: Record<string, MockModuleState> = {
  completed: 'in_progress',
  pending_claim: 'pending_claim',
  pending: 'pending',
  in_progress: 'in_progress',
}

export function instantiateModulesForTask(taskId: string, taskType: string): void {
  const bp = findBlueprint(taskType)
  for (const m of bp.modules) {
    const st = stateMap[m.initial_state] ?? 'pending_claim'
    const mod: MockTaskModule = {
      id: `tm_${taskId}_${m.module_key}`,
      task_id: taskId,
      module_key: m.module_key,
      state: st,
      claimed_by: null,
      allowed_actions: defaultActions(m.module_key, st),
      updated_at: nowISO(),
    }
    mockTaskModules.push(mod)
  }
}

function defaultActions(moduleKey: string, state: MockModuleState): string[] {
  if (state === 'pending_claim') {
    if (moduleKey === 'basic_info') return ['update_basic_info']
    return ['claim', 'reassign']
  }
  if (moduleKey === 'audit' && state === 'in_progress') {
    return ['approve', 'reject', 'update_reference_files']
  }
  if (moduleKey === 'design' || moduleKey === 'retouch' || moduleKey === 'customization') {
    return ['claim', 'submit', 'asset_upload_session_create', 'reassign']
  }
  return ['claim', 'submit']
}
