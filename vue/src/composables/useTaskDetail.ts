import { ref, type Ref } from 'vue'
import { useTasksStore } from '@/stores/tasks'
import type { Task } from '@/domain/types/task'

/**
 * 任务详情拉取与读取，与 `tasksStore.loadTaskById` 对齐（含 `task` + `task_detail` 合并），
 * 不单独持有未合并的 `getDetail` 原始 envelope。
 */
export function useTaskDetail() {
  const tasksStore = useTasksStore()
  const loading: Ref<boolean> = ref(false)

  async function load(taskId: string): Promise<void> {
    loading.value = true
    try {
      await tasksStore.loadTaskById(taskId)
    } finally {
      loading.value = false
    }
  }

  function getTaskById(id: string): Task | null {
    return tasksStore.getById(id) ?? null
  }

  return { load, loading, getTaskById }
}
