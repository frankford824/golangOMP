// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({
  assetWorkbenchApi: {
    listUploadDirectories: vi.fn().mockResolvedValue([
      { id: 8, name: 'C类', oss_prefix: 'c', difficulty_class: 'C', allowed_file_types: [], enabled: true, sort_order: 1 },
    ]),
    listDifficultyClasses: vi.fn().mockResolvedValue([{ id: 1, code: 'C', name: 'C', enabled: true, sort_order: 1 }]),
  },
}))

import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import { useUploadCenterStore } from '@aw/shared/drive/uploadCenter.store'
import { withDriveUploadRelativePath } from '@aw/shared/drive/useDriveUpload'
import UploadPage from './UploadPage.vue'

describe('UploadPage simple piecework copy', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 22, display_name: 'simple-user' },
      is_admin: false,
      capabilities: ['asset.workbench.submit'],
    } as never)
  })

  it('uses compact picker labels and keeps displayed work count aligned with piecework groups', async () => {
    const uploadCenter = useUploadCenterStore()
    uploadCenter.addItems([
      new File(['a'], 'a.png', { type: 'image/png' }),
      new File(['b'], 'b.png', { type: 'image/png' }),
    ], { source: 'upload-page' })
    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    const dropzoneButtons = wrapper.get('.aw-dropzone').findAll('button').map((button) => button.text().trim())
    expect(dropzoneButtons).toEqual(['文件', '文件夹', '提交上传'])
    expect(wrapper.get('.aw-dropzone > span').text()).toBe('点击提交上传会自动完成上传和提交。允许：全部格式')
    expect(wrapper.get('.aw-dropzone__hint').text()).toBe('仔细核对作品数量后，点击上传 2个作品=计件数量')
    expect(wrapper.find('.aw-dropzone__piecework-confirmation').exists()).toBe(false)
    expect(wrapper.get('.aw-dropzone').text()).not.toContain('选择文件')

    uploadCenter.clearIdle()
    uploadCenter.addItems([
      withDriveUploadRelativePath(new File(['a'], 'front.png', { type: 'image/png' }), '套装/front.png'),
      withDriveUploadRelativePath(new File(['b'], 'side.png', { type: 'image/png' }), '套装/side.png'),
    ], { source: 'upload-page' })
    await flushPromises()

    expect(wrapper.get('.aw-dropzone__hint').text()).toBe('仔细核对作品数量后，点击上传 1个作品=计件数量')
    const clearButton = wrapper.get('button[aria-label="清空待上传队列"]')
    expect(clearButton.attributes('title')).toBe('仅移除待上传和上传失败的项目')
    await clearButton.trigger('click')
    expect(uploadCenter.uploadPageItems).toHaveLength(0)
    expect(wrapper.text()).toContain('等待文件')
  })

  it('uses the same annotated copy for admin uploads', async () => {
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 1, display_name: 'admin' },
      is_admin: true,
      capabilities: ['asset.workbench.submit', 'asset.workbench.manage'],
    } as never)
    useUploadCenterStore().addItems([
      new File(['archive'], 'work.zip', { type: 'application/zip' }),
    ], { source: 'upload-page' })

    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    const dropzone = wrapper.get('.aw-dropzone')
    expect(dropzone.findAll('button').map((button) => button.text().trim())).toEqual(['文件', '文件夹', '提交上传'])
    expect(dropzone.get(':scope > span').text()).toBe('点击提交上传会自动完成上传和提交。允许：全部格式')
    expect(dropzone.get('.aw-dropzone__hint').text()).toBe('仔细核对作品数量后，点击上传 1个作品=计件数量')
    expect(dropzone.text()).not.toContain('上传并生成记录')
    expect(dropzone.text()).not.toContain('先选择文件，或把文件拖到上传区')
    expect(wrapper.get('button[aria-label="清空待上传队列"]').text()).toBe('清空队列')
  })
})
