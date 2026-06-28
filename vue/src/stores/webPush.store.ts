import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { notificationsApi } from '@/services/api/notificationsApi'
import {
  ensurePushSubscription,
  getCurrentPushSubscription,
  isWebPushSupported,
  loadNotificationPreferences,
  loadWebPushConfig,
  serializePushSubscription,
  type WebPushRuntimeState,
} from '@/services/webPush'

export const useWebPushStore = defineStore('webPush', () => {
  const supported = ref(isWebPushSupported())
  const serverEnabled = ref(false)
  const preferenceEnabled = ref(false)
  const subscribed = ref(false)
  const permission = ref<NotificationPermission | 'unsupported'>(
    supported.value ? Notification.permission : 'unsupported',
  )
  const publicKey = ref('')
  const keyHash = ref('')
  const reason = ref('')
  const loading = ref(false)
  const errorMessage = ref('')
  let messageListenerAttached = false

  const active = computed(() => supported.value && serverEnabled.value && preferenceEnabled.value && subscribed.value)
  const state = computed<WebPushRuntimeState>(() => ({
    supported: supported.value,
    permission: permission.value,
    enabled: active.value,
    subscribed: subscribed.value,
    reason: reason.value,
  }))

  async function refresh(): Promise<void> {
    supported.value = isWebPushSupported()
    permission.value = supported.value ? Notification.permission : 'unsupported'
    errorMessage.value = ''
    const [config, pref] = await Promise.all([
      loadWebPushConfig(),
      loadNotificationPreferences(),
    ])
    serverEnabled.value = Boolean(config.enabled)
    publicKey.value = String(config.public_key ?? '')
    keyHash.value = String(config.vapid_key_hash ?? '')
    reason.value = String(config.reason ?? '')
    preferenceEnabled.value = Boolean(pref.web_push_enabled)
    const prefKeyHash = String(pref.vapid_key_hash ?? '')
    let current = await getCurrentPushSubscription()
    if (current && keyHash.value && prefKeyHash && prefKeyHash !== keyHash.value) {
      await notificationsApi.deleteCurrentWebPushSubscription(current.endpoint).catch(() => undefined)
      await current.unsubscribe().catch(() => false)
      current = null
    }
    subscribed.value = Boolean(current)
  }

  async function initAfterLogin(): Promise<void> {
    attachServiceWorkerMessageListener()
    await refresh().catch((err) => {
      errorMessage.value = err instanceof Error ? err.message : '通知设置加载失败'
    })
    if (!supported.value || !serverEnabled.value || !preferenceEnabled.value) return
    if (Notification.permission !== 'granted') return
    const current = await ensurePushSubscription({
      enabled: serverEnabled.value,
      public_key: publicKey.value,
      vapid_key_hash: keyHash.value,
    }).catch(() => null)
    if (!current) return
    await notificationsApi.registerWebPushSubscription(serializePushSubscription(current)).catch(() => undefined)
    subscribed.value = true
  }

  async function enable(): Promise<void> {
    attachServiceWorkerMessageListener()
    loading.value = true
    errorMessage.value = ''
    try {
      await refresh()
      const subscription = await ensurePushSubscription({
        enabled: serverEnabled.value,
        public_key: publicKey.value,
        vapid_key_hash: keyHash.value,
      })
      await notificationsApi.registerWebPushSubscription(serializePushSubscription(subscription))
      preferenceEnabled.value = true
      subscribed.value = true
      permission.value = Notification.permission
    } catch (err) {
      errorMessage.value = err instanceof Error ? err.message : '开启系统通知失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function disable(): Promise<void> {
    loading.value = true
    errorMessage.value = ''
    try {
      const subscription = await getCurrentPushSubscription()
      if (subscription) {
        await notificationsApi.deleteCurrentWebPushSubscription(subscription.endpoint).catch(() => undefined)
        await subscription.unsubscribe().catch(() => false)
      }
      await notificationsApi.patchPreferences({ web_push_enabled: false }).catch(() => undefined)
      preferenceEnabled.value = false
      subscribed.value = false
      permission.value = supported.value ? Notification.permission : 'unsupported'
    } finally {
      loading.value = false
    }
  }

  async function sendTest(): Promise<void> {
    loading.value = true
    errorMessage.value = ''
    try {
      await notificationsApi.sendWebPushTest()
    } catch (err) {
      errorMessage.value = err instanceof Error ? err.message : '发送测试通知失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function cleanupOnLogout(): Promise<void> {
    const subscription = await getCurrentPushSubscription().catch(() => null)
    if (subscription) {
      await notificationsApi.deleteCurrentWebPushSubscription(subscription.endpoint).catch(() => undefined)
      await subscription.unsubscribe().catch(() => false)
    }
    preferenceEnabled.value = false
    subscribed.value = false
  }

  function attachServiceWorkerMessageListener(): void {
    if (messageListenerAttached || !('serviceWorker' in navigator)) return
    messageListenerAttached = true
    navigator.serviceWorker.addEventListener('message', (event) => {
      const data = event.data || {}
      if (data.type === 'web_push_navigate' && typeof data.url === 'string') {
        window.location.assign(data.url)
        return
      }
      if (data.type === 'web_push_subscription_changed' && data.subscription?.endpoint) {
        void notificationsApi.registerWebPushSubscription({
          endpoint: String(data.subscription.endpoint),
          keys: {
            p256dh: String(data.subscription.keys?.p256dh ?? ''),
            auth: String(data.subscription.keys?.auth ?? ''),
          },
          user_agent: navigator.userAgent,
          platform: navigator.platform,
        }).catch(() => undefined)
      }
    })
  }

  return {
    supported,
    serverEnabled,
    preferenceEnabled,
    subscribed,
    permission,
    publicKey,
    keyHash,
    reason,
    loading,
    errorMessage,
    active,
    state,
    refresh,
    initAfterLogin,
    enable,
    disable,
    sendTest,
    cleanupOnLogout,
  }
})
