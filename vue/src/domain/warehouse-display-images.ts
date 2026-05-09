import type { BackendAsset } from '@/services/apiTypes'

const RASTER_PREVIEW_EXT = /\.(jpe?g|png|webp|gif)$/i
const RASTER_MIME = /^image\/(jpeg|pjpeg|png|gif|webp|bmp)$/i

export interface WarehouseDisplayImageEntry {
  url: string
  assetId?: string
}

function normalizeText(v: unknown): string {
  return typeof v === 'string' ? v.trim() : ''
}

function normalizedTaskType(taskType: unknown): string {
  return normalizeText(taskType).toUpperCase()
}

function normalizedFileRole(fileRole: unknown): string {
  return normalizeText(fileRole).toLowerCase()
}

function resolveAssetRole(asset: BackendAsset): string {
  const rec = asset as Record<string, unknown>
  return normalizedFileRole(rec.file_role ?? rec.asset_kind ?? rec.role)
}

function normalizedScopeSkuCode(value: unknown): string {
  return normalizeText(value).toLowerCase()
}

function normalizedRowSkuCode(value: unknown): string {
  return normalizeText(value).toLowerCase()
}

function isPurchaseTask(taskType: unknown): boolean {
  return normalizedTaskType(taskType) === 'PURCHASE_TASK'
}

function isRasterVersion(version: Record<string, unknown>, url: string): boolean {
  const mime = normalizeText(version.mime_type ?? version.mimeType).toLowerCase()
  const fileName = normalizeText(version.file_name ?? version.fileName ?? version.original_filename)
  if (mime && RASTER_MIME.test(mime)) return true
  if (RASTER_PREVIEW_EXT.test(fileName)) return true
  return RASTER_PREVIEW_EXT.test(url)
}

function versionLooksDelivery(version: Record<string, unknown>): boolean {
  const kind = normalizedFileRole(
    version.file_role ??
      version.asset_type ??
      version.asset_kind ??
      version.role,
  )
  const isDeliveryFlag = version.is_delivery_file === true || version.isDeliveryFile === true
  return isDeliveryFlag || kind === 'delivery' || kind === 'final'
}

function pushVersionImageUrls(asset: BackendAsset, out: WarehouseDisplayImageEntry[]) {
  const assetId = normalizeText(asset.id) || undefined
  for (const version of asset.versions ?? []) {
    const url = normalizeText(version.download_url)
    if (!url) continue
    const vr = version as Record<string, unknown>
    if (!isRasterVersion(vr, url)) continue
    out.push({ url, assetId })
  }
}

/**
 * 仓库执行节点图片规则（改为资产主链）：
 * - 非采购：展示当前商品维度的定稿交付（delivery/final + scope_sku_code 对齐）
 * - 采购：仅展示当前商品维度参考图（reference + scope_sku_code 对齐）
 * - 无图：返回空数组（由调用方隐藏或空态）
 */
export function warehouseDisplayImageEntries(
  taskType: string | null | undefined,
  currentSkuCode: string | null | undefined,
  assets: BackendAsset[],
): WarehouseDisplayImageEntry[] {
  const sku = normalizedRowSkuCode(currentSkuCode)
  const purchase = isPurchaseTask(taskType)
  const out: WarehouseDisplayImageEntry[] = []
  for (const asset of assets) {
    const role = resolveAssetRole(asset)
    const hasDeliveryVersion = (asset.versions ?? []).some((v) =>
      versionLooksDelivery(v as Record<string, unknown>),
    )
    const roleMatches = purchase
      ? role === 'reference'
      : role === 'delivery' || role === 'final' || hasDeliveryVersion
    if (!roleMatches) continue
    const scopeSkuCode = normalizedScopeSkuCode((asset as Record<string, unknown>).scope_sku_code)
    if (sku) {
      if (scopeSkuCode && scopeSkuCode !== sku) continue
    } else if (scopeSkuCode) {
      continue
    }
    pushVersionImageUrls(asset, out)
  }
  return out
}

/**
 * 非采购任务：多 SKU 下无 scope_sku_code 的共享定稿图。
 */
export function warehouseSharedFinalImageEntries(
  taskType: string | null | undefined,
  assets: BackendAsset[],
): WarehouseDisplayImageEntry[] {
  if (isPurchaseTask(taskType)) return []
  const out: WarehouseDisplayImageEntry[] = []
  for (const asset of assets) {
    const role = resolveAssetRole(asset)
    const hasDeliveryVersion = (asset.versions ?? []).some((v) =>
      versionLooksDelivery(v as Record<string, unknown>),
    )
    if (role !== 'delivery' && role !== 'final' && !hasDeliveryVersion) continue
    const scopeSkuCode = normalizedScopeSkuCode((asset as Record<string, unknown>).scope_sku_code)
    if (scopeSkuCode) continue
    pushVersionImageUrls(asset, out)
  }
  return out
}
