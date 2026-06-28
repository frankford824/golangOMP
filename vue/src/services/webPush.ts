import { notificationsApi, type WebPushConfigResponse, type NotificationPreferencesResponse } from '@/services/api/notificationsApi'

export interface WebPushRuntimeState {
  supported: boolean
  permission: NotificationPermission | 'unsupported'
  enabled: boolean
  subscribed: boolean
  reason?: string
}

function unwrapData<T>(raw: unknown): T {
  const root = raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {}
  return ((root.data && typeof root.data === 'object') ? root.data : root) as T
}

export function isWebPushSupported(): boolean {
  return (
    typeof window !== 'undefined' &&
    window.isSecureContext &&
    'Notification' in window &&
    'serviceWorker' in navigator &&
    'PushManager' in window
  )
}

export function base64UrlToUint8Array(value: string): Uint8Array {
  const padding = '='.repeat((4 - (value.length % 4)) % 4)
  const base64 = `${value}${padding}`.replace(/-/g, '+').replace(/_/g, '/')
  const raw = typeof globalThis.atob === 'function'
    ? globalThis.atob(base64)
    : Buffer.from(base64, 'base64').toString('binary')
  const output = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i += 1) {
    output[i] = raw.charCodeAt(i)
  }
  return output
}

export async function loadWebPushConfig(signal?: AbortSignal): Promise<WebPushConfigResponse> {
  const res = await notificationsApi.webPushConfig(signal)
  return unwrapData<WebPushConfigResponse>(res.data)
}

export async function loadNotificationPreferences(signal?: AbortSignal): Promise<NotificationPreferencesResponse> {
  const res = await notificationsApi.getPreferences(signal)
  return unwrapData<NotificationPreferencesResponse>(res.data)
}

export async function registerWebPushServiceWorker(publicKey: string): Promise<ServiceWorkerRegistration> {
  const registration = await navigator.serviceWorker.register('/web-push-sw.js', { scope: '/' })
  await navigator.serviceWorker.ready
  registration.active?.postMessage({ type: 'web_push_config', publicKey })
  return registration
}

export async function getCurrentPushSubscription(): Promise<PushSubscription | null> {
  if (!isWebPushSupported()) return null
  const registration = await navigator.serviceWorker.getRegistration('/')
  return registration ? registration.pushManager.getSubscription() : null
}

export async function ensurePushSubscription(config: WebPushConfigResponse): Promise<PushSubscription> {
  if (!isWebPushSupported()) throw new Error('当前浏览器不支持 Web Push')
  if (!config.enabled || !config.public_key) throw new Error('Web Push 未启用')
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') throw new Error('通知权限未允许')
  const registration = await registerWebPushServiceWorker(config.public_key)
  const existing = await registration.pushManager.getSubscription()
  if (existing) return existing
  return registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: base64UrlToUint8Array(config.public_key).buffer as ArrayBuffer,
  })
}

export function serializePushSubscription(subscription: PushSubscription) {
  const json = subscription.toJSON()
  return {
    endpoint: subscription.endpoint,
    keys: {
      p256dh: String(json.keys?.p256dh ?? ''),
      auth: String(json.keys?.auth ?? ''),
    },
    user_agent: navigator.userAgent,
    platform: navigator.platform,
  }
}
