import { computed, shallowRef } from 'vue'
import http, { getToken } from '@/services/http'
import { usePermissionsStore } from '@/stores/permissions'

export const oaEnabled = import.meta.env.VITE_OA_FEATURES === 'true'

type OAAccess = {
  id: string | number
  activated: boolean
  super_admin?: boolean
  actions: Record<string, boolean>
}

const snapshot = shallowRef<{ token: string; access: OAAccess | null; loadedAt: number }>()
let pending: { token: string; result: Promise<void> } | undefined

export async function ensureOAAccess() {
  if (!oaEnabled) return
  const token = getToken()
  if (!token) return
  if (snapshot.value?.token === token && Date.now() - snapshot.value.loadedAt < 30_000) return
  if (pending?.token === token) return pending.result
  const result = (async () => {
    try {
      // The supplemental endpoint may be disabled during a staged rollout.
      // The core store alone owns sign-out and restoration of the formal session.
      const response = await http.get<{ data?: OAAccess }>('/oa-api/me', { timeout: 2500, validateStatus: () => true })
      if (getToken() === token) {
        const access = response.status === 200 && response.data?.data?.activated === true ? response.data.data : null
        snapshot.value = { token, access, loadedAt: Date.now() }
      }
    } catch {
      if (getToken() === token) snapshot.value = { token, access: null, loadedAt: Date.now() }
    }
  })()
  pending = { token, result }
  await result
  if (pending?.result === result) pending = undefined
}

export function useOAAccess() {
  const permissions = oaEnabled ? usePermissionsStore() : undefined
  const effective = computed(() => {
    const user = permissions?.currentUser
    const copy = snapshot.value
    if (!oaEnabled || !user || !copy?.access || copy.token !== getToken() || String(copy.access.id) !== String(user.id)) return undefined
    return copy.access
  })
  const currentProfile = computed(() => {
    const user = permissions?.currentUser
    return user ? { id: user.id, name: user.name, super_admin: effective.value?.super_admin === true } : undefined
  })
  const activated = computed(() => effective.value?.activated === true)
  const can = (key: string) => effective.value?.actions?.[key] === true
  return { activated, can, currentProfile }
}
