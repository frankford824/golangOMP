// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest'
import { transferDownload } from './downloadTransfer'

describe('native asset download handoff', () => {
  it.each([0, 1024, 197_000_000, 5 * 1024 ** 3])('hands off %s bytes without fetching the body', async (size) => {
    const fetcher = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('must not fetch'))
    const handoff = vi.fn()
    const progress = vi.fn()
    try {
      const result = await transferDownload(
        { downloadUrl: 'https://oss.example.com/file.psd', filename: '设计源文件.psd', fileSize: size },
        new AbortController().signal, progress, { handoff },
      )
      expect(result).toEqual({ mode: 'browser', receivedBytes: 0, totalBytes: size, speedBytesPerSecond: 0 })
      expect(handoff).toHaveBeenCalledOnce()
      expect(progress).not.toHaveBeenCalled()
      expect(fetcher).not.toHaveBeenCalled()
    } finally { fetcher.mockRestore() }
  })

  it('does not initiate an already cancelled download', async () => {
    const controller = new AbortController(); controller.abort()
    const handoff = vi.fn()
    await expect(transferDownload({ downloadUrl: 'https://oss.example.com/file', filename: 'a', fileSize: 1 }, controller.signal, vi.fn(), { handoff })).rejects.toMatchObject({ name: 'AbortError' })
    expect(handoff).not.toHaveBeenCalled()
  })

  it('rejects executable URL schemes', async () => {
    const handoff = vi.fn()
    await expect(transferDownload({ downloadUrl: 'javascript:void(0)', filename: 'a', fileSize: 1 }, new AbortController().signal, vi.fn(), { handoff })).rejects.toThrow('协议无效')
    expect(handoff).not.toHaveBeenCalled()
  })
})
