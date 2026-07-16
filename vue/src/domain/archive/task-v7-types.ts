/**
 * Read-only historical values retained solely for rendering immutable pre-cutover evidence.
 * Activity filters, API query builders and action decisions must use `ActiveTaskStatus`.
 */
export type HistoricalTaskStatus =
  | 'PendingAuditA'
  | 'RejectedByAuditA'
  | 'PendingAuditB'
  | 'RejectedByAuditB'
  | 'PendingOutsource'
  | 'Outsourcing'
  | 'PendingOutsourceReview'
  | 'PendingCustomizationReview'
  | 'PendingCustomizationProduction'
  | 'PendingEffectReview'
  | 'PendingEffectRevision'
  | 'PendingProductionTransfer'
  | 'PendingWarehouseQC'
  | 'RejectedByWarehouse'
  | 'PendingWarehouseReceive'
  | 'PendingClose'

export type HistoricalTaskBusinessType = 'PURCHASE_TASK'

export type HistoricalWarehouseSubStatus =
  | 'NOT_REQUIRED'
  | 'PENDING_RECEIVE'
  | 'RECEIVED'
  | 'RETURNED'
  | 'PACKING'
  | 'DONE'

export type HistoricalPurchaseSubStatus =
  | 'NOT_REQUIRED'
  | 'PENDING'
  | 'IN_PROGRESS'
  | 'PURCHASED'
  | 'INBOUND_DONE'

export type HistoricalCloseStatus = 'NOT_READY' | 'READY' | 'CLOSED'
