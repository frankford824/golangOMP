import { AxiosError, type AxiosAdapter, type AxiosRequestConfig, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { ensureMockSeed } from './seed'
import { dispatchMockRequest } from './handlers'
import { startMockWsTicker } from './ws'

function parseBody(data: unknown): Record<string, unknown> | null {
  if (!data) return null
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data) as unknown
      return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : null
    } catch {
      return null
    }
  }
  if (typeof data === 'object') return data as Record<string, unknown>
  return null
}

function normalizePath(url?: string): string {
  if (!url) return ''
  try {
    const parsed = new URL(url, 'http://mock.local')
    return parsed.pathname
  } catch {
    return url
  }
}

export function isMockEnabled(): boolean {
  return import.meta.env.MODE === 'test' || import.meta.env.VITE_USE_MOCK === 'true'
}

type MockFamily =
  | 'auth'
  | 'me'
  | 'users'
  | 'org'
  | 'tasks'
  | 'task-assets'
  | 'assets'
  | 'drafts'
  | 'notifications'
  | 'batch'
  | 'erp'
  | 'cost-management'
  | 'search'

const DEFAULT_MOCK_FAMILIES: MockFamily[] = [
  'auth',
  'me',
  'users',
  'org',
  'tasks',
  'task-assets',
  'assets',
  'drafts',
  'notifications',
  'batch',
  'erp',
  'cost-management',
  'search',
]

function configuredMockFamilies(): Set<string> {
  const raw = String(import.meta.env.VITE_MOCK_FAMILIES ?? '').trim()
  if (!raw) return new Set(DEFAULT_MOCK_FAMILIES)
  return new Set(
    raw
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean),
  )
}

export function inferMockFamily(path: string): MockFamily | undefined {
  if (path.startsWith('/v1/auth/')) return 'auth'
  if (path === '/v1/me' || path.startsWith('/v1/me/')) {
    if (path.includes('/notifications')) return 'notifications'
    if (path.includes('/task-drafts')) return 'drafts'
    return 'me'
  }
  if (path.startsWith('/v1/users') || path.startsWith('/v1/access/')) return 'users'
  if (path.startsWith('/v1/org') || path.startsWith('/v1/departments/')) return 'org'
  if (path.startsWith('/v1/task-drafts')) return 'drafts'
  if (path.includes('/batch-create/')) return 'batch'
  if (path.startsWith('/v1/erp/') || path.startsWith('/v1/products') || path.startsWith('/v1/categories')) return 'erp'
  if (path.startsWith('/v1/cost-management/') || path.startsWith('/v1/cost-rules') || path.startsWith('/v1/cost-rule-bindings')) return 'cost-management'
  if (path.startsWith('/v1/design-sources/')) return 'search'
  if (path.startsWith('/v1/assets') || path.startsWith('/v1/resource-groups') || path.startsWith('/v1/tasks/reference-upload')) return 'assets'
  if (path.includes('/asset-center/')) return 'task-assets'
  if (path.startsWith('/v1/tasks/') || path === '/v1/tasks' || path.startsWith('/v1/task-board/')) return 'tasks'
  return undefined
}

export function shouldHandleWithMock(config: AxiosRequestConfig): boolean {
  if (!isMockEnabled()) return false
  const path = normalizePath(config.url)
  if (!path.startsWith('/v1/')) return false
  const family = inferMockFamily(path)
  if (!family) return false
  return configuredMockFamilies().has(family)
}

export function createMockAdapter(): AxiosAdapter {
  ensureMockSeed()
  return async (config: InternalAxiosRequestConfig): Promise<AxiosResponse> => {
    const response = dispatchMockRequest({
      method: (config.method ?? 'GET').toUpperCase(),
      path: normalizePath(config.url),
      query: (config.params ?? {}) as Record<string, unknown>,
      body: parseBody(config.data),
    })

    const axiosResponse: AxiosResponse = {
      data: response.data,
      status: response.status,
      statusText: response.status >= 200 && response.status < 300 ? 'OK' : 'ERROR',
      headers: {},
      config,
      request: undefined,
    }

    const validate = config.validateStatus ?? ((status: number) => status >= 200 && status < 300)
    if (!validate(axiosResponse.status)) {
      throw new AxiosError(
        `Request failed with status code ${axiosResponse.status}`,
        String(axiosResponse.status),
        config,
        undefined,
        axiosResponse,
      )
    }
    return axiosResponse
  }
}

export function initializeMockRuntime(): void {
  if (!isMockEnabled()) return
  ensureMockSeed()
  startMockWsTicker()
}
