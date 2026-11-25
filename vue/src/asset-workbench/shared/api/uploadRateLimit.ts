import { parseApiErrorPayload } from '@/utils/api-message-zh'

const maxRetries = 3
const recoveryWindow = 15_000
const recoverySpacing = 250

let blockedUntil = 0
let paceUntil = 0
let nextRequestAt = 0

function wait(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const onAbort = () => {
      clearTimeout(timer)
      reject(signal?.reason)
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

async function waitForUploadRequest(signal?: AbortSignal): Promise<void> {
  for (;;) {
    signal?.throwIfAborted()
    const now = performance.now()
    const readyAt = Math.max(blockedUntil, nextRequestAt)
    if (now >= readyAt) {
      if (now < paceUntil) nextRequestAt = now + recoverySpacing
      return
    }
    await wait(readyAt - now, signal)
  }
}

export async function withUploadRateLimit<T>(request: () => Promise<T>, signal?: AbortSignal): Promise<T> {
  for (let attempt = 0; ; attempt += 1) {
    await waitForUploadRequest(signal)
    signal?.throwIfAborted()
    try {
      return await request()
    } catch (error) {
      const status = (error as { status?: number } | null)?.status ?? parseApiErrorPayload(error).status
      // A throttle response rejects the request before processing. Other
      // failures can leave a POST outcome unknown and require separate handling.
      if (status !== 429) throw error
      const delay = 1000 * 2 ** Math.min(attempt, maxRetries - 1)
      blockedUntil = Math.max(blockedUntil, performance.now() + delay)
      paceUntil = blockedUntil + recoveryWindow
      if (attempt >= maxRetries) throw error
    }
  }
}
