import type { TaskAssetVersion } from '@/domain/types/task'
import { versionTotalFileCount } from '@/utils/task-ui-labels'

function normalizedAssetKind(v: TaskAssetVersion): string {
  return String(v.assetKind ?? '').trim().toLowerCase()
}

function hasUsableFile(v: TaskAssetVersion): boolean {
  const hasPreviewable = Array.isArray(v.fileRefs) && v.fileRefs.some((u) => Boolean(u?.trim()))
  if (hasPreviewable) return true
  return Boolean(v.nonPreviewFiles?.some((item) => Boolean(item.url?.trim())))
}

export function isTaskAssetVersionUnavailable(v: TaskAssetVersion): boolean {
  if (versionTotalFileCount(v) <= 0) return true
  return !hasUsableFile(v)
}

function parseUploadedAtScore(value: string | undefined): number {
  const text = String(value ?? '').trim()
  if (!text) return 0
  const normalized = /^\d{4}-\d{2}-\d{2}\s+\d{2}:/.test(text)
    ? text.replace(/\s+/, 'T')
    : text
  const t = Date.parse(normalized)
  return Number.isFinite(t) ? t : 0
}

function numericIdScore(id: string): number {
  const direct = Number(id)
  if (Number.isFinite(direct)) return direct
  const match = String(id).match(/(\d+)(?!.*\d)/)
  return match ? Number(match[1]) : 0
}

function compareVersionRecency(
  a: { version: TaskAssetVersion; index: number },
  b: { version: TaskAssetVersion; index: number },
): number {
  const at = parseUploadedAtScore(a.version.uploadedAt)
  const bt = parseUploadedAtScore(b.version.uploadedAt)
  if (at !== bt) return at - bt

  const av = a.version.rootVersionNo ?? 0
  const bv = b.version.rootVersionNo ?? 0
  if (av !== bv) return av - bv

  const aid = numericIdScore(a.version.id)
  const bid = numericIdScore(b.version.id)
  if (aid !== bid) return aid - bid

  return a.index - b.index
}

function latestIndex(items: Array<{ version: TaskAssetVersion; index: number }>): number {
  if (!items.length) return -1
  let picked = items[0]!
  for (const item of items.slice(1)) {
    if (compareVersionRecency(item, picked) > 0) picked = item
  }
  return picked.index
}

export function preferredTaskAssetVersionIndex(
  versions: TaskAssetVersion[],
  isUnavailable: (version: TaskAssetVersion) => boolean = isTaskAssetVersionUnavailable,
): number {
  if (!versions.length) return -1
  const available = versions
    .map((version, index) => ({ version, index }))
    .filter((item) => !isUnavailable(item.version))
  if (!available.length) return -1

  const delivery = available.filter((item) => normalizedAssetKind(item.version) === 'delivery')
  return latestIndex(delivery.length ? delivery : available)
}
