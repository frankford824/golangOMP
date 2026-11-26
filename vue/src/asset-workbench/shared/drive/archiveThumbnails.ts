// All mounted archive browsers in this tab share the same preview bandwidth.
const concurrency = 3
export const maxAutoThumbnailBytes = 2 * 1024 * 1024
let active = 0
const waiting = new Set<() => void>()

async function acquire(signal: AbortSignal) {
  signal.throwIfAborted()
  if (active < concurrency) {
    active++
    return
  }
  await new Promise<void>((resolve, reject) => {
    const start = () => {
      waiting.delete(start)
      signal.removeEventListener('abort', abort)
      resolve()
    }
    const abort = () => {
      waiting.delete(start)
      reject(signal.reason)
    }
    waiting.add(start)
    signal.addEventListener('abort', abort, { once: true })
  })
}

export async function loadArchiveThumbnail<T>(fetch: () => Promise<T>, signal: AbortSignal): Promise<T> {
  await acquire(signal)
  try {
    signal.throwIfAborted()
    return await fetch()
  } finally {
    const next = waiting.values().next().value
    if (next) next()
    else active--
  }
}
