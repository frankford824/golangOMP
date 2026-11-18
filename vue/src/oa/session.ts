import { readonly, shallowRef } from 'vue'
import { api, type OAProfile } from './api'

const profile = shallowRef<OAProfile>()
let revision = 0
export const currentProfile = readonly(profile)
export const can = (action: string) => profile.value?.actions[action] === true

export async function loadProfile() {
  const requestRevision = ++revision
  const snapshot = await api<OAProfile>('/oa-api/me')
  if (requestRevision === revision) profile.value = snapshot
}

export function resetProfile() {
  revision += 1
  profile.value = undefined
}
