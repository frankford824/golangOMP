// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useDownloadCenterStore } from './downloadCenter.store'
import type { DownloadTransferProgress, DownloadTransferResult } from './downloadTransfer'

describe('asset workbench global download center', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps one active task for repeated clicks and records live progress', async () => {
    const store = useDownloadCenterStore()
    let finishTransfer: ((result: DownloadTransferResult) => void) | undefined
    let reportProgress: ((progress: DownloadTransferProgress) => void) | undefined
    const transfer = vi.fn((_meta, _signal, onProgress) => {
      reportProgress = onProgress
      return new Promise<DownloadTransferResult>((resolve) => {
        finishTransfer = resolve
      })
    })
    const request = {
      key: 'material:external:42',
      displayName: '海报.psd',
      sourceLabel: '外部资源',
      fileSize: 100,
      resolve: vi.fn().mockResolvedValue({
        downloadUrl: 'https://oss.example.com/poster.psd',
        filename: '海报.psd',
        fileSize: 100,
      }),
      transfer,
    }

    const first = store.enqueue(request)
    await vi.waitFor(() => expect(first.item.status).toBe('downloading'))
    const repeated = store.enqueue(request)
    expect(repeated.duplicate).toBe(true)
    expect(store.items).toHaveLength(1)

    reportProgress?.({ receivedBytes: 40, totalBytes: 100, speedBytesPerSecond: 20, progress: 40 })
    expect(first.item.progress).toBe(40)
    expect(first.item.speedBytesPerSecond).toBe(20)

    finishTransfer?.({ mode: 'tracked', receivedBytes: 100, totalBytes: 100, speedBytesPerSecond: 25 })
    await vi.waitFor(() => expect(first.item.status).toBe('completed'))
    expect(first.item.progress).toBe(100)
    expect(store.summaryText).toBe('已完成 1 个')
  })

  it('cancels an active download and allows an explicit retry', async () => {
    const store = useDownloadCenterStore()
    const cancelledTransfer = vi.fn((_meta, signal: AbortSignal) => new Promise<DownloadTransferResult>((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('下载已取消', 'AbortError')), { once: true })
    }))
    const request = {
      key: 'drive-file:7',
      displayName: '成品图.jpg',
      resolve: vi.fn().mockResolvedValue({
        downloadUrl: 'https://oss.example.com/image.jpg',
        filename: '成品图.jpg',
        fileSize: 200,
      }),
      transfer: cancelledTransfer,
    }

    const first = store.enqueue(request)
    await vi.waitFor(() => expect(first.item.status).toBe('downloading'))
    store.cancel(first.item.id)
    await vi.waitFor(() => expect(first.item.status).toBe('cancelled'))

    const retried = store.enqueue({
      ...request,
      transfer: vi.fn(async (_meta, _signal, onProgress): Promise<DownloadTransferResult> => {
        onProgress({ receivedBytes: 200, totalBytes: 200, speedBytesPerSecond: 100, progress: 100 })
        return { mode: 'tracked', receivedBytes: 200, totalBytes: 200, speedBytesPerSecond: 100 }
      }),
    })
    expect(retried.item.id).toBe(first.item.id)
    expect(retried.duplicate).toBe(false)
    await vi.waitFor(() => expect(first.item.status).toBe('completed'))
  })
})
