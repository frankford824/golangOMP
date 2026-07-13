/**
 * 规范资产上传：POST /v1/assets/upload-sessions → 执行 `oss_direct` 上传计划
 * → complete；失败或 complete 失败时调用 cancel 终止 MAIN 会话。
 */
import type { AssetUploadProgress, ReferenceFileRef } from '@/services/api/assetsApi'
import {
  assetsApi,
  assertAssetCenterUploadCompleteOk,
  normalizeAssetCenterCompleteData,
  type CompleteAssetUploadSessionPayload,
  type CreateAssetUploadSessionPayload,
} from '@/services/api/assetsApi'
import { taskAssetsApi } from '@/services/api/taskAssetsApi'
import {
  parseOssDirectPlan,
  type OssDirectPlan,
  type OssDirectUploadResult,
  type RemoteUploadPlan,
  runOssDirectUploadPlan,
} from '@/services/upload/ossDirectUpload'
import { parseApiErrorPayload } from '@/utils/api-message-zh'
import { resolveFileMimeType } from '@/utils/mime'
import { formatUploadFailureMessage } from '@/utils/upload-errors'

type AssetUploadSessionCreateFn = (
  payload: CreateAssetUploadSessionPayload,
  signal?: AbortSignal,
) => ReturnType<typeof assetsApi.createAssetUploadSession>

export interface TaskAssetUploadFlowOptions {
  signal?: AbortSignal
  onProgress?: (p: AssetUploadProgress) => void
  /** 追加在 remark 末尾（如多 SKU 后缀） */
  remarkSuffix?: string
  /** P 图需求明细 ID；写入 create upload-session 的 retouch_requirement_id */
  retouchRequirementId?: number
  /** 特殊任务内上传入口，例如已结单审核补传。 */
  createSession?: AssetUploadSessionCreateFn
}

export interface ReferenceUploadFlowOptions {
  taskId?: string | null
  assetId?: string | number
  targetSkuCode?: string
  /** P 图需求明细 ID；仅在有 taskId 的 upload-session 路径下生效 */
  retouchRequirementId?: number
  ownerModuleKey?: string
  uploadPolicy?: 'append_only' | 'replace' | string
  signal?: AbortSignal
}

export interface PreparedTaskAssetUploadSession {
  sessionId: string
  taskId?: string
  ossDirect?: NonNullable<ReturnType<typeof parseOssDirectPlan>['ossDirect']>
  remote?: RemoteUploadPlan
  expectedSize?: number
  assetKind: CreateAssetUploadSessionPayload['asset_kind']
  targetSkuCode?: string
  remark: string
  fileHash?: string
  sessionMime?: string
  completeEndpoint?: string
}

function uploadErrorResponseData(err: unknown): unknown {
  if (!err || typeof err !== 'object') return undefined
  const value = err as {
    response?: { data?: unknown }
    responseData?: unknown
  }
  return value.response?.data ?? value.responseData
}

