/** Only explicit server-signed thumbnails may render UUID task/SKU references.
 * Bound revisions retain the controlled immutable task-asset preview route.
 */
export function referenceThumbnailURL(reference: unknown): string | undefined {
  if (!reference || typeof reference !== 'object') return undefined
  const file = reference as Record<string, unknown>
  if (file.formal_task_asset_id) return undefined
  return typeof file.thumbnail_url === 'string' ? file.thumbnail_url.trim() || undefined : undefined
}
