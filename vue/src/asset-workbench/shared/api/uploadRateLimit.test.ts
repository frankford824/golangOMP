import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const throttled = Object.assign(new Error('Too Many Requests'), { status: 429 })

describe('workbench upload throttling', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.useFakeTimers({ toFake: ['performance', 'setTimeout', 'clearTimeout'] })
  })
  afterEach(() => vi.useRealTimers())

  it('backs off on 429 and spaces waiting upload requests apart', async () => {
    const { withUploadRateLimit } = await import('./uploadRateLimit')
    const startedAt = performance.now()
    const times: number[] = []
    const first = vi.fn(async () => {
      times.push(performance.now() - startedAt)
      return 'retried'
    }).mockRejectedValueOnce(throttled)
    const retried = withUploadRateLimit(first)
    await vi.advanceTimersByTimeAsync(0)
    const second = withUploadRateLimit(async () => {
      times.push(performance.now() - startedAt)
      return 'second'
    })
    await vi.advanceTimersByTimeAsync(1250)
    await expect(Promise.all([retried, second])).resolves.toEqual(['retried', 'second'])
    expect(times).toEqual([1000, 1250])
  })

  it('limits retries and does not retry a failure with an unknown POST outcome', async () => {
    const { withUploadRateLimit } = await import('./uploadRateLimit')
    const request = vi.fn().mockRejectedValue(throttled)
    const completed = expect(withUploadRateLimit(request)).rejects.toBe(throttled)
    await vi.advanceTimersByTimeAsync(7000)
    await completed
    expect(request).toHaveBeenCalledTimes(4)

    const unknown = new Error('network failure')
    const another = vi.fn().mockRejectedValue(unknown)
    const failure = expect(withUploadRateLimit(another)).rejects.toBe(unknown)
    await vi.advanceTimersByTimeAsync(4250)
    await failure
    expect(another).toHaveBeenCalledTimes(1)
  })

  it('cancels a retry while waiting for the next request window', async () => {
    const { withUploadRateLimit } = await import('./uploadRateLimit')
    const request = vi.fn().mockRejectedValue(throttled)
    const controller = new AbortController()
    const outcome = expect(withUploadRateLimit(request, controller.signal)).rejects.toMatchObject({ name: 'AbortError' })
    await vi.advanceTimersByTimeAsync(0)
    controller.abort()
    await outcome
    await vi.advanceTimersByTimeAsync(1000)
    expect(request).toHaveBeenCalledTimes(1)
  })
})
