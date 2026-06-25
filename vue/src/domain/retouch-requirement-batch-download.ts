import type { RetouchRequirement } from '@/domain/types/retouch-requirement'
import {
  collectRetouchRequirementBatchAssetIds,
  retouchRequirementReferenceRefsToDisplayItems,
  retouchSourceAssetsToDisplayItems,
  type RetouchReferenceDisplayItem,
  type RetouchSourceFileDisplayItem,
} from '@/domain/retouch-requirement-assets'
import { assetsApi, type AssetBatchDownloadFailure, type AssetBatchDownloadItem } from '@/services/api/assetsApi'
import {
  buildTimestampedZipFilename,
  downloadBatchAsZip,
  sanitizeZipEntryName,
  type BatchZipDownloadSource,
} from '@/utils/batchZipDownload'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

/** Align with POST /v1/assets/batch-download limit (OpenAPI maxItems: 100). */
export const MAX_RETouch_BATCH_DOWNLOAD_ASSETS = 100

export const RETOUCH_ZIP_REFERENCE_DIR = '参考图'
export const RETOUCH_ZIP_SOURCE_DIR = '素材文件'

export type RetouchBatchDownloadScope =
  | 'all_attachments'
  | 'requirement_all'
  | 'requirement_references'
  | 'requirement_sources'

export interface RetouchBatchDownloadPlanEntry {
  key: string
  assetId?: number
  preferredFilename: string
  /** Directory inside zip, e.g. `需求1/参考图` (no trailing slash). */
  zipPath: string
  downloadUrl?: string
}

export interface RetouchBatchDownloadPlan {
  entries: RetouchBatchDownloadPlanEntry[]
  assetIdCount: number
  legacyCount: number
  /** Attachments visible in UI but missing numeric asset id and download_url. */
  skippedUnavailableCount: number
}

export interface RetouchBatchDownloadValidation {
  ok: boolean
  message?: string
  assetIdCount: number
  totalEntryCount: number
}

export function formatRetouchRequirementFolderLabel(requirementIndex: number): string {
  return `需求${requirementIndex + 1}`
}

