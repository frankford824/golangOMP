import type { Task, TaskAssetVersion } from '@/domain/types/task'

/** 版本内可预览图 + 不可预览源文件（如 PSD）的总数，用于角标「N 图」 */
export function versionTotalFileCount(v: TaskAssetVersion): number {
  if (typeof v.totalFileCount === 'number' && v.totalFileCount > 0) {
    return v.totalFileCount
  }
  return v.fileRefs.length + (v.nonPreviewFiles?.length ?? 0)
}

/**
 * 与「创建任务」三合一表单中设计需求类字段标题对齐（原品：修改要求；新品等：设计需求说明）。
 */
export function taskCreateDesignRequirementLabel(
  task: Pick<Task, 'businessType' | 'taskType'>,
): string {
  const k = task.businessType ?? task.taskType
  return k === 'ORIGINAL_PRODUCT_DEV' ? '修改要求' : '设计需求说明'
}
