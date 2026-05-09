export interface TaskDraftRecord {
  id: string
  draft_id?: string
  task_type: string
  payload: Record<string, unknown>
  created_at?: string
  updated_at?: string
  expires_at?: string
}
