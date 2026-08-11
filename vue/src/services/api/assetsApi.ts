/**
 * 资产 API（canonical）
 * - 列表：GET /v1/tasks/{id}/assets、GET /v1/assets
 * - 上传：POST /v1/assets/upload-sessions + complete/cancel
 * - 元数据：GET /v1/assets/{id}/download、GET /v1/assets/{id}/preview
 */

import axios from 'axios'
import http from '@/services/http'
import type { BackendAsset, BackendAssetVersion } from '@/services/apiTypes'

export type AssetKind = 'reference' | 'source' | 'delivery' | 'preview' | 'design_thumb' | 'erp_product_image'

/** POST /v1/assets/upload-sessions 请求体（新集成，不传 upload_mode） */
export interface CreateAssetUploadSessionPayload {
  task_id?: string | number
  /** Existing design asset id. When present, the upload replaces that asset's current version. */
  asset_id?: string | number
  asset_kind: AssetKind
  /** 规范字段名 */
  file_name: string
  expected_size?: number
  mime_type?: string
  file_hash?: string
  remark?: string
  reason?: string
  source_asset_id?: string | number
  /** P 图需求明细 ID；非空表示资产绑定到该条 retouch requirement */
  retouch_requirement_id?: number
  target_sku_code?: string
  owner_module_key?: string
  upload_policy?: 'append_only' | 'replace' | string
}

/** 创建会话返回 */
export interface AssetUploadSessionCreateResponse {
  data?: {
    session?: {
      id?: string
      session_id?: string
      expected_size?: number | null
      session_status?: string | null
      upload_status?: string | null
    }
    oss_direct?: {
      /** 与 `upload_strategy` 二选一；部分存量上传记录只保存 `mode`。 */
      mode?: 'single_part' | 'multipart' | string
      upload_strategy?: 'single_part' | 'multipart' | string
      upload_url?: string | null
      part_upload_url_template?: string | null
      method?: string | null
      headers?: Record<string, string>
      request_headers?: Record<string, string>
      parts_total?: number | null
      part_size_hint?: number | null
      /** 与 `part_size_hint` 等价，OSS 直传常见字段名 */
      part_size?: number | null
      expected_size?: number | null
      required_upload_content_type?: string | null
      bucket?: string | null
      endpoint?: string | null
      object_key?: string | null
      upload_id?: string | null
      expires_at?: string | null
      parts?: Array<{
        part_number: number
        upload_url: string
        method?: string | null
        expires_at?: string | null
      }>
      part_urls?: string[] | Record<string, string>
    }
    /** remote 上传通道（oss_direct 不可用时的 fallback） */
    remote?: {
      upload_url?: string | null
      required_upload_content_type?: string | null
      method?: string | null
    }
    /** 部分响应在 `data` 根级重复 `required_upload_content_type` */
    required_upload_content_type?: string | null
    upload_strategy?: 'single_part' | 'multipart' | string
    complete_endpoint?: string | null
    cancel_endpoint?: string | null
  }
}

export interface AssetUploadProgress {
  loaded: number
  total: number
  percent: number
  partsCompleted?: number
  partsTotal?: number
}

export interface AssetUploadOptions {
  signal?: AbortSignal
  onProgress?: (progress: AssetUploadProgress) => void
  progressByteTotal?: number
  method?: string
  extraHeaders?: Record<string, string>
  /**
   * 与 POST /v1/assets/upload-sessions 请求体中的 `mime_type` 一致；
   * 直传 OSS 时用于 Blob Content-Type，避免与签名不一致（SignatureDoesNotMatch）。
   */
  mimeTypeForUpload?: string
}

/** complete 响应体 */
export interface AssetCenterUploadCompleteData {
  session?: {
    id?: string
    session_id?: string
    session_status?: string | null
    upload_status?: string | null
    expected_size?: number | null
  }
  asset?: BackendAsset
  version?: BackendAssetVersion
}

export interface AssetCenterUploadCompleteResponse {
  data?: AssetCenterUploadCompleteData
}

export interface AssetUploadCompleteOssPart {
  part_number: number
  etag: string
}

export interface CompleteAssetUploadSessionPayload {
  file_hash?: string
  remark?: string
  reason?: string
  /** OSS multipart finalize 字段 */
  oss_upload_id?: string
  oss_object_key?: string
  oss_parts?: AssetUploadCompleteOssPart[]
  upload_content_type?: string
}

