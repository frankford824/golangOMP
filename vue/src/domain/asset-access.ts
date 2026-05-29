/**
 * 资产预览/下载元数据（GET /v1/assets/{id}/preview | /download）与 URL→资产根 id 辅助。
 * canonical 主链仅消费后端返回的 `download_url` / `download_mode`。
 */
import axios from 'axios'
import type { BackendAsset, BackendAssetVersion } from '@/services/apiTypes'
import type { AssetDownloadMeta } from '@/services/api/assetsApi'
import { assetsApi } from '@/services/api/assetsApi'
import { toRelativeAssetUrl } from '@/utils/url'

export type AssetPreviewMetaStatus = 'ok' | 'preparing' | 'not_found' | 'unavailable' | 'error'

export interface AssetPreviewMetaResult {
  status: AssetPreviewMetaStatus
  /** 用于 <img src> 的地址（已相对化） */
  displayUrl?: string
  downloadUrl?: string
  message?: string
}

export type AssetDownloadMetaStatus = 'ok' | 'preparing' | 'not_found' | 'forbidden' | 'error'

export interface AssetDownloadMetaResult {
  status: AssetDownloadMetaStatus
  downloadUrl?: string
  /** From GET /v1/assets/{id}/download (`ResolveAssetDownloadFilename` on backend). */
  filename?: string
  message?: string
}

/** 会话级短缓存：避免 /preview 回落 /download 时重复打下载元数据 */
const DOWNLOAD_META_TTL_MS = 60_000
const downloadMetaCache = new Map<string, { result: AssetDownloadMetaResult; expiresAt: number }>()
const downloadMetaInflight = new Map<string, Promise<AssetDownloadMetaResult>>()

const PREVIEW_META_TTL_MS = 60_000
const previewMetaCache = new Map<string, { result: AssetPreviewMetaResult; expiresAt: number }>()
const previewMetaInflight = new Map<string, Promise<AssetPreviewMetaResult>>()

function readPreviewMetaCache(id: string): AssetPreviewMetaResult | undefined {
  const hit = previewMetaCache.get(id)
  if (!hit || hit.expiresAt <= Date.now()) {
    if (hit) previewMetaCache.delete(id)
    return undefined
  }
  return hit.result
}

function writePreviewMetaCache(id: string, result: AssetPreviewMetaResult) {
  if (result.status !== 'ok' || !result.displayUrl) return
  previewMetaCache.set(id, { result, expiresAt: Date.now() + PREVIEW_META_TTL_MS })
}

function readDownloadCache(id: string): AssetDownloadMetaResult | undefined {
  const hit = downloadMetaCache.get(id)
  if (!hit || hit.expiresAt <= Date.now()) {
    if (hit) downloadMetaCache.delete(id)
    return undefined
  }
  return hit.result
}

function writeDownloadCache(id: string, result: AssetDownloadMetaResult) {
  if (result.status !== 'ok' || !result.downloadUrl) return
  downloadMetaCache.set(id, { result, expiresAt: Date.now() + DOWNLOAD_META_TTL_MS })
}

/**
 * 在已通过 `GET /v1/assets/{id}/download`（或其它同源载荷）拿到下载地址时写入缓存，
 * 供 `fetchAssetPreviewMeta` 回落路径复用，避免重复请求。
 */
export function primeAssetDownloadMetaCache(assetId: string, responseBody: unknown): void {
  const id = assetId.trim()
  if (!id) return
  const meta = unwrapDownloadPayload(responseBody)
  const downloadUrl = normalizeDisplayUrl(pickMetaUrl(meta))
  if (!downloadUrl) return
  writeDownloadCache(id, { status: 'ok', downloadUrl })
}

function unwrapDownloadPayload(body: unknown): AssetDownloadMeta | undefined {
  const root = body && typeof body === 'object' ? (body as Record<string, unknown>) : {}
  const data = root.data
  if (data && typeof data === 'object') return data as AssetDownloadMeta
  if (Object.keys(root).length > 0) return root as AssetDownloadMeta
  return undefined
}

