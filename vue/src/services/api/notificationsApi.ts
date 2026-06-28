import http from '@/services/http'

export interface BroadcastNotificationPayload {
  audience: 'all' | 'users'
  user_ids?: number[]
  title: string
  content: string
}

export interface WebPushConfigResponse {
  enabled: boolean
  public_key?: string
  vapid_key_hash?: string
  subject?: string
  reason?: string
}

export interface NotificationPreferencesResponse {
  user_id: number
  web_push_enabled: boolean
  last_test_sent_at?: string
  vapid_key_hash?: string
}

export interface WebPushSubscriptionPayload {
  endpoint: string
  keys: {
    p256dh: string
    auth: string
  }
  user_agent?: string
  platform?: string
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
  webPushConfig: (signal?: AbortSignal) =>
    http.get<WebPushConfigResponse>('/v1/me/notifications/web-push/config', { signal }),
  registerWebPushSubscription: (payload: WebPushSubscriptionPayload, signal?: AbortSignal) =>
    http.post('/v1/me/notifications/web-push/subscriptions', payload, { signal }),
  deleteCurrentWebPushSubscription: (endpoint: string, signal?: AbortSignal) =>
    http.delete('/v1/me/notifications/web-push/subscriptions/current', { data: { endpoint }, signal }),
  sendWebPushTest: (signal?: AbortSignal) =>
    http.post('/v1/me/notifications/web-push/test', {}, { signal }),
  getPreferences: (signal?: AbortSignal) =>
    http.get<NotificationPreferencesResponse>('/v1/me/notifications/preferences', { signal }),
  patchPreferences: (payload: { web_push_enabled?: boolean }, signal?: AbortSignal) =>
    http.patch<NotificationPreferencesResponse>('/v1/me/notifications/preferences', payload, { signal }),
  broadcast: (payload: BroadcastNotificationPayload, signal?: AbortSignal) =>
    http.post('/v1/notifications/broadcast', payload, { signal }),
}
