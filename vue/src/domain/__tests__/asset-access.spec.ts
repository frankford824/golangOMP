import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getAssetPreviewMetaMock } = vi.hoisted(() => ({
  getAssetPreviewMetaMock: vi.fn(),
}))

vi.mock('@/services/api/assetsApi', () => ({
  assetsApi: {
    getAssetPreviewMeta: getAssetPreviewMetaMock,
  },
}))

import { fetchAssetPreviewMeta } from '@/domain/asset-access'

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
})
