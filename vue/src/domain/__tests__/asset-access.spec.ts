import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getAssetPreviewMetaMock } = vi.hoisted(() => ({
  getAssetPreviewMetaMock: vi.fn(),
}))

vi.mock('@/services/api/assetsApi', () => ({
  assetsApi: {
    getAssetPreviewMeta: getAssetPreviewMetaMock,
  },
}))

import { fetchAssetPreviewMeta, invalidateAssetAccessCache } from '@/domain/asset-access'

beforeEach(() => {
  getAssetPreviewMetaMock.mockReset()
})

describe('fetchAssetPreviewMeta', () => {
  it('maps HTTP 409 to preparing and retries the same asset on the next poll', async () => {
    getAssetPreviewMetaMock.mockRejectedValueOnce({
      isAxiosError: true,
      response: { status: 409 },
    })

    const preparing = await fetchAssetPreviewMeta('990041')

    expect(preparing).toEqual({
      status: 'preparing',
      message: '正在生成预览，请稍后自动刷新',
    })

    getAssetPreviewMetaMock.mockResolvedValueOnce({
      data: {
        data: {
          download_url: 'https://oss.example.com/previews/990041.webp',
        },
      },
    })

    const ready = await fetchAssetPreviewMeta('990041')

    expect(ready.status).toBe('ok')
    expect(ready.displayUrl).toBe('https://oss.example.com/previews/990041.webp')
    expect(getAssetPreviewMetaMock).toHaveBeenCalledTimes(2)
  })

  it('invalidates preview metadata after a resource version is replaced', async () => {
    getAssetPreviewMetaMock
      .mockResolvedValueOnce({ data: { data: { download_url: 'https://oss.example.com/previews/990042-v1.webp' } } })
      .mockResolvedValueOnce({ data: { data: { download_url: 'https://oss.example.com/previews/990042-v2.webp' } } })

    const before = await fetchAssetPreviewMeta('990042')
    invalidateAssetAccessCache('990042')
    const after = await fetchAssetPreviewMeta('990042')

    expect(before.displayUrl).toContain('v1.webp')
    expect(after.displayUrl).toContain('v2.webp')
    expect(getAssetPreviewMetaMock).toHaveBeenCalledTimes(2)
  })
})
