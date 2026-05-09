export interface TaskDetailResponseContract {
  task: Record<string, unknown>
  modules: Array<Record<string, unknown>>
}

export interface TaskDraftContract {
  id: string
  task_type: string
  payload: Record<string, unknown>
  expires_at: string
}

export interface NotificationContract {
  id: string
  type: string
  title: string
  content: string
  read: boolean
}
