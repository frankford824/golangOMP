import http from '@/services/http'

export interface BroadcastNotificationPayload {
  audience: 'all' | 'users'
  user_ids?: number[]
  title: string
  content: string
}

export const notificationsApi = {
  list: (
    params?: {
      is_read?: boolean
      limit?: number
      cursor?: string
    },
    signal?: AbortSignal,
  ) => http.get('/v1/me/notifications', { params, signal }),
  markRead: (id: number, signal?: AbortSignal) =>
    http.post(`/v1/me/notifications/${encodeURIComponent(id)}/read`, {}, { signal }),
  readAll: (signal?: AbortSignal) => http.post('/v1/me/notifications/read-all', {}, { signal }),
  unreadCount: (signal?: AbortSignal) => http.get('/v1/me/notifications/unread-count', { signal }),
  broadcast: (payload: BroadcastNotificationPayload, signal?: AbortSignal) =>
    http.post('/v1/notifications/broadcast', payload, { signal }),
}
