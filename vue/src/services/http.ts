import { isOaApiUrl, oaResponseError, showOaRequestError } from '@/oa/requestErrors'
/**
 * 统一 HTTP 客户端（axios 封装）
 *
 * 核心设计：
 * - baseURL 固定为 '/'（相对路径），开发环境由 vite proxy 转发到后端，
 *   生产环境由 nginx proxy_pass 转发（见 deploy/nginx.conf.example）
 * - 后续版本升级只需改 vite.config.ts 中的 proxy target，
 *   此文件和所有 API 调用代码无需任何修改
 */

import { usageRequestHeaders } from '@/shared/telemetry/context'
import axios, { type AxiosResponse, type AxiosError } from 'axios'
import { parseApiErrorPayload, resolveApiUserMessage } from '@/utils/api-message-zh'
import { createMockAdapter, isMockEnabled } from '@/mocks'
import { getOASessionId, markOASession, OA_LEGACY_TOKEN_KEY, OA_SESSION_EXPIRED, oaCookieSessionEnabled } from '@/services/oaSession'

export const TOKEN_STORAGE_KEY = oaCookieSessionEnabled ? OA_LEGACY_TOKEN_KEY : 'access_token'

// OA uses the return value only to associate client state with a session.
export function getToken(): string | null {
  return oaCookieSessionEnabled ? getOASessionId() : getBearerToken()
}

export function getBearerToken(): string | null {
  return oaCookieSessionEnabled ? null : localStorage.getItem(TOKEN_STORAGE_KEY)
}

export function setToken(token: string): void {
  if (oaCookieSessionEnabled) return
  if (getToken() !== token && typeof window !== 'undefined') window.dispatchEvent(new Event('asset-media-session-reset'))
  localStorage.setItem(TOKEN_STORAGE_KEY, token)
}

export function clearToken(): void {
  if (oaCookieSessionEnabled) {
    markOASession(false)
    return
  }
  localStorage.removeItem(TOKEN_STORAGE_KEY)
  if (typeof window !== 'undefined') window.dispatchEvent(new Event('asset-media-session-reset'))
}

const http = axios.create({
  // 使用相对路径，开发环境由 Vite dev proxy 转发到后端，
  // 生产环境由 nginx proxy_pass 转发（见 deploy/nginx.conf.example）
  baseURL: '/',
  timeout: 30_000,
  headers: {
    'Content-Type': 'application/json',
  },
  adapter: isMockEnabled() ? createMockAdapter() : undefined,
})

export interface HttpAppError extends Error {
  oaSessionId?: string | null
  responseData?: unknown
  status?: number
  code?: string
  denyCode?: string
  traceId?: string
  fields?: unknown
}

function withHttpAppError(
  message: string,
  extra?: {
    oaSessionId?: string | null
    responseData?: unknown
    status?: number
    code?: string
    denyCode?: string
    traceId?: string
    fields?: unknown
  },
): HttpAppError {
  const e = new Error(message) as HttpAppError
  if (extra?.oaSessionId !== undefined) e.oaSessionId = extra.oaSessionId
  if (extra?.responseData !== undefined) e.responseData = extra.responseData
  if (extra?.status !== undefined) e.status = extra.status
  if (extra?.code !== undefined) e.code = extra.code
  if (extra?.denyCode !== undefined) e.denyCode = extra.denyCode
  if (extra?.traceId !== undefined) e.traceId = extra.traceId
  if (extra?.fields !== undefined) e.fields = extra.fields
  return e
}

function pickValidationFields(data: unknown): unknown {
  if (!data || typeof data !== 'object') return undefined
  const root = Array.isArray(data) ? data[0] : data
  if (!root || typeof root !== 'object') return undefined
  const o = root as Record<string, unknown>
  const err = o.error && typeof o.error === 'object' ? (o.error as Record<string, unknown>) : undefined
  return err?.fields ?? err?.details ?? o.fields ?? o.details
}

// ─── 请求拦截器 ───────────────────────────────────────────────────────────────
type OASessionRequest = { oaSessionId?: string | null }

http.interceptors.request.use((config) => {
  if (oaCookieSessionEnabled) (config as OASessionRequest).oaSessionId = getOASessionId()
  const token = getBearerToken()
  if (token) {
    config.headers['Authorization'] = `Bearer ${token}`
  }
  if (config.url?.startsWith('/v1/') || config.url?.startsWith('/api/')) {
    for (const [name, value] of Object.entries(usageRequestHeaders())) config.headers[name] = value
  }
  if (import.meta.env.VITE_OA_FEATURES === 'true' && isOaApiUrl(config.url)) {
    config.headers['X-OA-Error-Format'] = 'v2'
  }
  config.headers['X-Frontend-Version'] = import.meta.env.VITE_APP_VERSION ?? 'v1'

  return config
})

function activeOASessionFailed(url: string | undefined, requested: string | null | undefined): boolean {
  if (!requested || requested !== getOASessionId()) return false
  if (url === '/v1/auth/login' || url === '/v1/auth/register' || url === '/v1/auth/asset-cookie') return false
  return url?.startsWith('/v1/') === true
}

// ─── 响应拦截器 ───────────────────────────────────────────────────────────────
http.interceptors.response.use(
  (response: AxiosResponse) => response,
  async (error: AxiosError) => {
    const status = error.response?.status
    const oaRequest = import.meta.env.VITE_OA_FEATURES === 'true' && isOaApiUrl(error.config?.url)

    if (status === 401) {
      if (!oaCookieSessionEnabled) {
        clearToken()
      } else if (activeOASessionFailed(error.config?.url, (error.config as OASessionRequest | undefined)?.oaSessionId)) {
        window.dispatchEvent(new Event(OA_SESSION_EXPIRED))
      }
    } else if (status === 403 && !oaRequest) {
      console.warn('[http] 403 权限不足:', error.config?.url)
    }

    if (oaRequest && error.response?.data instanceof Blob) {
      const blob = error.response.data
      if (blob.size <= 64 * 1024 && blob.type.startsWith('application/json')) {
        try { error.response.data = JSON.parse(await blob.text()) } catch { /* Use the status and correlation header. */ }
      }
    }
    const parsed = parseApiErrorPayload(error)
    const oaError = oaRequest ? oaResponseError(error) : undefined
    const oaSessionId = (error.config as OASessionRequest | undefined)?.oaSessionId
    const method = error.config?.method?.toUpperCase()
    if (oaError && error.code !== 'ERR_CANCELED' && error.config?.url !== '/oa-api/session/migrate' && error.config?.url !== '/v1/auth/me' && (
      (status !== undefined && status >= 500) ||
      (method && method !== 'GET' && method !== 'HEAD' && error.config?.url !== '/v1/auth/asset-cookie')
    )) showOaRequestError({ ...oaError, oaSessionId })

    return Promise.reject(
      withHttpAppError(oaError?.message ?? resolveApiUserMessage(error), {
        oaSessionId,
        responseData: error.response?.data,
        status,
        code: parsed.code,
        denyCode: parsed.denyCode,
        traceId: oaRequest ? oaError?.traceId : parsed.traceId,
        fields: status === 422 ? pickValidationFields(error.response?.data) : undefined,
      }),
    )
  },
)

export default http
