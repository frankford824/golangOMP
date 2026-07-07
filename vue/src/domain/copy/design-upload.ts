/** 设计稿交付上传：文案集中管理（组件内禁止散落硬编码） */
export const DESIGN_UPLOAD_MAX_FILE_SIZE_MB = 1024
export const DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES = DESIGN_UPLOAD_MAX_FILE_SIZE_MB * 1024 * 1024
export const DESIGN_UPLOAD_MAX_FILE_SIZE_LABEL = '1GB'

export function designUploadTooLargeMessage(fileName?: string): string {
  const base = `文件大小不能超过 ${DESIGN_UPLOAD_MAX_FILE_SIZE_LABEL}`
  const trimmed = fileName?.trim()
  return trimmed ? `${trimmed} 超过 ${DESIGN_UPLOAD_MAX_FILE_SIZE_LABEL}，已拒绝上传` : base
}

export const DESIGN_UPLOAD_COPY = {
  uploadingToServer: '正在上传至服务器...',
  uploadComplete: '上传完成',
  retry: '重试',
  dropHint: '点击、拖拽或粘贴上传本次设计稿（可多选，任意格式）',
  sizeLimitHint: `单文件不超过 ${DESIGN_UPLOAD_MAX_FILE_SIZE_LABEL}，超过将被拒绝上传`,
  batchSubmitHint: '批量 SKU 会一次汇总提交；每桶需绑定对应商品的 SKU。',
  reading: '读取中...',
  pendingLabel: '待提交文件',
  submitAudit: '提交审核',
  submitHintIdle: '提交后设计稿将进入审核队列，本次文件锁定为新版本',
  submitHintNeedFiles: '请先上传本次设计稿',
  submitHintUploading: '正在提交设计稿',
  submitErrorFallback: '提交审核失败，请确认设计稿已上传并重试',
  /** 选文件后本地假进度（非真实上传） */
  pickProcessing: '正在处理文件…',
} as const

/** 选文件假进度 + 提交成功态停留（毫秒） */
export const DESIGN_UPLOAD_TIMING = {
  pickFakeDurationMs: 900,
  successDisplayMs: 800,
} as const
