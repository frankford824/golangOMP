/**
 * 任务参考图上传规则（创建任务 / 任务详情与设计稿一致）
 * 与设计稿同源单文件上限；不做格式与张数前端限制。
 */
import {
  DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES,
  DESIGN_UPLOAD_MAX_FILE_SIZE_MB,
  designUploadTooLargeMessage,
} from '@/domain/copy/design-upload'

/** 与设计稿交付上传一致，避免两处上限分叉 */
export const REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES = DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES
export const REFERENCE_UPLOAD_MAX_FILE_SIZE_MB = DESIGN_UPLOAD_MAX_FILE_SIZE_MB

/** 非空、有文件名的本地文件即可进入上传队列 */
export function isAcceptableReferenceFile(f: File): boolean {
  return f.size > 0 && f.name.trim().length > 0
}

export function formatReferenceSizeError(maxMb: number, file: File): string {
  return `单张需 ≤${maxMb}MB，当前约 ${(file.size / 1024 / 1024).toFixed(2)}MB`
}

/** 与 design-upload 文案一致（单文件超过上限） */
export function referenceFileTooLargeMessage(fileName?: string): string {
  return designUploadTooLargeMessage(fileName)
}
