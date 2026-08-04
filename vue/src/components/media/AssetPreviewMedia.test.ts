// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  fetchAssetPreview: vi.fn(),
  fetchTaskAssetPreview: vi.fn(),
  materialize: vi.fn(),
  revoke: vi.fn(),
}))

vi.mock('@/domain/asset-access', () => ({
  fetchAssetPreviewMeta: mocks.fetchAssetPreview,
  fetchTaskAssetPreviewMeta: mocks.fetchTaskAssetPreview,
}))

vi.mock('@/domain/asset-preview-image', () => ({
  materializePreviewImageUrl: mocks.materialize,
  normalizePreviewAssetId: (raw: unknown) => {
    const value = String(raw ?? '').trim()
    return /^\d+$/.test(value) ? value : ''
  },
  revokeMaterializedPreviewImage: mocks.revoke,
}))

import AssetPreviewMedia from './AssetPreviewMedia.vue'

describe('AssetPreviewMedia', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.fetchTaskAssetPreview.mockResolvedValue({ status: 'ok', displayUrl: '/v1/assets/files/task-preview.webp' })
    mocks.materialize.mockResolvedValue({ displaySrc: 'blob:task-preview' })
  })

  it('resolves immutable revision members through the authenticated task-asset API', async () => {
    const wrapper = mount(AssetPreviewMedia, {
      props: {
        taskAssetId: '24174',
        fallbackSrc: '/v1/task-assets/24174/preview',
        alt: '历史成品',
      },
    })
    await flushPromises()

    expect(mocks.fetchTaskAssetPreview).toHaveBeenCalledWith('24174')
    expect(mocks.fetchAssetPreview).not.toHaveBeenCalled()
    expect(mocks.materialize).toHaveBeenCalledWith('/v1/assets/files/task-preview.webp', '24174')
    expect(wrapper.get('img.apm-img').attributes('src')).toBe('blob:task-preview')
  })
})
