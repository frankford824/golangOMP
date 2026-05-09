import { unreadNotificationCount } from './db/notifications'

export type MockWsEventType =
  | 'task_pool_count_changed'
  | 'my_task_updated'
  | 'notification_arrived'

export interface MockWsEventDetail {
  type: MockWsEventType
  payload: Record<string, unknown>
}

const MOCK_EVENT_NAME = 'message'
const mockWsTarget = new EventTarget()
let timer: number | undefined

function emit(detail: MockWsEventDetail): void {
  mockWsTarget.dispatchEvent(new CustomEvent(MOCK_EVENT_NAME, { detail }))
}

export function getMockWebSocketTarget(): EventTarget {
  return mockWsTarget
}

export function startMockWsTicker(): void {
  if (timer) return
  timer = window.setInterval(() => {
    emit({
      type: 'task_pool_count_changed',
      payload: {
        count: Math.max(1, Math.floor(Math.random() * 8)),
      },
    })
    emit({
      type: 'notification_arrived',
      payload: {
        unread_count: unreadNotificationCount(),
      },
    })
  }, 12_000)
}

export function stopMockWsTicker(): void {
  if (!timer) return
  window.clearInterval(timer)
  timer = undefined
}
