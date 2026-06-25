import type { BatchPreviewRow, BatchViolation } from '@/services/api/batchSkuApi'
import type { SingleTaskExcelDraft, ExcelAssistViolation } from '@/services/api/excelAssistApi'
import type { TaskBatchItem, TaskSkuCodeType } from '@/domain/types'
import { isErpProductNameTooLong } from '@/domain/erp-product-name'

export type { SingleTaskExcelDraft, ExcelAssistViolation }

export type ExcelAssistTaskType = 'new_product_development' | 'purchase_task'

export type ExcelAssistFlow = 'new_batch' | 'new_single' | 'purchase_single' | 'original_single'

export interface ExcelAssistSingleSubmitForm {
  draft: SingleTaskExcelDraft | null
  violations: ExcelAssistViolation[]
  groupId: string
  dueAt: string | null
}

export interface MapExcelSingleTaskInput {
  draft: SingleTaskExcelDraft
  pageNote?: string
}

export interface MapExcelPurchaseSingleTaskInput {
  draft: SingleTaskExcelDraft
  pageNote?: string
}

export interface MapExcelOriginalSingleTaskInput {
  draft: SingleTaskExcelDraft
  pageNote?: string
}

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

export function excelAssistFlowLabel(flow: ExcelAssistFlow): string {
  if (flow === 'purchase_single') return '采购单 SKU'
  if (flow === 'new_single') return '新款单 SKU'
  if (flow === 'original_single') return '原款开发'
  return '新款批量 SKU'
}

function buildErpProductSnapshotFromDraft(draft: SingleTaskExcelDraft): Record<string, unknown> {
  const erp = draft.erp_product
  const productName = draft.product_name?.trim() ?? erp?.product_name?.trim() ?? erp?.name?.trim() ?? ''
  const skuCode = draft.sku_code?.trim() ?? erp?.sku_code?.trim() ?? ''
  const productId = draft.product_id?.trim() ?? erp?.product_id?.trim() ?? ''
  return {
    product_id: productId,
    sku_code: skuCode,
    name: productName,
    product_name: productName,
    category_code: draft.category_code?.trim() ?? erp?.category_code?.trim() ?? '',
    category_name: draft.category_name?.trim() ?? erp?.category_name?.trim() ?? '',
    image_url: draft.image_url?.trim() ?? erp?.image_url?.trim() ?? '',
  }
}

function parseNumericProductId(productId: string | undefined): string | undefined {
  if (!productId?.trim()) return undefined
  const n = Number.parseInt(productId.trim(), 10)
  if (!Number.isFinite(n) || Number.isNaN(n)) return undefined
  return String(n)
}

export function mapExcelPreviewToSingleTask(input: MapExcelSingleTaskInput): Record<string, unknown> {
  const { draft, pageNote } = input
  const remarkParts = [pageNote?.trim(), draft.remark?.trim()].filter(Boolean)
  return {
    taskType: 'NEW_PRODUCT_DEV',
    skuMode: 'single',
    productSource: 'new',
    category: draft.product_i_id?.trim(),
    productCategoryCode: draft.product_i_id?.trim(),
    productName: draft.product_name?.trim() ?? '',
    designRequirement: draft.design_requirement?.trim() ?? '',
    prefillSpecText: draft.spec_text?.trim() || undefined,
    material: draft.material?.trim() || undefined,
    materialOther: draft.material_other?.trim() || undefined,
    note: remarkParts.length > 0 ? remarkParts.join('\n') : undefined,
  }
}

export function mapExcelPreviewToPurchaseSingleTask(
  input: MapExcelPurchaseSingleTaskInput,
): Record<string, unknown> {
  const { draft, pageNote } = input
  const remarkParts = [pageNote?.trim(), draft.remark?.trim()].filter(Boolean)
  const quantity =
    typeof draft.quantity === 'number' && Number.isFinite(draft.quantity) && draft.quantity > 0
      ? draft.quantity
      : undefined
  return {
    taskType: 'PURCHASE_TASK',
    skuMode: 'single',
    productSource: 'new',
    category: draft.product_i_id?.trim(),
    productCategoryCode: draft.product_i_id?.trim(),
    productName: draft.product_name?.trim() ?? '',
    prefillSpecText: draft.spec_text?.trim() ?? '',
    costPriceMode: 'template',
    purchaseInfo: {
      status: 'PendingPurchase',
      supplierName: '',
      quantity,
    },
    note: remarkParts.length > 0 ? remarkParts.join('\n') : undefined,
    syncErpOnCreate: true,
    requiresAssetVersions: false,
    businessType: 'PURCHASE_TASK',
    businessLane: 'normal',
    workflowLane: 'normal',
  }
}

export function canSubmitExcelAssistSingle(form: ExcelAssistSingleSubmitForm): boolean {
  if (!form.groupId.trim()) return false
  if (!form.dueAt) return false
  if (form.violations.length > 0) return false
  const draft = form.draft
  if (!draft) return false
  if (!draft.product_i_id?.trim()) return false
  if (!draft.product_name?.trim()) return false
  if (!draft.design_requirement?.trim()) return false
  if (isErpProductNameTooLong(draft.product_name)) return false
  return true
}

export function mapExcelPreviewToOriginalSingleTask(
  input: MapExcelOriginalSingleTaskInput,
): Record<string, unknown> {
  const { draft, pageNote } = input
  const remarkParts = [pageNote?.trim(), draft.remark?.trim()].filter(Boolean)
  const productName = draft.product_name?.trim() ?? draft.product_name_snapshot?.trim() ?? ''
  const skuCode = draft.sku_code?.trim() ?? ''
  const numericProductId = parseNumericProductId(draft.product_id)
  const payload: Record<string, unknown> = {
    taskType: 'ORIGINAL_PRODUCT_DEV',
    skuMode: 'single',
    productSource: 'existing',
    sku: skuCode,
    productName,
    designRequirement: draft.change_request?.trim() ?? '',
    prefillSpecText: draft.spec_text?.trim() || undefined,
    erpProductSnapshot: buildErpProductSnapshotFromDraft(draft),
    businessType: 'ORIGINAL_PRODUCT_DEV',
    requiresAssetVersions: true,
    note: remarkParts.length > 0 ? remarkParts.join('\n') : undefined,
  }
  if (numericProductId != null) {
    payload.productId = numericProductId
  }
  return payload
}

export function canSubmitExcelAssistOriginalSingle(form: ExcelAssistSingleSubmitForm): boolean {
  if (!form.groupId.trim()) return false
  if (!form.dueAt) return false
  if (form.violations.length > 0) return false
  const draft = form.draft
  if (!draft) return false
  if (!draft.sku_code?.trim()) return false
  if (!draft.change_request?.trim()) return false
  if (!draft.product_name?.trim()) return false
  if (isErpProductNameTooLong(draft.product_name)) return false
  return true
}

export function canSubmitExcelAssistPurchaseSingle(form: ExcelAssistSingleSubmitForm): boolean {
  if (!form.groupId.trim()) return false
  if (!form.dueAt) return false
  if (form.violations.length > 0) return false
  const draft = form.draft
  if (!draft) return false
  if (!draft.product_i_id?.trim()) return false
  if (!draft.product_name?.trim()) return false
  if (!draft.spec_text?.trim()) return false
  const quantity = draft.quantity
  if (quantity == null || !Number.isFinite(quantity) || quantity <= 0) return false
  if (isErpProductNameTooLong(draft.product_name)) return false
  return true
}
