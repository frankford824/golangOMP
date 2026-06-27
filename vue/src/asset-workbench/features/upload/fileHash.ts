type WorkerResponse = {
  id: string
  hash?: string
  error?: string
}

export function computeWorkbenchFileHash(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const id = crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`
    const worker = new Worker(new URL('./fileHash.worker.ts', import.meta.url), { type: 'module' })
    const cleanup = () => worker.terminate()
    worker.onmessage = (event: MessageEvent<WorkerResponse>) => {
      if (event.data.id !== id) return
      cleanup()
      if (event.data.error) {
        reject(new Error(event.data.error))
        return
      }
      resolve(event.data.hash ?? '')
    }
    worker.onerror = (event) => {
      cleanup()
      reject(new Error(event.message || '文件哈希计算失败'))
    }
    worker.postMessage({ id, file })
  })
}
