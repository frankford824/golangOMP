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

export type DriveUploadFile = File & {
  assetWorkbenchRelativePath?: string
  webkitRelativePath?: string
}

interface DroppedFileSystemEntry {
  name: string
  isFile: boolean
  isDirectory: boolean
}

interface DroppedFileEntry extends DroppedFileSystemEntry {
  file: (success: (file: File) => void, failure?: (error: DOMException) => void) => void
}

interface DroppedDirectoryEntry extends DroppedFileSystemEntry {
  createReader: () => {
    readEntries: (success: (entries: DroppedFileSystemEntry[]) => void, failure?: (error: DOMException) => void) => void
  }
}

type DropItemWithEntry = DataTransferItem & {
  webkitGetAsEntry?: () => unknown
}

export function createDriveUploadQueue(files: FileList | File[] | null | undefined): DriveUploadQueueItem[] {
  if (!files) return []
  return Array.from(files).map((file) => ({
    id: crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`,
    file,
    relativePath: driveUploadRelativePath(file),
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

export async function filesFromDriveDrop(dataTransfer: DataTransfer | null): Promise<File[]> {
  if (!dataTransfer) return []
  const entries = Array.from(dataTransfer.items || [])
    .map(entryFromDropItem)
    .filter((entry): entry is DroppedFileSystemEntry => !!entry)
  if (!entries.length) return Array.from(dataTransfer.files ?? [])
  const groups = await Promise.all(entries.map((entry) => filesFromEntry(entry)))
  return groups.flat()
}

export function driveUploadRelativePath(file: File) {
  const withPath = file as DriveUploadFile
  return normalizeRelativePath(withPath.assetWorkbenchRelativePath || withPath.webkitRelativePath || file.name)
}

export function isSafeDriveUploadPath(relativePath: string) {
  if (!relativePath || relativePath.includes('\x00') || relativePath.includes(':')) return false
  return relativePath.split('/').every((part) => {
    const value = part.trim()
    return value && value !== '.' && value !== '..' && !value.startsWith('.') && value !== '@eaDir' && value !== '#recycle' && value !== '__MACOSX'
  })
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
      items: uploadedItems.map((item) => ({
        difficulty_class: options.difficultyClass || undefined,
        finalized: true,
        page_count: 1,
        item_count: 1,
        upload_session_ids: item.sessionId ? [item.sessionId] : [],
      })),
    })
  } catch (err) {
    throw new Error(resolveApiUserMessage(err, { fallback: '上传完成，但提交记录生成失败' }))
  }
  return uploadedItems.length
}

function normalizeRelativePath(value: string) {
  return value.replace(/\\/g, '/').replace(/^\/+/, '')
}

function entryFromDropItem(item: DataTransferItem): DroppedFileSystemEntry | null {
  const entry = (item as DropItemWithEntry).webkitGetAsEntry?.()
  return entry ? (entry as DroppedFileSystemEntry) : null
}

async function filesFromEntry(entry: DroppedFileSystemEntry, parentPath = ''): Promise<File[]> {
  const relativePath = normalizeRelativePath(parentPath ? `${parentPath}/${entry.name}` : entry.name)
  if (entry.isFile) {
    const file = await fileFromEntry(entry as DroppedFileEntry)
    return file.size > 0 ? [withDriveUploadRelativePath(file, relativePath)] : []
  }
  if (!entry.isDirectory) return []
  const children = await readDirectoryEntries(entry as DroppedDirectoryEntry)
  const groups = await Promise.all(children.map((child) => filesFromEntry(child, relativePath)))
  return groups.flat()
}

function fileFromEntry(entry: DroppedFileEntry): Promise<File> {
  return new Promise((resolve, reject) => entry.file(resolve, reject))
}

async function readDirectoryEntries(entry: DroppedDirectoryEntry): Promise<DroppedFileSystemEntry[]> {
  const reader = entry.createReader()
  const entries: DroppedFileSystemEntry[] = []
  while (true) {
    const batch = await new Promise<DroppedFileSystemEntry[]>((resolve, reject) => reader.readEntries(resolve, reject))
    if (!batch.length) break
    entries.push(...batch)
  }
  return entries
}

export function useDriveUpload() {
  return {
    createDriveUploadQueue,
    uploadDriveQueue,
  }
}
