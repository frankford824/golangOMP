import { computed } from 'vue'
import { usePermissionsStore } from '@/stores/permissions'

export function useAuth() {
  const store = usePermissionsStore()
  const actorRole = computed(() => store.currentUser?.role ?? '')
  /** 与 POST /v1/tasks/{id}/cancel `force: true` 的门禁对齐：超管 / 部门管理员（由 /me → role 推导，已含 is_super_admin / is_department_admin）。 */
  const isDeptAdminPlus = computed(() => store.isSuperAdmin || store.isDeptAdmin)
  return {
    actorRole,
    isDeptAdminPlus,
  }
}
