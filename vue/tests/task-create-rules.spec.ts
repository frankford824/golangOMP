import { describe, it, expect } from 'vitest'
import { canSubmitTask, getTaskCreateCompletionHint } from '../src/domain/task-create-rules'
import { buildRetouchRequirementsPayload } from '../src/domain/retouch-requirements'
import { createEmptyRetouchRequirementDraft } from '../src/domain/types/retouch-requirement'
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

function retouchForm(overrides: Partial<TaskCreateFormModel> = {}): TaskCreateFormModel {
  const dueAt = new Date(Date.now() + 86400000).toISOString()
  return baseForm({
    referenceFileRefs: [],
    designRequirement: '',
    dueAt,
    retouchRequirements: [
      {
        ...createEmptyRetouchRequirementDraft(1),
        description: '精修背景',
      },
    ],
    ...overrides,
  })
}

describe('canSubmitTask — RETOUCH_TASK', () => {
  it('allows submit without task-level referenceFileRefs when requirement + dueAt present', () => {
    const form = retouchForm({ referenceFileRefs: [] })
    expect(canSubmitTask('RETOUCH_TASK', form)).toBe(true)
  })

  it('rejects when no valid requirement description', () => {
    const form = retouchForm({
      retouchRequirements: [{ ...createEmptyRetouchRequirementDraft(1), description: '   ' }],
    })
    expect(canSubmitTask('RETOUCH_TASK', form)).toBe(false)
    expect(getTaskCreateCompletionHint('RETOUCH_TASK', form)).toBe('请至少填写 1 条 P 图需求描述')
  })

  it('rejects when dueAt is missing', () => {
    const form = retouchForm({ dueAt: null })
    expect(canSubmitTask('RETOUCH_TASK', form)).toBe(false)
    expect(getTaskCreateCompletionHint('RETOUCH_TASK', form)).toBe('请填写截止时间')
  })
})

describe('buildRetouchRequirementsPayload', () => {
  it('omits pending upload files from POST payload', () => {
    const file = new File(['x'], 'ref.png', { type: 'image/png' })
    const payload = buildRetouchRequirementsPayload([
      {
        description: '需求一',
        sortOrder: 1,
        pendingReferenceFiles: [file],
        pendingSourceFiles: [file],
      },
    ])
    expect(payload).toHaveLength(1)
    expect(payload[0]).toEqual({
      description: '需求一',
      sort_order: 1,
    })
    expect(payload[0]).not.toHaveProperty('pendingReferenceFiles')
    expect(payload[0]).not.toHaveProperty('pendingSourceFiles')
  })

  it('omits sku_code and spec when not provided on draft', () => {
    const payload = buildRetouchRequirementsPayload([
      { description: '仅描述', sortOrder: 1, remark: '备注说明' },
    ])
    expect(payload[0]).toEqual({ description: '仅描述', remark: '备注说明', sort_order: 1 })
    expect(payload[0]).not.toHaveProperty('sku_code')
    expect(payload[0]).not.toHaveProperty('spec')
  })
})

describe('canSubmitTask — other kinds unchanged', () => {
  it('ORIGINAL_PRODUCT_DEV still requires designRequirement', () => {
    const form = baseForm({
      sku: 'SKU-1',
      productId: '1',
      productName: 'Product',
      designRequirement: '',
      dueAt: new Date(Date.now() + 86400000).toISOString(),
    })
    expect(canSubmitTask('ORIGINAL_PRODUCT_DEV', form)).toBe(false)
  })

  it('NEW_PRODUCT_DEV still requires designRequirement for single mode', () => {
    const form = newProductSingleForm({ designRequirement: '' })
    expect(canSubmitTask('NEW_PRODUCT_DEV', form)).toBe(false)
  })
})
