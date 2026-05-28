import { describe, it, expect } from 'vitest'
import {
  buildRetouchRequirementsPayload,
  normalizeRetouchRequirementDrafts,
  normalizeRetouchRequirementDraftsWithPending,
} from '@/domain/retouch-requirements'
import type { RetouchRequirementDraft } from '@/domain/types/retouch-requirement'

describe('retouch-requirements payload', () => {
  it('buildRetouchRequirementsPayload omits pending local files', () => {
    const drafts: RetouchRequirementDraft[] = [
      {
        description: '需求 A',
        sortOrder: 1,
        pendingReferenceFiles: [new File(['a'], 'ref-a.png', { type: 'image/png' })],
        pendingSourceFiles: [new File(['b'], 'src-a.psd', { type: 'application/octet-stream' })],
      },
    ]
    const payload = buildRetouchRequirementsPayload(drafts)
    expect(payload).toEqual([
      {
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
        description: 'x',
        pendingReferenceFiles: [new File(['x'], 'r.png')],
      },
    ])
    expect(normalized[0]).toEqual({ description: 'x', sortOrder: 1 })
    expect(normalized[0].pendingReferenceFiles).toBeUndefined()
  })

  it('normalizeRetouchRequirementDraftsWithPending keeps pending files', () => {
    const ref = new File(['a'], 'ref.png')
    const src = new File(['b'], 'src.psd')
    const normalized = normalizeRetouchRequirementDraftsWithPending([
      { description: 'x', pendingReferenceFiles: [ref], pendingSourceFiles: [src] },
    ])
    expect(normalized[0].pendingReferenceFiles).toEqual([ref])
    expect(normalized[0].pendingSourceFiles).toEqual([src])
  })
})
