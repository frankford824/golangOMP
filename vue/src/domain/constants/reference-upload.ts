/** 任务参考图上传规则（创建任务 / 任务详情）：不做格式与张数前端限制。 */

export const REFERENCE_UPLOAD_MAX_FILE_SIZE_MB = 300
export const REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES = REFERENCE_UPLOAD_MAX_FILE_SIZE_MB * 1024 * 1024
export const REFERENCE_UPLOAD_MAX_FILE_SIZE_LABEL = '300MB'

/** 非空、有文件名的本地文件即可进入上传队列 */
export function isAcceptableReferenceFile(f: File): boolean {
  return f.size > 0 && f.name.trim().length > 0
}

export function formatReferenceSizeError(maxMb: number, file: File): string {
  return `单张需 ≤${maxMb}MB，当前约 ${(file.size / 1024 / 1024).toFixed(2)}MB`
}

export function referenceFileTooLargeMessage(fileName?: string): string {
  const base = `文件大小不能超过 ${REFERENCE_UPLOAD_MAX_FILE_SIZE_LABEL}`
  const trimmed = fileName?.trim()
  return trimmed ? `${trimmed} 超过 ${REFERENCE_UPLOAD_MAX_FILE_SIZE_LABEL}，已拒绝上传` : base
}
