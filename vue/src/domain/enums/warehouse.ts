import type { WarehouseReceiptStatus } from '../types/warehouse'

export type { WarehouseReceiptStatus }

export const WAREHOUSE_RECEIPT_STATUS_LABELS: Record<WarehouseReceiptStatus, string> = {
  pending: '待接收',
  received: '已接收',
  returned: '已退回',
  archived: '已归档',
}

export function getWarehouseReceiptStatusLabel(status: WarehouseReceiptStatus): string {
  return WAREHOUSE_RECEIPT_STATUS_LABELS[status] ?? status
}
