import axios from 'axios'
import type { AssetUploadOptions } from '@/services/api/assetsApi'

const MULTIPART_UPLOAD_CONCURRENCY = 4
const GLOBAL_OSS_PUT_CONCURRENCY = 6
const OSS_PUT_MAX_ATTEMPTS = 3
const OSS_PUT_RETRY_BASE_DELAY_MS = 250

let activeOssPuts = 0
const pendingOssPutAcquires: Array<{
  signal?: AbortSignal
  resolve: (release: () => void) => void
  reject: (reason?: unknown) => void
  onAbort?: () => void
}> = []

function releaseOssPutSlot() {
  activeOssPuts = Math.max(0, activeOssPuts - 1)
  while (pendingOssPutAcquires.length > 0 && activeOssPuts < GLOBAL_OSS_PUT_CONCURRENCY) {
    const pending = pendingOssPutAcquires.shift()
    if (!pending) return
    if (pending.signal?.aborted) {
      pending.reject(pending.signal.reason ?? new DOMException('Aborted', 'AbortError'))
      continue
    }
    if (pending.onAbort) pending.signal?.removeEventListener('abort', pending.onAbort)
    activeOssPuts += 1
    let released = false
    pending.resolve(() => {
      if (released) return
      released = true
      releaseOssPutSlot()
    })
  }
}

function acquireOssPutSlot(signal?: AbortSignal): Promise<() => void> {
  if (signal?.aborted) {
    return Promise.reject(signal.reason ?? new DOMException('Aborted', 'AbortError'))
  }
  if (activeOssPuts < GLOBAL_OSS_PUT_CONCURRENCY) {
    activeOssPuts += 1
    let released = false
    return Promise.resolve(() => {
      if (released) return
      released = true
      releaseOssPutSlot()
    })
  }
  return new Promise((resolve, reject) => {
    const pending = { signal, resolve, reject } as (typeof pendingOssPutAcquires)[number]
    if (signal) {
      pending.onAbort = () => {
        const index = pendingOssPutAcquires.indexOf(pending)
        if (index >= 0) pendingOssPutAcquires.splice(index, 1)
        reject(signal.reason ?? new DOMException('Aborted', 'AbortError'))
      }
      signal.addEventListener('abort', pending.onAbort, { once: true })
    }
    pendingOssPutAcquires.push(pending)
  })
}

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
  signal: AbortSignal | undefined,
  onProgress?: (loaded: number) => void,
): Promise<string | undefined> {
  const releaseSlot = await acquireOssPutSlot(signal)
  let res
  try {
    res = await axios.request({
      method,
      url,
      data: blob,
      headers,
      signal,
      onUploadProgress: (ev) => onProgress?.(Math.min(blob.size, ev.loaded)),
    })
  } finally {
    releaseSlot()
  }
  const etagRaw = readResponseHeader(res.headers, 'ETag')
  if (etagRaw == null) return undefined
  const etag = String(etagRaw).trim().replace(/^"+|"+$/g, '')
  return etag || undefined
}

function isRetryableOssPutError(err: unknown): boolean {
  if (!axios.isAxiosError(err)) return false
  if (err.code === 'ERR_CANCELED' || err.name === 'CanceledError') return false
  const status = err.response?.status
  if (status == null) return true
  return status === 408 || status === 425 || status === 429 || status >= 500
}

