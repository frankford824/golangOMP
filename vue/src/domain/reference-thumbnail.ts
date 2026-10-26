/** Only explicit server-signed thumbnails may render UUID task/SKU references.
 * Bound revisions retain the controlled immutable task-asset preview route.
 */
export function referenceThumbnailURL(reference: unknown): string | undefined {
  if (!reference || typeof reference !== 'object') return undefined
  const file = reference as Record<string, unknown>
  if (file.formal_task_asset_id) return undefined
  return typeof file.thumbnail_url === 'string' ? file.thumbnail_url.trim() || undefined : undefined
}

/** SKU rows can retain upload-time URLs while the task union has renewed locators.
 * Reuse metadata only for one exact stable reference ID, never by filename or position.
 */
export function withTaskReferencePreview(reference: Record<string, unknown>, taskReferences: unknown[]): Record<string, unknown> {
  if (reference.formal_task_asset_id || reference.task_asset_id) return reference
  const identity = (value: Record<string, unknown>) => {
    const id = typeof value.ref_id === 'string' && value.ref_id.trim() ? value.ref_id : value.asset_id
    return typeof id === 'string' ? id.trim() : ''
  }
  const id = identity(reference)
  if (!id) return reference
  const matches = taskReferences.filter((item): item is Record<string, unknown> => !!item && typeof item === 'object' && identity(item as Record<string, unknown>) === id)
  if (matches.length !== 1) return reference
  const current = matches[0]!
  const download = typeof current.download_url === 'string' ? current.download_url.trim() : ''
  const preview = typeof current.preview_url === 'string' && current.preview_url.trim() ? current.preview_url.trim() : download
  return { ...reference, thumbnail_url: referenceThumbnailURL(current), download_url: download || undefined,
    url: download || undefined, download_url_expires_at: current.download_url_expires_at, preview_url: preview || undefined }
}
