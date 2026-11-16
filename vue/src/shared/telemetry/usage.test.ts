// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beginUsageOperation, installUsageTelemetry, observeUsageOperation } from './usage'
import { currentUsageContext, usageRequestHeaders } from './context'
import { downloadAssetFileWithOriginalFilename } from '@/utils/assetFileDownload'

const stops: Array<() => void> = []
function visibility(value: 'visible' | 'hidden') {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value })
  document.dispatchEvent(new Event('visibilitychange'))
}
function setup(appId: 'main-ops' | 'asset-workbench' = 'main-ops') {
  let token: string | null = 'account-a'
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/a', name: 'page-a', component: {} },
    { path: '/b', name: 'page-b', component: {} },
    { path: '/blocked', name: 'blocked', component: {} },
    { path: '/login', component: {}, meta: { public: true } },
  ] })
  const send = vi.fn().mockResolvedValue(undefined)
  const stop = installUsageTelemetry(router, { appId, getToken: () => token, send })
  stops.push(stop)
  return { router, send, stop, setToken: (value: string | null) => { token = value }, views: () => send.mock.calls.map(c => c[0]).filter(e => e.event_type === 'page_view') }
}
beforeEach(() => { visibility('visible') })
afterEach(() => { stops.splice(0).forEach(stop => stop()); vi.useRealTimers(); vi.restoreAllMocks() })

describe('authenticated two-app usage telemetry', () => {
  it.each(['main-ops', 'asset-workbench'] as const)('records one successful navigation for %s without query secrets', async app => {
    const s = setup(app)
    s.router.beforeEach(to => to.path === '/blocked' ? false : true)
    await s.router.push('/a?token=secret&q=private#sensitive')
    const view = s.views()[0]
    expect(view.payload.app_id).toBe(app)
    expect(view.page_url).toMatch(/\/a$/)
    expect(JSON.stringify(view)).not.toMatch(/secret|private|sensitive/)
    expect(usageRequestHeaders()['X-Usage-App']).toBe(app)
    expect(usageRequestHeaders()['X-Usage-Page']).toBe(view.payload.page_view_id)
    await s.router.push('/a?token=secret&q=private#sensitive')
    await s.router.push('/blocked')
    expect(s.views()).toHaveLength(1)
    expect(installUsageTelemetry(s.router, { appId: app, getToken: () => 'account-a' })).toBe(s.stop)
  })

  it('does not count anonymous routes or carry a session across accounts', async () => {
    const s = setup(); s.setToken(null)
    await s.router.push('/a')
    expect(s.send).not.toHaveBeenCalled()
    s.setToken('account-a'); await s.router.push('/b')
    const session = s.views()[0].payload.session_id
    const finish = beginUsageOperation('download.file')
    s.setToken('account-b')
    expect(usageRequestHeaders()).toEqual({})
    window.dispatchEvent(new Event('asset-media-session-reset'))
    await s.router.push('/a')
    expect(s.views()[1].payload.session_id).not.toBe(session)
    const before = s.send.mock.calls.length
    finish('handed_off')
    expect(s.send.mock.calls.length).toBe(before)
    await s.router.push('/login')
    expect(s.views()).toHaveLength(2)
    expect(currentUsageContext()).toBeUndefined()
  })

  it('records only visible intervals and does not double-flush hide/pagehide', async () => {
    vi.useFakeTimers()
    const s = setup(); await s.router.push('/a')
    await vi.advanceTimersByTimeAsync(15000)
    visibility('hidden'); window.dispatchEvent(new Event('pagehide'))
    await vi.advanceTimersByTimeAsync(120000)
    visibility('visible'); await vi.advanceTimersByTimeAsync(3000)
    await s.router.push('/b')
    const intervals = s.send.mock.calls.map(c => c[0]).filter(e => e.event_type === 'page_engagement')
    expect(intervals.map(e => e.payload.visible_ms)).toEqual([15000, 3000])
  })

  it('keeps operation origin and distinguishes handoff, cancellation, and failure', async () => {
    const s = setup(); await s.router.push('/a')
    const origin = s.views()[0].payload.page_view_id
    const finish = beginUsageOperation('download.file', { filename: 'private.psd', file_size: 20 })
    await s.router.push('/b'); finish('handed_off'); finish('succeeded')
    const done = s.send.mock.calls.map(c => c[0]).filter(e => e.action === 'download.file' && e.payload.phase === 'finished')
    expect(done).toHaveLength(1)
    expect(done[0].outcome).toBe('handed_off')
    expect(done[0].payload.page_view_id).toBe(origin)
    expect(JSON.stringify(done)).not.toContain('private.psd')
    await expect(observeUsageOperation('upload.transfer', {}, async () => { throw new DOMException('secret-url', 'AbortError') })).rejects.toThrow()
    await expect(observeUsageOperation('upload.transfer', {}, async () => { throw Object.assign(new Error('token=secret'), { code: 'source_version_changed', status: 409 }) })).rejects.toThrow()
    const events = s.send.mock.calls.map(c => c[0]).filter(e => e.action === 'upload.transfer' && e.payload.phase === 'finished')
    expect(events.map(e => e.outcome)).toEqual(['cancelled', 'failed'])
    expect(events[1].payload.error_code).toBe('source_version_changed')
    expect(JSON.stringify(events)).not.toContain('secret')
  })

  it('bounds pending sends and never turns telemetry rejection into a business failure', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/a', component: {} }] })
    const send = vi.fn(() => new Promise<void>(() => undefined))
    const stop = installUsageTelemetry(router, { appId: 'main-ops', getToken: () => 'a', send }); stops.push(stop)
    await router.push('/a')
    for (let i = 0; i < 30; i++) beginUsageOperation('upload.transfer')('succeeded')
    expect(send.mock.calls.length).toBe(16)
    stop()
    const s = setup(); s.send.mockRejectedValue(new Error('offline')); await s.router.push('/a')
    await expect(observeUsageOperation('upload.workflow', {}, async () => 42)).resolves.toBe(42)
  })

  it('records a cancelled main-site download even when the UI helper returns an error result', async () => {
    const s = setup(); await s.router.push('/a')
    const controller = new AbortController(); controller.abort()
    const result = await downloadAssetFileWithOriginalFilename({ downloadUrl: 'https://oss.example/file?sig=secret', preferredFilename: 'private.pdf', signal: controller.signal })
    expect(result.ok).toBe(false)
    const event = s.send.mock.calls.map(c => c[0]).find(e => e.action === 'download.file' && e.payload.phase === 'finished')
    expect(event.outcome).toBe('cancelled')
    expect(JSON.stringify(event)).not.toMatch(/secret|private/)
  })
})
