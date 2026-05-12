import type { TaskBatchItem, TaskBatchTemplateValues, TaskKind } from '@/domain/types'

export function newBatchItemClientKey(): string {
  return `bi_${Date.now()}_${Math.random().toString(36).slice(2, 11)}`
}

export function deepCloneJson<T>(v: T): T {
  return JSON.parse(JSON.stringify(v)) as T
}

export function defaultBatchTemplateValues(kind: TaskKind): TaskBatchTemplateValues {
  if (kind === 'PURCHASE_TASK') {
    return {
      productName: '',
      productChannel: undefined,
      material: undefined,
      materialOther: undefined,
      purchaseSku: undefined,
      skuRuleId: null,
      costPriceMode: 'template',
      costPriceAmount: undefined,
      quantity: undefined,
      baseSalePrice: undefined,
      referenceFileRefs: [],
    }
  }
  return {
    productName: '',
    productShortName: '',
    categoryCode: undefined,
    material: undefined,
    materialOther: undefined,
    designRequirement: '',
    newSku: undefined,
    skuRuleId: null,
    costPriceMode: 'manual',
    costUnitPrice: undefined,
    quantity: undefined,
    basePriceAmount: undefined,
    productReferenceUrl: undefined,
    referenceFileRefs: [],
  }
}

export interface CreateBatchItemsOptions {
  inheritFields: boolean
  inheritRefs: boolean
  taskKind: TaskKind
}

export function createBatchItemsFromTemplate(
  template: TaskBatchTemplateValues,
  count: number,
  opts: CreateBatchItemsOptions,
): TaskBatchItem[] {
  const n = Math.min(50, Math.max(1, Math.floor(count)))
  const out: TaskBatchItem[] = []

  const baseFields: TaskBatchTemplateValues = opts.inheritFields
    ? deepCloneJson(template)
    : defaultBatchTemplateValues(opts.taskKind)

  const baseNameRaw = (template.productName ?? '').trim()
  const baseName = baseNameRaw || '商品'

  for (let i = 0; i < n; i++) {
    const row = deepCloneJson(baseFields) as TaskBatchTemplateValues
    row.newSku = undefined
    row.purchaseSku = undefined
    const refs = opts.inheritRefs && Array.isArray(template.referenceFileRefs)
      ? deepCloneJson(template.referenceFileRefs)
      : []
    row.referenceFileRefs = refs
    // 默认名称：基础名 + 序号（商品1、商品2…）
    row.productName = `${baseName}${i + 1}`

    const item: TaskBatchItem = {
      ...row,
      clientKey: newBatchItemClientKey(),
      _editedFromTemplate: false,
    }

    out.push(item)
  }

  return out
}

export type BatchItemStatus = 'generated' | 'modified' | 'pending' | 'error'

export function getBatchItemDisplayStatus(item: TaskBatchItem, taskKind: TaskKind): BatchItemStatus {
  if (item._editedFromTemplate) return 'modified'
  const sku =
    taskKind === 'PURCHASE_TASK' ? item.purchaseSku?.trim() : item.newSku?.trim()
  if (sku) return 'generated'
  return 'pending'
}

export function batchItemStatusLabel(s: BatchItemStatus): string {
  switch (s) {
    case 'generated':
      return '已生成'
    case 'modified':
      return '已修改'
    case 'pending':
      return '待完善'
    case 'error':
      return '异常'
    default:
      return '—'
  }
}
