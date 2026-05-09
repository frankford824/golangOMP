import { computed } from 'vue'

export type TaskPoolMode = 'pending_assign'

/**
 * 任务中心「未指派任务」Tab 统一语义：
 * - 所有角色统一走任务级列表（GET /v1/tasks）
 * - 通过 task_status=PendingAssign 查看可接单任务
 */
export function useTaskPoolMode() {
  const mode = computed<TaskPoolMode>(() => {
    return 'pending_assign'
  })

  return { mode }
}
