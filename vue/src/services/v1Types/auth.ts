import type { BackendUser, FrontendAccess } from '@/services/apiTypes'

export interface AuthSession {
  session_id?: string
  token?: string
  token_type?: string
  expires_at?: string
}

export interface AuthResult {
  user?: BackendUser
  session?: AuthSession
  frontend_access?: FrontendAccess
}

export interface V1DataEnvelope<T> {
  data?: T
}

export type LoginResultEnvelope = V1DataEnvelope<AuthResult>
