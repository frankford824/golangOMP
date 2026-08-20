import type {
  CreateSettlementSupplementPayload,
  SettlementSupplementRow,
  SupplementPermissionRow,
  UploadDirectoryRow,
} from '@aw/shared/api/assetWorkbenchApi'

type NamedFile = Pick<File, 'name'>

function normalizedFilename(value: string) {
  return value.trim().toLocaleLowerCase('zh-CN')
}

export function filterSupplementUploadFiles(files: File[] | FileList | null | undefined, allowedFileTypes: string[] = []) {
  const candidates = Array.from(files ?? [])
  const allowed = allowedFileTypes.map((value) => value.trim().toLowerCase().replace(/^\.+/, '')).filter(Boolean)
  const accepted = candidates.filter((file) => file.size > 0 && supplementFileAllowed(file, allowed))
  return { files: accepted, ignored: candidates.length - accepted.length }
}

function supplementFileAllowed(file: File, allowed: string[]) {
  if (!allowed.length) return true
  const extension = file.name.includes('.') ? file.name.split('.').pop()?.toLowerCase() || '' : ''
  const mimeType = file.type.trim().toLowerCase()
  return allowed.some((value) => (
    (extension && value === extension)
    || (mimeType && value === mimeType)
    || (mimeType && value.endsWith('/*') && mimeType.startsWith(value.slice(0, -1)))
  ))
}

export function duplicateSupplementFileNames(files: NamedFile[], supplements: SettlementSupplementRow[]): string[] {
  const existing = new Set(
    supplements
      .filter((item) => item.status !== 'voided')
      .flatMap((item) => [item.order_no, ...(item.files ?? []).map((file) => file.original_filename)])
      .map(normalizedFilename)
      .filter(Boolean),
  )
  const selected = new Set<string>()
  const duplicates = new Set<string>()
  for (const file of files) {
    const normalized = normalizedFilename(file.name)
    if (!normalized) continue
    if (existing.has(normalized) || selected.has(normalized)) duplicates.add(file.name)
    selected.add(normalized)
  }
  return Array.from(duplicates)
}

export function buildSelfSupplementPayload(
  file: NamedFile,
  sessionIds: string | string[],
  supplementDate: string,
  permission: SupplementPermissionRow,
  directory: UploadDirectoryRow,
): CreateSettlementSupplementPayload {
  return {
    payee_user_id: permission.payee_user_id,
    business_month: permission.business_month,
    order_no: file.name,
    supplement_date: supplementDate,
    difficulty_class: directory.difficulty_class,
    finalized: true,
    page_count: 1,
    gross_amount: 0,
    status: 'approved',
    upload_session_ids: Array.isArray(sessionIds) ? sessionIds : [sessionIds],
  }
}
