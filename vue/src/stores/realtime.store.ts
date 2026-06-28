import { defineStore } from 'pinia'
import { ref } from 'vue'
import { formatNotification } from '@/domain/notification-text'
import { authApi } from '@/services/api/authApi'
import { V1SocketClient, type V1WsEventDetail } from '@/services/ws/v1Socket'
import { useNotificationsStore, type NotificationItem } from '@/stores/notifications.store'
import { useWebPushStore } from '@/stores/webPush.store'

export const useRealtimeStore = defineStore('realtime', () => {
  const started = ref(false)
  const browserPermission = ref<NotificationPermission>(
    typeof window !== 'undefined' && 'Notification' in window ? Notification.permission : 'denied',
  )
  let client: V1SocketClient | null = null

  function start(): void {
    if (started.value) return
    const notificationsStore = useNotificationsStore()
    client = new V1SocketClient({
      onMessage: (event) => void handleMessage(event),
      onFallbackPoll: () => void notificationsStore.load(),
    })
    void authApi.refreshAssetCookie()
      .catch(() => undefined)
      .finally(() => client?.connect())
    started.value = true
  }

  function stop(): void {
    client?.disconnect()
    client = null
    started.value = false
  }

  async function requestBrowserPermission(): Promise<NotificationPermission> {
    if (typeof window === 'undefined' || !('Notification' in window)) {
      browserPermission.value = 'denied'
      return browserPermission.value
    }
    if (Notification.permission !== 'default') {
      browserPermission.value = Notification.permission
      return browserPermission.value
    }
    browserPermission.value = await Notification.requestPermission()
    return browserPermission.value
  }

  async function handleMessage(event: V1WsEventDetail): Promise<void> {
    if (event.type !== 'notification_arrived') return
    const notificationsStore = useNotificationsStore()
    notificationsStore.applyUnreadCount(event.payload.unread_count)
    const notificationID = Number(event.payload.notification_id)
    await notificationsStore.load().catch(() => undefined)
    if (!Number.isFinite(notificationID) || notificationID <= 0) return
    const item = notificationsStore.items.find((candidate) => candidate.id === notificationID)
    if (item) showBrowserNotification(item)
  }

  function showBrowserNotification(item: NotificationItem): void {
    if (typeof window === 'undefined' || !('Notification' in window)) return
    if (useWebPushStore().active) return
    browserPermission.value = Notification.permission
    if (browserPermission.value !== 'granted') return
    if (document.visibilityState === 'visible') return
    const text = formatNotification(item.notification_type, item.payload as Record<string, unknown> | undefined)
    const notice = new Notification(text.title, {
      body: text.content,
      tag: `workflow-notification-${item.id}`,
    })
    notice.onclick = () => {
      window.focus()
      const payload = (item.payload ?? {}) as Record<string, unknown>
      const taskID = Number(payload.task_id)
      if (Number.isFinite(taskID) && taskID > 0) {
        window.location.assign(`/tasks/${taskID}`)
      }
    }
  }

  return {
    started,
    browserPermission,
    start,
    stop,
    requestBrowserPermission,
  }
})
