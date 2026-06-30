import { runOssDirectUploadPlan, type OssDirectPlan } from '@/services/upload/ossDirectUpload'

import { assetWorkbenchApi, type UploadPlan } from '@aw/shared/api/assetWorkbenchApi'

import { computeWorkbenchFileHash } from './fileHash'

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

export async function uploadWorkbenchFile(
  file: File,
  options: {
    signal?: AbortSignal
    onProgress?: (progress: WorkbenchUploadProgress) => void
    uploadDirectoryId?: number
  } = {},
): Promise<UploadedWorkbenchFile> {
  const fileHash = await computeWorkbenchFileHash(file)
  const created = await assetWorkbenchApi.createUploadSession(
    {
      original_filename: file.name,
      file_size: file.size,
      mime_type: file.type || 'application/octet-stream',
      file_hash: fileHash,
      upload_directory_id: options.uploadDirectoryId,
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
    await assetWorkbenchApi.cancelUploadSession(sessionId, options.signal).catch(() => undefined)
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
