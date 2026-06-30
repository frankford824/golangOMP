import type { InjectionKey } from 'vue'
import { inject } from 'vue'

export type RefreshUnreadCount = () => void | Promise<void>

export const refreshUnreadCountKey: InjectionKey<RefreshUnreadCount> = Symbol('awRefreshUnreadCount')

export function useWorkbenchUnreadRefresh() {
  return inject(refreshUnreadCountKey, undefined)
}
