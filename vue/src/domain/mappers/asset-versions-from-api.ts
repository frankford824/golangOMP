import type { AssetVersionType, TaskAssetVersion } from '../types/task'
import { toRelativeAssetUrl } from '@/utils/url'

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === 'object' ? (v as Record<string, unknown>) : null
}

function pickString(o: Record<string, unknown>, keys: string[]): string | undefined {
  for (const k of keys) {
    const v = o[k]
    if (typeof v === 'string' && v.trim() !== '') return v
  }
  return undefined
}

function pickNumber(o: Record<string, unknown>, keys: string[]): number | undefined {
  for (const k of keys) {
    const v = o[k]
    if (typeof v === 'number' && Number.isFinite(v)) return v
  }
  return undefined
}

function pickBoolean(o: Record<string, unknown>, keys: string[]): boolean | undefined {
  for (const k of keys) {
    const v = o[k]
    if (typeof v === 'boolean') return v
  }
  return undefined
}

/**
 * 用于设计稿版本 img 预览的 URL：
 * 优先 canonical `download_url`，旧字段仅保留为兼容 fallback。
 */
function pickVersionPreviewUrl(o: Record<string, unknown>): string | undefined {
  const tryNorm = (s: string | undefined): string | undefined => {
    if (!s?.trim()) return undefined
    return toRelativeAssetUrl(s) ?? s.trim()
  }

  const canonical = tryNorm(pickString(o, ['download_url', 'downloadUrl']))
  if (canonical) return canonical

  const direct = tryNorm(pickString(o, ['file_url', 'fileUrl', 'url']))
  if (direct) return direct

  const file = asRecord(o.file ?? o.asset_file ?? o.assetFile)
  if (file) {
    const nested = tryNorm(
      pickString(file, ['download_url', 'downloadUrl', 'public_url', 'publicUrl', 'file_url', 'fileUrl', 'url']),
    )
    if (nested) return nested
  }

  const pub = tryNorm(pickString(o, ['public_url', 'publicUrl']))
  if (pub) return pub

  return (
    pickString(o, ['lan_url', 'lanUrl', 'tailscale_url', 'tailscaleUrl'])?.trim() || undefined
  )
}

/** 资产根 id：兼容 JSON 里 `asset_id` 为数字 */
function pickAssetRootIdStr(o: Record<string, unknown>): string | undefined {
  const n = pickNumber(o, ['asset_id', 'assetId'])
  if (n !== undefined && Number.isFinite(n) && n > 0) return String(Math.trunc(n))
  const s = pickString(o, ['asset_id', 'assetId'])?.trim()
  return s || undefined
}

function timelineSortKey(o: Record<string, unknown>): number {
  const t =
    pickString(o, ['uploaded_at', 'uploadedAt', 'created_at', 'createdAt']) ??
    pickString(o, ['updated_at', 'updatedAt'])
  const ms = t ? Date.parse(t) : NaN
  if (Number.isFinite(ms)) return ms
  const id = pickNumber(o, ['id']) ?? 0
  return id
}

const PREVIEW_IMAGE_EXT = /\.(jpe?g|png|webp|gif)$/i
const SYSTEM_DERIVED_PREVIEW_NAME_RE = /^(preview|design-thumb)\.png$/i
const SYSTEM_DERIVED_PREVIEW_PATH_RE = /\/(preview\/preview\.png|design_thumb\/design-thumb\.png)(\?|$)/i

function labelForRow(o: Record<string, unknown>): string {
  return (
    pickString(o, ['original_filename', 'originalFilename', 'filename', 'name']) ??
    'file'
  )
}

/**
 * 后端会为 PSD 等源文件生成系统衍生预览图（preview/design-thumb）。
 * 这些行不应作为独立“版本记录”展示，否则会出现额外灰色 V2/V3。
 */
