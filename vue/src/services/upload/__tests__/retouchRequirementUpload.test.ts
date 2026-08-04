import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { RetouchRequirement, RetouchRequirementDraft } from '@/domain/types/retouch-requirement'
import { uploadRetouchRequirementPendingAssets } from '../retouchRequirementUpload'

vi.mock('@/services/upload/assetUploadFlow', () => ({
  uploadReferenceFileRef: vi.fn(),
  uploadTaskFileViaAssetSession: vi.fn(),
}))

import { uploadReferenceFileRef, uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'

describe('uploadRetouchRequirementPendingAssets', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('uploads pending files by sort_order alignment with retouchRequirementId', async () => {
    const refFile = new File(['r'], 'ref-1.png', { type: 'image/png' })
    const srcFile = new File(['s'], 'src-1.psd', { type: 'application/octet-stream' })
    const drafts: RetouchRequirementDraft[] = [
      {
        description: '第一条',
        sortOrder: 2,
        pendingReferenceFiles: [refFile],
        pendingSourceFiles: [srcFile],
      },
      {
        description: '第二条',
        sortOrder: 1,
      },
    ]
    const created: RetouchRequirement[] = [
      { id: 20, taskId: 1, description: '第二条', sortOrder: 1 },
      { id: 10, taskId: 1, description: '第一条', sortOrder: 2 },
    ]

    vi.mocked(uploadReferenceFileRef).mockResolvedValue({} as never)
    vi.mocked(uploadTaskFileViaAssetSession).mockResolvedValue({} as never)

    const result = await uploadRetouchRequirementPendingAssets('task-1', created, drafts)

    expect(result.failures).toHaveLength(0)
    expect(result.referenceUploaded).toBe(1)
    expect(result.sourceUploaded).toBe(1)
    expect(uploadReferenceFileRef).toHaveBeenCalledWith(refFile, {
      taskId: 'task-1',
      retouchRequirementId: 10,
      ownerModuleKey: 'basic_info',
      uploadPolicy: 'append_only',
      signal: undefined,
    })
    expect(uploadTaskFileViaAssetSession).toHaveBeenCalledWith(
      'task-1',
      srcFile,
      { asset_kind: 'source', remark: 'src-1.psd' },
      { retouchRequirementId: 10, signal: undefined },
    )
  })

  it('records failure when requirement id is missing', async () => {
    const refFile = new File(['r'], 'ref.png')
    const drafts: RetouchRequirementDraft[] = [
      { description: 'only', sortOrder: 1, pendingReferenceFiles: [refFile] },
    ]
    const created: RetouchRequirement[] = [{ id: 0, taskId: 1, description: 'only', sortOrder: 1 }]

    const result = await uploadRetouchRequirementPendingAssets('task-2', created, drafts)

    expect(result.failures.length).toBeGreaterThan(0)
    expect(uploadReferenceFileRef).not.toHaveBeenCalled()
  })
})
