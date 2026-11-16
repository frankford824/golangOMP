export type UsageApp = 'main-ops' | 'asset-workbench'
export interface UsageContext {
  app_id: UsageApp
  session_id: string
  page_view_id: string
  route_pattern: string
  page_url: string
  page_name: string
  automation: boolean
  task_id?: number
}

let context: UsageContext | undefined
let tokenReader: (() => string | null) | undefined
let ownerToken: string | null = null

export function setUsageContext(next: UsageContext | undefined, readToken?: () => string | null) {
  context = next
  if (readToken) tokenReader = readToken
  try { ownerToken = next ? tokenReader?.() ?? null : null } catch { ownerToken = null }
}

export function currentUsageContext(): UsageContext | undefined {
  // A logout/account switch must not reuse another account's page or session.
  try { if (!ownerToken || tokenReader?.() !== ownerToken) return undefined } catch { return undefined }
  return context ? { ...context } : undefined
}

export function usageRequestHeaders(): Record<string, string> {
  const current = currentUsageContext()
  if (!current) return {}
  return {
    'X-Usage-App': current.app_id,
    'X-Usage-Session': current.session_id,
    'X-Usage-Page': current.page_view_id,
    'X-Usage-Automation': current.automation ? 'true' : 'false',
  }
}

export function safeUsageURL(value: string): string {
  try {
    const url = new URL(value, window.location.origin)
    if (!['https:', 'http:'].includes(url.protocol)) return ''
    return `${url.origin}${url.pathname}`.slice(0, 512)
  } catch { return '' }
}
