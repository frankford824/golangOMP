import type { WarehouseReceipt, WarehouseReceiptStatus } from '../types/warehouse'
import { getWarehouseReceiptStatusLabel } from '../enums/warehouse'

export interface WarehouseReceiptRow {
  taskId: string
  taskNo: string
  sku: string
  productName: string
  finalVersionRef: string
  specSummary: string
  auditCompletedAt: string
  isOutsourceReturn: boolean
  status: WarehouseReceiptStatus
}

export function warehouseReceiptToRow(dto: WarehouseReceipt): WarehouseReceiptRow {
  return {
    taskId: dto.taskId,
    taskNo: dto.taskNo,
    sku: dto.sku,
    productName: dto.productName,
    finalVersionRef: dto.finalVersionRef,
    specSummary: dto.specSummary,
    auditCompletedAt: dto.auditCompletedAt,
    isOutsourceReturn: dto.isOutsourceReturn ?? false,
    status: dto.status,
  }
}

export function getWarehouseStatusLabel(status: WarehouseReceiptStatus): string {
  return getWarehouseReceiptStatusLabel(status)
}
