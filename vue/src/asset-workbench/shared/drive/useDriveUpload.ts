import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'

export type DriveUploadQueueStatus = 'queued' | 'uploading' | 'uploaded' | 'failed'

export interface DriveUploadQueueItem {
  id: string
  file: File
  progress: number
  status: DriveUploadQueueStatus
  sessionId?: string
  error?: string
}

export interface DriveUploadOptions {
  orderNo: string
  directoryId?: number
  difficultyClass?: string
  onItemChange?: (item: DriveUploadQueueItem) => void
}

export function createDriveUploadQueue(files: FileList | File[] | null | undefined): DriveUploadQueueItem[] {
  if (!files) return []
  return Array.from(files).map((file) => ({
    id: crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`,
    file,
    progress: 0,
    status: 'queued',
  }))
}

export async function uploadDriveQueue(queue: DriveUploadQueueItem[], options: DriveUploadOptions): Promise<number> {
  const orderNo = options.orderNo.trim()
  if (!orderNo) throw new Error('订单号必填')

  for (const item of queue) {
    if (item.status !== 'queued' && item.status !== 'failed') continue
    item.status = 'uploading'
    item.progress = 0
    item.error = ''
    options.onItemChange?.(item)
    try {
      const uploaded = await uploadWorkbenchFile(item.file, {
        uploadDirectoryId: options.directoryId,
        onProgress: (progress) => {
          item.progress = progress.percent
          options.onItemChange?.(item)
        },
      })
      item.sessionId = uploaded.sessionId
      item.progress = 100
      item.status = 'uploaded'
      options.onItemChange?.(item)
    } catch (err) {
      item.status = 'failed'
      item.error = err instanceof Error ? err.message : '上传失败'
      options.onItemChange?.(item)
    }
  }

  const uploadedItems = queue.filter((item) => item.status === 'uploaded' && item.sessionId)
  if (!uploadedItems.length) throw new Error('没有文件上传成功，请重试')

  await assetWorkbenchApi.createSubmission({
    notes: '',
    items: uploadedItems.map((item) => ({
      order_no: orderNo,
      difficulty_class: options.difficultyClass || undefined,
      finalized: true,
      page_count: 1,
      item_count: 1,
      upload_session_ids: item.sessionId ? [item.sessionId] : [],
    })),
  })
  return uploadedItems.length
}

export function useDriveUpload() {
  return {
    createDriveUploadQueue,
    uploadDriveQueue,
  }
}
