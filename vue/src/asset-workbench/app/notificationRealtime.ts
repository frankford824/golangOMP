import type { V1WsEventDetail } from '@/services/ws/v1Socket'

export function assetWorkbenchNotificationUnreadCount(event: V1WsEventDetail): number | null {
  if (event.type !== 'notification_arrived') return null
  if (event.payload.scope !== 'asset_workbench') return null
  const count = Number(event.payload.unread_count)
  if (!Number.isFinite(count) || count < 0) return null
  return Math.trunc(count)
}