function waitForRetry(delayMs: number, signal?: AbortSignal): Promise<void> {
  if (signal?.aborted) {
    return Promise.reject(signal.reason ?? new DOMException('Aborted', 'AbortError'))
  }
  return new Promise((resolve, reject) => {
    const timer = globalThis.setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, delayMs)
    const onAbort = () => {
      globalThis.clearTimeout(timer)
      reject(signal?.reason ?? new DOMException('Aborted', 'AbortError'))
    }
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

async function putBlobWithRetry(
  url: string,
  blob: Blob,
  method: string,
  headers: Record<string, string> | undefined,
  signal: AbortSignal | undefined,
  onProgress?: (loaded: number) => void,
): Promise<string | undefined> {
  let lastError: unknown
  for (let attempt = 1; attempt <= OSS_PUT_MAX_ATTEMPTS; attempt++) {
    try {
      return await putBlob(url, blob, method, headers, signal, onProgress)
    } catch (err) {
      lastError = err
      if (attempt >= OSS_PUT_MAX_ATTEMPTS || !isRetryableOssPutError(err)) throw err
      onProgress?.(0)
      const jitterMs = Math.floor(Math.random() * 100)
      await waitForRetry(OSS_PUT_RETRY_BASE_DELAY_MS * 2 ** (attempt - 1) + jitterMs, signal)
    }
  }
  throw lastError
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
    throw new Error('文件格式与上传入口不一致，请重新选择文件后上传。')
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
      throw new Error('上传入口未准备好，请刷新后重试；如仍失败请联系管理员。')
    }
    await putBlobWithRetry(
      singleUrl,
      wholeBlob,
      method,
      headers,
      options?.signal,
      (loaded) => emitProgress(options, loaded, byteTotal, { completed: 0, total: 1 }),
    )
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
    throw new Error('上传入口未准备好，请刷新后重试；如仍失败请联系管理员。')
  }

  const partLoaded = new Array<number>(totalParts).fill(0)
  const uploadedParts = new Array<OssUploadedPart | undefined>(totalParts)
  let completedParts = 0
  let nextPartNo = 1
  let firstError: unknown
  const uploadController = new AbortController()
  const abortFromCaller = () => uploadController.abort(options?.signal?.reason)
  if (options?.signal?.aborted) abortFromCaller()
  else options?.signal?.addEventListener('abort', abortFromCaller, { once: true })

  const emitAggregateProgress = () => {
    const loaded = partLoaded.reduce((sum, value) => sum + value, 0)
    emitProgress(options, Math.min(byteTotal, loaded), byteTotal, {
      completed: completedParts,
      total: totalParts,
    })
  }

  const uploadPart = async (partNo: number) => {
    const partUrl = partUrls[partNo - 1] ?? (template ? replacePartPlaceholder(template, partNo) : '')
    if (!partUrl) {
      throw new Error('上传入口未准备好，请刷新后重试；如仍失败请联系管理员。')
    }
    const blob = slicePart(file, partNo, totalParts, plan.part_size_hint, requiredMime)
    const etag = await putBlobWithRetry(
      partUrl,
      blob,
      method,
      headers,
      uploadController.signal,
      (loaded) => {
        partLoaded[partNo - 1] = loaded
        emitAggregateProgress()
      },
    )
    if (!etag) {
      throw new Error('文件已上传，但浏览器无法确认上传结果，请重新上传；如仍失败请联系管理员。')
    }
    uploadedParts[partNo - 1] = { part_number: partNo, etag }
    partLoaded[partNo - 1] = blob.size
    completedParts += 1
    emitAggregateProgress()
  }

  const worker = async () => {
    while (!uploadController.signal.aborted) {
      const partNo = nextPartNo
      nextPartNo += 1
      if (partNo > totalParts) return
      try {
        await uploadPart(partNo)
      } catch (err) {
        if (firstError == null) firstError = err
        uploadController.abort(err)
        return
      }
    }
  }

  try {
    const workerCount = Math.min(MULTIPART_UPLOAD_CONCURRENCY, totalParts)
    await Promise.all(Array.from({ length: workerCount }, () => worker()))
  } finally {
    options?.signal?.removeEventListener('abort', abortFromCaller)
  }
  if (firstError != null) throw firstError
  if (options?.signal?.aborted) {
    throw options.signal.reason ?? new DOMException('Aborted', 'AbortError')
  }
  const confirmedParts = uploadedParts.filter((part): part is OssUploadedPart => part != null)
  if (confirmedParts.length !== totalParts) {
    throw new Error('文件分片未全部上传，请重新上传；如仍失败请联系管理员。')
  }
  return {
    mode: 'multipart',
    oss_upload_id: plan.upload_id?.trim() || undefined,
    oss_object_key: plan.object_key?.trim() || undefined,
    upload_content_type: requiredMime || undefined,
    oss_parts: confirmedParts,
  }
}
