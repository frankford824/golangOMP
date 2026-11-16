import { beginUsageOperation } from '@/shared/telemetry/usage'
import { runOssDirectUploadPlan, type OssDirectPlan } from '@/services/upload/ossDirectUpload'

import { assetWorkbenchApi, type UploadPlan } from '@aw/shared/api/assetWorkbenchApi'

export interface WorkbenchUploadProgress {
  loaded: number
  total: number
  percent: number
  partsCompleted?: number
  partsTotal?: number
}

export interface UploadedWorkbenchFile {
  sessionId: string
  filename: string
}

async function uploadWorkbenchFileInternal(
  file: File,
  options: {
    signal?: AbortSignal
    onProgress?: (progress: WorkbenchUploadProgress) => void
    uploadDirectoryId?: number
    uploadBatchId?: string
    relativePath?: string
    isFolderUpload?: boolean
    expectedBusinessMonth?: string
  } = {},
): Promise<UploadedWorkbenchFile> {
  const created = await assetWorkbenchApi.createUploadSession(
    {
      original_filename: file.name,
      file_size: file.size,
      mime_type: file.type || 'application/octet-stream',
      upload_directory_id: options.uploadDirectoryId,
      upload_batch_id: options.uploadBatchId,
      relative_path: options.relativePath,
      is_folder_upload: options.isFolderUpload,
      expected_business_month: options.expectedBusinessMonth,
    },
    options.signal,
  )
  const sessionId = created.session?.session_id?.trim()
  if (!sessionId) {
    throw new Error('上传会话缺少 session_id')
  }
  if (!created.plan) {
    throw new Error('上传会话缺少 OSS 直传计划')
  }
  const plan = normalizeWorkbenchUploadPlan(created.plan)
  try {
    const result = await runOssDirectUploadPlan(file, plan, {
      signal: options.signal,
      progressByteTotal: file.size,
      onProgress: options.onProgress,
      mimeTypeForUpload: file.type || 'application/octet-stream',
    })
    await assetWorkbenchApi.completeUploadSession(
      sessionId,
      { parts: result.oss_parts ?? [] },
      options.signal,
    )
    return { sessionId, filename: file.name }
  } catch (err) {
    // The upload signal is commonly already aborted here. Cleanup needs an
    // independent request so OSS single objects / multipart state are removed.
    await assetWorkbenchApi.cancelUploadSession(sessionId).catch(() => undefined)
    throw err
  }
}

function normalizeWorkbenchUploadPlan(plan: UploadPlan): OssDirectPlan {
  const parts = plan.parts ?? []
  return {
    mode: plan.mode,
    upload_strategy: plan.mode,
    upload_url: plan.upload_url,
    part_urls: parts.map((part) => part.upload_url).filter(Boolean),
    parts_total: parts.length,
    part_size_hint: plan.part_size,
    method: plan.method,
    required_upload_content_type: plan.required_upload_content_type,
    object_key: plan.object_key,
    upload_id: plan.upload_id,
  }
}

export async function uploadWorkbenchFile(file: File, options: Parameters<typeof uploadWorkbenchFileInternal>[1] = {}): Promise<UploadedWorkbenchFile> {
  const finish = beginUsageOperation('upload.workflow', { file_size: file.size })
  try {
    const result = await uploadWorkbenchFileInternal(file, options)
    finish('succeeded', undefined, { phase_detail: 'server_confirmed', upload_session_id: result.sessionId })
    return result
  } catch (error) { finish(options.signal?.aborted ? 'cancelled' : 'failed', error); throw error }
}
