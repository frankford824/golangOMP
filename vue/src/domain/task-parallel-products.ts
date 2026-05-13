import type { Task } from '@/domain/types/task'
import type { ReferenceFileRef } from '@/services/api/assetsApi'

/**
 * 运行态「并列商品」视图行。
 * 批量任务：真实商品仅来自 `sku_items`（用户新增商品在运行态的投影），不包含创建阶段「填写模板」辅助数据。
 */
export interface TaskParallelProductRow {
  index: number
  id?: number
  label: string
  skuCode: string | null
  productName: string
  productShortName?: string
  categoryCode?: string
  materialMode?: string
  designRequirement?: string
  referenceFileRefs: ReferenceFileRef[]
  skuStatus?: string
  filingStatus?: string
  sequenceNo?: number
  quantity?: number
  baseSalePrice?: number
  costPrice?: number
  estimatedCost?: number
  costPriceMode?: string
  manualCostOverride?: boolean
  manualCostOverrideReason?: string
  requiresManualReview?: boolean
}

export function buildParallelProductRows(task: Task): TaskParallelProductRow[] {
  const taskRefs = task.referenceFileRefs ?? []
  const items = task.skuItems ?? []

  if (items.length > 0) {
    return items.map((item, i) => ({
      index: i,
      id: item.id,
      label: `商品 ${i + 1}`,
      skuCode: item.skuCode ?? task.sku ?? null,
      productName: (item.productNameSnapshot ?? task.productName ?? '').trim() || '—',
      productShortName: item.productShortName ?? task.productShortName,
      categoryCode: item.categoryCode ?? task.newProductCategoryCode ?? task.erpCategoryCode,
      materialMode: item.materialMode ?? task.newProductMaterial,
      designRequirement: item.designRequirement ?? task.designRequirement,
      referenceFileRefs: [...(item.referenceFileRefs ?? [])],
      skuStatus: item.skuStatus,
      filingStatus: item.filing_status,
      sequenceNo: item.sequenceNo ?? i + 1,
      quantity: item.quantity,
      baseSalePrice: item.baseSalePrice,
      costPrice: item.costPrice,
      estimatedCost: item.estimatedCost,
      costPriceMode: item.costPriceMode,
      manualCostOverride: item.manualCostOverride,
      manualCostOverrideReason: item.manualCostOverrideReason,
      requiresManualReview: item.requiresManualReview,
    }))
  }

  return [
    {
      index: 0,
      label: '商品 1',
      skuCode: task.sku,
      productName: task.productName?.trim() || '—',
      productShortName: task.productShortName,
      categoryCode: task.newProductCategoryCode ?? task.erpCategoryCode,
      materialMode: task.newProductMaterial,
      designRequirement: task.designRequirement,
      referenceFileRefs: [...taskRefs],
      sequenceNo: 1,
    },
  ]
}

/** 兼容旧调用：批量任务不再前置模板行，恒为 false */
export function prependsBatchTemplateProductRow(_task: Task): boolean {
  return false
}

/** @deprecated 批量任务已不以「首条与主 SKU 重复」区分模板，保留导出避免大范围删改 */
export function firstSkuItemDuplicatesPrimary(_task: Task): boolean {
  return false
}
