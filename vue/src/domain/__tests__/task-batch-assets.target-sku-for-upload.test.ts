import { describe, expect, it } from 'vitest'
import type { Task } from '../types/task'
import { targetSkuCodeForUpload } from '../task-batch-assets'

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    sku: null,
    isBatchTask: false,
    batchMode: undefined,
    skuItems: undefined,
    ...overrides,
  } as Task
}

describe('targetSkuCodeForUpload', () => {
  it('returns undefined for single task even when task.sku exists', () => {
    const task = makeTask({ sku: 'SINGLE-SKU' })
    const target = targetSkuCodeForUpload(task, { kind: 'single' })
    expect(target).toBeUndefined()
  })

  it('returns undefined for purchase task guard', () => {
    const task = makeTask({
      sku: 'MAIN-SKU',
      isBatchTask: true,
      skuItems: [{ skuCode: 'SKU-1' }] as Task['skuItems'],
    })
    const target = targetSkuCodeForUpload(task, { kind: 'product', productIndex: 0 }, { isPurchase: true })
    expect(target).toBeUndefined()
  })

  it('returns task sku for batch single selection', () => {
    const task = makeTask({
      sku: 'MAIN-SKU',
      isBatchTask: true,
      skuItems: [
        { skuCode: 'SKU-1' },
        { skuCode: 'SKU-2' },
      ] as Task['skuItems'],
    })
    const target = targetSkuCodeForUpload(task, { kind: 'single' })
    expect(target).toBe('MAIN-SKU')
  })

  it('returns selected sku item for batch product selection', () => {
    const task = makeTask({
      sku: 'MAIN-SKU',
      isBatchTask: true,
      skuItems: [
        { skuCode: 'SKU-1' },
        { skuCode: 'SKU-2' },
      ] as Task['skuItems'],
    })
    const target = targetSkuCodeForUpload(task, { kind: 'product', productIndex: 1 })
    expect(target).toBe('SKU-2')
  })

  it('returns undefined for out-of-range index or blank sku item', () => {
    const task = makeTask({
      sku: 'MAIN-SKU',
      isBatchTask: true,
      skuItems: [{ skuCode: '  ' }] as Task['skuItems'],
    })
    expect(targetSkuCodeForUpload(task, { kind: 'product', productIndex: 0 })).toBeUndefined()
    expect(targetSkuCodeForUpload(task, { kind: 'product', productIndex: 9 })).toBeUndefined()
  })
})
