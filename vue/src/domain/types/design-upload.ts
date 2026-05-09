import type { DesignDeliveryUploadPhase } from '@/domain/enums/upload-status'

/** 待提交区单条文件（与 DesignWorkbench / TaskDetail 共用） */
export interface DesignPendingFile {
  file: File
  fileName: string
  previewUrl: string | null
  extension: string
  objectUrl?: string
}

/** 多商品分桶提交：每桶内文件共用同一 remark 后缀 */
export interface DesignDeliveryAuditBatch {
  files: File[]
  remarkSuffix: string
  targetSkuCode?: string
}

/** design.store 中单次交付上传会话（用于进度条 / 成功 / 失败 / 重试） */
export interface DesignDeliveryUploadSession {
  taskId: string
  phase: DesignDeliveryUploadPhase
  /** 展示用 0–100，经平滑插值 */
  displayPercent: number
  /** 上传进度目标 0–100 */
  targetPercent: number
  currentFileIndex: number
  totalFiles: number
  fileName: string
  fileSizeBytes: number
  multipartLabel: string
  errorMessage: string
  pendingFiles: File[]
  /** 多桶提交失败重试时用；与 pendingFiles 二选一语义 */
  pendingBatches?: DesignDeliveryAuditBatch[]
}
