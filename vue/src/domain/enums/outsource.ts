import type { OutsourceOrderStatus } from '../types/outsource'

export type { OutsourceOrderStatus }

export const OUTSOURCE_ORDER_STATUS_LABELS: Record<OutsourceOrderStatus, string> = {
  draft: '草稿',
  sent: '已发送',
  in_progress: '处理中',
  returned: '已回传',
  reviewing: '复核中',
  review_passed: '复核通过',
  review_rejected: '复核打回',
  closed: '已关闭',
}

export function getOutsourceOrderStatusLabel(status: OutsourceOrderStatus): string {
  return OUTSOURCE_ORDER_STATUS_LABELS[status] ?? status
}
