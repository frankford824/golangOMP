import type { BackendAsset } from '@/services/apiTypes'

/**
 * 解析 GET /v1/assets/{id} 响应体（含 `{ data: DesignAsset }` 或裸对象）。
 * 后端常见返回 `id` 为 number，与 OpenAPI 一致；勿仅接受 string。
 */
export function normalizeAssetDetailFromApi(body: unknown): BackendAsset | null {
  if (!body || typeof body !== 'object') return null
  const root = body as Record<string, unknown>
  const candidate = (root.data && typeof root.data === 'object'
    ? root.data
    : root) as Record<string, unknown>
  const id = candidate.id
  if (id === undefined || id === null) return null
  if (typeof id !== 'string' && typeof id !== 'number') return null
  return candidate as BackendAsset
}
