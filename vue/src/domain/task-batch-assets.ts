import type { BackendAsset } from '@/services/apiTypes'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import type { Task, TaskAssetVersion, TaskSkuItem } from '@/domain/types/task'
import { buildParallelProductRows } from '@/domain/task-parallel-products'

/**
 * 并列商品与资产/参考图作用域：
 * - single：无 sku_items 的整任务视角
 * - product：sku_items 下标，与并列 Tab「商品 1…」一致
 */
export type BatchObjectSelection = { kind: 'single' } | { kind: 'product'; productIndex: number }

function skuFromRecord(o: Record<string, unknown>): string | undefined {
  const v =
    o.scope_sku_code ??
    o.scopeSkuCode ??
    o.sku_code ??
    o.skuCode ??
    o.target_sku_code ??
    o.related_sku_code ??
    o.batch_sku_code ??
    o.item_sku_code
  if (typeof v === 'string' && v.trim()) return v.trim()
  return undefined
}

export function taskHasSkuItemsForBatchUi(task: Task): boolean {
  const itemCount = task.skuItems?.length ?? 0
  const mode = (task.batchMode ?? '').toLowerCase()
  if (itemCount > 1) return true
  // 已标批量但详情尚未带 sku_items：勿走「多商品 Tab」逻辑，避免 task.skuItems 为 undefined 时读 .length
  if (itemCount === 0 && (task.isBatchTask === true || mode === 'multiple')) return false
  return task.isBatchTask === true || mode === 'multiple'
}

export function primarySkuCodesForTask(task: Task): string[] {
  const out: string[] = []
  const p = task.primarySkuCode?.trim()
  if (p) out.push(p)
  const t = task.sku?.trim()
  if (t && !out.includes(t)) out.push(t)
  return out
}

export function subSkuItem(task: Task, subIndex: number): TaskSkuItem | undefined {
  const items = task.skuItems
  if (!items?.length || subIndex < 0 || subIndex >= items.length) return undefined
  return items[subIndex]
}

export function activeSkuCodeForSelection(task: Task, sel: BatchObjectSelection): string | null {
  if (!taskHasSkuItemsForBatchUi(task)) {
    const single = task.sku?.trim()
    return single || null
  }
  if (sel.kind === 'single') {
    const single = task.sku?.trim()
    return single || null
  }
  const code = task.skuItems?.[sel.productIndex]?.skuCode?.trim()
  return code || null
}

/**
 * @param parallelRowIndex 与 `buildParallelProductRows(task)[i].index` 一致
 */
export function selectionFromProductIndex(task: Task, parallelRowIndex: number): BatchObjectSelection {
  if (!taskHasSkuItemsForBatchUi(task)) return { kind: 'single' }
  const n = task.skuItems?.length ?? 0
  if (n < 1) return { kind: 'single' }
  return {
    kind: 'product',
    productIndex: Math.min(Math.max(0, parallelRowIndex), n - 1),
  }
}

export function parallelProductTabCount(task: Task): number {
  return buildParallelProductRows(task).length
}

export function backendAssetMatchesObject(
  asset: BackendAsset,
  sel: BatchObjectSelection,
  task: Task,
): boolean {
  if (!taskHasSkuItemsForBatchUi(task)) return true
  const ar = asset as Record<string, unknown>
  const assetSku = skuFromRecord(ar)

  if (sel.kind === 'single') {
    if (!assetSku) return true
    return primarySkuCodesForTask(task).includes(assetSku)
  }

  /** 无 SKU 作用域的资产为共享，不并入某一商品 Tab */
  if (!assetSku) {
    return false
  }
  const itemSku = task.skuItems?.[sel.productIndex]?.skuCode?.trim()
  return !!(itemSku && itemSku === assetSku)
}

/** 批量任务下后端资产行未带 SKU 时视为共享（参考图/交付/源文件列表分区用） */
export function backendAssetIsSharedForBatch(asset: BackendAsset, task: Task): boolean {
  if (!taskHasSkuItemsForBatchUi(task)) return false
  const ar = asset as Record<string, unknown>
  return !skuFromRecord(ar)
}

