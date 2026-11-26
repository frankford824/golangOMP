type ArchiveSource = { original_filename?: string; display_name?: string; file_type?: string }

const supported = new Set(['zip', 'rar', 'tar', '7z'])

export function archiveFormatOf(file?: ArchiveSource | null) {
  if (!file) return ''
  const name = file.original_filename || file.display_name || ''
  const extension = name.match(/\.([^.\/]+)$/)?.[1]?.toLowerCase() || ''
  return supported.has(extension) || file.file_type === 'archive' ? extension || 'archive' : ''
}

export function canOpenArchive(file?: ArchiveSource | null) {
  return supported.has(archiveFormatOf(file))
}
