import type { LegacyTaskStatus } from '@/domain/types/task'

const TASK_LIST_STATUS_FILTER_EXPANSION: Partial<Record<LegacyTaskStatus, LegacyTaskStatus[]>> = {
  // “进行中”是任务中心的业务口径：已有人处理的定制制作也应能被筛到。
  InProgress: [
    'InProgress',
    'Assigned',
    'PendingCustomizationProduction',
    'PendingEffectRevision',
  ],
  PendingAuditA: ['PendingAuditA', 'PendingAuditB'],
  RejectedByAuditA: ['RejectedByAuditA', 'RejectedByAuditB'],
  Outsourcing: [
    'Outsourcing',
    'PendingOutsourceReview',
    'PendingCustomizationReview',
    'PendingCustomizationProduction',
    'PendingEffectRevision',
    'PendingProductionTransfer',
  ],
  Completed: ['Completed', 'PendingClose'],
}

export function expandTaskListStatusFilter(statuses: LegacyTaskStatus[]): LegacyTaskStatus[] {
  const out = new Set<LegacyTaskStatus>()
  for (const status of statuses) {
    const expanded = TASK_LIST_STATUS_FILTER_EXPANSION[status]
    if (expanded?.length) {
      for (const item of expanded) out.add(item)
      continue
    }
    out.add(status)
  }
  return Array.from(out)
}
