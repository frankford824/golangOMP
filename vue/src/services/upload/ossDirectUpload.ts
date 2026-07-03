import axios from 'axios'
import type { AssetUploadOptions } from '@/services/api/assetsApi'

export interface OssDirectPlan {
  /** 与后端 `upload_strategy` 或 `mode`（如 single_part / multipart）对齐 */
  mode?: string | null
  upload_strategy?: string | null
  upload_url?: string | null
  part_upload_url_template?: string | null
  part_urls?: string[]
  parts_total?: number | null
  /** 兼容 `part_size_hint` 与运行时常见的 `part_size` */
  part_size_hint?: number | null
  method?: string | null
  headers?: Record<string, string> | null
  required_upload_content_type?: string | null
  object_key?: string | null
  upload_id?: string | null
}

export interface OssUploadedPart {
  part_number: number
  etag: string
}

export interface RemoteUploadPlan {
  upload_url: string
  required_upload_content_type?: string
  method?: string
}

export interface OssDirectUploadResult {
  mode: 'single_part' | 'multipart' | 'unknown'
  oss_upload_id?: string
  oss_object_key?: string
  upload_content_type?: string
  oss_parts?: OssUploadedPart[]
}

function normalizeHeaders(input: unknown): Record<string, string> {
  if (!input || typeof input !== 'object') return {}
  const result: Record<string, string> = {}
  for (const [k, v] of Object.entries(input as Record<string, unknown>)) {
    if (v == null) continue
    result[k] = String(v)
  }
  return result
}

function readResponseHeader(headers: unknown, name: string): string | undefined {
  if (!headers || typeof headers !== 'object') return undefined
  const maybeGetter = (headers as { get?: unknown }).get
  if (typeof maybeGetter === 'function') {
    const value = maybeGetter.call(headers, name) ?? maybeGetter.call(headers, name.toLowerCase())
    if (value != null) {
      const trimmed = String(value).trim()
      if (trimmed) return trimmed
    }
  }
  const record = headers as Record<string, unknown>
  const lower = name.toLowerCase()
  for (const [key, value] of Object.entries(record)) {
    if (key.toLowerCase() !== lower || value == null) continue
    const trimmed = String(value).trim()
    if (trimmed) return trimmed
  }
  return undefined
}

function strOrUndef(v: unknown): string | undefined {
  if (typeof v !== 'string') return undefined
  const t = v.trim()
  return t || undefined
}

function numOrUndef(v: unknown): number | undefined {
  if (typeof v === 'number' && Number.isFinite(v) && v > 0) return Math.floor(v)
  if (typeof v === 'string' && v.trim()) {
    const n = parseInt(v, 10)
    if (Number.isFinite(n) && n > 0) return n
  }
  return undefined
}

function emitProgress(
  options: AssetUploadOptions | undefined,
  loaded: number,
  total: number,
  parts?: { completed: number; total: number },
) {
  if (!options?.onProgress) return
  const safeTotal = total > 0 ? total : loaded
  options.onProgress({
    loaded,
    total: safeTotal,
    percent: safeTotal > 0 ? Math.min(100, Math.round((loaded / safeTotal) * 100)) : 0,
    partsCompleted: parts?.completed,
    partsTotal: parts?.total,
  })
}

function normalizeMimeType(mimeType: string | undefined): string {
  return (mimeType ?? '').trim().toLowerCase()
}

function replacePartPlaceholder(template: string, partNo: number): string {
  return template
    .replace(/%7Bpart_no%7D/gi, String(partNo))
    .replace(/%7BpartNo%7D/gi, String(partNo))
    .replace(/\{part_no\}/gi, String(partNo))
    .replace(/\{partNo\}/gi, String(partNo))
}

function resolvePartUrls(raw: unknown): string[] {
  if (Array.isArray(raw)) {
    return raw
      .map((item) => (typeof item === 'string' ? item.trim() : ''))
      .filter(Boolean)
  }
  if (raw && typeof raw === 'object') {
    return Object.entries(raw as Record<string, unknown>)
      .sort((a, b) => Number(a[0]) - Number(b[0]))
      .map(([, value]) => (typeof value === 'string' ? value.trim() : ''))
      .filter(Boolean)
  }
  return []
}

function slicePart(
  file: File,
  partNo1Based: number,
  partsTotal: number,
  partSizeHint?: number | null,
  mimeType?: string,
): Blob {
  const mime = mimeType?.trim() || file.type || undefined
  const size = file.size
  if (partSizeHint != null && partSizeHint > 0) {
    const start = (partNo1Based - 1) * partSizeHint
    if (start >= size) return new Blob([])
    const end =
      partNo1Based >= partsTotal ? size : Math.min(start + partSizeHint, size)
    return file.slice(start, end, mime)
  }

  const base = Math.floor(size / partsTotal)
  const rem = size % partsTotal
  let start = 0
  for (let i = 1; i < partNo1Based; i++) {
    start += base + (i <= rem ? 1 : 0)
  }
  const len = base + (partNo1Based <= rem ? 1 : 0)
  return file.slice(start, start + len, mime)
}

