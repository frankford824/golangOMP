import type { SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'

export function matchesClientMaterialQuery(asset: SystemAssetRow, rawQuery: string): boolean {
  const query = rawQuery.trim().toLowerCase()
  if (!query) return true

  return [
    asset.product_name,
    asset.asset_no,
    asset.original_filename,
    asset.file_name,
    asset.resource_id,
    asset.scope_sku_code,
    asset.sku_code,
    asset.primary_sku_code,
    asset.task_no,
    asset.source_label,
    asset.origin_path,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
    .includes(query)
}