function isSystemDerivedPreviewRow(o: Record<string, unknown>): boolean {
  const kind = (pickString(o, ['asset_kind', 'assetKind', 'asset_type', 'assetType']) ?? '').toLowerCase()
  const role = (pickString(o, ['current_version_role', 'currentVersionRole']) ?? '').toLowerCase()
  const versionType =
    (pickString(o, ['version_type', 'versionType', 'asset_version_type', 'assetVersionType', 'type']) ?? '')
      .toLowerCase()
  const fileName =
    pickString(o, ['original_filename', 'originalFilename', 'filename', 'name'])?.trim() ?? ''
  const previewUrl =
    pickVersionPreviewUrl(o) ??
    pickString(o, ['download_url', 'downloadUrl', 'file_url', 'fileUrl', 'url']) ??
    ''

  if (previewUrl && SYSTEM_DERIVED_PREVIEW_PATH_RE.test(previewUrl)) return true

  if (SYSTEM_DERIVED_PREVIEW_NAME_RE.test(fileName)) {
    if (
      versionType === 'derivative' ||
      kind.includes('preview') ||
      kind.includes('thumb') ||
      role.includes('preview') ||
      role.includes('thumb')
    ) {
      return true
    }
  }

  return false
}

function uploaderIdString(o: Record<string, unknown>): string {
  const n = pickNumber(o, ['uploaded_by', 'uploadedBy'])
  if (n !== undefined) return String(n)
  const s = pickString(o, ['uploaded_by', 'uploadedBy'])
  return s ?? ''
}

/**
 * 将后端扁平 asset_versions / design_assets 内嵌版本映射为前端 AssetVersionType。
 * 交付流：asset_type=delivery 且 warehouse_ready / approved_for_flow 为定稿（仓库/审核可读）。
 */
function inferAssetVersionType(o: Record<string, unknown>): AssetVersionType {
  const fromApi =
    pickString(o, ['version_type', 'versionType', 'asset_version_type', 'assetVersionType']) ??
    pickString(o, ['type'])
  if (fromApi) {
    const t = fromApi.toLowerCase()
    if (t === 'final') return 'final'
    if (t === 'derivative') return 'derivative'
    if (t === 'reference') return 'reference'
    if (t === 'revision') return 'revision'
    if (t === 'draft') return 'draft'
  }

  const role = (pickString(o, ['current_version_role', 'currentVersionRole']) ?? '').toLowerCase()
  const assetType = (pickString(o, ['asset_type', 'assetType']) ?? '').toLowerCase()
  const isDeliveryFile = pickBoolean(o, ['is_delivery_file', 'isDeliveryFile']) === true
  const warehouseReady = pickBoolean(o, ['warehouse_ready', 'warehouseReady']) === true
  const approvedForFlow = pickBoolean(o, ['approved_for_flow', 'approvedForFlow']) === true

  const isDelivery = assetType === 'delivery' || isDeliveryFile
  if (isDelivery && (warehouseReady || approvedForFlow)) return 'final'

  if (
    isDelivery &&
    (role.includes('warehouse_ready') ||
      role.includes('approved') ||
      role === 'current_warehouse_ready_version')
  ) {
    return 'final'
  }

  return 'draft'
}