/** 创建前参考图 fallback（当前后端仍要求 create-session 必填 task_id） */
export interface UploadReferenceForNewTaskResponse {
  data?: ReferenceFileRef
}

/** 下载元数据（GET /v1/assets/{id}/download） */
export interface AssetDownloadMeta {
  download_mode?: string
  download_url?: string | null
  expires_at?: string | null
  access_hint?: string | null
  preview_available?: boolean
  filename?: string
  file_size?: number
  mime_type?: string
}

export interface ReferenceFileRef {
  asset_id?: string
  ref_id?: string
  upload_request_id?: string
  filename?: string
  mime_type?: string
  file_size?: number | null
  /** 规范业务文件访问入口；勿使用已废弃的 `url` */
  download_url?: string | null
  source?: string
  status?: string
  /** 后端 v1.18+：OSS object key，用于 re-sign；不要拿来解析文件名 */
  storage_key?: string
  /** 后端 v1.18+：download_url 的过期时间，ISO8601 with tz（如 +08:00） */
  download_url_expires_at?: string
  [key: string]: unknown
}

function emitUploadProgress(options: AssetUploadOptions | undefined, loaded = 0, total?: number) {
  if (!options?.onProgress) return
  const safeTotal = typeof total === 'number' && total > 0 ? total : loaded
  options.onProgress({
    loaded,
    total: safeTotal,
    percent: safeTotal > 0 ? Math.min(100, Math.round((loaded / safeTotal) * 100)) : 0,
  })
}

export function normalizeAssetCenterCompleteData(body: unknown): AssetCenterUploadCompleteData {
  const root = (body && typeof body === 'object' ? body : {}) as Record<string, unknown>
  const inner = (root.data && typeof root.data === 'object' ? root.data : root) as Record<string, unknown>
  return {
    session: inner.session as AssetCenterUploadCompleteData['session'],
    asset: inner.asset as BackendAsset | undefined,
    version: inner.version as BackendAssetVersion | undefined,
  }
}

export function assertAssetCenterUploadCompleteOk(parsed: AssetCenterUploadCompleteData): void {
  const s = parsed.session
  if (!s) return
  const st = String(s.session_status ?? '').trim()
  const us = String(s.upload_status ?? '').trim()
  if (st && st !== 'completed') {
    throw new Error(`上传确认异常：session_status 为「${st}」，期望 completed`)
  }
  if (us && us !== 'uploaded') {
    throw new Error(`上传确认异常：upload_status 为「${us}」，期望 uploaded`)
  }
}

export function formatMultipartPartsLabel(progress: AssetUploadProgress): string {
  if (progress.partsTotal == null || progress.partsTotal <= 0) return ''
  const c = progress.partsCompleted ?? 0
  return `分片 ${c}/${progress.partsTotal}`
}

export interface AssetBatchDownloadPayload {
  asset_ids?: number[]
  resource_ids?: string[]
  naming_mode?: 'original' | 'business'
}

export interface AssetBatchDownloadItem {
  asset_id: number
  resource_id?: string
  source_type?: 'system' | 'external' | string
  task_id: number
  filename: string
  file_size: number
  mime_type?: string
  download_url: string
  expires_at?: string | null
}

export interface AssetBatchDownloadFailure {
  asset_id: number
  resource_id?: string
  source_type?: 'system' | 'external' | string
  task_id?: number
  filename?: string
  reason: string
}

export interface AssetBatchDownloadManifest {
  items: AssetBatchDownloadItem[]
  failures?: AssetBatchDownloadFailure[]
  success_count: number
  failure_count: number
  total_size: number
  expires_at?: string | null
}

export interface AssetBatchDownloadResponse {
  data?: AssetBatchDownloadManifest
}

export type AssetBatchSearchStatus = 'matched' | 'not_found' | 'error'

export interface AssetBatchSearchPayload {
  terms: string[]
  format_filter?: 'jpg_png' | 'jpg' | 'png' | 'tif' | 'webp' | 'image' | 'design' | 'pdf' | 'archive' | 'all'
  asset_kind?: 'auto' | 'all' | 'delivery' | 'reference' | 'source' | 'preview' | 'other'
}

export interface AssetBatchSearchResult {
  term: string
  status: AssetBatchSearchStatus
  message: string
  candidates: number
  package_folder?: string
  asset?: BackendAsset
  assets?: BackendAsset[]
}

