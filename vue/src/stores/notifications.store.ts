import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { notificationsApi } from '@/services/api/notificationsApi'
import type { NotificationRecord } from '@/services/v1Types'

export interface NotificationItem extends NotificationRecord {
  // Backward-compatible fields for legacy views.
  title?: string
  content?: string
  type?: string
  read?: boolean
}

function unwrapItems(raw: unknown): NotificationItem[] {
  const root = raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {}
  const data = root.data
  if (Array.isArray(data)) return data as NotificationItem[]
  const body = data && typeof data === 'object' ? (data as Record<string, unknown>) : root
  const items = body.items ?? body.notifications ?? []
  return Array.isArray(items) ? (items as NotificationItem[]) : []
}

function unreadFromRaw(raw: unknown, items: NotificationItem[]): number {
  const root = raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {}
  const body = root.data && typeof root.data === 'object' ? (root.data as Record<string, unknown>) : root
  const count = body.unread_count ?? body.count
  return typeof count === 'number'
    ? count
    : items.filter((item) => !(item.read ?? item.is_read)).length
}

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref<NotificationItem[]>([])
  const unreadCount = ref(0)
  const loading = ref(false)

  let listGeneration = 0
  let countGeneration = 0
  const pagedOA = import.meta.env.VITE_OA_FEATURES === 'true'

  const unreadItems = computed(() => items.value.filter((item) => !(item.read ?? item.is_read)))

  async function load(): Promise<void> {
    const generation = ++listGeneration
    loading.value = true
    try {
      const res = await notificationsApi.list()
      if (generation !== listGeneration) return
      const nextItems = unwrapItems(res.data)
      items.value = nextItems
      if (pagedOA) await refreshUnreadCount()
      else unreadCount.value = unreadFromRaw(res.data, nextItems)
    } finally {
      if (generation === listGeneration) loading.value = false
    }
  }

  async function refreshUnreadCount(): Promise<void> {
    const generation = ++countGeneration
    const res = await notificationsApi.unreadCount()
    if (generation === countGeneration) unreadCount.value = unreadFromRaw(res.data, items.value)
  }

  function applyUnreadCount(value: unknown): void {
    const n = Number(value)
    if (value != null && Number.isFinite(n)) {
      countGeneration++
      unreadCount.value = n
    }
  }

  function reset(): void {
    listGeneration++
    countGeneration++
    items.value = []
    unreadCount.value = 0
    loading.value = false
  }

  async function markRead(id: number): Promise<void> {
    await notificationsApi.markRead(id)
    await load()
  }

  async function readAll(): Promise<void> {
    await notificationsApi.readAll()
    await load()
  }

  return {
    items,
    unreadItems,
    unreadCount,
    loading,
    load,
    refreshUnreadCount,
    applyUnreadCount,
    reset,
    markRead,
    readAll,
  }
})
