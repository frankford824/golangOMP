import type { Task } from '../types/task'
import { getTaskStatusLabel } from '../enums/task-status'

export interface TaskViewModel extends Task {
  statusLabel: string
}

export function taskToViewModel(dto: Task): TaskViewModel {
  return {
    ...dto,
    statusLabel: getTaskStatusLabel(dto.status),
  }
}