export interface AssetBatchSearchManifest {
  results: AssetBatchSearchResult[]
  matched_count: number
  failed_count: number
}

export interface AssetBatchSearchResponse {
  data?: AssetBatchSearchManifest
}

export interface AssetExcelPackageRow {
  row_number?: number
  order_no: string
  sku_code: string
  sku_name?: string
  quantity: number
  address?: string
  keyword?: string
}

export interface AssetExcelPackageItem {
  row_number?: number
  order_no: string
  sku_code: string
  sku_name?: string
  quantity: number
  asset_id: number
  resource_id?: string
  source_type?: string
  task_id: number
  task_no?: string
  filename: string
  file_size: number
  mime_type?: string
  download_url: string
  address?: string
  origin_path?: string
  package_folder?: string
  expires_at?: string | null
}

export interface AssetExcelPackageFailure {
  row_number?: number
  order_no?: string
  sku_code?: string
  sku_name?: string
  quantity?: number
  address?: string
  reason: string
  message: string
}

export interface AssetExcelPackageManifest {
  items: AssetExcelPackageItem[]
  failures?: AssetExcelPackageFailure[]
  success_count: number
  failure_count: number
  total_files: number
  total_size: number
  expires_at?: string | null
}

export interface AssetExcelPackagePreviewResponse {
  data?: AssetExcelPackageManifest
}

export type AssetExcelPackageJobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'expired'

export interface AssetExcelPackageJob {
  job_id: string
  status: AssetExcelPackageJobStatus
  total_count: number
  processed_count: number
  failed_count: number
  error_message?: string
  download_url?: string
  filename?: string
  manifest?: AssetExcelPackageManifest
  created_at: string
  started_at?: string | null
  finished_at?: string | null
  expires_at?: string | null
}

export interface AssetExcelPackageJobResponse {
  data?: AssetExcelPackageJob
}

export type AssetExcelPackageFormat = 'tif' | 'jpg' | 'png' | 'jpg_png' | 'image'

