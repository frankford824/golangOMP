// Bounded recovery for read requests during a backend cutover. Never wrap a
// price, confirmation, binding or other mutation with this helper.
export async function retryCostRead<T>(read: () => Promise<T>, signal?: AbortSignal): Promise<T> {
  for (let attempt = 0; ; attempt++) {
    signal?.throwIfAborted()
    try { return await read() }
    catch (error) {
      const status = (error as { status?: number })?.status
      if (signal?.aborted || attempt >= 2 || ![502, 503, 504].includes(status ?? 0)) throw error
      await new Promise<void>((resolve, reject) => {
        const finish = () => { signal?.removeEventListener('abort', abort); resolve() }
        const timer = setTimeout(finish, attempt === 0 ? 500 : 1000)
        const abort = () => { clearTimeout(timer); signal?.removeEventListener('abort', abort); reject(signal?.reason ?? new DOMException('Aborted', 'AbortError')) }
        signal?.addEventListener('abort', abort, { once: true })
        if (signal?.aborted) abort()
      })
    }
  }
}
