import { describe, expect, it } from 'vitest'

import { assetWorkbenchNotificationUnreadCount } from './notificationRealtime'

describe('asset-workbench realtime notification scope', () => {
  it('accepts only asset-workbench notification arrivals', () => {
    expect(assetWorkbenchNotificationUnreadCount({
      type: 'notification_arrived',
      payload: { scope: 'asset_workbench', unread_count: 6 },
    })).toBe(6)
    expect(assetWorkbenchNotificationUnreadCount({
      type: 'notification_arrived',
      payload: { scope: 'main_ops', unread_count: 12 },
    })).toBeNull()
    expect(assetWorkbenchNotificationUnreadCount({
      type: 'notification_arrived',
      payload: { unread_count: 12 },
    })).toBeNull()
    expect(assetWorkbenchNotificationUnreadCount({
      type: 'my_task_updated',
      payload: { scope: 'asset_workbench', unread_count: 12 },
    })).toBeNull()
  })

  it('rejects invalid counts', () => {
    expect(assetWorkbenchNotificationUnreadCount({
      type: 'notification_arrived',
      payload: { scope: 'asset_workbench', unread_count: 'unknown' },
    })).toBeNull()
    expect(assetWorkbenchNotificationUnreadCount({
      type: 'notification_arrived',
      payload: { scope: 'asset_workbench', unread_count: -1 },
    })).toBeNull()
  })
})
