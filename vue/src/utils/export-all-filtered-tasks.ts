import type { Task } from '@/domain/types/task'
import {
  TASK_EXPORT_ALL_PAGE_SIZE,
  TASK_EXPORT_MAX_TOTAL,
} from '@/constants/task-export'
import type { TaskListParams } from '@/services/apiTypes'

export class TaskExportAllError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'TaskExportAllError'
  }
}

export function stripTaskListPagination(params: TaskListParams): TaskListParams {
  const next = { ...params }
  delete next.page
  delete next.page_size
  return next
}

type LoadTaskListPage = (
  params: TaskListParams,
) => Promise<{ items: Task[]; total: number }>

/**
 * 按任务中心最近一次列表筛选条件分页拉取全部任务（不改 store 列表）。
 */
export async function fetchAllFilteredTasks(
  baseParams: TaskListParams | null,
  loadPage: LoadTaskListPage,
  options?: { pageSize?: number; maxTotal?: number },
): Promise<{ items: Task[]; total: number }> {
  if (!baseParams || Object.keys(baseParams).length === 0) {
    throw new TaskExportAllError('请先在任务中心加载任务列表后再导出全部筛选结果')
  }

  const pageSize = options?.pageSize ?? TASK_EXPORT_ALL_PAGE_SIZE
  const maxTotal = options?.maxTotal ?? TASK_EXPORT_MAX_TOTAL
  const filterParams = stripTaskListPagination(baseParams)

  let first: { items: Task[]; total: number }
  try {
    first = await loadPage({ ...filterParams, page: 1, page_size: pageSize })
  } catch (e) {
    const detail = e instanceof Error ? e.message : '未知错误'
    throw new TaskExportAllError(`拉取任务列表失败：${detail}`)
  }
  const total = first.total

  if (total === 0) {
    throw new TaskExportAllError('当前筛选条件下没有可导出的任务')
  }
  if (total > maxTotal) {
    throw new TaskExportAllError(
      `当前筛选结果超过 ${maxTotal} 条，请缩小筛选条件或后续使用后台导出`,
    )
  }

  const all: Task[] = [...first.items]
  const pageCount = Math.ceil(total / pageSize)

  for (let page = 2; page <= pageCount; page++) {
    try {
      const { items } = await loadPage({ ...filterParams, page, page_size: pageSize })
      all.push(...items)
    } catch (e) {
      const detail = e instanceof Error ? e.message : '未知错误'
      throw new TaskExportAllError(`拉取第 ${page} 页任务失败：${detail}`)
    }
  }

  return { items: all, total }
}
