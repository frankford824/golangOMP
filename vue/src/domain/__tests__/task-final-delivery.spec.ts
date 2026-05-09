import { describe, expect, it } from 'vitest'
import {
  latestDeliveryBatchVersionsForSelection,
  latestDeliveryVersionForSelection,
} from '../task-final-delivery'
import type { Task, TaskAssetVersion } from '../types/task'

function version(input: Partial<TaskAssetVersion> & { id: string }): TaskAssetVersion {
  return {
    id: input.id,
    type: input.type ?? 'final',
    assetKind: input.assetKind,
    uploaderId: 'u1',
    uploaderName: 'tester',
    uploadedAt: input.uploadedAt ?? '2026-04-29T00:00:00Z',
    note: input.note,
    assetRootId: input.assetRootId,
    assetNo: input.assetNo,
    rootVersionNo: input.rootVersionNo,
    fileRefs: input.fileRefs ?? [],
    nonPreviewFiles: input.nonPreviewFiles ?? [],
    scopeSkuCode: input.scopeSkuCode,
    previewAvailable: input.previewAvailable,
    totalFileCount: input.totalFileCount,
  }
}

function task(input: Partial<Task>): Task {
  return {
    id: '1',
    taskNo: 'T-1',
    businessType: 'NEW_PRODUCT_DEV',
    taskType: 'NEW_PRODUCT_DEV',
    status: 'InProgress',
    mainStatus: 'InProgress',
    designSubStatus: undefined,
    requesterId: 'u1',
    requesterName: 'ops',
    designerId: null,
    designerName: null,
    referenceFileRefs: [],
    dueAt: null,
    priority: 'normal',
    needOutsource: false,
    assetVersions: [],
    ...input,
  } as Task
}

describe('latestDeliveryVersionForSelection', () => {
  it('does not use reference assets as final delivery preview', () => {
    const result = latestDeliveryVersionForSelection(
      task({
        assetVersions: [version({ id: 'ref-1', assetKind: 'reference', fileRefs: ['/ref.jpg'] })],
      }),
    )

    expect(result).toBeNull()
  })

  it('returns the latest displayable delivery version', () => {
    const result = latestDeliveryVersionForSelection(
      task({
        assetVersions: [
          version({ id: 'd-1', assetKind: 'delivery', fileRefs: ['/old.jpg'] }),
          version({ id: 's-1', assetKind: 'source', nonPreviewFiles: [{ label: 'src.psd', url: '/src.psd' }] }),
          version({ id: 'd-2', assetKind: 'delivery', fileRefs: ['/new.jpg'] }),
        ],
      }),
    )

    expect(result?.id).toBe('d-2')
  })

  it('returns the latest delivery batch in one review round', () => {
    const result = latestDeliveryBatchVersionsForSelection(
      task({
        assetVersions: [
          version({
            id: 'old',
            assetKind: 'delivery',
            fileRefs: ['/old.jpg'],
            assetNo: 'AST-0001',
            uploadedAt: '2026-04-28T10:00:00Z',
          }),
          version({
            id: 'ast-2',
            assetKind: 'delivery',
            nonPreviewFiles: [{ label: 'ast-2.psd', url: '/ast-2.psd' }],
            assetNo: 'AST-0002',
            uploadedAt: '2026-04-29T03:00:00Z',
          }),
          version({
            id: 'ast-3',
            assetKind: 'delivery',
            fileRefs: ['/ast-3.jpg'],
            assetNo: 'AST-0003',
            uploadedAt: '2026-04-29T03:04:00Z',
          }),
        ],
      }),
    )

    expect(result.map((row) => row.id)).toEqual(['ast-3', 'ast-2'])
  })

  it('filters delivery versions by active batch SKU', () => {
    const result = latestDeliveryVersionForSelection(
      task({
        isBatchTask: true,
        batchMode: 'multiple',
        skuItems: [
          { id: 1, skuCode: 'SKU-A', referenceFileRefs: [] },
          { id: 2, skuCode: 'SKU-B', referenceFileRefs: [] },
        ],
        assetVersions: [
          version({ id: 'a', assetKind: 'delivery', fileRefs: ['/a.jpg'], scopeSkuCode: 'SKU-A' }),
          version({ id: 'b', assetKind: 'delivery', fileRefs: ['/b.jpg'], scopeSkuCode: 'SKU-B' }),
          version({ id: 'shared', assetKind: 'delivery', fileRefs: ['/shared.jpg'] }),
        ],
      }),
      { kind: 'product', productIndex: 0 },
    )

    expect(result?.id).toBe('a')
  })
})
