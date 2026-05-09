import type { BackendUser } from '@/services/apiTypes'

export interface MeProfile extends BackendUser {
  nickname?: string
  phone?: string
  account?: string
}

export interface MeOrgProfile {
  department?: string
  team?: string
  teams?: string[]
  roles?: string[]
  managed_departments?: string[]
  managed_teams?: string[]
}
