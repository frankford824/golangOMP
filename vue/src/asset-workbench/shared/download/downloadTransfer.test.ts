import { describe, expect, it, vi } from 'vitest'

import { transferDownload, type DownloadTransferProgress } from './downloadTransfer'

describe('asset workbench streamed download transfer', () => {
  it('reports real byte progress and speed before saving the file', async () => {
    let nowValue = 0
    const chunks = [new Uint8Array([1, 2, 3]), new Uint8Array([4, 5, 6])]
    let chunkIndex = 0
    const body = {
      getReader: () => ({
        read: async () => {
          nowValue += 500
          if (chunkIndex >= chunks.length) return { done: true, value: undefined }
          return { done: false, value: chunks[chunkIndex++] }
        },
      }),
    }
    const response = {
      ok: true,
      status: 200,
      headers: { get: (name: string) => name.toLowerCase() === 'content-length' ? '6' : 'application/octet-stream' },
      body,
    } as unknown as Response
    const progress: DownloadTransferProgress[] = []
    const saveBlob = vi.fn()

    const result = await transferDownload(
      { downloadUrl: 'https://oss.example.com/file.psd', filename: 'file.psd', fileSize: 6 },
      new AbortController().signal,
      (value) => progress.push(value),
      {
        fetcher: vi.fn().mockResolvedValue(response),
        now: () => nowValue,
        progressIntervalMs: 250,
        saveBlob,
      },
    )

    expect(result.mode).toBe('tracked')
    expect(result.receivedBytes).toBe(6)
    expect(result.speedBytesPerSecond).toBeGreaterThan(0)
    expect(progress.map((item) => item.progress)).toEqual([50, 99, 100])
    expect(saveBlob).toHaveBeenCalledOnce()
    expect(saveBlob.mock.calls[0][0]).toBeInstanceOf(Blob)
  })

  it('hands the URL to the browser when cross-origin streaming is unavailable', async () => {
    const handoff = vi.fn()
    const result = await transferDownload(
      { downloadUrl: 'https://oss.example.com/file.psd', filename: 'file.psd', fileSize: 1024 },
      new AbortController().signal,
      vi.fn(),
      {
        fetcher: vi.fn().mockRejectedValue(new TypeError('Failed to fetch')),
        handoff,
      },
    )

    expect(result.mode).toBe('browser')
    expect(handoff).toHaveBeenCalledOnce()
  })

  it('does not buffer oversized files in browser memory', async () => {
    const handoff = vi.fn()
    const fetcher = vi.fn()
    const result = await transferDownload(
      { downloadUrl: 'https://oss.example.com/large.zip', filename: 'large.zip', fileSize: 2048 },
      new AbortController().signal,
      vi.fn(),
      { fetcher, handoff, maxBufferedBytes: 1024 },
    )

    expect(result.mode).toBe('browser')
    expect(fetcher).not.toHaveBeenCalled()
    expect(handoff).toHaveBeenCalledOnce()
  })
})
