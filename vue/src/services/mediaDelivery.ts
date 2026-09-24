import { ref } from 'vue'
import http, { getToken } from '@/services/http'

export type MediaState = 'ready' | 'queued' | 'processing' | 'failed' | 'unsupported' | 'missing' | 'source_disabled' | 'temporarily_unavailable'
export interface MediaAccess {
  resource_kind: 'asset' | 'task_asset' | 'external_asset' | 'client_material' | 'package'
  resource_id: string
  item_id?: number
  purpose: 'preview' | 'download'
  rendition: 'thumbnail' | 'preview' | 'original'
  delivery?: 'auto' | 'lan' | 'cloud'
  expected_source_version?: string
}
export interface MediaDeliveryInfo {
  download_mode: string
  download_url?: string | null
  filename: string
  file_size: number
  mime_type?: string
  preview_available?: boolean
  access_hint?: string
  expires_at?: string
  source_version?: string
  content_id?: string
  rendition?: string
  state?: MediaState
  job_id?: string
  request_id?: string
  retry_after?: number
  error_code?: string
  retryable?: boolean
  lan_delivery?: { state: string; url?: string; gateway_id?: string; expires_at?: string }
  cloud_delivery?: { state: string; url?: string; expires_at?: string }
  items?: MediaDeliveryInfo[]
}

const ENABLED_KEY = 'yongbo-media-lan-enabled-v1'
const GATEWAY = 'https://media-cache.yongbo.cloud'
export const lanMediaEnabled = ref(typeof localStorage !== 'undefined' && localStorage.getItem(ENABLED_KEY) === '1')
export const lanMediaBusy = ref(false)
export const lanMediaNotice = ref('')
let probeExpires = 0
let probeAvailable = false
let probeFlight: Promise<boolean> | undefined
let sessionGeneration = 0

function resetProbe() { probeExpires = 0; probeAvailable = false; probeFlight = undefined }
function resetSession() { sessionGeneration += 1; resetProbe(); previewCache.clear(); previewFlights.clear() }
if (typeof window !== 'undefined') {
  window.addEventListener('online', resetProbe)
  window.addEventListener('offline', resetProbe)
  ;(navigator as Navigator & { connection?: EventTarget }).connection?.addEventListener('change', resetProbe)
  window.addEventListener('asset-media-session-reset', resetSession)
  window.addEventListener('storage', (event) => {
    if (event.key === 'access_token') window.dispatchEvent(new Event('asset-media-session-reset'))
    if (event.key === ENABLED_KEY) { lanMediaEnabled.value = event.newValue === '1'; resetProbe() }
  })
}

export async function probeLANMedia(firstPermission = false): Promise<boolean> {
  if (!firstPermission && !lanMediaEnabled.value) return false
  if (!firstPermission && Date.now() < probeExpires) return probeAvailable
  if (probeFlight) return probeFlight
  const generation = sessionGeneration
  const controller = new AbortController()
  // The first action may wait for a browser permission dialog. Later probes
  // are bounded to two seconds and never generate repeated permission prompts.
  const timer = window.setTimeout(() => controller.abort(), firstPermission ? 60_000 : 2_000)
  const flight = (async () => {
    try {
      const capabilities = await http.get<{ data: { lan_available: boolean; gateway_id: string; gateway_url: string } }>('/v1/assets/media/capabilities', { signal: controller.signal })
      const capability = capabilities.data.data
      if (!capability.lan_available || capability.gateway_id !== 'company-nas' || capability.gateway_url.replace(/\/$/, '') !== GATEWAY) {
        if (generation === sessionGeneration) { probeAvailable = false; probeExpires = Date.now() + 30_000 }
        return false
      }
      const response = await fetch(`${GATEWAY}/edge/v1/ping`, { signal: controller.signal, credentials: 'omit', cache: 'no-store' })
      const info = response.ok ? await response.json() : undefined
      const ready = info?.gateway_id === 'company-nas' && info?.status === 'ready'
      if (generation === sessionGeneration) { probeAvailable = ready; probeExpires = Date.now() + (ready ? 300_000 : 30_000) }
      return ready
    } catch {
      if (generation === sessionGeneration) { probeAvailable = false; probeExpires = Date.now() + 30_000 }
      return false
    } finally { window.clearTimeout(timer) }
  })()
  probeFlight = flight
  try { return await flight } finally { if (probeFlight === flight) probeFlight = undefined }
}

