import type {
  CreateSettlementSupplementPayload,
  SettlementSupplementRow,
  SupplementPermissionRow,
  UploadDirectoryRow,
} from '@aw/shared/api/assetWorkbenchApi'

type NamedFile = Pick<File, 'name'>

const supplementImageExtension = /\.(avif|bmp|gif|heic|heif|jpe?g|png|svg|tiff?|webp)$/i

function normalizedFilename(value: string) {
  return value.trim().toLocaleLowerCase('zh-CN')
}

export function isSupplementImageFile(file: File): boolean {
  if (file.size <= 0) return false
  return file.type.toLowerCase().startsWith('image/') || supplementImageExtension.test(file.name)
}

export function filterSupplementImageFiles(files: File[] | FileList | null | undefined) {
  const candidates = Array.from(files ?? [])
  const accepted = candidates.filter(isSupplementImageFile)
  return { files: accepted, ignored: candidates.length - accepted.length }
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
  sessionId: string,
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
    upload_session_ids: [sessionId],
  }
}