export function assetVersionMatchesObject(
  v: TaskAssetVersion,
  sel: BatchObjectSelection,
  task: Task,
): boolean {
  if (!taskHasSkuItemsForBatchUi(task)) return true
  const scope = v.scopeSkuCode?.trim()
  if (sel.kind === 'single') {
    if (!scope) return true
    return primarySkuCodesForTask(task).includes(scope)
  }
  /** 无 scope 的版本为共享，不归入某一商品 Tab */
  if (!scope) {
    return false
  }
  const itemSku = task.skuItems?.[sel.productIndex]?.skuCode?.trim()
  return !!(itemSku && itemSku === scope)
}

/** 严格 SKU 过滤：只命中 scope_sku_code==activeSkuCode，不包含空 scope。 */
export function assetVersionMatchesActiveSku(
  v: TaskAssetVersion,
  sel: BatchObjectSelection,
  task: Task,
): boolean {
  if (!taskHasSkuItemsForBatchUi(task)) return true
  const scope = v.scopeSkuCode?.trim()
  if (!scope) return false
  const activeSku = activeSkuCodeForSelection(task, sel)
  return !!(activeSku && activeSku === scope)
}

export function assetVersionIsSharedForBatch(v: TaskAssetVersion, task: Task): boolean {
  if (!taskHasSkuItemsForBatchUi(task)) return false
  return !v.scopeSkuCode?.trim()
}

export function referenceRefsForObject(task: Task, sel: BatchObjectSelection): ReferenceFileRef[] {
  const taskRefs = task.referenceFileRefs ?? []
  if (!taskHasSkuItemsForBatchUi(task)) {
    return [...taskRefs]
  }
  if (sel.kind === 'single') return []
  const refs = task.skuItems?.[sel.productIndex]?.referenceFileRefs ?? []
  return [...refs]
}

/** @deprecated Use referenceRefsForObject for expiry-aware callers */
export function referenceUrlsForObject(task: Task, sel: BatchObjectSelection): string[] {
  return referenceRefsForObject(task, sel)
    .map((r) => r.download_url)
    .filter((u): u is string => !!u)
}

/**
 * 批量任务：任务顶层 `reference_file_refs` 仅作共享/汇总展示，不当作每个 SKU Tab 的专属列表。
 */
export function taskLevelReferenceSummaryRefs(task: Task): ReferenceFileRef[] {
  if (!taskHasSkuItemsForBatchUi(task)) return []
  return [...(task.referenceFileRefs ?? [])]
}

/** @deprecated Use taskLevelReferenceSummaryRefs for expiry-aware callers */
export function taskLevelReferenceSummaryUrls(task: Task): string[] {
  return taskLevelReferenceSummaryRefs(task)
    .map((r) => r.download_url)
    .filter((u): u is string => !!u)
}

export function objectSwitcherLabel(sel: BatchObjectSelection, task: Task): string {
  if (!taskHasSkuItemsForBatchUi(task)) return '商品 1'
  if (sel.kind === 'single') return '商品 1'
  return `商品 ${sel.productIndex + 1}`
}

export function designUploadCaption(sel: BatchObjectSelection, task: Task): string {
  if (!taskHasSkuItemsForBatchUi(task)) return '上传设计稿'
  if (sel.kind === 'single') return '上传设计稿'
  return `上传商品 ${sel.productIndex + 1} 设计稿`
}

export function deliveryRemarkSuffix(sel: BatchObjectSelection, task: Task): string {
  if (!taskHasSkuItemsForBatchUi(task)) return ''
  if (sel.kind === 'single') return ''
  const item = task.skuItems?.[sel.productIndex]
  const code = item?.skuCode?.trim()
  const labelN = sel.productIndex + 1
  return code ? ` [商品${labelN}:${code}]` : ` [商品${labelN}]`
}
