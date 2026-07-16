import type { TaskBusinessType, TaskType } from '../types/task'

/**
 * 任务业务分型枚举。
 *
 * - ORIGINAL_PRODUCT_DEV：原品开发 / 旧品改图
 * - NEW_PRODUCT_DEV：新品开发
 * - SKU_PLANNING：策划 SKU（独立创建页，生成后直接结单）
 * - RETOUCH_TASK：P 图任务（图片精修）
 */
export const TaskTypeEnum = {
  ORIGINAL_PRODUCT_DEV: 'ORIGINAL_PRODUCT_DEV',
  NEW_PRODUCT_DEV: 'NEW_PRODUCT_DEV',
  SKU_PLANNING: 'SKU_PLANNING',
  RETOUCH_TASK: 'RETOUCH_TASK',
} as const

export type TaskTypeEnumValue = (typeof TaskTypeEnum)[keyof typeof TaskTypeEnum]

/**
 * 兼容旧有 TaskType / TaskBusinessType 定义的归一化函数。
 *
 * 说明：
 * - 目前 TaskType 与 TaskBusinessType 已与业务分型含义对齐，这里仅做防御性兜底。
 * - 若后续后端返回的字符串存在大小写或别名差异，可在此统一收口。
 */
export function normalizeTaskType(type: TaskType | TaskBusinessType): TaskTypeEnumValue {
  switch (type) {
    case 'SKU_PLANNING':
      return TaskTypeEnum.SKU_PLANNING
    case 'NEW_PRODUCT_DEV':
      return TaskTypeEnum.NEW_PRODUCT_DEV
    case 'RETOUCH_TASK':
      return TaskTypeEnum.RETOUCH_TASK
    case 'ORIGINAL_PRODUCT_DEV':
    default:
      return TaskTypeEnum.ORIGINAL_PRODUCT_DEV
  }
}
