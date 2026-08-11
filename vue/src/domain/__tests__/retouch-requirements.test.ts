import { describe, it, expect } from 'vitest'
import {
  buildRetouchRequirementsPayload,
  normalizeRetouchRequirementDrafts,
  normalizeRetouchRequirementDraftsWithPending,
  selectRetouchRequirementForReferenceSupplement,
} from '@/domain/retouch-requirements'
import type { RetouchRequirement, RetouchRequirementDraft } from '@/domain/types/retouch-requirement'

describe('retouch-requirements payload', () => {
  it('buildRetouchRequirementsPayload omits pending local files', () => {
    const drafts: RetouchRequirementDraft[] = [
      {
			skuCode: 'SKU-A',
        description: '需求 A',
        sortOrder: 1,
        pendingReferenceFiles: [new File(['a'], 'ref-a.png', { type: 'image/png' })],
        pendingSourceFiles: [new File(['b'], 'src-a.psd', { type: 'application/octet-stream' })],
      },
    ]
    const payload = buildRetouchRequirementsPayload(drafts)
    expect(payload).toEqual([
      {
			sku_code: 'SKU-A',
        description: '需求 A',
        sort_order: 1,
      },
    ])
    expect(payload[0]).not.toHaveProperty('pendingReferenceFiles')
    expect(payload[0]).not.toHaveProperty('pending_source_files')
  })

  it('normalizeRetouchRequirementDrafts strips pending files', () => {
    const normalized = normalizeRetouchRequirementDrafts([
      {
			skuCode: 'SKU-X',
        description: 'x',
        pendingReferenceFiles: [new File(['x'], 'r.png')],
      },
    ])
		expect(normalized[0]).toEqual({ description: 'x', skuCode: 'SKU-X', sortOrder: 1 })
    expect(normalized[0].pendingReferenceFiles).toBeUndefined()
  })

  it('normalizeRetouchRequirementDraftsWithPending keeps pending files', () => {
    const ref = new File(['a'], 'ref.png')
    const src = new File(['b'], 'src.psd')
    const normalized = normalizeRetouchRequirementDraftsWithPending([
		{ description: 'x', skuCode: 'SKU-X', pendingReferenceFiles: [ref], pendingSourceFiles: [src] },
    ])
    expect(normalized[0].pendingReferenceFiles).toEqual([ref])
    expect(normalized[0].pendingSourceFiles).toEqual([src])
  })

  it('selects the first persisted requirement without reference files for supplement upload', () => {
    const requirements = [
      {
        id: 108,
        taskId: 2003,
        description: '需求 2',
        sortOrder: 2,
        referenceFileRefs: [{ ref_id: 'ref-2' }],
      },
      {
        id: 107,
        taskId: 2003,
        description: '需求 1',
        sortOrder: 1,
        referenceFileRefs: [],
      },
    ] as RetouchRequirement[]

    expect(selectRetouchRequirementForReferenceSupplement(requirements)?.id).toBe(107)
  })

  it('falls back to the first persisted requirement when every requirement already has references', () => {
    const requirements = [
      {
        id: 109,
        taskId: 2003,
        description: '需求 2',
        sortOrder: 2,
        referenceFileRefs: [{ ref_id: 'ref-2' }],
      },
      {
        id: 108,
        taskId: 2003,
        description: '需求 1',
        sortOrder: 1,
        referenceFileRefs: [{ ref_id: 'ref-1' }],
      },
    ] as RetouchRequirement[]

    expect(selectRetouchRequirementForReferenceSupplement(requirements)?.id).toBe(108)
  })
})
