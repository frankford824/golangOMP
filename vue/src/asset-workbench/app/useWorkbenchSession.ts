import { useRouter } from 'vue-router'

import { authApi } from '@/services/api/authApi'
import { clearToken } from '@/services/http'

export function useWorkbenchSession() {
  const router = useRouter()

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // Local token cleanup is the source of truth for leaving this app.
    } finally {
      clearToken()
      await router.replace('/login')
    }
  }

  return { logout }
}
