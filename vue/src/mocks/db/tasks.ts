export type MockTaskStatus =
  | 'pending_claim'
  | 'in_progress'
  | 'submitted'
  | 'approved'
  | 'rejected'
  | 'completed'
  | 'cancelled'
  | 'closed'

export interface MockTask {
  id: string
  task_no: string
  task_type: string
  title: string
  priority: 'normal' | 'critical' | 'low' | 'high'
  status: MockTaskStatus
  created_by: string
  created_at: string
  updated_at: string
}

export const mockTasks: MockTask[] = []

export function upsertTask(task: MockTask): void {
  const index = mockTasks.findIndex((item) => item.id === task.id)
  if (index === -1) {
    mockTasks.push(task)
    return
  }
  mockTasks[index] = task
}
