import type { InjectionKey, Ref } from 'vue'

/** 任务详情页：并列商品当前下标，供「商品与编码信息」与「设计与资产」共用同一切换器 */
export interface TaskDetailProductIndexContext {
  productIndex: Ref<number>
  setProductIndex: (i: number) => void
}

export const TASK_DETAIL_PRODUCT_INDEX_KEY: InjectionKey<TaskDetailProductIndexContext> = Symbol(
  'taskDetailProductIndex',
)
