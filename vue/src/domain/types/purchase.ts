// 仅在 PURCHASE_TASK 下使用的采购子状态
export type PurchaseStatus =
  | 'NotRequired' // 不需要采购（默认值）
  | 'PendingPurchase' // 待采购：有采购需求但未下单
  | 'Purchasing' // 采购中：已下单/跟进中
  | 'Purchased' // 已采购：可参与结单判断
  | 'Cancelled' // 已取消：不再参与采购与结单

export interface PurchasePrice {
  amount: number
  currency: string
}

// 采购信息聚合结构：挂在 Task.purchaseInfo 上
export interface PurchaseInfo {
  status: PurchaseStatus

  supplierId?: string
  supplierName?: string

  quantity?: number
  unit?: string

  purchasePrice?: PurchasePrice

  expectedArrivalAt?: string

  warehouseLocationCode?: string
  warehouseLocationName?: string

  note?: string
}

