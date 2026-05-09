export interface MockTaskDraft {
  id: string
  task_type: string
  payload: Record<string, unknown>
  created_by: string
  created_at: string
  updated_at: string
  expires_at: string
}

export const mockTaskDrafts: MockTaskDraft[] = []

export function removeTaskDraft(draftId: string): boolean {
  const index = mockTaskDrafts.findIndex((draft) => draft.id === draftId)
  if (index === -1) return false
  mockTaskDrafts.splice(index, 1)
  return true
}