/** 一条 API 行 → 一个 TaskAssetVersion（不合并 upload_session / 批次） */
function mapAssetVersionRow(o: Record<string, unknown>): TaskAssetVersion | null {
  if (isSystemDerivedPreviewRow(o)) return null

  const idRaw = pickNumber(o, ['id'])
  const id = idRaw !== undefined ? String(idRaw) : pickString(o, ['id']) ?? '0'
  const assetKind = pickString(o, ['asset_kind', 'assetKind', 'asset_type', 'assetType'])?.trim()
  const assetRootId =
    pickAssetRootIdStr(o) ??
    pickString(o, ['asset_version_asset_id', 'assetVersionAssetId'])?.trim()
  const assetNo =
    pickString(o, ['asset_no', 'assetNo'])?.trim() ??
    pickString(o, ['asset_version_asset_no', 'assetVersionAssetNo'])?.trim()
  const rootVersionNo = pickNumber(o, ['version_no', 'versionNo'])

  const uploaderId = uploaderIdString(o)
  const uploaderName =
    pickString(o, ['uploader_name', 'uploaderName', 'uploaded_by_name', 'uploadedByName']) ?? '—'

  const uploadedAt =
    pickString(o, ['uploaded_at', 'uploadedAt', 'created_at', 'createdAt']) ?? ''

  const note = pickString(o, ['note', 'comment', 'description'])
  const previewAvailable = pickBoolean(o, ['preview_available', 'previewAvailable'])
  const scopeSkuCode = pickString(o, [
    'scope_sku_code',
    'scopeSkuCode',
    'sku_code',
    'skuCode',
    'target_sku_code',
    'related_sku_code',
    'batch_sku_code',
    'item_sku_code',
  ])

  const fileRefsRaw = o.fileRefs ?? o.file_refs
  if (Array.isArray(fileRefsRaw)) {
    const urls = fileRefsRaw
      .map((x) => (typeof x === 'string' ? x.trim() : ''))
      .filter((x) => x.length > 0)
    const fileRefs: string[] = []
    const nonPreview: Array<{ label: string; url?: string }> = []
    for (let i = 0; i < urls.length; i++) {
      const u = urls[i]!
      const lower = u.toLowerCase()
      const looksPsd = lower.includes('.psd') && !PREVIEW_IMAGE_EXT.test(u)
      if (looksPsd) {
        nonPreview.push({ label: `file_${i + 1}.psd`, url: u })
      } else {
        fileRefs.push(toRelativeAssetUrl(u) ?? u)
      }
    }
    const totalFileCount = fileRefs.length + nonPreview.length
    if (totalFileCount === 0) return null
    return {
      id,
      type: inferAssetVersionType(o),
      ...(assetKind ? { assetKind } : {}),
      uploaderId,
      uploaderName,
      uploadedAt,
      note,
      ...(assetRootId ? { assetRootId } : {}),
      ...(assetNo ? { assetNo } : {}),
      ...(typeof rootVersionNo === 'number' ? { rootVersionNo } : {}),
      ...(scopeSkuCode ? { scopeSkuCode } : {}),
      fileRefs,
      ...(typeof previewAvailable === 'boolean' ? { previewAvailable } : {}),
      nonPreviewFiles: nonPreview.length ? nonPreview : undefined,
      totalFileCount: totalFileCount > 0 ? totalFileCount : undefined,
    }
  }

  const url = pickVersionPreviewUrl(o)
  const label = labelForRow(o)

  const fileRefs: string[] = []
  const nonPreviewFiles: Array<{ label: string; url?: string }> = []

  // Detail contract: image preview is available only when preview_available=true and download_url exists.
  if (previewAvailable === true && url) {
    fileRefs.push(url)
  } else if (url) {
    nonPreviewFiles.push({ label, url: url || undefined })
  }

  const totalFileCount = fileRefs.length + nonPreviewFiles.length
  if (totalFileCount === 0) return null

  return {
    id,
    type: inferAssetVersionType(o),
    ...(assetKind ? { assetKind } : {}),
    uploaderId,
    uploaderName,
    uploadedAt,
    note,
    ...(assetRootId ? { assetRootId } : {}),
    ...(assetNo ? { assetNo } : {}),
    ...(typeof rootVersionNo === 'number' ? { rootVersionNo } : {}),
    ...(scopeSkuCode ? { scopeSkuCode } : {}),
    fileRefs,
    ...(typeof previewAvailable === 'boolean' ? { previewAvailable } : {}),
    nonPreviewFiles: nonPreviewFiles.length ? nonPreviewFiles : undefined,
    totalFileCount,
  }
}

/**
 * 按资产根分组 + 根内按 `version_no` 升序的规范化。
 *
 * 与旧实现（全局按上传时间排序）的区别：
 * - 同一 asset_no 下的版本（V1→V2→V3）严格紧邻且按根内版本号排列；
 * - 不同资产根（delivery / source / reference）不再被时间线拉到一起比较；
 * - 根的相对顺序按「首个版本的上传时间」稳定化（近似旧视觉顺序）。
 *
 * @param flatRows 扁平 asset_versions（带 asset_id 以便归并）
 * @param rootOrder 来自 design_assets[] 的根顺序/元信息（asset_id → 排序键 + 基础字段）
 */
