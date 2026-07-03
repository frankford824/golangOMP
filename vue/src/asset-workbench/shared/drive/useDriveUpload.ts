import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'

export type DriveUploadQueueStatus = 'queued' | 'uploading' | 'uploaded' | 'failed'

export interface DriveUploadQueueItem {
  id: string
  file: File
  relativePath: string
  progress: number
  status: DriveUploadQueueStatus
  sessionId?: string
  error?: string
}

export interface DriveUploadOptions {
  directoryId?: number
  difficultyClass?: string
  expectedBusinessMonth?: string
  onItemChange?: (item: DriveUploadQueueItem) => void
}

export function createDriveUploadQueue(files: FileList | File[] | null | undefined): DriveUploadQueueItem[] {
  if (!files) return []
  return Array.from(files).map((file) => ({
    id: crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`,
    file,
    relativePath: fileRelativePath(file),
    progress: 0,
    status: 'queued',
  }))
}

export async function uploadDriveQueue(queue: DriveUploadQueueItem[], options: DriveUploadOptions): Promise<number> {
  const uploadBatchId = crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`
  const expectedBusinessMonth = options.expectedBusinessMonth || currentBusinessMonth()
  for (const item of queue) {
    if (item.status !== 'queued' && item.status !== 'failed') continue
    item.status = 'uploading'
    item.progress = 0
    item.error = ''
    options.onItemChange?.(item)
    try {
      const uploaded = await uploadWorkbenchFile(item.file, {
        uploadDirectoryId: options.directoryId,
        uploadBatchId,
        relativePath: item.relativePath,
        isFolderUpload: item.relativePath.includes('/'),
        expectedBusinessMonth,
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
    expected_business_month: expectedBusinessMonth,
    month_rollover_ack: false,
    items: [{
      difficulty_class: options.difficultyClass || undefined,
      finalized: true,
      page_count: uploadedItems.length,
      item_count: uploadedItems.length,
      upload_session_ids: uploadedItems.map((item) => item.sessionId).filter(Boolean) as string[],
    }],
  })
  return uploadedItems.length
}

function fileRelativePath(file: File) {
  const withPath = file as File & { webkitRelativePath?: string }
  return (withPath.webkitRelativePath || file.name).replace(/\\/g, '/').replace(/^\/+/, '')
}

function currentBusinessMonth() {
  const parts = new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
  }).formatToParts(new Date())
  const year = parts.find((part) => part.type === 'year')?.value || new Date().getFullYear().toString()
  const month = parts.find((part) => part.type === 'month')?.value || String(new Date().getMonth() + 1).padStart(2, '0')
  return `${year}-${month}`
}

export function useDriveUpload() {
  return {
    createDriveUploadQueue,
    uploadDriveQueue,
  }
}
