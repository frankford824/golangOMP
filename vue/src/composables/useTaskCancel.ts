import { ref } from 'vue'
import { tasksApi } from '@/services/api/tasksApi'
import type { CancelRequest } from '@/services/apiTypes'

export function useTaskCancel() {
  const loading = ref(false)
  const needForceConfirm = ref(false)

  async function cancel(taskId: string, payload: CancelRequest): Promise<void> {
    loading.value = true
    needForceConfirm.value = false
    try {
      await tasksApi.cancel(taskId, payload as unknown as Record<string, unknown>)
    } catch (error) {
      const status =
        (error as { status?: number; response?: { status?: number } }).status ??
        (error as { response?: { status?: number } }).response?.status
      const denyCode = (error as { denyCode?: string }).denyCode
      if (status === 409 && !payload.force && (!denyCode || denyCode === 'task_already_claimed')) {
        needForceConfirm.value = true
        return
      }
      throw error
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    needForceConfirm,
    cancel,
  }
}
