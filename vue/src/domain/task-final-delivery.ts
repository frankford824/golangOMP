import type { BatchObjectSelection } from '@/domain/task-batch-assets'
import { assetVersionMatchesActiveSku, taskHasSkuItemsForBatchUi } from '@/domain/task-batch-assets'
import type { Task, TaskAssetVersion } from '@/domain/types/task'

const DELIVERY_BATCH_WINDOW_MS = 15 * 60 * 1000

function hasDisplayableFile(version: TaskAssetVersion): boolean {
  return (version.fileRefs?.length ?? 0) > 0 || (version.nonPreviewFiles?.length ?? 0) > 0
}

export function isDeliveryAssetVersion(version: TaskAssetVersion): boolean {
  return String(version.assetKind ?? '').trim().toLowerCase() === 'delivery'
}

export function latestDeliveryVersionForSelection(
  task: Task,
  selection: BatchObjectSelection = { kind: 'single' },
): TaskAssetVersion | null {
  return latestDeliveryBatchVersionsForSelection(task, selection)[0] ?? null
}

function versionSortScore(version: TaskAssetVersion): number {
  const t = Date.parse(String(version.uploadedAt ?? ''))
  if (Number.isFinite(t)) return t
  const n = Number(version.id)
  if (Number.isFinite(n)) return n
  return 0
}

function versionRootKey(version: TaskAssetVersion): string {
  return (
    String(version.assetRootId ?? '').trim() ||
    String(version.assetNo ?? '').trim() ||
    `id:${version.id}`
  )
}

function compareByNewest(a: TaskAssetVersion, b: TaskAssetVersion): number {
  const dt = versionSortScore(b) - versionSortScore(a)
  if (dt !== 0) return dt
  const av = a.rootVersionNo ?? 0
  const bv = b.rootVersionNo ?? 0
  if (av !== bv) return bv - av
  return String(b.id).localeCompare(String(a.id))
}

function isInLatestBatch(version: TaskAssetVersion, latestScore: number): boolean {
  if (latestScore <= 0) return false
  const score = versionSortScore(version)
  if (score <= 0) return false
  return latestScore - score <= DELIVERY_BATCH_WINDOW_MS
}

export function latestDeliveryBatchVersionsForSelection(
  task: Task,
  selection: BatchObjectSelection = { kind: 'single' },
): TaskAssetVersion[] {
  const versions = task.assetVersions ?? []
  const candidates = versions.filter((version) => {
    if (!isDeliveryAssetVersion(version) || !hasDisplayableFile(version)) return false
    if (!taskHasSkuItemsForBatchUi(task)) return true
    return assetVersionMatchesActiveSku(version, selection, task)
  })
  if (!candidates.length) return []

  // Per asset root keep only the latest upload, then aggregate the latest submission batch.
  const latestPerRoot = new Map<string, TaskAssetVersion>()
  for (const version of candidates) {
    const key = versionRootKey(version)
    const prev = latestPerRoot.get(key)
    if (!prev || compareByNewest(version, prev) < 0) {
      latestPerRoot.set(key, version)
    }
  }
  const collapsed = [...latestPerRoot.values()].sort(compareByNewest)
  const latest = collapsed[0]
  if (!latest) return []
  const latestScore = versionSortScore(latest)
  if (latestScore <= 0) return [latest]

  const batch = collapsed.filter((version) => isInLatestBatch(version, latestScore))
  return (batch.length ? batch : [latest]).sort((a, b) => versionSortScore(b) - versionSortScore(a))
}
