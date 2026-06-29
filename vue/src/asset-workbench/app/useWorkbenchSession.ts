import { useRouter } from 'vue-router'

import { authApi } from '@/services/api/authApi'
import { clearToken } from '@/services/http'
import { useAssetWorkbenchSessionStore } from './session.store'

export function useWorkbenchSession() {
  const router = useRouter()
  const session = useAssetWorkbenchSessionStore()

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // Local token cleanup is the source of truth for leaving this app.
    } finally {
      clearToken()
      session.reset()
      await router.replace('/login')
    }
  }

  return { logout }
}