export const assetsApi = {
  /**
   * 任务上下文资产列表
   * GET /v1/tasks/{id}/assets
   */
  list: (taskId: string, signal?: AbortSignal) => http.get<BackendAsset[]>(`/v1/tasks/${taskId}/assets`, { signal }),

  /**
   * 创建资产上传会话（后端返回 upload_strategy 与远端计划）
   * POST /v1/assets/upload-sessions
   */
  createAssetUploadSession: (payload: CreateAssetUploadSessionPayload, signal?: AbortSignal) =>
    http.post<AssetUploadSessionCreateResponse>('/v1/assets/upload-sessions', payload, { signal }),

  /**
   * GET /v1/assets/upload-sessions/{session_id}
   */
  getUploadSessionStatus: (sessionId: string, signal?: AbortSignal) =>
    http.get<AssetUploadSessionCreateResponse>(`/v1/assets/upload-sessions/${sessionId}`, { signal }),

  /**
   * POST /v1/assets/upload-sessions/{session_id}/complete
   */
  completeAssetUploadSession: (sessionId: string, payload?: CompleteAssetUploadSessionPayload, signal?: AbortSignal) =>
    http.post<AssetCenterUploadCompleteResponse>(`/v1/assets/upload-sessions/${sessionId}/complete`, payload ?? {}, {
      signal,
    }),

  completeAssetUploadSessionAtEndpoint: (
    endpoint: string,
    payload?: CompleteAssetUploadSessionPayload,
    signal?: AbortSignal,
  ) =>
    http.post<AssetCenterUploadCompleteResponse>(endpoint, payload ?? {}, {
      signal,
    }),

  /**
   * POST /v1/assets/upload-sessions/{session_id}/cancel
   */
  cancelAssetUploadSession: (sessionId: string, payload?: Record<string, unknown>, signal?: AbortSignal) =>
    http.post(`/v1/assets/upload-sessions/${sessionId}/cancel`, payload ?? {}, {
      signal,
    }),

  batchSearchAssets: (payload: AssetBatchSearchPayload, signal?: AbortSignal) =>
    http.post<AssetBatchSearchResponse>('/v1/assets/search/batch', payload, {
      signal,
    }),

  batchDownload: (
    assetRefs: Array<number | string>,
    options?: { namingMode?: 'original' | 'business'; signal?: AbortSignal },
  ) => {
    const assetIds: number[] = []
    const resourceIds: string[] = []
    for (const ref of assetRefs) {
      if (typeof ref === 'number' && Number.isInteger(ref) && ref > 0) {
        assetIds.push(ref)
        continue
      }
      const value = String(ref ?? '').trim()
      if (value) resourceIds.push(value)
    }
    return http.post<AssetBatchDownloadResponse>(
      '/v1/assets/batch-download',
      {
        ...(assetIds.length ? { asset_ids: assetIds } : {}),
        ...(resourceIds.length ? { resource_ids: resourceIds } : {}),
        naming_mode: options?.namingMode,
      } as AssetBatchDownloadPayload,
      { signal: options?.signal },
    )
  },

  excelPackagePreview: (
    rows: AssetExcelPackageRow[],
    formatFilter: AssetExcelPackageFormat = 'image',
    signal?: AbortSignal,
  ) =>
    http.post<AssetExcelPackagePreviewResponse>(
      '/v1/assets/excel-package/preview',
      { rows, format_filter: formatFilter },
      { signal },
    ),

  excelPackagePreviewFile: (file: File, formatFilter: AssetExcelPackageFormat = 'image', signal?: AbortSignal) => {
    const form = new FormData()
    form.append('file', file)
    form.append('format_filter', formatFilter)
    return http.post<AssetExcelPackagePreviewResponse>('/v1/assets/excel-package/preview-file', form, {
      signal,
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  createExcelPackageJob: (
    rows: AssetExcelPackageRow[],
    formatFilter: AssetExcelPackageFormat = 'image',
    signal?: AbortSignal,
  ) =>
    http.post<AssetExcelPackageJobResponse>(
      '/v1/assets/excel-package/jobs',
      { rows, format_filter: formatFilter },
      { signal },
    ),

  createExcelPackageFileJob: (file: File, formatFilter: AssetExcelPackageFormat = 'image', signal?: AbortSignal) => {
    const form = new FormData()
    form.append('file', file)
    form.append('format_filter', formatFilter)
    return http.post<AssetExcelPackageJobResponse>('/v1/assets/excel-package/jobs/file', form, {
      signal,
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  getExcelPackageJob: (jobId: string, signal?: AbortSignal) =>
    http.get<AssetExcelPackageJobResponse>(`/v1/assets/excel-package/jobs/${encodeURIComponent(jobId)}`, { signal }),

  /** GET /v1/assets/{id} */
  getAsset: (assetId: string, signal?: AbortSignal) => http.get<BackendAsset>(`/v1/assets/${assetId}`, { signal }),

  /**
   * GET /v1/assets/{id}/download
   * 返回 JSON 元数据（含 download_url）；字节流由 download_url 指向的代理路由提供。
   */
  getAssetDownloadMeta: (assetId: string, signal?: AbortSignal) =>
    http.get<{ data?: AssetDownloadMeta }>(`/v1/assets/${assetId}/download`, {
      signal,
    }),

  /** GET /v1/assets/{id}/preview */
  getAssetPreviewMeta: (assetId: string, signal?: AbortSignal) =>
    http.get<{ data?: AssetDownloadMeta }>(`/v1/assets/${assetId}/preview`, {
      signal,
    }),

  deleteAsset: (assetId: string, payload: { reason: string }, signal?: AbortSignal) =>
    http.delete(`/v1/assets/${assetId}`, { data: payload, signal }),

  /**
   * 创建前参考图单文件 fallback
   * POST /v1/tasks/reference-upload
   */
  uploadReferenceForNewTask: (file: File, signal?: AbortSignal) => {
    const form = new FormData()
    form.append('file', file)
    return http.post<UploadReferenceForNewTaskResponse>('/v1/tasks/reference-upload', form, {
      timeout: 90_000,
      signal,
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  uploadToRemoteUrl: (uploadUrl: string, file: File, options?: AssetUploadOptions) => {
    const method = (options?.method ?? 'PUT').toUpperCase()
    const headers: Record<string, string> = {
      'Content-Type': file.type || 'application/octet-stream',
      ...(options?.extraHeaders ?? {}),
    }
    return axios.request({
      method,
      url: uploadUrl,
      data: file,
      headers,
      signal: options?.signal,
      onUploadProgress: (event) => emitUploadProgress(options, event.loaded, event.total),
    })
  },
}
