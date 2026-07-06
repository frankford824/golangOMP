import { assetWorkbenchApi, type ArchiveVirtualFile } from '@aw/shared/api/assetWorkbenchApi'

export async function createArchiveEntryObjectUrl(fileId: number, file: ArchiveVirtualFile, signal?: AbortSignal): Promise<string> {
  const blob = await assetWorkbenchApi.getArchiveEntryBlob(fileId, file.path, 'inline', signal)
  return URL.createObjectURL(blob)
}

export async function downloadArchiveEntryBlob(fileId: number, file: ArchiveVirtualFile, signal?: AbortSignal): Promise<void> {
  const blob = await assetWorkbenchApi.getArchiveEntryBlob(fileId, file.path, 'attachment', signal)
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = file.name || 'archive-entry'
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}
