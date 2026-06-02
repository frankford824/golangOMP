import { describe, expect, it } from 'vitest'
import {
  buildRetouchBatchDownloadPlan,
  collectRetouchRequirementAssetIdsForScope,
  formatRetouchRequirementFolderLabel,
  resolveRetouchSingleAttachmentFilename,
  validateRetouchBatchDownloadPlan,
  MAX_RETouch_BATCH_DOWNLOAD_ASSETS,
} from '@/domain/retouch-requirement-batch-download'
import type { RetouchRequirement } from '@/domain/types/retouch-requirement'

function sampleRequirement(overrides: Partial<RetouchRequirement> = {}): RetouchRequirement {
  return {
    id: 10,
    taskId: 1,
    description: '需求一',
    sortOrder: 1,
    referenceFileRefs: [
      {
        asset_id: '101',
        download_url: 'https://cdn.example/ref.jpg',
        filename: '运营参考.jpg',
      },
      {
        asset_id: 'ref-legacy',
        download_url: 'https://cdn.example/legacy.png',
        filename: 'legacy.png',
      },
    ],
    sourceAssets: [
      {
        id: '201',
        file_role: 'source',
        current_version: {
          id: '301',
          file_name: 'pack.psd',
          download_url: 'https://cdn.example/pack.psd',
        },
      } as never,
    ],
    ...overrides,
  }
}

describe('buildRetouchBatchDownloadPlan', () => {
  const requirements = [
    sampleRequirement(),
    sampleRequirement({
      id: 11,
      description: '需求二',
      sortOrder: 2,
      referenceFileRefs: [{ asset_id: '102', download_url: 'https://x/b.png', filename: 'b.png' }],
      sourceAssets: [],
    }),
  ]

  it('collects all requirements for all_attachments with zip folders', () => {
    const plan = buildRetouchBatchDownloadPlan(requirements, 'all_attachments')
    expect(plan.entries.some((e) => e.zipPath === '需求1/参考图' && e.assetId === 101)).toBe(true)
    expect(plan.entries.some((e) => e.zipPath === '需求1/素材文件' && e.assetId === 201)).toBe(true)
    expect(plan.entries.some((e) => e.zipPath === '需求2/参考图' && e.assetId === 102)).toBe(true)
    expect(plan.assetIdCount).toBe(3)
    expect(plan.legacyCount).toBe(1)
  })

  it('scopes to one requirement references only', () => {
    const plan = buildRetouchBatchDownloadPlan(requirements, 'requirement_references', 0)
    expect(plan.entries.every((e) => e.zipPath.startsWith('需求1/参考图'))).toBe(true)
    expect(plan.entries.some((e) => e.zipPath.includes('素材'))).toBe(false)
  })

  it('scopes to one requirement sources only', () => {
    const plan = buildRetouchBatchDownloadPlan(requirements, 'requirement_sources', 0)
    expect(plan.entries).toHaveLength(1)
    expect(plan.entries[0].zipPath).toBe('需求1/素材文件')
    expect(plan.entries[0].preferredFilename).toBe('pack.psd')
  })

  it('uses SKU and requirement description for retouch batch filenames when available', () => {
    const plan = buildRetouchBatchDownloadPlan(
      [
        sampleRequirement({
          skuCode: 'NSKT000277',
          description: '端午节挂饰/蓝色',
        }),
      ],
      'requirement_sources',
      0,
    )
    expect(plan.entries[0].preferredFilename).toBe('NSKT000277-端午节挂饰_蓝色.psd')
  })

  it('keeps explicit original filename for single retouch attachment download', () => {
    const req = sampleRequirement({ skuCode: 'NSKT000277', description: '端午节挂饰' })
    expect(resolveRetouchSingleAttachmentFilename(req, 0, '客户原始附件.png', true)).toBe(
      '客户原始附件.png',
    )
  })

  it('requirement_all merges references and sources for one row', () => {
    const plan = buildRetouchBatchDownloadPlan(requirements, 'requirement_all', 0)
    expect(plan.entries.length).toBeGreaterThanOrEqual(3)
  })

  it('skips non-numeric legacy asset_id but keeps download_url fallback', () => {
    const plan = buildRetouchBatchDownloadPlan(requirements, 'all_attachments')
    const legacy = plan.entries.find((e) => !e.assetId && e.preferredFilename === 'legacy.png')
    expect(legacy?.assetId).toBeUndefined()
    expect(legacy?.downloadUrl).toContain('legacy.png')
    expect(legacy?.preferredFilename).toBe('legacy.png')
  })
})

describe('validateRetouchBatchDownloadPlan', () => {
  it('rejects when asset id count exceeds batch limit', () => {
    const entries = Array.from({ length: MAX_RETouch_BATCH_DOWNLOAD_ASSETS + 1 }, (_, i) => ({
      key: `a-${i}`,
      assetId: i + 1,
      preferredFilename: `f-${i}.jpg`,
      zipPath: '需求1/参考图',
    }))
    const validation = validateRetouchBatchDownloadPlan({
      entries,
      assetIdCount: entries.length,
      legacyCount: 0,
      skippedUnavailableCount: 0,
    })
    expect(validation.ok).toBe(false)
    expect(validation.message).toContain('100')
  })
})

describe('collectRetouchRequirementAssetIdsForScope', () => {
  it('collects reference and source ids separately', () => {
    const req = sampleRequirement()
    expect(collectRetouchRequirementAssetIdsForScope(req, 'references').referenceAssetIds).toEqual([
      101,
    ])
    expect(collectRetouchRequirementAssetIdsForScope(req, 'sources').sourceAssetIds).toEqual([201])
    expect(collectRetouchRequirementAssetIdsForScope(req, 'all').referenceAssetIds).toEqual([101])
  })
})

describe('formatRetouchRequirementFolderLabel', () => {
  it('uses 1-based Chinese labels', () => {
    expect(formatRetouchRequirementFolderLabel(0)).toBe('需求1')
    expect(formatRetouchRequirementFolderLabel(2)).toBe('需求3')
  })
})
