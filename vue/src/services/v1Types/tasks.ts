export interface AllowedActionsObject {
  actions?: string[]
}

export type AllowedActions = string[] | AllowedActionsObject | null

export interface TaskModuleScope {
  in_scope?: boolean
  deny_code?: string
}

export interface TaskModuleReadModel {
  module_key: string
  state?: string
  scope?: TaskModuleScope
  allowed_actions?: AllowedActions
  projection?: Record<string, unknown>
}

export interface TaskDetailReadModel {
  task: Record<string, unknown>
  modules: TaskModuleReadModel[]
  timeline?: unknown[]
  comments?: unknown[]
}
