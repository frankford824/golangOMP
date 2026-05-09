/**
 * 对后端返回的资源 URL 做最小归一化：
 * - 兼容旧代理路由时，当前站点同源 URL 转为相对路径，便于继续走 Vite proxy
 * - canonical `download_url` / `preview` 若已是外部直连 OSS 签名链接，则必须原样保留
 */
export function toRelativeAssetUrl(url: string | undefined | null): string | undefined {
  if (!url || typeof url !== 'string') return undefined
  const trimmed = url.trim()
  if (!trimmed) return undefined
  if (trimmed.startsWith('/')) return trimmed

  try {
    if (!trimmed.startsWith('http://') && !trimmed.startsWith('https://')) {
      return trimmed
    }
    const u = new URL(trimmed)
    const pathWithQuery = `${u.pathname}${u.search}${u.hash}`

    // 仅将当前前端同源地址折叠为相对路径，避免破坏 OSS 直链签名。
    if (typeof window !== 'undefined') {
      const current = new URL(window.location.origin)
      if (u.origin === current.origin) {
        return pathWithQuery || undefined
      }
    }

    return trimmed
  } catch {
    return trimmed
  }
}
