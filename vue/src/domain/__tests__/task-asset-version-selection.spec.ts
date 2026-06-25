import { describe, expect, it } from 'vitest'
import {
  isTaskAssetVersionUnavailable,
  preferredTaskAssetVersionIndex,
} from '../task-asset-version-selection'
import type { TaskAssetVersion } from '../types/task'

function version(input: Partial<TaskAssetVersion> & { id: string }): TaskAssetVersion {
  return {
    id: input.id,
    type: input.type ?? 'final',
    assetKind: input.assetKind,
    uploaderId: 'u1',
    uploaderName: 'tester',
    uploadedAt: input.uploadedAt ?? '',
    fileRefs: input.fileRefs ?? [],
    nonPreviewFiles: input.nonPreviewFiles ?? [],
    rootVersionNo: input.rootVersionNo,
    totalFileCount: input.totalFileCount,
  }
}

describe('preferredTaskAssetVersionIndex', () => {
  it('prefers the newest usable delivery version even when an older image appears first', () => {
    const versions = [
      version({
        id: '5501',
        assetKind: 'delivery',
        uploadedAt: '2026-06-09T10:00:00Z',
        fileRefs: ['/main-01.jpg'],
      }),
      version({
        id: '6316',
        assetKind: 'delivery',
        uploadedAt: '2026-06-12 01:30:09',
        nonPreviewFiles: [{ label: '修改2026.zip', url: '/download/6316' }],
      }),
    ]

    expect(preferredTaskAssetVersionIndex(versions)).toBe(1)
  })

  it('skips unfinished placeholders and falls back to the newest usable source when no delivery exists', () => {
    const versions = [
      version({ id: '1', assetKind: 'delivery', uploadedAt: '2026-06-12T00:00:00Z', totalFileCount: 1 }),
      version({
        id: '2',
        assetKind: 'source',
        uploadedAt: '2026-06-11T00:00:00Z',
        nonPreviewFiles: [{ label: 'source.psd', url: '/download/source.psd' }],
      }),
    ]

    expect(isTaskAssetVersionUnavailable(versions[0]!)).toBe(true)
    expect(preferredTaskAssetVersionIndex(versions)).toBe(1)
  })
})
