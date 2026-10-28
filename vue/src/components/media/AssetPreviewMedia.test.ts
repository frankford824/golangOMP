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
  invalidateAssetAccessCache: vi.fn(),
  invalidateTaskAssetAccessCache: vi.fn(),
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

  it.each(['unavailable', 'error', 'not_found', 'preparing'])('never substitutes an original URL when preview is %s', async (status) => {
    mocks.fetchTaskAssetPreview.mockResolvedValue({ status, message: '文件处理中' })
    const wrapper = mount(AssetPreviewMedia, {
      props: { taskAssetId: '24174', fallbackSrc: 'https://example.com/200MB-original.jpg' },
    })
    await flushPromises()
    expect(mocks.materialize).not.toHaveBeenCalled()
    expect(wrapper.find('img.apm-img').exists()).toBe(false)
    wrapper.unmount()
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

    expect(mocks.fetchTaskAssetPreview).toHaveBeenCalledWith('24174', undefined, 'thumbnail')
    expect(mocks.fetchAssetPreview).not.toHaveBeenCalled()
    expect(mocks.materialize).toHaveBeenCalledWith('/v1/assets/files/task-preview.webp', '24174')
    expect(wrapper.get('img.apm-img').attributes('src')).toBe('blob:task-preview')
  })

  it('lets a navigation card receive image clicks without requesting a detail preview', async () => {
    const navigate = vi.fn()
    const wrapper = mount({
      components: { AssetPreviewMedia },
      setup: () => ({ navigate }),
      template: '<button @click="navigate"><AssetPreviewMedia task-asset-id="24174" :open-full-on-click="false" /></button>',
    })
    await flushPromises()
    const initialRequests = mocks.fetchTaskAssetPreview.mock.calls.length
    await wrapper.get('img.apm-img').trigger('click')
    expect(navigate).toHaveBeenCalledTimes(1)
    expect(mocks.fetchTaskAssetPreview).toHaveBeenCalledTimes(initialRequests)
    expect(wrapper.findComponent(AssetPreviewMedia).emitted('open-full')).toBeUndefined()
    wrapper.unmount()
  })

  it('keeps standalone lightbox clicks isolated from the parent by default', async () => {
    const navigate = vi.fn()
    const wrapper = mount({
      components: { AssetPreviewMedia },
      setup: () => ({ navigate }),
      template: '<button @click="navigate"><AssetPreviewMedia task-asset-id="24174" /></button>',
    })
    await flushPromises()
    await wrapper.get('img.apm-img').trigger('click')
    await flushPromises()
    expect(navigate).not.toHaveBeenCalled()
    expect(wrapper.findComponent(AssetPreviewMedia).emitted('open-full')).toHaveLength(1)
    wrapper.unmount()
  })
})
