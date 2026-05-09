export type WarehouseReceiptStatus = 'pending' | 'received' | 'returned' | 'archived'

export interface WarehouseReceipt {
  id: string
  taskId: string
  taskNo: string
  sku: string
  productName: string
  finalVersionRef: string
  specSummary: string
  auditCompletedAt: string
  isOutsourceReturn: boolean
  status: WarehouseReceiptStatus
  warehouseNote?: string
  receivedAt?: string
}
