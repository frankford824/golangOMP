import type { PurchaseStatus } from '../types/purchase'

export type { PurchaseStatus }

export const PURCHASE_STATUS_LABELS: Record<PurchaseStatus, string> = {
  NotRequired: '不需要采购',
  PendingPurchase: '待采购',
  Purchasing: '采购中',
  Purchased: '已采购',
  Cancelled: '已取消',
}

export function getPurchaseStatusLabel(status: PurchaseStatus): string {
  return PURCHASE_STATUS_LABELS[status] ?? status
}

