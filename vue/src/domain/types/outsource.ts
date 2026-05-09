export type OutsourceOrderStatus =
  | 'draft'
  | 'sent'
  | 'in_progress'
  | 'returned'
  | 'reviewing'
  | 'review_passed'
  | 'review_rejected'
  | 'closed'

export interface OutsourceOrder {
  id: string
  orderNo: string
  taskId: string
  taskNo: string
  sku: string
  productName: string
  outsourceType: string
  supplierId: string
  supplierName: string
  deliveryRequirement: string
  specNote: string
  status: OutsourceOrderStatus
  createdAt: string
  returnedAt?: string
  reviewResult?: 'passed' | 'rejected'
  reviewNote?: string
}
