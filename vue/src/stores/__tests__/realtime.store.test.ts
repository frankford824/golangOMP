// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const mocks = vi.hoisted(() => ({
  socketOptions: null as null | { onMessage: (event: { type: string; payload: Record<string, unknown> }) => void },
  connect: vi.fn(),
  disconnect: vi.fn(),
  refreshAssetCookie: vi.fn(() => Promise.resolve()),
  listNotifications: vi.fn(() => new Promise<never>(() => undefined)),
}))

vi.mock('@/services/ws/v1Socket', () => ({
  V1SocketClient: class {
    constructor(options: { onMessage: (event: { type: string; payload: Record<string, unknown> }) => void }) {
      mocks.socketOptions = options
    }

    connect() {
      mocks.connect()
    }

    disconnect() {
      mocks.disconnect()
    }
  },
}))

vi.mock('@/services/api/authApi', () => ({
  authApi: { refreshAssetCookie: mocks.refreshAssetCookie },
}))

vi.mock('@/services/api/notificationsApi', () => ({
  notificationsApi: {
    list: mocks.listNotifications,
    unreadCount: vi.fn(),
    markRead: vi.fn(),
    readAll: vi.fn(),
  },
}))

import { useNotificationsStore } from '@/stores/notifications.store'
import { isMainOpsNotificationArrival, useRealtimeStore } from '@/stores/realtime.store'

describe('main-ops realtime notification scope', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    mocks.socketOptions = null
    mocks.connect.mockClear()
    mocks.disconnect.mockClear()
    mocks.refreshAssetCookie.mockClear()
    mocks.listNotifications.mockClear()
  })

  it('ignores asset-workbench events without changing the main-ops badge or list', async () => {
    const notifications = useNotificationsStore()
    notifications.applyUnreadCount(4)
    useRealtimeStore().start()
    await Promise.resolve()

    mocks.socketOptions?.onMessage({
      type: 'notification_arrived',
      payload: { scope: 'asset_workbench', unread_count: 19, notification_id: 91 },
    })
    await Promise.resolve()

    expect(notifications.unreadCount).toBe(4)
    expect(mocks.listNotifications).not.toHaveBeenCalled()
  })

  it.each([
    [{ scope: 'main_ops', unread_count: 7, notification_id: 12 }, 'main_ops'],
    [{ unread_count: 8, notification_id: 13 }, 'legacy no-scope'],
  ])('consumes %s events and refreshes the main-ops list', async (payload, _label) => {
    const notifications = useNotificationsStore()
    useRealtimeStore().start()
    await Promise.resolve()

    mocks.socketOptions?.onMessage({ type: 'notification_arrived', payload })
    await Promise.resolve()

    expect(notifications.unreadCount).toBe(payload.unread_count)
    expect(mocks.listNotifications).toHaveBeenCalledTimes(1)
  })

  it('keeps the scope predicate narrow', () => {
    expect(isMainOpsNotificationArrival({ type: 'notification_arrived', payload: { scope: 'main_ops' } })).toBe(true)
    expect(isMainOpsNotificationArrival({ type: 'notification_arrived', payload: {} })).toBe(true)
    expect(isMainOpsNotificationArrival({ type: 'notification_arrived', payload: { scope: 'asset_workbench' } })).toBe(false)
    expect(isMainOpsNotificationArrival({ type: 'notification_arrived', payload: { scope: 1 } })).toBe(false)
    expect(isMainOpsNotificationArrival({ type: 'my_task_updated', payload: {} })).toBe(false)
  })
})
