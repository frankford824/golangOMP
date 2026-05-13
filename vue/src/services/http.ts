/**
 * 统一 HTTP 客户端（axios 封装）
 *
 * 核心设计：
 * - baseURL 固定为 '/'（相对路径），开发环境由 vite proxy 转发到后端，
 *   生产环境由 nginx proxy_pass 转发（见 deploy/nginx.conf.example）
 * - 后续版本升级只需改 vite.config.ts 中的 proxy target，
 *   此文件和所有 API 调用代码无需任何修改
 */

import axios, { type AxiosResponse, type AxiosError } from 'axios'
import { parseApiErrorPayload, resolveApiUserMessage } from '@/utils/api-message-zh'
import { createMockAdapter, isMockEnabled } from '@/mocks'

const TOKEN_KEY = 'access_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
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
    responseData?: unknown
    status?: number
    code?: string
    denyCode?: string
    traceId?: string
    fields?: unknown
  },
): HttpAppError {
  const e = new Error(message) as HttpAppError
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
http.interceptors.request.use((config) => {
  // 自动附加 Bearer token
  const token = getToken()
  if (token) {
    config.headers['Authorization'] = `Bearer ${token}`
  }
  config.headers['X-Frontend-Version'] = import.meta.env.VITE_APP_VERSION ?? 'v1'

  return config
})

// ─── 响应拦截器 ───────────────────────────────────────────────────────────────
http.interceptors.response.use(
  (response: AxiosResponse) => response,
  (error: AxiosError) => {
    const status = error.response?.status
    const parsed = parseApiErrorPayload(error)

    if (status === 401) {
      // token 无效或过期：清除本地 token；路由守卫负责统一跳转登录页。
      clearToken()
    } else if (status === 403) {
      console.warn('[http] 403 权限不足:', error.config?.url)
    }

    return Promise.reject(
      withHttpAppError(resolveApiUserMessage(error), {
        responseData: error.response?.data,
        status,
        code: parsed.code,
        denyCode: parsed.denyCode,
        traceId: parsed.traceId,
        fields: status === 422 ? pickValidationFields(error.response?.data) : undefined,
      }),
    )
  },
)

export default http
