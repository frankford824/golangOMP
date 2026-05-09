export type MockModuleState =
  | 'pending'
  | 'pending_claim'
  | 'in_progress'
  | 'submitted'
  | 'approved'
  | 'rejected'
  | 'closed'

export interface MockTaskModule {
  id: string
  task_id: string
  module_key: string
  state: MockModuleState
  claimed_by?: string | null
  allowed_actions: string[]
  updated_at: string
}

export const mockTaskModules: MockTaskModule[] = []

export function listTaskModules(taskId: string): MockTaskModule[] {
  return mockTaskModules.filter((module) => module.task_id === taskId)
}
