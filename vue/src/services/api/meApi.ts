import http from '@/services/http'
import type { MeOrgProfile, MeProfile } from '@/services/v1Types'

export interface UpdateMeProfilePayload {
  nickname?: string
  phone?: string
  email?: string
}

export interface ChangeMyPasswordPayload {
  old_password: string
  new_password: string
  password_confirmation?: string
}

export const meApi = {
  getProfile: (signal?: AbortSignal) =>
    http.get<{ data?: MeProfile } | MeProfile>('/v1/me', { signal }),

  patchProfile: (payload: UpdateMeProfilePayload, signal?: AbortSignal) =>
    http.patch<{ data?: MeProfile } | MeProfile>('/v1/me', payload, { signal }),

  changePassword: (payload: ChangeMyPasswordPayload, signal?: AbortSignal) =>
    http.post<{ data?: { message?: string } }>('/v1/me/change-password', payload, { signal }),

  getMyOrg: (signal?: AbortSignal) =>
    http.get<{ data?: MeOrgProfile } | MeOrgProfile>('/v1/me/org', { signal }),
}
