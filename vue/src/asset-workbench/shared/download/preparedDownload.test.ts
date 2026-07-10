import { describe, expect, it, vi } from 'vitest'

import type { SystemAssetDownloadInfo } from '@aw/shared/api/assetWorkbenchApi'
import { downloadIsPreparing, previewIsPreparing, waitForPreparedDownload, waitForPreparedPreview } from './preparedDownload'

function downloadInfo(overrides: Partial<SystemAssetDownloadInfo> = {}): SystemAssetDownloadInfo {
  return {
    download_mode: 'direct',
    filename: 'poster.psd',
    file_size: 1024,
    ...overrides,
  }
}

describe('prepared external material download', () => {
  it('recognizes queued preparation metadata', () => {
    expect(downloadIsPreparing(downloadInfo({ access_hint: 'external_netdisk_prepare_required' }))).toBe(true)
    expect(downloadIsPreparing(downloadInfo({ download_url: 'https://oss.example.com/poster.psd' }))).toBe(false)
  })

  it('polls until the OSS download URL is ready', async () => {
    const refresh = vi
      .fn<() => Promise<SystemAssetDownloadInfo>>()
      .mockResolvedValueOnce(downloadInfo({ access_hint: 'external_netdisk_prepare_required' }))
      .mockResolvedValueOnce(downloadInfo({ download_url: 'https://oss.example.com/poster.psd' }))
    const delay = vi.fn(async () => undefined)

    const result = await waitForPreparedDownload(
      downloadInfo({ access_hint: 'external_netdisk_prepare_required' }),
      refresh,
      { attempts: 3, intervalMs: 0, delay },
    )

    expect(result.download_url).toBe('https://oss.example.com/poster.psd')
    expect(refresh).toHaveBeenCalledTimes(2)
    expect(delay).toHaveBeenCalledTimes(2)
  })

  it('rejects missing URLs that are not being prepared', async () => {
    await expect(waitForPreparedDownload(downloadInfo(), vi.fn(), { delay: async () => undefined })).rejects.toThrow('暂时无法下载')
  })

  it('polls a pending preview until its derived image is ready', async () => {
    const pending = { asset_id: 42, status: 'pending', preparing: true, preview_available: false }
    const ready = { asset_id: 42, status: 'ready', preparing: false, preview_available: true, preview_url: 'https://oss.example.com/poster.webp' }
    const refresh = vi.fn().mockResolvedValue(ready)

    expect(previewIsPreparing(pending)).toBe(true)
    await expect(waitForPreparedPreview(pending, refresh, { intervalMs: 0, delay: async () => undefined })).resolves.toEqual(ready)
  })
})