function normalizeDisplayUrl(raw: string | null | undefined): string | undefined {
  if (!raw?.trim()) return undefined
  return toRelativeAssetUrl(raw.trim()) ?? raw.trim()
}

export function pickDownloadFilenameFromMeta(meta: AssetDownloadMeta | undefined): string {
  if (!meta) return ''
  const raw = meta as Record<string, unknown>
  for (const key of ['filename', 'file_name', 'original_filename', 'originalFilename'] as const) {
    const value = raw[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function pickMetaUrl(meta: AssetDownloadMeta | undefined): string | undefined {
  if (!meta) return undefined
  const raw = meta as Record<string, unknown>
  const candidates = [
    raw.download_url,
    raw.downloadUrl,
    raw.preview_url,
    raw.previewUrl,
    raw.file_url,
    raw.fileUrl,
    raw.url,
  ]
  for (const item of candidates) {
    if (typeof item === 'string' && item.trim()) return item.trim()
  }
  return undefined
}

/**
 * GET /v1/assets/{id}/preview
 * - 200：返回展示 URL（优先 download_url）
 * - 404：资产不存在
 * - 403：无权限
 * - 409：当前不可预览（如 source/PSD 无衍生预览时）；不应与 404「资源不存在」混淆，通常仍可下载原文件
 */
export async function fetchAssetPreviewMeta(
  assetId: string,
  signal?: AbortSignal,
): Promise<AssetPreviewMetaResult> {
  const id = assetId.trim()
  if (!id) return { status: 'error', message: '缺少资产 id' }
  const cached = readPreviewMetaCache(id)
  if (cached) return cached
  const inflight = previewMetaInflight.get(id)
  if (inflight) return inflight

  const p = (async (): Promise<AssetPreviewMetaResult> => {
    try {
      const res = await assetsApi.getAssetPreviewMeta(id, signal)
      const meta = unwrapDownloadPayload(res.data)
      const url = normalizeDisplayUrl(pickMetaUrl(meta))
      if (url) {
        const out: AssetPreviewMetaResult = {
          status: 'ok',
          displayUrl: url,
          downloadUrl: normalizeDisplayUrl(pickMetaUrl(meta)),
        }
        writePreviewMetaCache(id, out)
        return out
      }
      const raw = meta as Record<string, unknown> | undefined
      const hint = String(raw?.access_hint ?? raw?.accessHint ?? '').trim()
      if (hint.includes('prepare_required')) {
        return { status: 'preparing', message: '正在准备预览，请稍后自动刷新' }
      }
      return { status: 'unavailable', message: '预览地址为空' }
    } catch (e) {
      if (axios.isAxiosError(e)) {
        const st = e.response?.status
        if (st === 404) return { status: 'not_found', message: '资源不存在' }
        if (st === 403) return { status: 'error', message: '无权限预览该资源' }
        if (st === 409)
          return { status: 'unavailable', message: '当前不可预览，仅可下载（源稿/PSD 常无直预览）' }
      }
      const msg = e instanceof Error ? e.message : '加载预览失败'
      return { status: 'error', message: msg }
    } finally {
      previewMetaInflight.delete(id)
    }
  })()

  previewMetaInflight.set(id, p)
  return p
}

function buildDownloadMetaResult(meta: AssetDownloadMeta | undefined): AssetDownloadMetaResult {
  const downloadUrl = normalizeDisplayUrl(pickMetaUrl(meta))
  if (!downloadUrl) {
    const raw = meta as Record<string, unknown> | undefined
    const hint = String(raw?.access_hint ?? raw?.accessHint ?? '').trim()
    if (hint.includes('prepare_required')) {
      return {
        status: 'preparing',
        message: '外部资源正在准备中，请稍后刷新后再下载',
      }
    }
    return { status: 'error', message: '下载地址为空' }
  }
  const filename = pickDownloadFilenameFromMeta(meta)
  return {
    status: 'ok',
    downloadUrl,
    filename: filename || undefined,
  }
}

/** GET /v1/assets/{id}/download → download_url + filename（代理字节流入口） */
export async function fetchAssetDownloadMetaResolved(
  assetId: string,
  signal?: AbortSignal,
): Promise<AssetDownloadMetaResult> {
  const id = assetId.trim()
  if (!id) return { status: 'error', message: '缺少资产 id' }
  const cached = readDownloadCache(id)
  if (cached) return cached
  const inflight = downloadMetaInflight.get(id)
  if (inflight) return inflight

  const p = (async (): Promise<AssetDownloadMetaResult> => {
    try {
      const res = await assetsApi.getAssetDownloadMeta(id, signal)
      const meta = unwrapDownloadPayload(res.data)
      const out = buildDownloadMetaResult(meta)
      if (out.status === 'ok') writeDownloadCache(id, out)
      return out
    } catch (e) {
      if (axios.isAxiosError(e)) {
        const st = e.response?.status
        if (st === 404) return { status: 'not_found', message: '资源不存在' }
        if (st === 403) return { status: 'forbidden', message: '无权限下载该资源' }
      }
      const msg = e instanceof Error ? e.message : '获取下载地址失败'
      return { status: 'error', message: msg }
    } finally {
      downloadMetaInflight.delete(id)
    }
  })()

  downloadMetaInflight.set(id, p)
  return p
}

/** GET /v1/assets/{id}/download → download_url（代理字节流入口） */
export async function fetchAssetDownloadUrl(
  assetId: string,
  signal?: AbortSignal,
): Promise<AssetDownloadMetaResult> {
  return fetchAssetDownloadMetaResolved(assetId, signal)
}

function pushUrl(map: Map<string, string>, raw: unknown, assetRootId: string) {
  if (typeof raw !== 'string' || !raw.trim()) return
  const norm = toRelativeAssetUrl(raw.trim()) ?? raw.trim()
  map.set(norm, assetRootId)
  if (norm !== raw.trim()) map.set(raw.trim(), assetRootId)
}

/**
 * 从任务资产列表建立「展示 URL → 资产根 id」，供仓库侧等与 fileRefs 对齐。
 */
export function buildDisplayUrlToAssetRootIdMap(assets: BackendAsset[]): Map<string, string> {
  const map = new Map<string, string>()
  for (const asset of assets) {
    const rootId = String(asset.id ?? '').trim()
    if (!rootId) continue
    const ar = asset as Record<string, unknown>
    pushUrl(map, ar.download_url ?? ar.downloadUrl, rootId)
    pushUrl(map, ar.public_url ?? ar.publicUrl, rootId)
    pushUrl(map, ar.lan_url ?? ar.lanUrl, rootId)
    pushUrl(map, ar.tailscale_url ?? ar.tailscaleUrl, rootId)
    for (const v of asset.versions ?? []) {
      const vr = v as Record<string, unknown>
      pushUrl(map, v.download_url, rootId)
      pushUrl(map, v.public_url, rootId)
      pushUrl(map, v.lan_url, rootId)
      pushUrl(map, v.tailscale_url, rootId)
      pushUrl(map, vr.file_url ?? vr.fileUrl, rootId)
    }
  }
  return map
}

export function lookupAssetRootIdForDisplayUrl(
  map: Map<string, string>,
  displayUrl: string,
): string | undefined {
  const u = displayUrl.trim()
  if (!u) return undefined
  const norm = toRelativeAssetUrl(u) ?? u
  return map.get(norm) ?? map.get(u)
}

/** 任务资产列表项：版本行上的 asset_id 优先，否则用资产根 id */
export function resolveBackendPreviewAssetId(
  asset: BackendAsset,
  version?: BackendAssetVersion | null,
): string | undefined {
  const vr = (version ?? null) as Record<string, unknown> | null
  const fromV = vr?.asset_id ?? vr?.assetId
  if (fromV != null && String(fromV).trim() !== '') return String(fromV).trim()
  const root = String(asset.id ?? '').trim()
  return root || undefined
}

function backendAssetKindLower(a: BackendAsset): string {
  const r = a as Record<string, unknown>
  return String(
    r.asset_kind ?? r.assetKind ?? r.asset_type ?? r.assetType ?? a.file_role ?? '',
  ).toLowerCase()
}

function backendAssetSourceParentId(a: BackendAsset): string {
  const r = a as Record<string, unknown>
  const v = r.source_asset_id ?? r.sourceAssetId
  if (v == null) return ''
  return String(v).trim()
}

function backendScopeSku(a: BackendAsset): string {
  const r = a as Record<string, unknown>
  return String(r.scope_sku_code ?? r.scopeSkuCode ?? '').trim()
}

/**
 * 按 `source_asset_id` 指向的源资产根 id，查找衍生 `preview` / `design_thumb` 资产 id（用于 GET /preview）。
 * 优先匹配 `preferScopeSkuCode`（批量任务），其次 `preview` 优于 `design_thumb`。
 */
export function findDerivedRasterPreviewAssetId(
  sourceAssetRootId: string,
  assets: BackendAsset[],
  opts?: { preferScopeSkuCode?: string },
): string | undefined {
  const src = sourceAssetRootId.trim()
  if (!src || !assets.length) return undefined
  const pref = opts?.preferScopeSkuCode?.trim()
  const candidates = assets.filter((a) => {
    const k = backendAssetKindLower(a)
    if (k !== 'preview' && k !== 'design_thumb') return false
    return backendAssetSourceParentId(a) === src
  })
  const rank = (a: BackendAsset): number => {
    const k = backendAssetKindLower(a)
    const scopeBonus = pref && backendScopeSku(a) === pref ? 0 : pref ? 4 : 0
    const kindBonus = k === 'preview' ? 0 : 2
    return scopeBonus + kindBonus
  }
  candidates.sort((a, b) => rank(a) - rank(b))
  for (const a of candidates) {
    const id = String(a.id ?? '').trim()
    if (id) return id
  }
  return undefined
}

function unwrapTaskAssetList(data: unknown): BackendAsset[] {
  if (Array.isArray(data)) return data
  if (data && typeof data === 'object') {
    const root = data as Record<string, unknown>
    const inner = root.data ?? root.items
    if (Array.isArray(inner)) return inner
    if (inner && typeof inner === 'object') {
      const mid = inner as Record<string, unknown>
      if (Array.isArray(mid.items)) return mid.items as BackendAsset[]
      if (Array.isArray(mid.data)) return mid.data as BackendAsset[]
    }
  }
  return []
}

/**
 * 拉取某资产预览元数据；若直接预览失败且提供 `taskId`，则按 `source_asset_id` 在任务资产列表中查找衍生 preview/thumb 再试。
 */
export async function fetchTaskAssetPreviewWithDerivedFallback(
  assetRootId: string,
  taskId: string | undefined,
  signal?: AbortSignal,
): Promise<AssetPreviewMetaResult> {
  const id = assetRootId.trim()
  if (!id) return { status: 'error', message: '缺少资产 id' }
  const first = await fetchAssetPreviewMeta(id, signal)
  if (first.status === 'ok') return first
  const tid = taskId?.trim()
  if (!tid) return first
  try {
    const res = await assetsApi.list(tid, signal)
    const list = unwrapTaskAssetList(res.data)
    const derived = findDerivedRasterPreviewAssetId(id, list)
    if (!derived || derived === id) return first
    const second = await fetchAssetPreviewMeta(derived, signal)
    if (second.status === 'ok') return second
  } catch {
    /* ignore list/preview failures */
  }
  return first
}