export function parseOssDirectPlan(body: unknown): {
  sessionId: string
  ossDirect: OssDirectPlan | null
  remote: RemoteUploadPlan | null
  completeEndpoint: string | null
  expectedSize?: number
} {
  const unwrap =
    body &&
    typeof body === 'object' &&
    'data' in body &&
    (body as { data: unknown }).data != null
      ? (body as { data: unknown }).data
      : body

  const rec = (unwrap && typeof unwrap === 'object' ? unwrap : {}) as Record<string, unknown>
  const sessionRaw = (rec.session as Record<string, unknown> | undefined) ?? rec
  const ossRaw = (rec.oss_direct as Record<string, unknown> | undefined) ?? null
  const sessionId = String(sessionRaw.id ?? sessionRaw.session_id ?? '').trim()

  const remoteRaw = (rec.remote as Record<string, unknown> | undefined) ?? null
  let remote: RemoteUploadPlan | null = null
  if (remoteRaw) {
    const remoteUrl = strOrUndef(remoteRaw.upload_url)
    if (remoteUrl) {
      remote = {
        upload_url: remoteUrl,
        required_upload_content_type: strOrUndef(remoteRaw.required_upload_content_type),
        method: strOrUndef(remoteRaw.method),
      }
    }
  }
  const completeEndpoint = strOrUndef(rec.complete_endpoint) ?? null

  if (!ossRaw) {
    return {
      sessionId,
      ossDirect: null,
      remote,
      completeEndpoint,
      expectedSize: numOrUndef(sessionRaw.expected_size),
    }
  }

  const headers = {
    ...normalizeHeaders(ossRaw.request_headers),
    ...normalizeHeaders(ossRaw.headers),
  }
  const partUrls = resolvePartUrls(ossRaw.part_urls)
  const nestedParts = Array.isArray(ossRaw.parts)
    ? (ossRaw.parts as Array<Record<string, unknown>>)
        .sort((a, b) => Number(a.part_number ?? 0) - Number(b.part_number ?? 0))
        .map((item) => String(item.upload_url ?? '').trim())
        .filter(Boolean)
    : []

  const expectedSize =
    numOrUndef(sessionRaw.expected_size) ?? numOrUndef(ossRaw.expected_size)

  const strategyFromApi =
    strOrUndef(ossRaw.upload_strategy) ??
    strOrUndef(ossRaw.mode) ??
    strOrUndef(rec.upload_strategy)

  const partSizeHint =
    numOrUndef(ossRaw.part_size_hint) ?? numOrUndef(ossRaw.part_size)

  const partsTotalFromParts = Array.isArray(ossRaw.parts) ? ossRaw.parts.length : undefined

  const requiredMime =
    strOrUndef(ossRaw.required_upload_content_type) ??
    strOrUndef(rec.required_upload_content_type)

  return {
    sessionId,
    expectedSize,
    remote,
    completeEndpoint,
    ossDirect: {
      mode: strOrUndef(ossRaw.mode),
      upload_strategy: strategyFromApi,
      upload_url: strOrUndef(ossRaw.upload_url),
      part_upload_url_template: strOrUndef(ossRaw.part_upload_url_template),
      part_urls: nestedParts.length ? nestedParts : partUrls,
      parts_total: numOrUndef(ossRaw.parts_total) ?? partsTotalFromParts,
      part_size_hint: partSizeHint,
      method: strOrUndef(ossRaw.method),
      headers,
      required_upload_content_type: requiredMime,
      object_key: strOrUndef(ossRaw.object_key) ?? strOrUndef(ossRaw.oss_object_key),
      upload_id: strOrUndef(ossRaw.upload_id) ?? strOrUndef(ossRaw.oss_upload_id),
    },
  }
}

async function putBlob(
  url: string,
  blob: Blob,
  method: string,
  headers: Record<string, string> | undefined,
  options: AssetUploadOptions | undefined,
  byteTotal: number,
  uploadedBefore: number,
  parts?: { completed: number; total: number },
): Promise<string | undefined> {
  const res = await axios.request({
    method,
    url,
    data: blob,
    headers,
    signal: options?.signal,
    onUploadProgress: (ev) => {
      emitProgress(options, uploadedBefore + ev.loaded, byteTotal, parts)
    },
  })
  const etagRaw = readResponseHeader(res.headers, 'ETag')
  if (etagRaw == null) return undefined
  const etag = String(etagRaw).trim().replace(/^"+|"+$/g, '')
  return etag || undefined
}

