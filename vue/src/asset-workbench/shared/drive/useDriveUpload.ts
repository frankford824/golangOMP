import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

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

type DriveUploadFile = File & {
  assetWorkbenchRelativePath?: string
  webkitRelativePath?: string
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

export function withDriveUploadRelativePath(file: File, relativePath: string): File {
  const uploadFile = file as DriveUploadFile
  const normalized = normalizeRelativePath(relativePath || file.name)
  try {
    Object.defineProperty(uploadFile, 'assetWorkbenchRelativePath', {
      value: normalized,
      configurable: true,
    })
  } catch {
    uploadFile.assetWorkbenchRelativePath = normalized
  }
  return uploadFile
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
      item.error = resolveApiUserMessage(err, { fallback: '上传失败，请重试' })
      options.onItemChange?.(item)
    }
  }

  const uploadedItems = queue.filter((item) => item.status === 'uploaded' && item.sessionId)
  if (!uploadedItems.length) throw new Error('没有文件上传成功，请重试')

  try {
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
  } catch (err) {
    throw new Error(resolveApiUserMessage(err, { fallback: '上传完成，但提交记录生成失败' }))
  }
  return uploadedItems.length
}

function fileRelativePath(file: File) {
  const withPath = file as DriveUploadFile
  return normalizeRelativePath(withPath.assetWorkbenchRelativePath || withPath.webkitRelativePath || file.name)
}

function normalizeRelativePath(value: string) {
  return value.replace(/\\/g, '/').replace(/^\/+/, '')
}

export function useDriveUpload() {
  return {
    createDriveUploadQueue,
    uploadDriveQueue,
  }
}
