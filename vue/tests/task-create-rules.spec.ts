import { describe, it, expect } from 'vitest'
import { canSubmitTask, getTaskCreateCompletionHint } from '../src/domain/task-create-rules'
import type { TaskCreateFormModel, TaskBatchItem } from '../src/domain/types/task-create'

function baseForm(overrides: Partial<TaskCreateFormModel> = {}): TaskCreateFormModel {
  return {
    productId: null,
    productName: '',
    sku: null,
    groupId: '',
    groupName: '',
    assigneeId: null,
    assigneeName: null,
    designRequirement: '',
    referenceFileRefs: [],
    dueAt: null,
    priority: 'low',
    customizationRequired: false,
    note: '',
    costPriceAmount: undefined,
    costPriceCurrency: 'CNY',
    purchaseQuantity: undefined,
    skuMode: 'single',
    batchItems: [],
    ...overrides,
  }
}

function newProductSingleForm(overrides: Partial<TaskCreateFormModel> = {}): TaskCreateFormModel {
  return baseForm({
    productName: 'Test Product',
    productShortName: 'TP',
    category: '常规kt板',
    material: 'PVC',
    designRequirement: 'Design it',
    groupId: 'team-1',
    dueAt: new Date(Date.now() + 86400000).toISOString(),
    ...overrides,
  })
}

function makeBatchItem(overrides: Partial<TaskBatchItem> = {}): TaskBatchItem {
  return {
    clientKey: 'item-1',
    productName: 'Batch Product',
    productShortName: 'BP',
    categoryCode: '常规kt板',
    material: 'PVC',
    designRequirement: 'Design batch',
    ...overrides,
  }
}

describe('canSubmitTask — batch mode', () => {
  it('batch NEW_PRODUCT_DEV with 1 item returns false (minimum is 2)', () => {
    const form = newProductSingleForm({
      skuMode: 'multiple',
      batchItems: [makeBatchItem()],
    })
    expect(canSubmitTask('NEW_PRODUCT_DEV', form)).toBe(false)
  })

  it('batch NEW_PRODUCT_DEV with 2 items and all fields returns true', () => {
    const form = newProductSingleForm({
      skuMode: 'multiple',
      batchItems: [
        makeBatchItem({ clientKey: 'a' }),
        makeBatchItem({ clientKey: 'b' }),
      ],
    })
    expect(canSubmitTask('NEW_PRODUCT_DEV', form)).toBe(true)
  })

  it('batch ORIGINAL_PRODUCT_DEV always returns false', () => {
    const form = baseForm({
      skuMode: 'multiple',
      sku: 'X',
      productId: '1',
      productName: 'P',
      designRequirement: 'D',
      groupId: 'G',
      dueAt: new Date(Date.now() + 86400000).toISOString(),
      batchItems: [makeBatchItem(), makeBatchItem()],
    })
    expect(canSubmitTask('ORIGINAL_PRODUCT_DEV', form)).toBe(false)
  })
})

describe('canSubmitTask — new product material removed', () => {
  it('single NEW_PRODUCT_DEV without material returns true', () => {
    const form = newProductSingleForm({ material: undefined })
    expect(canSubmitTask('NEW_PRODUCT_DEV', form)).toBe(true)
  })

  it('single NEW_PRODUCT_DEV with empty material returns true', () => {
    const form = newProductSingleForm({ material: '  ' })
    expect(canSubmitTask('NEW_PRODUCT_DEV', form)).toBe(true)
  })

  it('single NEW_PRODUCT_DEV with material returns true', () => {
    const form = newProductSingleForm({ material: 'PVC' })
    expect(canSubmitTask('NEW_PRODUCT_DEV', form)).toBe(true)
  })
})

describe('getTaskCreateCompletionHint — batch minimum', () => {
  it('returns hint about minimum 2 items for batch with 1 item', () => {
    const form = newProductSingleForm({
      skuMode: 'multiple',
      batchItems: [makeBatchItem()],
    })
    expect(getTaskCreateCompletionHint('NEW_PRODUCT_DEV', form)).toBe('批量模式至少需要 2 个商品')
  })
})

describe('getTaskCreateCompletionHint — new product without material', () => {
  it('does not require material in new product', () => {
    const form = newProductSingleForm({ material: undefined })
    expect(getTaskCreateCompletionHint('NEW_PRODUCT_DEV', form)).toBe('可提交')
  })
})
