import { mockNotifications, unreadNotificationCount } from '../db/notifications'
import type { MockHandler } from './types'
import { getMockWebSocketTarget } from '../ws'

function pushWsNotification(unreadCount: number): void {
  const target = getMockWebSocketTarget()
  target.dispatchEvent(
    new CustomEvent('message', {
      detail: {
        type: 'notification_arrived',
        payload: { unread_count: unreadCount },
      },
    }),
  )
}

export const notificationsHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/me/notifications') {
    return {
      status: 200,
      data: {
        data: mockNotifications,
        next_cursor: '',
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/me/notifications/unread-count') {
    return {
      status: 200,
      data: {
        unread_count: unreadNotificationCount(),
      },
    }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/me\/notifications\/[^/]+\/read$/)) {
    const id = Number(request.path.split('/')[4] ?? '')
    const target = mockNotifications.find((item) => item.id === id)
    if (!target) return { status: 404, data: { message: 'notification not found' } }
    target.is_read = true
    pushWsNotification(unreadNotificationCount())
    return { status: 204, data: null }
  }

  if (request.method === 'POST' && request.path === '/v1/me/notifications/read-all') {
    mockNotifications.forEach((item) => {
      item.is_read = true
    })
    pushWsNotification(unreadNotificationCount())
    return { status: 204, data: null }
  }

  return null
}
