import type { ReferenceFileRef } from '@/services/api/assetsApi'
import { toRelativeAssetUrl } from '@/utils/url'

/**
 * `reference_file_refs`：字符串（历史 URL）或 `ReferenceFileRef` 对象。
 *
 * v1.18+：后端返回的 presigned `download_url` 带 `download_url_expires_at`，
 * 此时 URL 是外部 OSS 直链，**不得** 经 `toRelativeAssetUrl` 折叠为相对路径。
 * 仅对无过期标记的 legacy URL 继续做同源折叠。
 */
export function parseReferenceFileRefs(raw: unknown): ReferenceFileRef[] {
  if (!Array.isArray(raw)) return []
  return raw.flatMap((item) => {
    if (typeof item === 'string') {
      const url = (toRelativeAssetUrl(item) ?? item.trim()) || ''
      return url ? [{ download_url: url } as ReferenceFileRef] : []
    }
    if (item && typeof item === 'object') {
      const obj = { ...(item as ReferenceFileRef) }
      if (obj.download_url && !obj.download_url_expires_at) {
        obj.download_url = toRelativeAssetUrl(obj.download_url as string) ?? obj.download_url
      }
      return obj.download_url ? [obj] : []
    }
    return []
  })
}
