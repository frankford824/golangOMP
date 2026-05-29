import type { BatchPreviewRow, BatchViolation } from '@/services/api/batchSkuApi'
import type { TaskBatchItem, TaskSkuCodeType } from '@/domain/types'
import { isErpProductNameTooLong } from '@/domain/erp-product-name'

export type ExcelAssistTaskType = 'new_product_development' | 'purchase_task'

export interface MapExcelPreviewOptions {
  skuCodeType?: TaskSkuCodeType
}

export interface ExcelAssistSubmitForm {
  taskType: ExcelAssistTaskType | null
  batchItems: TaskBatchItem[]
  violations: BatchViolation[]
  groupId: string
  dueAt: string | null
}

function pickCostPriceMode(raw: unknown): 'manual' | 'template' | undefined {
  if (typeof raw !== 'string') return undefined
  const v = raw.trim().toLowerCase()
  if (v === 'manual' || v === 'template') return v
  return undefined
}

function pickFiniteNumber(raw: unknown): number | undefined {
  if (typeof raw === 'number' && Number.isFinite(raw)) return raw
  if (typeof raw === 'string' && raw.trim() !== '') {
    const n = Number(raw)
    if (Number.isFinite(n)) return n
  }
  return undefined
}

function pickVariantJson(raw: unknown): Record<string, unknown> | undefined {
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
    return raw as Record<string, unknown>
  }
  if (typeof raw === 'string' && raw.trim() !== '') {
    try {
      const parsed = JSON.parse(raw) as unknown
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        return parsed as Record<string, unknown>
      }
    } catch {
      return undefined
    }
  }
  return undefined
}

export function mapExcelPreviewToBatchItems(
  taskType: ExcelAssistTaskType,
  preview: BatchPreviewRow[],
  options?: MapExcelPreviewOptions,
): TaskBatchItem[] {
  const skuCodeType = options?.skuCodeType ?? 'regular'

  return preview.map((row, idx) => {
    const base: TaskBatchItem = {
      clientKey: `excel-${idx + 1}`,
      productName: String(row.product_name ?? '').trim(),
      skuCodeType,
      referenceFileRefs: (row.reference_file_refs ?? []).map((ref) => ({ ...ref })),
    }

    if (taskType === 'new_product_development') {
      base.designRequirement = String(row.design_requirement ?? '').trim() || undefined
      if (row.product_i_id?.trim()) base.productIId = row.product_i_id.trim()
      return base
    }

    const categoryCode = row.category_code?.trim()
    if (categoryCode) base.categoryCode = categoryCode

    const costPriceMode = pickCostPriceMode(row.cost_price_mode)
    if (costPriceMode) base.costPriceMode = costPriceMode

    const quantity = pickFiniteNumber(row.quantity)
    if (quantity != null) base.quantity = quantity

    const baseSalePrice = pickFiniteNumber(row.base_sale_price)
    if (baseSalePrice != null) base.baseSalePrice = baseSalePrice

    const costPrice = pickFiniteNumber(row.cost_price)
    if (costPrice != null) base.costPriceAmount = costPrice

    if (row.product_i_id?.trim()) base.productIId = row.product_i_id.trim()
    if (row.purchase_sku?.trim()) base.purchaseSku = row.purchase_sku.trim()

    const variantJson = pickVariantJson(row.variant_json)
    if (variantJson) base.variantJson = variantJson

    return base
  })
}

export function canSubmitExcelAssistBatch(form: ExcelAssistSubmitForm): boolean {
  if (!form.taskType) return false
  if (!form.groupId.trim()) return false
  if (!form.dueAt) return false
  if (!Array.isArray(form.batchItems) || form.batchItems.length < 2) return false
  if (form.violations.length > 0) return false
  if (form.batchItems.some((item) => isErpProductNameTooLong(item.productName))) return false
  return true
}

export function excelAssistTaskTypeLabel(taskType: ExcelAssistTaskType): string {
  return taskType === 'purchase_task' ? '采购批量 SKU' : '新款批量 SKU'
}
