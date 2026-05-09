import type { TaskCreateFormModel, TaskKind } from './types'
import { endOfBeijingDayMs, taskInstantMs } from '@/utils/date'

function dueAtMs(value: string): number {
  return /^\d{4}-\d{2}-\d{2}$/.test(value) ? endOfBeijingDayMs(value) : taskInstantMs(value)
}

/**
 * 任务创建表单是否满足提交条件。
 *
 * 仅做前端门禁判断，不参与状态机裁决。
 *
 * 后端契约对齐要点（docs/openapi.yaml CreateTaskRequest）：
 * - 批量 SKU（batch_sku_mode=multiple）至少 2 条 batch_items。
 * - new_product_development 批量 Excel 仅要求产品名称与设计要求，类别与 SKU 由后端处理。
 */
export function canSubmitTask(kind: TaskKind, form: TaskCreateFormModel, now: Date = new Date()): boolean {
  const isBatchMode = form.skuMode === 'multiple'
  if (isBatchMode && (kind === 'ORIGINAL_PRODUCT_DEV' || kind === 'RETOUCH_TASK')) return false
  if (isBatchMode && kind !== 'ORIGINAL_PRODUCT_DEV' && kind !== 'RETOUCH_TASK') {
    if (!Array.isArray(form.batchItems) || form.batchItems.length < 2) return false
  }

  let base = false

  if (kind === 'ORIGINAL_PRODUCT_DEV') {
    // 原品开发：
    // - 必须通过 ERP 选品绑定产品（productId + sku）
    // - 必须填写修改要求
    // - 必须选择截止时间
    base = !!(
      form.sku &&
      form.productId &&
      form.productName &&
      form.designRequirement.trim() &&
      form.dueAt
    )
  } else if (kind === 'NEW_PRODUCT_DEV') {
    // 新品开发：
    // - 必须填写产品名称
    // - 必须填写分类编码
    // - 必须填写设计需求
    // - 必须选择截止时间
    if (isBatchMode) {
      base = !!form.dueAt
    } else {
      base = !!(
        form.productName &&
        form.category &&
        form.designRequirement.trim() &&
        form.dueAt
      )
    }
  } else if (kind === 'PURCHASE_TASK') {
    // 采购任务：
    // - 分类编码 / 产品名称 / 规格尺寸必填
    // - 产品名称必填
    // - 成本计价方式必填（costPriceMode）
    // - 数量必填
    // - 截止时间必填
    base = !!(
      form.category &&
      form.productName &&
      form.prefillSpecText?.trim() &&
      form.costPriceMode &&
      form.dueAt
    )
    if (!base) return false
    if (form.purchaseQuantity == null || Number.isNaN(form.purchaseQuantity)) return false
    // 成本计价方式为 manual 时，必须填写成本
    if (form.costPriceMode === 'manual') {
      if (form.costPriceAmount == null || Number.isNaN(form.costPriceAmount)) return false
    }
  } else if (kind === 'RETOUCH_TASK') {
    base = !!(
      form.referenceFileRefs.length > 0 &&
      form.designRequirement.trim() &&
      form.dueAt
    )
  }

  if (!base) return false

  if (form.customizationRequired && !form.customizationSourceType) return false

  // 截止时间存在时，不允许早于当前时间（预留 1 分钟容差）
  if (form.dueAt) {
    const dueMs = dueAtMs(form.dueAt)
    if (dueMs < now.getTime() - 60000) return false
  }

  return true
}

/**
 * 任务创建表单的完成度提示文案。
 *
 * 用于右侧“必填完成度”展示，与 canSubmitTask 对应。
 */
export function getTaskCreateCompletionHint(
  kind: TaskKind,
  form: TaskCreateFormModel,
  now: Date = new Date(),
): string {
  const isBatchMode = form.skuMode === 'multiple'
  if (isBatchMode && (kind === 'ORIGINAL_PRODUCT_DEV' || kind === 'RETOUCH_TASK')) {
    return '当前任务类型不支持批量 SKU 创建'
  }
  if (isBatchMode && (!Array.isArray(form.batchItems) || form.batchItems.length < 2)) {
    return '批量模式至少需要 2 个商品'
  }

  if (canSubmitTask(kind, form, now)) return '可提交'

  let base = false

  if (kind === 'ORIGINAL_PRODUCT_DEV') {
    base = !!(form.sku && form.productId && form.productName && form.groupId)
  } else if (kind === 'NEW_PRODUCT_DEV') {
    base = form.skuMode === 'multiple' ? !!form.dueAt : !!(form.productName && form.category)
  } else if (kind === 'PURCHASE_TASK') {
    base = !!(form.productName && form.category)
  } else if (kind === 'RETOUCH_TASK') {
    base = !!(form.referenceFileRefs.length > 0 && form.designRequirement.trim())
  }

  if (base && form.dueAt) {
    const dueMs = dueAtMs(form.dueAt)
    if (dueMs < now.getTime() - 60000) {
      return '截止时间不能早于当前时间'
    }
  }

  if (form.customizationRequired && !form.customizationSourceType) {
    return '定制任务请先选择定制来源类型'
  }

  if (kind === 'ORIGINAL_PRODUCT_DEV') {
    if (!form.productId || !form.sku) return '请选择 ERP 产品并绑定 SKU'
    if (!form.designRequirement.trim()) return '请填写修改要求'
    if (!form.dueAt) return '请填写截止时间'
    return '请完善原品开发必填信息'
  }
  if (kind === 'NEW_PRODUCT_DEV') {
    if (form.skuMode !== 'multiple' && !form.productName) return '请填写产品名称'
    if (form.skuMode !== 'multiple' && !form.category) return '请填写产品分类编码'
    if (form.skuMode !== 'multiple' && !form.designRequirement.trim()) return '请填写设计需求'
    if (!form.dueAt) return '请填写截止时间'
    return '请完善新品开发必填信息'
  }

  if (kind === 'PURCHASE_TASK') {
    if (!form.category) return '请填写产品分类编码'
    if (!form.productName) return '请填写产品名称'
    if (!form.prefillSpecText?.trim()) return '请填写规格尺寸'
    if (!form.costPriceMode) return '请选择成本计价方式'
    if (form.purchaseQuantity == null || Number.isNaN(form.purchaseQuantity)) return '请填写数量'
    if (form.costPriceMode === 'manual' && (form.costPriceAmount == null || Number.isNaN(form.costPriceAmount))) {
      return '成本计价方式为手动录入时，请填写成本'
    }
    if (!form.dueAt) return '请填写截止时间'
    return '请完善采购必填信息'
  }

  if (kind === 'RETOUCH_TASK') {
    if (form.referenceFileRefs.length === 0) return '请上传图片/附件'
    if (!form.designRequirement.trim()) return '请填写修改要求'
    if (!form.dueAt) return '请填写截止时间'
    return '请完善 P 图任务必填信息'
  }

  return '请完善必填信息'
}