function normalizeRowsGroupedByRoot(
  flatRows: Record<string, unknown>[],
  rootOrder?: Array<{ rootKey: string; sortKey: number; basePatch?: Record<string, unknown> }>,
): TaskAssetVersion[] {
  if (flatRows.length === 0) return []

  const groups = new Map<
    string,
    { sortKey: number; basePatch?: Record<string, unknown>; rows: Record<string, unknown>[] }
  >()

  const rootInfoMap = new Map<string, { sortKey: number; basePatch?: Record<string, unknown> }>()
  if (rootOrder) {
    for (const info of rootOrder) rootInfoMap.set(info.rootKey, info)
  }

  for (const row of flatRows) {
    const rootKey =
      pickAssetRootIdStr(row) ??
      pickString(row, ['asset_no', 'assetNo'])?.trim() ??
      `__orphan_${pickNumber(row, ['id']) ?? Math.random()}`

    const g = groups.get(rootKey)
    if (g) {
      g.rows.push(row)
    } else {
      const info = rootInfoMap.get(rootKey)
      groups.set(rootKey, {
        sortKey: info?.sortKey ?? timelineSortKey(row),
        basePatch: info?.basePatch,
        rows: [row],
      })
    }
  }

  // 稳定顺序：先按「根首版本上传时间」，然后按 rootKey，保证确定性
  const orderedGroups = Array.from(groups.entries()).sort((a, b) => {
    const sa = a[1].sortKey
    const sb = b[1].sortKey
    if (sa !== sb) return sa - sb
    return a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0
  })

  const out: TaskAssetVersion[] = []
  for (const [, group] of orderedGroups) {
    const sorted = [...group.rows].sort((a, b) => {
      const na = pickNumber(a, ['version_no', 'versionNo']) ?? 0
      const nb = pickNumber(b, ['version_no', 'versionNo']) ?? 0
      if (na !== nb) return na - nb
      // 同 version_no（不应出现）再退化为时间排序
      return timelineSortKey(a) - timelineSortKey(b)
    })
    for (const o of sorted) {
      const merged = group.basePatch ? { ...group.basePatch, ...o } : o
      const m = mapAssetVersionRow(merged)
      if (m) out.push(m)
    }
  }
  return out
}

/**
 * 从 design_assets[] 抽取「根信息」：asset_id → 排序键(首版本上传时间)、基础字段(asset_no / asset_type...)
 * 这些字段会在扁平版本行缺失时补齐到每一行上，以便 mapAssetVersionRow 能取到 asset_no / asset_type。
 */
function extractRootInfoFromDesignAssets(
  raw: unknown,
): Array<{ rootKey: string; sortKey: number; basePatch?: Record<string, unknown> }> {
  if (!Array.isArray(raw)) return []
  const out: Array<{ rootKey: string; sortKey: number; basePatch?: Record<string, unknown> }> = []
  for (const entry of raw) {
    const o = asRecord(entry)
    if (!o) continue
    const rootKey =
      pickAssetRootIdStr(o) ??
      pickString(o, ['id'])?.trim() ??
      pickString(o, ['asset_no', 'assetNo'])?.trim()
    if (!rootKey) continue

    const sortKey =
      timelineSortKey(
        asRecord(o.current_version ?? o.currentVersion) ??
          asRecord(o.approved_version ?? o.approvedVersion) ??
          asRecord(o.warehouse_ready_version ?? o.warehouseReadyVersion) ??
          o,
      )

    const basePatch: Record<string, unknown> = {}
    const assetNo = pickString(o, ['asset_no', 'assetNo'])?.trim()
    if (assetNo) basePatch.asset_no = assetNo
    const assetType = pickString(o, ['asset_type', 'assetType', 'asset_kind', 'assetKind'])?.trim()
    if (assetType) basePatch.asset_type = assetType
    const rootScope =
      pickString(o, ['scope_sku_code', 'scopeSkuCode']) ??
      pickString(o, ['sku_code', 'skuCode'])
    if (rootScope) basePatch.scope_sku_code = rootScope
    basePatch.asset_version_asset_id = rootKey

    out.push({
      rootKey,
      sortKey,
      basePatch: Object.keys(basePatch).length ? basePatch : undefined,
    })
  }
  return out
}

/**
 * GET /v1/tasks/{id} 除扁平 `asset_versions` 外，可能同时返回按资产根分组的 `design_assets`。
 * 若扁平列表为空，从此处收集版本行再交给 `mapAssetVersionRow`（与 upload-session complete 落库一致）。
 */
