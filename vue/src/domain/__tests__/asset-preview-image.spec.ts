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

  it('does not render non-image same-origin blobs as images', async () => {
    const url = '/v1/assets/123/preview'
    getMock.mockResolvedValue({
      data: new Blob(['{"download_url":"/v1/assets/files/a.jpg"}'], {
        type: 'application/json',
      }),
    })

    const image = await materializePreviewImageUrl(url)

    expect(image).toBeUndefined()
    expect(createObjectURLMock).not.toHaveBeenCalled()
  })
})
