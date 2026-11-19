// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { OA_SESSION_CHANGE_KEY } from '@/services/oaSession'
import { oaRequestError, showOaRequestError } from './requestErrors'

const mocks = vi.hoisted(() => ({ token: 'session-a', get: vi.fn() }))
const permissions = reactive({ currentUser: { id: '7', name: '测试用户' } })
vi.mock('@/services/http', () => ({
  default: { get: mocks.get },
  getToken: () => mocks.token,
}))
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => permissions }))

let access: typeof import('./access')
const grant = (id = 7, actions: Record<string, boolean> = { 'finance.apply': true }) => ({
  status: 200, data: { data: { id, activated: true, actions } },
})

beforeAll(async () => {
  vi.stubEnv('VITE_OA_FEATURES', 'true')
  access = await import('./access')
})
beforeEach(() => {
  vi.useFakeTimers()
  mocks.token = 'session-a'
  permissions.currentUser = { id: '7', name: '测试用户' }
  window.dispatchEvent(new Event('asset-media-session-reset'))
  mocks.get.mockReset()
  oaRequestError.value = undefined
})
afterEach(() => {
  window.dispatchEvent(new Event('asset-media-session-reset'))
  vi.useRealTimers()
})

describe('OA permission refresh', () => {
  it('coalesces checks and retries a transient failure once with a timeout above the core identity timeout', async () => {
    mocks.get.mockRejectedValueOnce(new Error('timeout')).mockResolvedValueOnce(grant())
    const view = access.useOAAccess()
    const first = access.ensureOAAccess()
    const concurrent = access.ensureOAAccess()
    expect(view.state.value).toBe('loading')
    expect(view.checking.value).toBe(true)
    expect(mocks.get).toHaveBeenCalledTimes(1)
    expect(mocks.get).toHaveBeenCalledWith('/oa-api/me', expect.objectContaining({ timeout: 7_000 }))
    await vi.advanceTimersByTimeAsync(250)
    await Promise.all([first, concurrent])
    expect(mocks.get).toHaveBeenCalledTimes(2)
    expect(view.state.value).toBe('ready')
    expect(view.can('finance.apply')).toBe(true)
  })

  it('retains only the same session’s last confirmed access during a temporary failure, for up to five minutes', async () => {
    mocks.get.mockResolvedValueOnce(grant()).mockResolvedValue({ status: 503 })
    const view = access.useOAAccess()
    await access.ensureOAAccess()
    await vi.advanceTimersByTimeAsync(30_000)
    const refresh = access.ensureOAAccess()
    await vi.advanceTimersByTimeAsync(250)
    await refresh
    expect(view.state.value).toBe('unavailable')
    expect(view.can('finance.apply')).toBe(true)
    await vi.advanceTimersByTimeAsync(269_750)
    expect(view.can('finance.apply')).toBe(false)
    expect(view.state.value).toBe('unavailable')
  })

  it('does not infer permissions on a first failure and allows an explicit retry after a rate-limit response', async () => {
    mocks.get.mockResolvedValueOnce({ status: 429 }).mockResolvedValueOnce(grant())
    const view = access.useOAAccess()
    await access.ensureOAAccess()
    expect(view.state.value).toBe('unavailable')
    expect(view.can('finance.apply')).toBe(false)
    await access.ensureOAAccess()
    expect(mocks.get).toHaveBeenCalledTimes(1)
    await access.ensureOAAccess(true)
    expect(view.state.value).toBe('ready')
    expect(view.can('finance.apply')).toBe(true)
  })

  it('shows the final real response trace for an unavailable authorization check and clears it on recovery', async () => {
    const headerId = '11111111111111111111111111111111'
    const bodyId = '22222222222222222222222222222222'
    mocks.get.mockResolvedValueOnce({ status: 503, headers: { 'x-trace-id': headerId } })
      .mockResolvedValueOnce({ status: 503, data: { error: { trace_id: bodyId } } })
      .mockResolvedValueOnce(grant())
    const check = access.ensureOAAccess()
    await vi.advanceTimersByTimeAsync(250)
    await check
    expect(oaRequestError.value?.traceId).toBe(bodyId)
    await access.ensureOAAccess(true)
    expect(oaRequestError.value).toBeUndefined()
  })

  it('clears the current error trace on cross-tab session change and leaves no trace for a transport failure', async () => {
    const headerId = '11111111111111111111111111111111'
    mocks.get.mockResolvedValueOnce({ status: 429, headers: { 'x-trace-id': headerId } })
      .mockRejectedValue(new Error('transport failure'))
    await access.ensureOAAccess()
    expect(oaRequestError.value?.traceId).toBe(headerId)
    mocks.token = 'session-b'
    window.dispatchEvent(new StorageEvent('storage', { key: OA_SESSION_CHANGE_KEY }))
    expect(oaRequestError.value).toBeUndefined()
    const check = access.ensureOAAccess()
    await vi.advanceTimersByTimeAsync(250)
    await check
    expect(oaRequestError.value).toBeUndefined()
  })

  it('keeps a later business request error when the authorization check recovers', async () => {
    const traceId = '11111111111111111111111111111111'
    mocks.get.mockResolvedValueOnce({ status: 429, headers: { 'x-trace-id': traceId } })
      .mockResolvedValueOnce(grant())
    await access.ensureOAAccess()
    const businessFailure = showOaRequestError({ message: '费用操作失败', traceId: '22222222222222222222222222222222' })
    await access.ensureOAAccess(true)
    expect(oaRequestError.value).toBe(businessFailure)
  })

  it.each([401, 403])('discards prior access immediately on explicit HTTP %s', async (status) => {
    mocks.get.mockResolvedValueOnce(grant()).mockResolvedValueOnce({ status })
    const view = access.useOAAccess()
    await access.ensureOAAccess()
    await access.ensureOAAccess(true)
    expect(view.state.value).toBe('denied')
    expect(view.can('finance.apply')).toBe(false)
    expect(mocks.get).toHaveBeenCalledTimes(2)
  })

  it('replaces an older grant with a verified response that removes the action', async () => {
    mocks.get.mockResolvedValueOnce(grant()).mockResolvedValueOnce(grant(7, {}))
    const view = access.useOAAccess()
    await access.ensureOAAccess()
    expect(view.can('finance.apply')).toBe(true)
    await access.ensureOAAccess(true)
    expect(view.state.value).toBe('ready')
    expect(view.can('finance.apply')).toBe(false)
  })

  it('discards a cached grant when another tab replaces the session and rejects a mismatched response', async () => {
    mocks.get.mockResolvedValue(grant())
    const view = access.useOAAccess()
    await access.ensureOAAccess()
    expect(view.can('finance.apply')).toBe(true)

    mocks.token = 'session-b'
    window.dispatchEvent(new StorageEvent('storage', { key: OA_SESSION_CHANGE_KEY }))
    expect(view.can('finance.apply')).toBe(false)
    permissions.currentUser = { id: '8', name: '另一个用户' }
    await access.ensureOAAccess()
    expect(view.state.value).toBe('denied')
    expect(view.can('finance.apply')).toBe(false)
  })

  it('ignores an in-flight response from a signed-out session even if its marker is reused', async () => {
    let resolve!: (value: ReturnType<typeof grant>) => void
    mocks.get.mockImplementationOnce(() => new Promise(resolver => { resolve = resolver }))
    const request = access.ensureOAAccess()
    window.dispatchEvent(new Event('asset-media-session-reset'))
    resolve(grant())
    await request
    expect(access.useOAAccess().can('finance.apply')).toBe(false)
  })
})
