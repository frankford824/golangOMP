import { nowISO } from '@/utils/date'

export interface MockTaskEvent {
  id: string
  task_id: string
  module_key: string
  event_type: string
  created_at: string
  payload: Record<string, unknown>
}

export const mockTaskEvents: MockTaskEvent[] = []

export function pushTaskEvent(event: Omit<MockTaskEvent, 'id' | 'created_at'>): MockTaskEvent {
  const next: MockTaskEvent = {
    id: `evt_${Date.now()}_${Math.floor(Math.random() * 1000)}`,
    created_at: nowISO(),
    ...event,
  }
  mockTaskEvents.unshift(next)
  return next
}