function readUploadDenyDetail(err: unknown, key: string): string | undefined {
  if (key === 'deny_code') {
    const denyCode = parseApiErrorPayload(err).denyCode
    if (denyCode) return denyCode
  }
  const raw = uploadErrorResponseData(err)
  if (!raw || typeof raw !== 'object') return undefined
  const root = raw as Record<string, unknown>
  const envelope = root.data && typeof root.data === 'object'
    ? root.data as Record<string, unknown>
    : root
  const apiError = envelope.error && typeof envelope.error === 'object'
    ? envelope.error as Record<string, unknown>
    : undefined
  const details = apiError?.details && typeof apiError.details === 'object'
    ? apiError.details as Record<string, unknown>
    : undefined
  const value = details?.[key]
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

/**
 * v1.8：`POST /v1/assets/upload-sessions/:id/complete` 在遇到 AssetVersion 竞态时
 * 返回 HTTP 409 + `code="CONFLICT"` + `details.deny_code="asset_version_race_retry"`。
 * 该错误表达的是“同 asset 的两个 complete 撞车”，单次自动重试（取消旧 session → 重新
 * 申请一次新 session → 再次 complete）即可恢复。其余 4xx / 5xx 不在本预判范围内。
 */
export function isAssetVersionRaceRetryError(err: unknown): boolean {
  const parsed = parseApiErrorPayload(err)
  const status = parsed.status
  if (status !== 409) return false
  if (parsed.code !== 'CONFLICT') return false
  const denyCode = readUploadDenyDetail(err, 'deny_code')
  return denyCode === 'asset_version_race_retry'
}

function isTaskStatusNotActionableUploadError(err: unknown): boolean {
  const code = parseApiErrorPayload(err).code
  const denyCode = readUploadDenyDetail(err, 'deny_code')
  const action = readUploadDenyDetail(err, 'action')
  return (
    code === 'PERMISSION_DENIED' &&
    denyCode === 'task_status_not_actionable' &&
    (action === 'asset_upload_session_complete' || action === 'asset_upload_session_cancel')
  )
}

function taskIdForSessionBody(taskId: string): string | number {
  const t = taskId.trim()
  const n = Number(t)
  if (Number.isFinite(n) && n > 0 && String(n) === t) return n
  return taskId
}

export function normalizeUploadSessionNumericID(
  value: string | number | undefined,
  fieldName: 'asset_id' | 'source_asset_id',
): number | undefined {
  if (value == null) return undefined
  if (typeof value === 'number') {
    if (Number.isSafeInteger(value) && value > 0) return value
    throw new Error(`${fieldName} 必须是有效的数字资产 ID`)
  }
  const raw = value.trim()
  if (!raw) return undefined
  if (!/^[1-9]\d*$/.test(raw)) {
    throw new Error(`${fieldName} 必须是有效的数字资产 ID`)
  }
  const parsed = Number(raw)
  if (!Number.isSafeInteger(parsed) || parsed <= 0) {
    throw new Error(`${fieldName} 必须是有效的数字资产 ID`)
  }
  return parsed
}

/** 仅接受有效正整数，供 retouch_requirement_id 写入 create-session。 */
export function normalizeRetouchRequirementId(value: number | undefined): number | undefined {
  if (value == null || !Number.isFinite(value)) return undefined
  const n = Math.trunc(value)
  if (n <= 0) return undefined
  return n
}

function resolveRetouchRequirementIdForPayload(
  intent: Pick<CreateAssetUploadSessionPayload, 'retouch_requirement_id'>,
  retouchRequirementId?: number,
): number | undefined {
  return normalizeRetouchRequirementId(intent.retouch_requirement_id ?? retouchRequirementId)
}

function isMultipartOssPlan(mode: string | null | undefined, strategy: string | null | undefined): boolean {
  return mode?.trim().toLowerCase() === 'multipart' || strategy?.trim().toLowerCase() === 'multipart'
}

/**
 * 创建会话、直传 OSS、MAIN complete；任一步失败则 cancel（忽略 cancel 自身错误）。
 */
async function uploadFileViaAssetSession(
  taskId: string | null | undefined,
  file: File,
  intent: Omit<CreateAssetUploadSessionPayload, 'task_id' | 'file_name'> & {
    file_name?: string
  },
  options?: TaskAssetUploadFlowOptions,
): Promise<ReturnType<typeof normalizeAssetCenterCompleteData>> {
  const prepared = await prepareTaskAssetUploadSession(taskId, file, intent, {
    signal: options?.signal,
    remarkSuffix: options?.remarkSuffix,
    retouchRequirementId: options?.retouchRequirementId,
    createSession: options?.createSession,
  })
  const completed = await completeWithAssetVersionRaceRetry(taskId, file, prepared, intent, {
    signal: options?.signal,
    onProgress: options?.onProgress,
    remarkSuffix: options?.remarkSuffix,
    retouchRequirementId: options?.retouchRequirementId,
    createSession: options?.createSession,
  })
  return completed.result
}

export async function prepareTaskAssetUploadSession(
  taskId: string | null | undefined,
  file: File,
  intent: Omit<CreateAssetUploadSessionPayload, 'task_id' | 'file_name'> & {
    file_name?: string
  },
  options?: Pick<TaskAssetUploadFlowOptions, 'signal' | 'remarkSuffix' | 'retouchRequirementId' | 'createSession'>,
): Promise<PreparedTaskAssetUploadSession> {
  const remarkBase = intent.remark ?? file.name
  const remark = remarkBase + (options?.remarkSuffix ?? '')
  const assetId = normalizeUploadSessionNumericID(intent.asset_id, 'asset_id')
  const sourceAssetId = normalizeUploadSessionNumericID(intent.source_asset_id, 'source_asset_id')

  const payload: CreateAssetUploadSessionPayload = {
    asset_id: assetId,
    asset_kind: intent.asset_kind,
    file_name: intent.file_name ?? file.name,
    expected_size: intent.expected_size ?? file.size,
    mime_type: intent.mime_type ?? resolveFileMimeType(file),
    file_hash: intent.file_hash,
    remark,
    reason: intent.reason,
    source_asset_id: sourceAssetId,
    target_sku_code: intent.target_sku_code,
    owner_module_key: intent.owner_module_key,
    upload_policy: intent.upload_policy,
  }
  const retouchRequirementId = resolveRetouchRequirementIdForPayload(intent, options?.retouchRequirementId)
  if (retouchRequirementId != null) {
    payload.retouch_requirement_id = retouchRequirementId
  }
  const sessionTaskId = taskId?.trim()
  if (sessionTaskId) {
    payload.task_id = taskIdForSessionBody(sessionTaskId)
  }

  let sessionRes
  try {
    if (options?.createSession) {
      sessionRes = await options.createSession(payload, options?.signal)
    } else if (sessionTaskId) {
      sessionRes = await assetsApi.createAssetUploadSession(payload, options?.signal)
    } else {
      sessionRes = await taskAssetsApi.createTaskCreateUploadSession(payload, options?.signal)
    }
  } catch (err) {
    throw new Error(formatUploadFailureMessage('create_session', err))
  }

  const parsedDirect = parseOssDirectPlan(sessionRes.data)
  const sessionId = parsedDirect.sessionId || null
  if (!sessionId) {
    throw new Error('创建上传会话成功但未返回 session_id')
  }
  const base = {
    sessionId,
    taskId: sessionTaskId || undefined,
    expectedSize: parsedDirect.expectedSize,
    assetKind: payload.asset_kind,
    targetSkuCode: payload.target_sku_code?.trim() || undefined,
    remark,
    fileHash: intent.file_hash,
    sessionMime: (payload.mime_type ?? '').trim() || undefined,
    completeEndpoint: parsedDirect.completeEndpoint ?? undefined,
  } as const
  if (parsedDirect.ossDirect) {
    return { ...base, ossDirect: parsedDirect.ossDirect }
  }
  if (parsedDirect.remote) {
    console.warn('[upload] oss_direct absent, falling back to remote', { session_id: sessionId })
    return { ...base, remote: parsedDirect.remote }
  }
  throw new Error('上传入口未准备好，请刷新后重试；如仍失败请联系管理员。')
}

export async function completePreparedTaskAssetUploadSession(
  prepared: PreparedTaskAssetUploadSession,
  file: File,
  options?: { signal?: AbortSignal; onProgress?: (p: AssetUploadProgress) => void },
): Promise<ReturnType<typeof normalizeAssetCenterCompleteData>> {
  if (!prepared.ossDirect && !prepared.remote) {
    throw new Error('上传入口未准备好，请刷新后重试；如仍失败请联系管理员。')
  }

  const completePayload: CompleteAssetUploadSessionPayload = {
    remark: prepared.remark,
    file_hash: prepared.fileHash,
  }

  if (prepared.ossDirect) {
    const transportLabel = 'oss_direct（主通道）'
    const byteTotal = prepared.expectedSize && prepared.expectedSize > 0 ? prepared.expectedSize : file.size
    let uploadResult: OssDirectUploadResult | null = null
    try {
      uploadResult = await runOssDirectUploadPlan(file, prepared.ossDirect, {
        signal: options?.signal,
        progressByteTotal: byteTotal,
        onProgress: options?.onProgress,
        mimeTypeForUpload: prepared.sessionMime || undefined,
      })
    } catch (err) {
      await cancelPreparedTaskAssetUploadSession(prepared.sessionId, options?.signal, prepared.taskId, prepared.ossDirect)
      throw new Error(formatUploadFailureMessage('part_upload', err, undefined, { transportLabel }))
    }
    const ossObjectKey = uploadResult?.oss_object_key?.trim()
    const uploadContentType =
      uploadResult?.upload_content_type?.trim() || prepared.sessionMime || resolveFileMimeType(file)
    if (!ossObjectKey) {
      await cancelPreparedTaskAssetUploadSession(prepared.sessionId, options?.signal, prepared.taskId, prepared.ossDirect)
      throw new Error('上传结果不完整，请重新上传；如仍失败请联系管理员。')
    }
    completePayload.oss_object_key = ossObjectKey
    completePayload.upload_content_type = uploadContentType
    if (isMultipartOssPlan(prepared.ossDirect.mode, prepared.ossDirect.upload_strategy)) {
      const ossUploadId = uploadResult?.oss_upload_id?.trim()
      const ossParts = uploadResult?.oss_parts ?? []
      if (!ossUploadId) {
        await cancelPreparedTaskAssetUploadSession(prepared.sessionId, options?.signal, prepared.taskId, prepared.ossDirect)
        throw new Error('上传结果不完整，请重新上传；如仍失败请联系管理员。')
      }
      if (!ossParts.length) {
        await cancelPreparedTaskAssetUploadSession(prepared.sessionId, options?.signal, prepared.taskId, prepared.ossDirect)
        throw new Error('上传结果不完整，请重新上传；如仍失败请联系管理员。')
      }
      completePayload.oss_upload_id = ossUploadId
      completePayload.oss_parts = ossParts
    }
  } else if (prepared.remote) {
    const transportLabel = 'remote（备用通道）'
    const mimeForUpload =
      prepared.remote.required_upload_content_type || prepared.sessionMime || resolveFileMimeType(file)
    try {
      await assetsApi.uploadToRemoteUrl(prepared.remote.upload_url, file, {
        signal: options?.signal,
        onProgress: options?.onProgress,
        method: prepared.remote.method || 'PUT',
        extraHeaders: mimeForUpload ? { 'Content-Type': mimeForUpload } : undefined,
      })
    } catch (err) {
      await cancelPreparedTaskAssetUploadSession(prepared.sessionId, options?.signal, prepared.taskId, prepared.ossDirect)
      throw new Error(formatUploadFailureMessage('part_upload', err, undefined, { transportLabel }))
    }
    if (mimeForUpload) {
      completePayload.upload_content_type = mimeForUpload
    }
  }

  const transportLabel = prepared.ossDirect ? 'oss_direct（主通道）' : 'remote（备用通道）'
  let completeRes
  try {
    completeRes = prepared.completeEndpoint
      ? await assetsApi.completeAssetUploadSessionAtEndpoint(
          prepared.completeEndpoint,
          completePayload,
          options?.signal,
        )
      : await assetsApi.completeAssetUploadSession(
          prepared.sessionId,
          completePayload,
          options?.signal,
        )
  } catch (err) {
    if (isAssetVersionRaceRetryError(err)) {
      throw err
    }
    if (!isTaskStatusNotActionableUploadError(err)) {
      await cancelPreparedTaskAssetUploadSession(prepared.sessionId, options?.signal, prepared.taskId, prepared.ossDirect)
    }
    throw new Error(formatUploadFailureMessage('main_complete', err, undefined, { transportLabel }))
  }

  const normalized = normalizeAssetCenterCompleteData(completeRes.data)
  assertAssetCenterUploadCompleteOk(normalized)
  return normalized
}

/**
 * v1.8 Round I：为 `asset_version_race_retry` 场景包一层单次自动重试。
 *
 * 行为：
 *   1) 先用传入的 `prepared` 调用 complete。
 *   2) 若抛出 `isAssetVersionRaceRetryError(err)` 的错误，且重试预算未耗尽：
 *      - cancel 旧 session
 *      - 用 `intent` 对同一 file 重新 prepare 一次（新的 sessionId）
 *      - 对新 session 再跑一次 oss_direct + complete
 *   3) 任意其他错误、或第二次仍命中 race-retry：按既有失败路径冒泡，交由调用方呈现。
 *
 * 返回值同时包含最终成功的 `prepared`，方便调用方更新 cancellable set / 上下文。
 */
export async function completeWithAssetVersionRaceRetry(
  taskId: string | null | undefined,
  file: File,
  prepared: PreparedTaskAssetUploadSession,
  intent: Omit<CreateAssetUploadSessionPayload, 'task_id' | 'file_name'> & {
    file_name?: string
  },
  options?: Pick<
    TaskAssetUploadFlowOptions,
    'signal' | 'onProgress' | 'remarkSuffix' | 'retouchRequirementId' | 'createSession'
  > & {
    /** 新一轮 prepare 完成后回调，常用于把新 sessionId 接续到取消集 */
    onRetryPrepared?: (next: PreparedTaskAssetUploadSession) => void
  },
): Promise<{
  result: ReturnType<typeof normalizeAssetCenterCompleteData>
  prepared: PreparedTaskAssetUploadSession
}> {
  const transportLabel = 'oss_direct（主通道）'
  try {
    const first = await completePreparedTaskAssetUploadSession(prepared, file, {
      signal: options?.signal,
      onProgress: options?.onProgress,
    })
    return { result: first, prepared }
  } catch (err) {
    if (!isAssetVersionRaceRetryError(err)) throw err
    // 单次预算：取消旧 session → 重新 prepare → 再跑一次 oss_direct + complete。
    await cancelPreparedTaskAssetUploadSession(prepared.sessionId, options?.signal, prepared.taskId, prepared.ossDirect)
    const retryPrepared = await prepareTaskAssetUploadSession(taskId, file, intent, {
      signal: options?.signal,
      remarkSuffix: options?.remarkSuffix,
      retouchRequirementId: options?.retouchRequirementId,
      createSession: options?.createSession,
    })
    options?.onRetryPrepared?.(retryPrepared)
    try {
      const second = await completePreparedTaskAssetUploadSession(retryPrepared, file, {
        signal: options?.signal,
        onProgress: options?.onProgress,
      })
      return { result: second, prepared: retryPrepared }
    } catch (secondErr) {
      // 第二次仍失败：若仍是 race-retry 或其他 axios 错误，走既有用户可见失败路径。
      if (isAssetVersionRaceRetryError(secondErr)) {
        throw new Error(
          formatUploadFailureMessage('main_complete', secondErr, undefined, { transportLabel }),
        )
      }
      throw secondErr
    }
  }
}

export async function cancelPreparedTaskAssetUploadSession(
  sessionId: string,
  _signal?: AbortSignal,
  taskId?: string,
  ossDirect?: OssDirectPlan,
): Promise<void> {
  try {
    // Cleanup must outlive the upload signal. In the most important failure path
    // that signal is already aborted, so reusing it would cancel this request
    // before the backend can delete the single object or abort multipart state.
    const payload = {
      ...(ossDirect?.object_key ? { oss_object_key: ossDirect.object_key } : {}),
      ...(ossDirect?.upload_id ? { oss_upload_id: ossDirect.upload_id } : {}),
    }
    if (taskId?.trim()) {
      await assetsApi.cancelAssetUploadSession(sessionId, payload)
    } else {
      await taskAssetsApi.abortTaskCreateUploadSession(sessionId, payload)
    }
  } catch {
    /* 尽力取消，不掩盖主错误 */
  }
}

export async function uploadTaskFileViaAssetSession(
  taskId: string,
  file: File,
  intent: Omit<CreateAssetUploadSessionPayload, 'task_id' | 'file_name'> & {
    file_name?: string
  },
  options?: TaskAssetUploadFlowOptions,
): Promise<ReturnType<typeof normalizeAssetCenterCompleteData>> {
  return uploadFileViaAssetSession(taskId, file, intent, options)
}

export async function uploadAuditSupplementFileViaAssetSession(
  taskId: string,
  file: File,
  payload: { reason: string; targetSkuCode?: string },
  options?: TaskAssetUploadFlowOptions,
): Promise<ReturnType<typeof normalizeAssetCenterCompleteData>> {
  const reason = payload.reason.trim()
  return uploadFileViaAssetSession(
    taskId,
    file,
    {
      asset_kind: 'delivery',
      target_sku_code: payload.targetSkuCode?.trim() || undefined,
      owner_module_key: 'audit',
      upload_policy: 'audit_post_close_supplement',
      remark: reason,
      reason,
    },
    {
      ...options,
      createSession: (sessionPayload, signal) =>
        assetsApi.createAuditSupplementUploadSession(taskId, sessionPayload, signal),
    },
  )
}

function toReferenceFileRef(
  uploaded: ReturnType<typeof normalizeAssetCenterCompleteData>,
  fallbackFile: File,
): ReferenceFileRef {
  const asset = uploaded.asset as (Record<string, unknown> & { id?: string }) | undefined
  const version = uploaded.version as Record<string, unknown> | undefined
  const assetId = String(asset?.id ?? '').trim()
  const downloadUrl =
    (typeof version?.download_url === 'string' && version.download_url.trim()) ||
    (typeof asset?.download_url === 'string' && String(asset.download_url).trim()) ||
    null
  const fileSize = version?.file_size
  const mimeType = version?.mime_type
  const fileName = version?.file_name
  return {
    asset_id: assetId || undefined,
    ref_id: assetId || undefined,
    filename: typeof fileName === 'string' && fileName.trim() ? fileName : fallbackFile.name,
    mime_type: typeof mimeType === 'string' && mimeType.trim() ? mimeType : resolveFileMimeType(fallbackFile),
    file_size: typeof fileSize === 'number' && Number.isFinite(fileSize) ? fileSize : fallbackFile.size,
    download_url: typeof downloadUrl === 'string' && downloadUrl.trim() ? downloadUrl : null,
    status: String(asset?.upload_status ?? '').trim() || 'uploaded',
    source: 'asset_upload_session',
  }
}

/**
 * 参考图上传统一入口：
 * - 已有 task_id：走 canonical `/v1/assets/upload-sessions`
 * - 创建前无 task_id：走 canonical `/v1/tasks/reference-upload-sessions` OSS 直传会话
 */
export async function uploadReferenceFileRef(
  file: File,
  options?: ReferenceUploadFlowOptions,
): Promise<ReferenceFileRef> {
  const taskId = options?.taskId?.trim()
  if (!taskId) {
    const sessionRes = await taskAssetsApi.createTaskCreateUploadSession(
      {
        asset_kind: 'reference',
        file_name: file.name,
        expected_size: file.size,
        mime_type: resolveFileMimeType(file),
        remark: file.name,
      },
      options?.signal,
    )
    const parsed = parseOssDirectPlan(sessionRes.data)
    if (!parsed.sessionId) {
      throw new Error('创建上传会话成功但未返回 session_id')
    }
    try {
      const completePayload: CompleteAssetUploadSessionPayload = { remark: file.name }
      if (parsed.ossDirect) {
        const result = await runOssDirectUploadPlan(file, parsed.ossDirect, {
          signal: options?.signal,
          progressByteTotal: file.size,
          mimeTypeForUpload: resolveFileMimeType(file),
        })
        const objectKey = result.oss_object_key?.trim()
        if (!objectKey) throw new Error('上传结果缺少 OSS object key')
        completePayload.oss_object_key = objectKey
        completePayload.upload_content_type = result.upload_content_type || resolveFileMimeType(file)
        if (result.mode === 'multipart') {
          if (!result.oss_upload_id || !result.oss_parts?.length) {
            throw new Error('上传结果不完整，请重新上传；如仍失败请联系管理员。')
          }
          completePayload.oss_upload_id = result.oss_upload_id
          completePayload.oss_parts = result.oss_parts
        }
      } else if (parsed.remote) {
        const mimeType = parsed.remote.required_upload_content_type || resolveFileMimeType(file)
        await assetsApi.uploadToRemoteUrl(parsed.remote.upload_url, file, {
          signal: options?.signal,
          method: parsed.remote.method || 'PUT',
          extraHeaders: { 'Content-Type': mimeType },
        })
        completePayload.upload_content_type = mimeType
      } else {
        throw new Error('上传入口未准备好，请刷新后重试；如仍失败请联系管理员。')
      }
      const completeRes = await taskAssetsApi.completeTaskCreateUploadSession(
        parsed.sessionId,
        completePayload,
        options?.signal,
      )
      const body = completeRes.data as unknown as Record<string, unknown>
      const data = (body?.data && typeof body.data === 'object' ? body.data : body) as Record<string, unknown>
      const ref = data?.ref_object as ReferenceFileRef | undefined
      if (!ref?.asset_id) throw new Error('参考图上传完成但未返回 ref_object')
      return ref
    } catch (err) {
      await taskAssetsApi.abortTaskCreateUploadSession(parsed.sessionId, {
        ...(parsed.ossDirect?.object_key ? { oss_object_key: parsed.ossDirect.object_key } : {}),
        ...(parsed.ossDirect?.upload_id ? { oss_upload_id: parsed.ossDirect.upload_id } : {}),
      }).catch(() => undefined)
      throw err
    }
  }
  const uploaded = await uploadFileViaAssetSession(
    taskId,
    file,
    {
      asset_kind: 'reference',
      asset_id: options?.assetId,
      target_sku_code: options?.targetSkuCode,
      owner_module_key: options?.ownerModuleKey,
      upload_policy: options?.uploadPolicy,
      remark: file.name,
    },
    {
      signal: options?.signal,
      retouchRequirementId: options?.retouchRequirementId,
    },
  )
  return toReferenceFileRef(uploaded, file)
}
