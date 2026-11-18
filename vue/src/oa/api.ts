import http from '@/services/http'

type Envelope<T> = T | { data: T }
function unwrap<T>(body: Envelope<T>): T {
  return typeof body === 'object' && body !== null && 'data' in body ? body.data : body
}

export async function api<T>(url: string, options?: { method?: string; body?: FormData }): Promise<T> {
  return unwrap((await http.request<Envelope<T>>({ url, method: options?.method || 'GET', data: options?.body,
    headers: options?.body ? { 'Content-Type': undefined } : undefined,
  })).data)
}

export async function send<T>(url: string, data: unknown, method = 'POST'): Promise<T> {
  return unwrap((await http.request<Envelope<T>>({ url, method, data })).data)
}

export async function download(url: string, filename: string) {
  const response = await http.get<Blob>(url, { responseType: 'blob' })
  const encodedName = /filename\*=UTF-8''([^;]+)/i.exec(String(response.headers['content-disposition'] || ''))?.[1]
  let receivedName = ''
  if (encodedName) {
    try { receivedName = decodeURIComponent(encodedName) } catch { /* Use the caller's fallback. */ }
  }
  saveFile(response.data, receivedName || filename)
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

export function dateInChina() {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts()
  const part = (kind: string) => parts.find(item => item.type === kind)?.value
  return `${part('year')}-${part('month')}-${part('day')}`
}