function collectVersionRowsFromDesignAssets(raw: unknown): Record<string, unknown>[] {
  if (!Array.isArray(raw)) return []
  const out: Record<string, unknown>[] = []
  for (const entry of raw) {
    const o = asRecord(entry)
    if (!o) continue
    const assetRootId =
      pickAssetRootIdStr(o) ??
      pickString(o, ['id'])?.trim()
    const assetScopeSku =
      pickString(o, ['scope_sku_code', 'scopeSkuCode']) ??
      pickString(o, ['sku_code', 'skuCode'])
    const nested =
      o.versions ??
      o.asset_versions ??
      o.assetVersions ??
      o.version_list ??
      o.versionList
    if (Array.isArray(nested)) {
      for (const v of nested) {
        const vr = asRecord(v)
        if (!vr) continue
        const vScope =
          pickString(vr, [
            'scope_sku_code',
            'scopeSkuCode',
            'sku_code',
            'skuCode',
            'target_sku_code',
          ]) ?? undefined
        const patch: Record<string, unknown> = {}
        if (!vScope && assetScopeSku) patch.scope_sku_code = assetScopeSku
        if (!pickAssetRootIdStr(vr) && assetRootId) patch.asset_version_asset_id = assetRootId
        out.push(Object.keys(patch).length ? { ...vr, ...patch } : vr)
      }
      continue
    }
    const embeddedPreferred =
      asRecord(o.warehouse_ready_version ?? o.warehouseReadyVersion) ??
      asRecord(o.approved_version ?? o.approvedVersion) ??
      asRecord(o.current_version ?? o.currentVersion)
    if (embeddedPreferred) {
      const vScope =
        pickString(embeddedPreferred, [
          'scope_sku_code',
          'scopeSkuCode',
          'sku_code',
          'skuCode',
          'target_sku_code',
        ]) ?? undefined
      const patch: Record<string, unknown> = {}
      if (!vScope && assetScopeSku) patch.scope_sku_code = assetScopeSku
      if (!pickAssetRootIdStr(embeddedPreferred) && assetRootId) {
        patch.asset_version_asset_id = assetRootId
      }
      out.push(Object.keys(patch).length ? { ...embeddedPreferred, ...patch } : embeddedPreferred)
      continue
    }
    if (
      pickString(o, ['uploaded_at', 'uploadedAt', 'created_at', 'createdAt']) != null ||
      pickNumber(o, ['id']) !== undefined ||
      pickVersionPreviewUrl(o) != null
    ) {
      const rootScope =
        pickString(o, [
          'scope_sku_code',
          'scopeSkuCode',
          'sku_code',
          'skuCode',
          'target_sku_code',
        ]) ?? undefined
      if (!rootScope && assetScopeSku) {
        out.push({
          ...o,
          scope_sku_code: assetScopeSku,
          ...(!pickAssetRootIdStr(o) && assetRootId
            ? { asset_version_asset_id: assetRootId }
            : {}),
        })
      } else {
        out.push(
          !pickAssetRootIdStr(o) && assetRootId
            ? { ...o, asset_version_asset_id: assetRootId }
            : o,
        )
      }
    }
  }
  return out
}

/**
 * 设计图读模型（方案 A：按资产根分组）：
 *
 * 优先策略 —— 同时拥有 `design_assets[]`(根) 与 `asset_versions[]`(扁平) 时：
 *   按资产根分组，根内按 `version_no` 升序。V1/V2/V3 是「同一 asset_no 内」的递增，
 *   不再跨根（source / delivery / reference ...）拉到同一时间线。
 *
 * 次要兼容 —— 仅有 `design_assets[]`：把分组下的嵌套 versions/current_version 展开后仍按根分组处理。
 * 最终兜底 —— 旧键 `design_asset_versions[]` 或仅有扁平 `asset_versions[]`（无根信息）：
 *   退化为「根内升序、根间稳定序」——通过 asset_id 做 ad-hoc 分组；缺 asset_id 则按时间排序。
 *
 * 注：`isSystemDerivedPreviewRow` 仍会过滤 preview/design_thumb 派生预览行，避免时间线灰条。
 */
export function normalizeAssetVersionsFromTaskRaw(taskRaw: unknown): TaskAssetVersion[] {
  const root = asRecord(taskRaw)
  if (!root) return []

  const designAssetsRaw = root.design_assets ?? root.designAssets
  const rootInfo = extractRootInfoFromDesignAssets(designAssetsRaw)

  const direct = root.asset_versions ?? root.assetVersions
  if (Array.isArray(direct)) {
    const rows = direct.map(asRecord).filter(Boolean) as Record<string, unknown>[]
    if (rows.length > 0) {
      return normalizeRowsGroupedByRoot(rows, rootInfo.length ? rootInfo : undefined)
    }
  }

  const fromGrouped = collectVersionRowsFromDesignAssets(designAssetsRaw)
  if (fromGrouped.length > 0) {
    return normalizeRowsGroupedByRoot(fromGrouped, rootInfo.length ? rootInfo : undefined)
  }

  const legacy = root.design_asset_versions ?? root.designAssetVersions
  if (!Array.isArray(legacy)) return []

  const rows = legacy.map(asRecord).filter(Boolean) as Record<string, unknown>[]
  return normalizeRowsGroupedByRoot(rows)
}
