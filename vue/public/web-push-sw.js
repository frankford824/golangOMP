/* eslint-disable no-restricted-globals */
let cachedPublicKey = ''

self.addEventListener('message', (event) => {
  const data = event.data || {}
  if (data.type === 'web_push_config' && typeof data.publicKey === 'string') {
    cachedPublicKey = data.publicKey
  }
})

self.addEventListener('push', (event) => {
  let payload = {}
  try {
    payload = event.data ? event.data.json() : {}
  } catch (_err) {
    payload = {}
  }
  const title = payload.title || '系统通知'
  const options = {
    body: payload.body || '你收到一条新通知',
    tag: payload.tag || `workflow-notification-${payload.notification_id || Date.now()}`,
    renotify: false,
    data: {
      url: payload.url || '/me/notifications',
      notification_id: payload.notification_id,
      task_id: payload.task_id,
    },
  }
  event.waitUntil(self.registration.showNotification(title, options))
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = new URL(event.notification.data?.url || '/me/notifications', self.location.origin)
  event.waitUntil(focusOrOpen(url))
})

self.addEventListener('pushsubscriptionchange', (event) => {
  if (!cachedPublicKey) return
  event.waitUntil(
    self.registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: base64UrlToUint8Array(cachedPublicKey),
    }).then((subscription) =>
      self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
        for (const client of clientList) {
          client.postMessage({ type: 'web_push_subscription_changed', subscription: subscription.toJSON() })
        }
      }),
    ).catch(() => undefined),
  )
})

async function focusOrOpen(url) {
  const allClients = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
  const targetPath = url.pathname
  for (const client of allClients) {
    const clientURL = new URL(client.url)
    if (clientURL.origin === url.origin && clientURL.pathname === targetPath) {
      await client.focus()
      return
    }
  }
  for (const client of allClients) {
    const clientURL = new URL(client.url)
    if (clientURL.origin === url.origin) {
      await client.focus()
      client.postMessage({ type: 'web_push_navigate', url: url.pathname + url.search + url.hash })
      return
    }
  }
  await self.clients.openWindow(url.pathname + url.search + url.hash)
}

function base64UrlToUint8Array(value) {
  const padding = '='.repeat((4 - (value.length % 4)) % 4)
  const base64 = `${value}${padding}`.replace(/-/g, '+').replace(/_/g, '/')
  const raw = self.atob(base64)
  const output = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i += 1) {
    output[i] = raw.charCodeAt(i)
  }
  return output
}
