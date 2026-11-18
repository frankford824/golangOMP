import { shallowRef } from 'vue'

const tokenKey = 'access_token'
export const tokenPresent = shallowRef(Boolean(localStorage.getItem(tokenKey)))
export const getToken = () => localStorage.getItem(tokenKey)
export function setToken(token: string) { localStorage.setItem(tokenKey, token); tokenPresent.value = true }
export function clearToken() { localStorage.removeItem(tokenKey); tokenPresent.value = false }

export type OAProfile = {
  id: string | number
  name: string
  employee_no?: string | number
  department_id?: string | number
  team_id?: string | number
  department?: string
  team?: string
  roles?: string[]
  super_admin: boolean
  actions: Record<string, boolean>
}

export class OAError extends Error {
  constructor(public status: number, message: string) { super(message) }
}

export async function request(path: string, options: RequestInit = {}) {
  const token = getToken()
  const headers = new Headers(options.headers)
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (options.body && !(options.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  const response = await fetch(path, { ...options, headers })
  if (response.ok) return response
  const body = await response.json().catch(() => ({}))
  if (response.status === 401 && path !== '/v1/auth/login') {
    clearToken()
    window.dispatchEvent(new Event('oa-session-expired'))
  }
  const fallback = response.status === 403 ? '当前账号没有此项权限' : `请求失败（${response.status}）`
  throw new OAError(response.status, (typeof body.error === 'string' ? body.error : body.error?.message) || body.message || fallback)
}

type Envelope<T> = T | { data: T }
export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await request(path, options)
  if (response.status === 204) return undefined as T
  const body = await response.json() as Envelope<T>
  return typeof body === 'object' && body !== null && 'data' in body ? body.data : body
}

export function send<T>(path: string, body: unknown, method = 'POST') {
  return api<T>(path, { method, body: JSON.stringify(body) })
}

export async function download(path: string, filename: string) {
  const response = await request(path)
  saveFile(await response.blob(), filename)
}

export function saveFile(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 10_000)
}

export function errorText(error: unknown) {
  return error instanceof Error ? error.message : '暂时无法完成操作，请稍后重试'
}

export function money(value: number | string | null | undefined) {
  if (value == null || value === '') return '—'
  const amount = /^(-?)(\d+)(?:\.(\d+))?$/.exec(String(value))
  if (!amount) return '—'
  const [, sign, whole, decimal = ''] = amount
  const cents = BigInt(whole) * 100n + BigInt(decimal.padEnd(2, '0').slice(0, 2)) + (Number(decimal[2] || 0) >= 5 ? 1n : 0n)
  const prefix = sign && cents !== 0n ? '-' : ''
  return `${prefix}¥${(cents / 100n).toLocaleString('zh-CN')}.${String(cents % 100n).padStart(2, '0')}`
}

export const opsURL = '/oa-work.html#'
export const assetURL = '/oa-assets.html#'

export function dateInChina() {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts()
  const part = (kind: string) => parts.find(item => item.type === kind)?.value
  return `${part('year')}-${part('month')}-${part('day')}`
}