export async function runOssDirectUploadPlan(
  file: File,
  plan: OssDirectPlan,
  options?: AssetUploadOptions,
): Promise<OssDirectUploadResult> {
  const method = (plan.method ?? 'PUT').toUpperCase()
  const requiredUploadContentType = plan.required_upload_content_type?.trim()
  const sessionMimeType = options?.mimeTypeForUpload?.trim()
  const requiredMime = requiredUploadContentType || sessionMimeType
  const normalizedRequired = normalizeMimeType(requiredUploadContentType)
  const normalizedSessionMime = normalizeMimeType(sessionMimeType)
  if (normalizedRequired && normalizedSessionMime && normalizedRequired !== normalizedSessionMime) {
    throw new Error(
      `上传会话 Content-Type 不一致：create-session=${sessionMimeType}，oss_direct.required_upload_content_type=${requiredUploadContentType}`,
    )
  }
  const byteTotal =
    options?.progressByteTotal != null && options.progressByteTotal > 0
      ? options.progressByteTotal
      : file.size
  const headers = { ...(plan.headers ?? {}) }
  if (requiredMime) {
    headers['Content-Type'] = requiredMime
  }

  const strategy = (plan.upload_strategy ?? '').toLowerCase().trim()
  const mode = (plan.mode ?? '').toLowerCase().trim()
  const normalizedMode =
    strategy === 'multipart' || mode === 'multipart'
      ? 'multipart'
      : strategy === 'single_part' || mode === 'single_part'
        ? 'single_part'
        : 'unknown'
  const singleUrl = plan.upload_url?.trim()
  const partUrls = plan.part_urls ?? []
  const template = plan.part_upload_url_template?.trim()

  const wholeBlob = requiredMime && file.size > 0 ? file.slice(0, file.size, requiredMime) : file

  const isSingleStrategy =
    strategy === 'single_part' || strategy === 'single' || strategy === 'put_object'
  const isMultipartStrategy = strategy === 'multipart'

  if (
    isSingleStrategy ||
    (!isMultipartStrategy && singleUrl && !partUrls.length && !template)
  ) {
    if (!singleUrl) {
      throw new Error('oss_direct.upload_strategy=single_part 但缺少 upload_url')
    }
    await putBlob(singleUrl, wholeBlob, method, headers, options, byteTotal, 0, {
      completed: 0,
      total: 1,
    })
    emitProgress(options, byteTotal, byteTotal, { completed: 1, total: 1 })
    return {
      mode: normalizedMode === 'unknown' ? 'single_part' : normalizedMode,
      oss_upload_id: plan.upload_id?.trim() || undefined,
      oss_object_key: plan.object_key?.trim() || undefined,
      upload_content_type: requiredMime || undefined,
    }
  }

  const totalParts =
    (partUrls.length ? partUrls.length : plan.parts_total) ??
    (plan.part_size_hint ? Math.max(1, Math.ceil(file.size / plan.part_size_hint)) : 0)
  if (!totalParts || totalParts <= 0) {
    throw new Error('oss_direct multipart 缺少 part_urls/parts_total/part_size_hint')
  }

  let uploaded = 0
  const uploadedParts: OssUploadedPart[] = []
  for (let partNo = 1; partNo <= totalParts; partNo++) {
    const partUrl = partUrls[partNo - 1] ?? (template ? replacePartPlaceholder(template, partNo) : '')
    if (!partUrl) {
      throw new Error(`oss_direct multipart 缺少第 ${partNo} 片上传 URL`)
    }
    const blob = slicePart(file, partNo, totalParts, plan.part_size_hint, requiredMime)
    const etag = await putBlob(partUrl, blob, method, headers, options, byteTotal, uploaded, {
      completed: partNo - 1,
      total: totalParts,
    })
    if (!etag) {
      throw new Error(
        `OSS multipart 第 ${partNo} 片上传已返回，但浏览器无法读取 ETag；请检查 OSS Bucket CORS Expose-Headers 是否包含 ETag`,
      )
    }
    uploadedParts.push({ part_number: partNo, etag })
    uploaded += blob.size
    emitProgress(options, uploaded, byteTotal, { completed: partNo, total: totalParts })
  }
  return {
    mode: 'multipart',
    oss_upload_id: plan.upload_id?.trim() || undefined,
    oss_object_key: plan.object_key?.trim() || undefined,
    upload_content_type: requiredMime || undefined,
    oss_parts: uploadedParts,
  }
}