function retouchBusinessAttachmentFilename(
  requirement: RetouchRequirement,
  requirementIndex: number,
  sourceFilename: string,
): string {
  const sku = String(requirement.skuCode ?? '').trim()
  const label = String(
    requirement.description || requirement.spec || requirement.remark || formatRetouchRequirementFolderLabel(requirementIndex),
  ).trim()
  if (!sku || !label) return sourceFilename
  const ext = (() => {
    const match = /\.([a-z0-9]{1,10})(?:$|[?#])/i.exec(sourceFilename.trim())
    return match ? `.${match[1]}` : ''
  })()
  const base = `${sku}-${label}`
  const fallback = sourceFilename.trim() || `${sku}-${formatRetouchRequirementFolderLabel(requirementIndex)}${ext}`
  return sanitizeZipEntryName(`${base}${ext}`, fallback)
}

export function resolveRetouchSingleAttachmentFilename(
  requirement: RetouchRequirement | undefined,
  requirementIndex: number,
  sourceFilename: string,
  hasOriginalFilename?: boolean,
): string {
  const filename = String(sourceFilename ?? '').trim()
  if (!requirement) return filename
  if (hasOriginalFilename === true) return filename
  if (hasOriginalFilename !== false && filename && !isGenericRetouchAttachmentName(filename)) {
    return filename
  }
  return retouchBusinessAttachmentFilename(requirement, requirementIndex, filename)
}

function isGenericRetouchAttachmentName(filename: string): boolean {
  const normalized = filename.trim()
  return /^参考图\s*\d+$/i.test(normalized) || /^素材\s+\S+$/i.test(normalized) || /^asset-\d+$/i.test(normalized)
}

export function resolveRetouchBatchZipPrefix(
  requirements: RetouchRequirement[],
  scope: RetouchBatchDownloadScope,
  requirementIndex?: number,
  taskBusinessName?: string,
): string {
  const selected =
    scope === 'all_attachments'
      ? requirements
      : requirementIndex != null && requirementIndex >= 0
        ? [requirements[requirementIndex]].filter(Boolean)
        : []
  const fallbackRequirement = selected[0] ?? requirements[0]
  const sku = resolveRetouchBatchZipSku(selected)
  const label = String(
    taskBusinessName ||
      fallbackRequirement?.description ||
      fallbackRequirement?.spec ||
      fallbackRequirement?.remark ||
      '',
  ).trim()
  const scopeLabel = retouchBatchScopeSuffix(scope, requirementIndex)
  const parts = [sku, label, scopeLabel].filter(Boolean)
  return sanitizeZipEntryName(parts.join('-'), 'retouch-requirements')
}

function resolveRetouchBatchZipSku(requirements: Array<RetouchRequirement | undefined>): string {
  const values = requirements
    .map((item) => String(item?.skuCode ?? '').trim())
    .filter(Boolean)
  if (!values.length) return ''
  const first = values[0]
  return values.every((value) => value === first) ? first : 'multi-sku'
}

function retouchBatchScopeSuffix(scope: RetouchBatchDownloadScope, requirementIndex?: number): string {
  if (scope === 'all_attachments') return ''
  const requirementLabel =
    requirementIndex != null && requirementIndex >= 0 ? formatRetouchRequirementFolderLabel(requirementIndex) : ''
  switch (scope) {
    case 'requirement_references':
      return [requirementLabel, RETOUCH_ZIP_REFERENCE_DIR].filter(Boolean).join('-')
    case 'requirement_sources':
      return [requirementLabel, RETOUCH_ZIP_SOURCE_DIR].filter(Boolean).join('-')
    case 'requirement_all':
      return requirementLabel
    default:
      return ''
  }
}

function buildReferenceEntries(
  requirement: RetouchRequirement,
  requirementIndex: number,
  refs: RetouchReferenceDisplayItem[],
  zipPathPrefix: string,
): RetouchBatchDownloadPlanEntry[] {
  const out: RetouchBatchDownloadPlanEntry[] = []
  refs.forEach((ref, index) => {
    const zipPath = `${zipPathPrefix}/${RETOUCH_ZIP_REFERENCE_DIR}`
    const assetId = ref.assetId ? Number.parseInt(ref.assetId, 10) : NaN
    if (Number.isFinite(assetId) && assetId > 0) {
      out.push({
        key: `ref-asset-${assetId}-${ref.key}`,
        assetId,
        preferredFilename: retouchBusinessAttachmentFilename(requirement, requirementIndex, ref.fileName),
        zipPath,
      })
      return
    }
    const downloadUrl = String(ref.downloadUrl ?? '').trim()
    if (downloadUrl && ref.fileName) {
      out.push({
        key: `ref-legacy-${ref.key}-${index}`,
        preferredFilename: retouchBusinessAttachmentFilename(requirement, requirementIndex, ref.fileName),
        zipPath,
        downloadUrl,
      })
    }
  })
  return out
}

function buildSourceEntries(
  requirement: RetouchRequirement,
  requirementIndex: number,
  sources: RetouchSourceFileDisplayItem[],
  zipPathPrefix: string,
): RetouchBatchDownloadPlanEntry[] {
  const out: RetouchBatchDownloadPlanEntry[] = []
  sources.forEach((file) => {
    const zipPath = `${zipPathPrefix}/${RETOUCH_ZIP_SOURCE_DIR}`
    const assetId = file.assetId ? Number.parseInt(file.assetId, 10) : NaN
    if (Number.isFinite(assetId) && assetId > 0) {
      out.push({
        key: `source-asset-${assetId}-${file.key}`,
        assetId,
        preferredFilename: retouchBusinessAttachmentFilename(requirement, requirementIndex, file.fileName),
        zipPath,
      })
      return
    }
    const downloadUrl = String(file.downloadUrl ?? '').trim()
    if (downloadUrl && file.fileName) {
      out.push({
        key: `source-legacy-${file.key}`,
        preferredFilename: retouchBusinessAttachmentFilename(requirement, requirementIndex, file.fileName),
        zipPath,
        downloadUrl,
      })
    }
  })
  return out
}

export function buildRetouchBatchDownloadPlan(
  requirements: RetouchRequirement[],
  scope: RetouchBatchDownloadScope,
  requirementIndex?: number,
): RetouchBatchDownloadPlan {
  const entries: RetouchBatchDownloadPlanEntry[] = []
  const indices =
    scope === 'all_attachments'
      ? requirements.map((_, index) => index)
      : requirementIndex != null && requirementIndex >= 0
        ? [requirementIndex]
        : []

  let skippedUnavailableCount = 0

  for (const index of indices) {
    const item = requirements[index]
    if (!item) continue
    const folder = formatRetouchRequirementFolderLabel(index)
    const key = item.id || item.sortOrder
    const refs = retouchRequirementReferenceRefsToDisplayItems(
      item.referenceFileRefs ?? [],
      `req-ref-${key}`,
    )
    const sources = retouchSourceAssetsToDisplayItems(item.sourceAssets ?? [])

    const includeRefs =
      scope === 'all_attachments' || scope === 'requirement_all' || scope === 'requirement_references'
    const includeSources =
      scope === 'all_attachments' || scope === 'requirement_all' || scope === 'requirement_sources'

    if (includeRefs) {
      entries.push(...buildReferenceEntries(item, index, refs, folder))
      skippedUnavailableCount += countSkippedReferenceAttachments(refs)
    }
    if (includeSources) {
      entries.push(...buildSourceEntries(item, index, sources, folder))
      skippedUnavailableCount += countSkippedSourceAttachments(sources)
    }
  }

  let assetIdCount = 0
  let legacyCount = 0
  for (const entry of entries) {
    if (entry.assetId != null && entry.assetId > 0) assetIdCount += 1
    else if (entry.downloadUrl) legacyCount += 1
  }

  return { entries, assetIdCount, legacyCount, skippedUnavailableCount }
}

function countSkippedReferenceAttachments(refs: RetouchReferenceDisplayItem[]): number {
  return refs.filter((ref) => {
    const assetId = ref.assetId ? Number.parseInt(ref.assetId, 10) : NaN
    if (Number.isFinite(assetId) && assetId > 0) return false
    const downloadUrl = String(ref.downloadUrl ?? '').trim()
    return !(downloadUrl && ref.fileName)
  }).length
}

function countSkippedSourceAttachments(sources: RetouchSourceFileDisplayItem[]): number {
  return sources.filter((file) => {
    const assetId = file.assetId ? Number.parseInt(file.assetId, 10) : NaN
    if (Number.isFinite(assetId) && assetId > 0) return false
    const downloadUrl = String(file.downloadUrl ?? '').trim()
    return !(downloadUrl && file.fileName)
  }).length
}

export function validateRetouchBatchDownloadPlan(plan: RetouchBatchDownloadPlan): RetouchBatchDownloadValidation {
  const totalEntryCount = plan.entries.length
  if (totalEntryCount === 0) {
    return {
      ok: false,
      message: '没有可下载的附件',
      assetIdCount: 0,
      totalEntryCount: 0,
    }
  }
  if (plan.assetIdCount > MAX_RETouch_BATCH_DOWNLOAD_ASSETS) {
    return {
      ok: false,
      message: `当前批量下载最多支持 ${MAX_RETouch_BATCH_DOWNLOAD_ASSETS} 个附件，请分需求下载。`,
      assetIdCount: plan.assetIdCount,
      totalEntryCount,
    }
  }
  return {
    ok: true,
    assetIdCount: plan.assetIdCount,
    totalEntryCount,
  }
}

export function countRetouchDownloadableAttachments(requirements: RetouchRequirement[]): number {
  const plan = buildRetouchBatchDownloadPlan(requirements, 'all_attachments')
  return plan.entries.length
}

function formatServerBatchFailure(item: AssetBatchDownloadFailure): string {
  return [
    `asset_id=${item.asset_id}`,
    item.task_id != null ? `task_id=${item.task_id}` : '',
    item.filename ? `filename=${item.filename}` : '',
    `reason=${item.reason || 'unavailable'}`,
  ]
    .filter(Boolean)
    .join(' ')
}

function manifestItemsToZipSources(
  manifestItems: AssetBatchDownloadItem[],
  plan: RetouchBatchDownloadPlan,
): BatchZipDownloadSource[] {
  const pathByAssetId = new Map<number, RetouchBatchDownloadPlanEntry>()
  for (const entry of plan.entries) {
    if (entry.assetId != null && entry.assetId > 0) {
      pathByAssetId.set(entry.assetId, entry)
    }
  }
  return manifestItems.map((item) => {
    const planned = pathByAssetId.get(item.asset_id)
    return {
      key: `asset-${item.asset_id}`,
      filename: planned?.preferredFilename || item.filename,
      zipPath: planned?.zipPath,
      downloadURL: item.download_url,
      fallbackName: planned?.preferredFilename || `asset-${item.asset_id}`,
      failureHint: `asset_id=${item.asset_id} filename=${item.filename || planned?.preferredFilename} reason=fetch_failed`,
    }
  })
}

function legacyEntriesToZipSources(entries: RetouchBatchDownloadPlanEntry[]): BatchZipDownloadSource[] {
  return entries
    .filter((entry) => !entry.assetId && entry.downloadUrl)
    .map((entry) => ({
      key: entry.key,
      filename: entry.preferredFilename,
      zipPath: entry.zipPath,
      downloadURL: entry.downloadUrl,
      fallbackName: entry.preferredFilename,
      failureHint: `${entry.key} filename=${entry.preferredFilename} reason=legacy_fetch_failed`,
    }))
}

export interface RetouchBatchDownloadRunResult {
  ok: boolean
  message?: string
  writtenCount?: number
  failureCount?: number
  skippedLegacyHint?: string
}

export async function runRetouchBatchDownload(
  plan: RetouchBatchDownloadPlan,
  zipPrefix: string,
  onStatus?: (message: string) => void,
): Promise<RetouchBatchDownloadRunResult> {
  const validation = validateRetouchBatchDownloadPlan(plan)
  if (!validation.ok) {
    return { ok: false, message: validation.message }
  }

  const assetIds = Array.from(
    new Set(
      plan.entries
        .map((entry) => entry.assetId)
        .filter((id): id is number => id != null && id > 0),
    ),
  )

  const legacySources = legacyEntriesToZipSources(plan.entries)
  let manifestZipItems: BatchZipDownloadSource[] = []
  let serverFailures: string[] = []

  if (assetIds.length > 0) {
    try {
      const res = await assetsApi.batchDownload(assetIds, { namingMode: 'business' })
      const manifest = res.data?.data
      const items = Array.isArray(manifest?.items) ? manifest.items : []
      if (!items.length && legacySources.length === 0) {
        return { ok: false, message: '没有可下载的附件' }
      }
      manifestZipItems = manifestItemsToZipSources(items, plan)
      serverFailures = (Array.isArray(manifest?.failures) ? manifest.failures : []).map(
        formatServerBatchFailure,
      )
    } catch (err) {
      return {
        ok: false,
        message: resolveApiUserMessage(err, { fallback: '批量下载失败，请稍后重试' }),
      }
    }
  }

  const zipItems = [...manifestZipItems, ...legacySources]
  if (!zipItems.length) {
    return { ok: false, message: '没有可下载的附件' }
  }

  try {
    const result = await downloadBatchAsZip({
      items: zipItems,
      zipFilename: buildTimestampedZipFilename(zipPrefix),
      serverFailures,
      onStatus,
    })
    const hints: string[] = []
    if (result.failureCount > 0) {
      hints.push(`${result.failureCount} 个文件未打包，详情见 ZIP 内 download_errors.txt`)
    }
    if (plan.skippedUnavailableCount > 0) {
      hints.push(
        `有 ${plan.skippedUnavailableCount} 个旧附件无法批量下载，请单独下载`,
      )
    } else if (plan.legacyCount > 0) {
      hints.push('部分旧附件无资产编号，已尝试直链打包；若失败请单独下载')
    }
    return {
      ok: true,
      writtenCount: result.writtenCount,
      failureCount: result.failureCount,
      message: hints.length ? hints.join('；') : undefined,
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : '批量下载失败'
    return { ok: false, message: msg }
  }
}

/** Collect asset ids for a requirement (testing / diagnostics). */
export function collectRetouchRequirementAssetIdsForScope(
  requirement: RetouchRequirement,
  scope: 'references' | 'sources' | 'all',
): { referenceAssetIds: number[]; sourceAssetIds: number[] } {
  const ids = collectRetouchRequirementBatchAssetIds(
    requirement.referenceFileRefs,
    requirement.sourceAssets,
  )
  if (scope === 'references') return { referenceAssetIds: ids.referenceAssetIds, sourceAssetIds: [] }
  if (scope === 'sources') return { referenceAssetIds: [], sourceAssetIds: ids.sourceAssetIds }
  return ids
}
