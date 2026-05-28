import type { AssetThumbItem } from '@/components/task-detail/AssetThumbStrip.vue'
import {
  dedupeReferenceFileRefs,
  parseReferenceFileRefs,
} from '@/domain/mappers/reference-file-refs'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import type { BackendAsset, BackendAssetVersion } from '@/services/apiTypes'
import { canPreviewUploadInline } from '@/domain/constants/upload-types'
import { toRelativeAssetUrl } from '@/utils/url'

function trimField(value: unknown): string {
  return String(value ?? '').trim()
}

function pickVersionDownloadUrl(version: Record<string, unknown>): string {
  const direct = trimField(version.download_url ?? version.downloadUrl)
  if (direct) {
    const expiresAt = trimField(version.download_url_expires_at ?? version.downloadUrlExpiresAt)
    return expiresAt ? direct : (toRelativeAssetUrl(direct) ?? direct)
  }
  const publicUrl = trimField(version.public_url ?? version.publicUrl)
  return publicUrl ? (toRelativeAssetUrl(publicUrl) ?? publicUrl) : ''
}

function normalizeSourceVersion(raw: unknown): BackendAssetVersion | null {
  if (!raw || typeof raw !== 'object') return null
  const ver = raw as Record<string, unknown>
  const id = trimField(ver.id)
  const downloadUrl = pickVersionDownloadUrl(ver)
  const fileName = trimField(ver.file_name ?? ver.original_filename ?? ver.originalFilename)
  const fileSizeRaw = ver.file_size ?? ver.fileSize
  const fileSize =
    typeof fileSizeRaw === 'number' && Number.isFinite(fileSizeRaw) ? fileSizeRaw : undefined
  return {
    id: id || '0',
    file_role: 'source',
    file_name: fileName || undefined,
    download_url: downloadUrl || undefined,
    preview_available: ver.preview_available === true || ver.previewAvailable === true,
    mime_type: trimField(ver.mime_type ?? ver.mimeType) || undefined,
    file_size: fileSize,
  }
}

/** Parse nested `reference_file_refs` on one retouch requirement row. */
export function parseRetouchRequirementReferenceFileRefs(raw: unknown): ReferenceFileRef[] {
  return dedupeReferenceFileRefs(parseReferenceFileRefs(raw))
}

/** Map nested `source_assets` (DesignAsset read model) to BackendAsset for display helpers. */
export function mapRetouchRequirementSourceAssetsFromApi(raw: unknown): BackendAsset[] {
  if (!Array.isArray(raw)) return []
  const out: BackendAsset[] = []
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const row = item as Record<string, unknown>
    const id = trimField(row.id)
    if (!id) continue
    const currentVersion = normalizeSourceVersion(row.current_version ?? row.currentVersion)
    const asset: BackendAsset = {
      id,
      task_id: row.task_id != null ? String(row.task_id) : undefined,
      file_role: trimField(row.asset_type ?? row.assetType ?? 'source') || 'source',
    }
    if (currentVersion) {
      asset.versions = [currentVersion]
      ;(asset as Record<string, unknown>).current_version = currentVersion
    }
    out.push(asset)
  }
  return out
}

export interface RetouchSourceFileDisplayItem {
  key: string
  fileName: string
  downloadUrl?: string
  sizeText?: string
  mimeType?: string
  imagePreviewUrl?: string
  previewAssetId?: string
}

export function formatRetouchAssetFileSize(bytes: number | undefined): string | undefined {
  if (bytes == null || !Number.isFinite(bytes) || bytes <= 0) return undefined
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}

function resolveSourceVersion(asset: BackendAsset): BackendAssetVersion | undefined {
  const rec = asset as Record<string, unknown>
  const current = rec.current_version ?? rec.currentVersion
  if (current && typeof current === 'object') {
    return normalizeSourceVersion(current) ?? undefined
  }
  const versions = asset.versions
  return Array.isArray(versions) && versions.length > 0 ? versions[0] : undefined
}

export function retouchSourceAssetsToDisplayItems(assets: BackendAsset[]): RetouchSourceFileDisplayItem[] {
  const out: RetouchSourceFileDisplayItem[] = []
  for (const asset of assets) {
    const version = resolveSourceVersion(asset)
    const fileName =
      trimField(version?.file_name) ||
      trimField((asset as Record<string, unknown>).asset_no) ||
      `素材 ${asset.id}`
    const downloadUrl = trimField(version?.download_url)
    const mimeType = trimField(version?.mime_type)
    const imagePreviewUrl =
      downloadUrl && (version?.preview_available === true || canPreviewUploadInline(fileName))
        ? downloadUrl
        : undefined
    out.push({
      key: `source-${asset.id}`,
      fileName,
      downloadUrl: downloadUrl || undefined,
      sizeText: formatRetouchAssetFileSize(
        typeof version?.file_size === 'number' ? version.file_size : undefined,
      ),
      mimeType: mimeType || undefined,
      imagePreviewUrl,
      previewAssetId: imagePreviewUrl ? String(asset.id) : undefined,
    })
  }
  return out
}

export function retouchRequirementReferenceRefsToThumbItems(
  refs: ReferenceFileRef[],
  keyPrefix: string,
): AssetThumbItem[] {
  return refs
    .map((ref, index) => {
      const src = trimField(ref.download_url)
      if (!src) return null
      const filename = trimField(ref.filename)
      return {
        key: `${keyPrefix}-${index}-${src}`,
        src,
        alt: filename || `参考图 ${index + 1}`,
        label: filename || `参考图 ${index + 1}`,
        downloadUrl: src,
      }
    })
    .filter((row) => row != null) as AssetThumbItem[]
}