export async function toggleLANMedia(): Promise<void> {
  if (lanMediaBusy.value) return
  if (lanMediaEnabled.value) {
    lanMediaEnabled.value = false; localStorage.removeItem(ENABLED_KEY); resetProbe()
    lanMediaNotice.value = '已切换为云端下载'; return
  }
  lanMediaBusy.value = true; lanMediaNotice.value = '请允许访问本地网络，正在连接公司文件服务…'
  try {
    if (await probeLANMedia(true)) {
      lanMediaEnabled.value = true; localStorage.setItem(ENABLED_KEY, '1')
      lanMediaNotice.value = '公司内网加速已启用'
    } else { lanMediaNotice.value = '暂未连接到公司文件服务，将继续使用云端。请确认公司网络、浏览器许可和代理设置。' }
  } finally { lanMediaBusy.value = false }
}

async function requestMedia(request: MediaAccess, signal?: AbortSignal): Promise<MediaDeliveryInfo> {
  const response = await http.post<{ data: MediaDeliveryInfo }>('/v1/assets/media/delivery', request, { signal })
  return response.data.data
}

export async function resolveMediaAccess(request: MediaAccess, signal?: AbortSignal): Promise<MediaDeliveryInfo> {
  const useLAN = request.delivery !== 'cloud' && await probeLANMedia()
  const chosen: MediaAccess = { ...request, delivery: useLAN ? 'lan' : 'cloud' }
  let info = await requestMedia(chosen, signal)
  if (!useLAN || !info.lan_delivery?.url || info.state !== 'ready') return info
  const local = new URL(info.lan_delivery.url)
  if (local.origin !== GATEWAY || info.lan_delivery.gateway_id !== 'company-nas') throw new Error('公司文件服务地址校验失败')
  const controller = new AbortController()
  const abort = () => controller.abort()
  signal?.addEventListener('abort', abort, { once: true })
  const timer = window.setTimeout(abort, 2_000)
  let fallback = false
  try {
    const head = await fetch(local.href, { method: 'HEAD', credentials: 'omit', cache: 'no-store', signal: controller.signal })
    if (head.status === 409 || head.status === 410) throw new Error('文件版本已变化或已不可用，请刷新后重试')
    if (head.status === 403) throw new Error('没有权限访问该文件')
    fallback = !head.ok
  } catch (error) {
    if (signal?.aborted) throw new DOMException('操作已取消', 'AbortError')
    if (error instanceof Error && !(error instanceof TypeError) && error.name !== 'AbortError') throw error
    fallback = true
  } finally { window.clearTimeout(timer); signal?.removeEventListener('abort', abort) }
  if (fallback) {
    probeAvailable = false; probeExpires = Date.now() + 30_000
    info = await requestMedia({ ...request, delivery: 'cloud', expected_source_version: info.source_version || request.expected_source_version }, signal)
  }
  return info
}

interface PreviewEntry { info: MediaDeliveryInfo; expires: number }
const previewCache = new Map<string, PreviewEntry>()
const previewFlights = new Map<string, Promise<MediaDeliveryInfo>>()

export async function resolveMediaPreview(request: Omit<MediaAccess, 'purpose'>, signal?: AbortSignal): Promise<MediaDeliveryInfo> {
  const generation = sessionGeneration
  // Session credentials are compared in memory only and never persisted or logged.
  const key = `${getToken() || ''}|${generation}|${request.resource_kind}|${request.resource_id}|${request.item_id || 0}|${request.expected_source_version || ''}|${request.rendition}|${lanMediaEnabled.value}`
  const cached = previewCache.get(key)
  if (cached && cached.expires > Date.now()) return cached.info
  const inflight = previewFlights.get(key)
  if (inflight) return inflight
  const pending = resolveMediaAccess({ ...request, purpose: 'preview' }, signal)
  previewFlights.set(key, pending)
  try {
    const info = await pending
    if (generation === sessionGeneration && info.state === 'ready' && info.download_url) {
      const signedExpiry = info.expires_at ? Date.parse(info.expires_at) - 30_000 : Date.now() + 60_000
      previewCache.set(key, { info, expires: Math.min(Date.now() + 60_000, signedExpiry) })
      while (previewCache.size > 512) previewCache.delete(previewCache.keys().next().value!)
    }
    return info
  } finally { if (previewFlights.get(key) === pending) previewFlights.delete(key) }
}

export function mediaMessage(info: MediaDeliveryInfo): string {
  if (info.state === 'source_disabled') return '该来源已停用，请返回素材库重新选择'
  if (info.state === 'missing') return '原文件已不可用，请刷新后重新选择'
  if (info.state === 'unsupported') return '当前格式暂不支持预览，可下载原件'
  if (info.state === 'failed') return info.retryable ? '准备失败，可稍后重试' : '该文件暂时无法生成预览或交付，请检查源文件'
  if (info.state === 'temporarily_unavailable') return '文件服务暂不可用，请稍后重试'
  return '文件正在准备中，可稍后在下载中心查看'
}
