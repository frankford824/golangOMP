import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}))

vi.mock('@/services/http', () => ({
  default: {
    get: getMock,
  },
}))

import {
  materializePreviewImageUrl,
  normalizePreviewAssetId,
  revokeMaterializedPreviewImage,
} from '@/domain/asset-preview-image'

const originalCreateObjectURL = URL.createObjectURL
const originalRevokeObjectURL = URL.revokeObjectURL
const createObjectURLMock = vi.fn(() => 'blob:preview-image')
const revokeObjectURLMock = vi.fn()

beforeAll(() => {
  Object.defineProperty(URL, 'createObjectURL', {
    configurable: true,
    value: createObjectURLMock,
  })
  Object.defineProperty(URL, 'revokeObjectURL', {
    configurable: true,
    value: revokeObjectURLMock,
  })
})

afterAll(() => {
  Object.defineProperty(URL, 'createObjectURL', {
    configurable: true,
    value: originalCreateObjectURL,
  })
  Object.defineProperty(URL, 'revokeObjectURL', {
    configurable: true,
    value: originalRevokeObjectURL,
  })
})

beforeEach(() => {
  getMock.mockReset()
  createObjectURLMock.mockClear()
  revokeObjectURLMock.mockClear()
})

describe('materializePreviewImageUrl', () => {
  it('renders same-origin image proxy blobs even when the server returns octet-stream', async () => {
    const url = '/v1/assets/files/tasks/RW-1/CGK000001/product-preview.jpg?signature=abc'
    getMock.mockResolvedValue({
      data: new Blob(['image-bytes'], { type: 'application/octet-stream' }),
    })

    const image = await materializePreviewImageUrl(url)

    expect(getMock).toHaveBeenCalledWith(url, { responseType: 'blob' })
    expect(createObjectURLMock).toHaveBeenCalledTimes(1)
    expect(image?.displaySrc).toBe('blob:preview-image')

    revokeMaterializedPreviewImage(image)
  })

  it('renders asset preview API blobs when the preview URL has no image extension', async () => {
    const url = '/v1/assets/123/preview'
    const pngHeader = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])
    getMock.mockResolvedValue({
      data: new Blob([pngHeader, 'image-bytes'], { type: 'application/octet-stream' }),
    })

    const image = await materializePreviewImageUrl(url)

    expect(getMock).toHaveBeenCalledWith(url, { responseType: 'blob' })
    expect(createObjectURLMock).toHaveBeenCalledTimes(1)
    expect(image?.displaySrc).toBe('blob:preview-image')

    revokeMaterializedPreviewImage(image)
  })

  it('uses direct image URLs returned by asset preview API metadata', async () => {
    const url = '/v1/assets/125/preview'
    const downloadURL = 'https://oss.example.com/tasks/RW-1/assets/AST-1/v1/preview/thumb.webp?signature=abc'
    getMock.mockResolvedValue({
      data: new Blob(
        [
          JSON.stringify({
            data: {
              download_url: downloadURL,
              preview_available: true,
              mime_type: 'image/webp',
            },
          }),
        ],
        { type: 'application/json' },
      ),
    })

    const image = await materializePreviewImageUrl(url)

    expect(getMock).toHaveBeenCalledWith(url, { responseType: 'blob' })
    expect(createObjectURLMock).not.toHaveBeenCalled()
    expect(image?.displaySrc).toBe(downloadURL)

    revokeMaterializedPreviewImage(image)
  })

  it('does not render non-image same-origin blobs as images', async () => {
    const url = '/v1/assets/124/preview'
    getMock.mockResolvedValue({
      data: new Blob(['{"data":{"download_url":"/v1/assets/files/a.pdf","mime_type":"application/pdf"}}'], {
        type: 'application/json',
      }),
    })

    const image = await materializePreviewImageUrl(url)

    expect(image).toBeUndefined()
    expect(createObjectURLMock).not.toHaveBeenCalled()
  })
})

describe('normalizePreviewAssetId', () => {
  it('accepts only numeric asset root IDs', () => {
    expect(normalizePreviewAssetId('6908')).toBe('6908')
    expect(normalizePreviewAssetId(6908)).toBe('6908')
    expect(normalizePreviewAssetId('dab491fa-9a6d-441b-bb4c-d8b0f8b9ff35')).toBe('')
    expect(normalizePreviewAssetId('/v1/assets/6908/preview')).toBe('')
  })
})
