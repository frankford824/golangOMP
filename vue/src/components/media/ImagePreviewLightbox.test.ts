// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('@/domain/asset-access', () => ({
  fetchAssetPreviewMeta: vi.fn(async () => null),
}))

vi.mock('@/domain/asset-preview-image', () => ({
  materializePreviewImageUrl: vi.fn(async (src: string) => ({ displaySrc: src })),
  normalizePreviewAssetId: vi.fn((value: unknown) => String(value ?? '')),
  revokeMaterializedPreviewImage: vi.fn(),
}))

vi.mock('@/utils/assetFileDownload', () => ({
  downloadAssetFileWithOriginalFilename: vi.fn(async () => ({ ok: true })),
}))

import ImagePreviewLightbox from './ImagePreviewLightbox.vue'

describe('ImagePreviewLightbox actual pixels and copy', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined })
    document.body.innerHTML = ''
  })

  it('shows the intrinsic pixel size and copies a PNG for Photoshop', async () => {
    const clipboardWrite = vi.fn(async () => undefined)
    class ClipboardItemMock {
      constructor(readonly data: Record<string, Blob>) {}
    }
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { write: clipboardWrite },
    })
    vi.stubGlobal('ClipboardItem', ClipboardItemMock)
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      status: 200,
      blob: async () => new Blob(['png'], { type: 'image/png' }),
    })))

    const wrapper = mount(ImagePreviewLightbox, {
      props: {
        modelValue: true,
        items: [{
          src: 'data:image/png;base64,cG5n',
          title: 'reference.png',
          alt: 'reference.png',
        }],
      },
      attachTo: document.body,
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    const image = wrapper.get('.image-preview-img')
    Object.defineProperty(image.element, 'naturalWidth', { configurable: true, value: 2400 })
    Object.defineProperty(image.element, 'naturalHeight', { configurable: true, value: 1600 })
    await image.trigger('load')
    await wrapper.get('[aria-label="实际像素 100%"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.get('.image-preview-stage').classes()).toContain('image-preview-stage--actual')
    const actualImage = wrapper.get('.image-preview-img')
    expect(actualImage.attributes('style')).toContain('width: 2400px')
    expect(actualImage.attributes('style')).toContain('height: 1600px')

    await wrapper.get('[aria-label="复制当前图片"]').trigger('click')
    await flushPromises()
    expect(clipboardWrite).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('已复制原图，可直接粘贴到 Photoshop')
  })
})
