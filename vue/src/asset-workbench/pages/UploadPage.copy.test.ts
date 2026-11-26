// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({
  assetWorkbenchApi: {
    listUploadDirectories: vi.fn().mockResolvedValue([
      { id: 8, name: 'C类', oss_prefix: 'c', difficulty_class: 'C', allowed_file_types: [], enabled: true, sort_order: 1 },
      { id: 9, name: 'A类', oss_prefix: 'a', difficulty_class: 'A', allowed_file_types: ['jpg'], enabled: true, sort_order: 2 },
    ]),
    listDifficultyClasses: vi.fn().mockResolvedValue([{ id: 1, code: 'C', name: 'C', enabled: true, sort_order: 1 }]),
    mySupplementMonths: vi.fn(),
    createSettlementSupplement: vi.fn(),
    mySettlement: vi.fn(),
    createSubmission: vi.fn().mockResolvedValue({ items: [] }),
  },
}))

vi.mock('@aw/features/upload/uploadFlow', () => ({ uploadWorkbenchFile: vi.fn() }))

import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'
import type { SubmissionDetail } from '@aw/shared/api/assetWorkbenchApi'
import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import { useUploadCenterStore } from '@aw/shared/drive/uploadCenter.store'
import { withDriveUploadRelativePath } from '@aw/shared/drive/useDriveUpload'
import UploadPage from './UploadPage.vue'

describe('UploadPage simple piecework copy', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    vi.clearAllMocks()
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
    expect(dropzoneButtons).toEqual(['选择文件', '选择文件夹', '提交上传'])
    expect(wrapper.get('.aw-dropzone > span').text()).toBe('本次添加到「C类」。目录允许：全部格式')
    expect(wrapper.get('.aw-dropzone__hint').text()).toBe('仔细核对作品数量后，点击上传 2个作品=计件数量')
    expect(wrapper.find('.aw-dropzone__piecework-confirmation').exists()).toBe(false)
    expect(wrapper.get('.aw-page-bar__actions').text()).toContain('表格校对')
    expect(wrapper.get('.aw-dropzone__actions').text()).toContain('提交上传')

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

  it('lets an income-hidden employee upload to their open month without requesting income data', async () => {
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 22 }, is_admin: false, capabilities: ['asset.workbench.submit'], profile: { settlement_visible: false },
    } as never)
    vi.mocked(assetWorkbenchApi.mySupplementMonths).mockResolvedValue({ payee_user_id: 22, months: ['2026-10'] })
    vi.mocked(uploadWorkbenchFile).mockResolvedValue({ sessionId: 'supplement-session', filename: 'sample.jpg' })
    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: true } } })
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '补交遗漏作品')!.trigger('click')
    await flushPromises()
    const panel = wrapper.get('.aw-self-supplement-upload')
    expect(panel.text()).toContain('已开放 2026-10')
    const input = panel.get('input[aria-label="选择补录文件"]')
    Object.defineProperty(input.element, 'files', { value: [new File(['ok'], 'sample.jpg')] })
    await input.trigger('change')
    vi.mocked(assetWorkbenchApi.mySupplementMonths).mockResolvedValue({ payee_user_id: 22, months: [] })
    await panel.findAll('button').find(b => b.text() === '上传 1 个补录作品')!.trigger('click')
    await flushPromises()
    expect(assetWorkbenchApi.createSettlementSupplement).toHaveBeenCalledWith(expect.objectContaining({
      payee_user_id: 22, business_month: '2026-10', upload_session_ids: ['supplement-session'],
    }))
    expect(panel.text()).toContain('1 个作品，共 1 个文件，归档到「C类」')
    expect(panel.text()).toContain('当前没有可用的补录权限')
    expect(wrapper.text()).not.toMatch(/¥|本月预计收入/)
    expect(assetWorkbenchApi.mySettlement).not.toHaveBeenCalled()
    wrapper.unmount()
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
    expect(dropzone.findAll('button').map((button) => button.text().trim())).toEqual(['选择文件', '选择文件夹', '提交上传'])
    expect(dropzone.get(':scope > span').text()).toBe('本次添加到「C类」。目录允许：全部格式')
    expect(dropzone.get('.aw-dropzone__hint').text()).toBe('仔细核对作品数量后，点击上传 1个作品=计件数量')
    expect(dropzone.text()).not.toContain('上传并生成记录')
    expect(dropzone.text()).not.toContain('先选择文件，或把文件拖到上传区')
    expect(wrapper.get('button[aria-label="清空待上传队列"]').text()).toBe('清空队列')
  })

  it('submits the whole folder once after retrying just its failed member', async () => {
    const first = withDriveUploadRelativePath(new File(['a'], 'front.png'), '套装/front.png')
    const second = withDriveUploadRelativePath(new File(['b'], 'side.png'), '套装/side.png')
    const upload = vi.mocked(uploadWorkbenchFile)
    upload.mockImplementation(async (file) => ({ sessionId: file.name, filename: file.name }))
    upload.mockImplementationOnce(async () => ({ sessionId: 'front-session', filename: first.name }))
    upload.mockRejectedValueOnce(new Error('network error'))
    const center = useUploadCenterStore()
    center.addItems([first, second], { source: 'upload-page' })
    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    await wrapper.get('.aw-dropzone__actions .aw-primary-button').trigger('click')
    await flushPromises()
    expect(assetWorkbenchApi.createSubmission).not.toHaveBeenCalled()
    expect(center.uploadPageItems.map((item) => item.status)).toEqual(['uploaded', 'failed'])
    expect(wrapper.get('[role="alert"]').text()).toContain('再整件提交')

    await wrapper.get('.aw-upload-directory-picker select').setValue('9')
    await wrapper.get('.aw-upload-failure-callout__actions .aw-primary-button').trigger('click')
    await flushPromises()
    expect(upload.mock.calls.map(([file]) => file.name)).toEqual(['front.png', 'side.png', 'side.png'])
    expect(upload.mock.calls.map(([, options]) => options?.uploadDirectoryId)).toEqual([8, 8, 8])
    expect(assetWorkbenchApi.createSubmission).toHaveBeenCalledTimes(1)
    expect(assetWorkbenchApi.createSubmission).toHaveBeenCalledWith(expect.objectContaining({
      items: [expect.objectContaining({ item_count: 1, upload_session_ids: ['front-session', 'side.png'] })],
    }))
    expect(center.uploadPageItems).toHaveLength(0)
  })

  it('reuses uploaded sessions when only the final submission failed', async () => {
    const upload = vi.mocked(uploadWorkbenchFile)
    upload.mockResolvedValue({ sessionId: 'existing-session', filename: 'sample.png' })
    vi.mocked(assetWorkbenchApi.createSubmission).mockRejectedValueOnce(Object.assign(new Error('请求过于频繁，请稍后再试'), {
      status: 429, responseData: { message: '请求过于频繁，请稍后再试' },
    }))
    useUploadCenterStore().addItems([new File(['a'], 'sample.png')], { source: 'upload-page' })
    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    const submit = wrapper.get('.aw-dropzone__actions .aw-primary-button')
    await submit.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('上传已完成，但提交失败')
    expect(submit.text()).toBe('继续提交')
    expect(wrapper.find('.aw-upload-failure-callout').exists()).toBe(false)

    await submit.trigger('click')
    await flushPromises()
    expect(upload).toHaveBeenCalledTimes(1)
    expect(assetWorkbenchApi.createSubmission).toHaveBeenCalledTimes(2)
    expect(vi.mocked(assetWorkbenchApi.createSubmission).mock.calls[1]).toEqual(
      vi.mocked(assetWorkbenchApi.createSubmission).mock.calls[0],
    )
  })

  it.each([false, true])('shows confirmed directory counts after the queue clears (admin=%s)', async (admin) => {
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 22 }, is_admin: admin, capabilities: ['asset.workbench.submit'],
    } as never)
    const center = useUploadCenterStore()
    const makeFile = (name: string) => new File(['sample'], name)
    center.addItems([
      withDriveUploadRelativePath(makeFile('front.jpg'), '海报/front.jpg'),
      withDriveUploadRelativePath(makeFile('back.jpg'), '海报/back.jpg'),
      makeFile('two.jpg'), makeFile('three.jpg'),
    ], { source: 'upload-page' })
    vi.mocked(uploadWorkbenchFile).mockImplementation(async (file) => ({ sessionId: file.name, filename: file.name }))
    const confirmedFile = (id: number, original_filename: string, upload_directory_name: string) =>
      ({ id, original_filename, upload_directory_name })
    vi.mocked(assetWorkbenchApi.createSubmission).mockResolvedValueOnce({ items: [
      { files: [confirmedFile(1, 'front.jpg', 'A类'), confirmedFile(2, 'back.jpg', 'A类')] },
      { files: [confirmedFile(3, 'two.jpg', 'A类')] },
      { files: [confirmedFile(4, 'three.jpg', 'C类')] },
    ] } as SubmissionDetail)
    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: true, WorkbenchFilePreview: true } } })
    await flushPromises()
    await wrapper.get('.aw-dropzone__actions .aw-primary-button').trigger('click')
    await flushPromises()
    expect(center.uploadPageItems).toHaveLength(0)
    expect(wrapper.get('.aw-inline-alert').text()).toContain('3 个作品，包含 4 个文件。A类 2 个作品、C类 1 个作品')
    expect(wrapper.get('[aria-label="已提交作品目录汇总"]').text()).toBe('A类 2 个作品、C类 1 个作品')
    expect(wrapper.findAll('figcaption').map((caption) => caption.text())).toEqual([
      'A类 · front.jpg', 'A类 · back.jpg', 'A类 · two.jpg', 'C类 · three.jpg',
    ])
    wrapper.unmount()
  })

  it('keeps earlier receipts across failed retries and starts a fresh receipt for the next batch', async () => {
    let rejectSecond = true
    const upload = vi.mocked(uploadWorkbenchFile)
    upload.mockImplementation(async (file) => {
      if (file.name === 'c.jpg' && rejectSecond) throw new Error('network error')
      return { sessionId: file.name, filename: file.name }
    })
    const submit = vi.mocked(assetWorkbenchApi.createSubmission)
    submit.mockImplementation(async (payload) => ({
      items: payload.items.map(({ upload_session_ids }) => ({
        files: upload_session_ids.map((name) => ({
          id: name === 'a.jpg' ? 1 : 2, original_filename: name,
          upload_directory_name: name === 'a.jpg' ? 'A类' : 'C类',
        })),
      })),
    } as SubmissionDetail))
    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: true, WorkbenchFilePreview: true } } })
    await flushPromises()
    const input = wrapper.get('input[aria-label="选择作品文件"]')
    const add = async (name: string) => {
      Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['data'], name)] })
      await input.trigger('change')
    }
    await wrapper.get('.aw-upload-directory-picker select').setValue('9')
    await add('a.jpg')
    await wrapper.get('.aw-upload-directory-picker select').setValue('8')
    await add('c.jpg')
    const primary = wrapper.get('.aw-dropzone__actions .aw-primary-button')
    const summary = () => wrapper.get('[aria-label="已提交作品目录汇总"]').text()
    await primary.trigger('click')
    await flushPromises()
    expect(summary()).toBe('A类 1 个作品')
    const retry = wrapper.get('.aw-upload-failure-callout__actions .aw-primary-button')
    await retry.trigger('click')
    await flushPromises()
    expect(summary()).toBe('A类 1 个作品')
    rejectSecond = false
    await retry.trigger('click')
    await flushPromises()
    expect(summary()).toBe('A类 1 个作品、C类 1 个作品')
    expect(wrapper.findAll('figcaption')).toHaveLength(2)
    expect(submit).toHaveBeenCalledTimes(2)
    await add('next.jpg')
    expect(wrapper.find('[aria-label="已提交作品目录汇总"]').exists()).toBe(false)
    await primary.trigger('click')
    await flushPromises()
    expect(summary()).toBe('C类 1 个作品')
    expect(wrapper.get('figcaption').text()).toBe('C类 · next.jpg')
    submit.mockReset().mockResolvedValue({ items: [] } as unknown as SubmissionDetail)
    wrapper.unmount()
  })

  it('keeps categories when adding more works and sends different directory IDs in one submission', async () => {
    const upload = vi.mocked(uploadWorkbenchFile)
    upload.mockImplementation(async (file) => ({ sessionId: file.name, filename: file.name }))
    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    const input = wrapper.get('input[aria-label="选择作品文件"]')
    const addFile = async (name: string) => {
      Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['sample'], name)] })
      await input.trigger('change')
    }

    await addFile('c.png')
    await wrapper.get('.aw-upload-directory-picker select').setValue('9')
    await addFile('a.jpg')
    expect(wrapper.get('[aria-label="队列目录汇总"]').text()).toMatch(/C类 · 1 件.*A类 · 1 件/)
    await wrapper.get('.aw-dropzone__actions .aw-primary-button').trigger('click')
    await flushPromises()

    expect(upload.mock.calls.map(([file, options]) => [file.name, options?.uploadDirectoryId])).toEqual([
      ['c.png', 8],
      ['a.jpg', 9],
    ])
    expect(assetWorkbenchApi.createSubmission).toHaveBeenCalledOnce()
    expect(vi.mocked(assetWorkbenchApi.createSubmission).mock.calls[0][0].items.map((item) => item.upload_session_ids)).toEqual([
      ['c.png'], ['a.jpg'],
    ])
  })

  it('bulk-assigns an entire folder, preserving a separate same-name folder and rejecting invalid target formats', async () => {
    const center = useUploadCenterStore()
    const folderFile = (name: string) => withDriveUploadRelativePath(new File(['sample'], name), '同名/' + name)
    center.addItems([folderFile('front.jpg'), folderFile('back.jpg')], { source: 'upload-page', uploadDirectoryId: 8, uploadDirectoryName: 'C类' })
    center.addItems([folderFile('other.png')], { source: 'upload-page', uploadDirectoryId: 8, uploadDirectoryName: 'C类' })
    const wrapper = mount(UploadPage, { global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === '批量改目录')!.trigger('click')
    expect(wrapper.findAll('input[aria-label^="选择作品 "]')).toHaveLength(2)
    const assignment = wrapper.get('#aw-queue-assign-directory')
    await assignment.get('select').setValue('9')
    await wrapper.get('input[aria-label="选择作品 同名/other.png"]').setValue(true)
    await assignment.get('.aw-primary-button').trigger('click')
    expect(assignment.get('[role="alert"]').text()).toContain('other.png')
    expect(center.uploadPageItems.map((item) => item.uploadDirectoryId)).toEqual([8, 8, 8])

    await wrapper.get('input[aria-label="选择作品 同名/other.png"]').setValue(false)
    await wrapper.get('input[aria-label="选择作品 同名/front.jpg"]').setValue(true)
    await assignment.get('.aw-primary-button').trigger('click')
    expect(center.uploadPageItems.map((item) => item.uploadDirectoryId)).toEqual([9, 9, 8])
    const upload = vi.mocked(uploadWorkbenchFile)
    upload.mockImplementation(async (file) => ({ sessionId: file.name, filename: file.name }))
    await wrapper.get('.aw-dropzone__actions .aw-primary-button').trigger('click')
    await flushPromises()
    expect(assetWorkbenchApi.createSubmission).toHaveBeenCalledOnce()
    const submitted = vi.mocked(assetWorkbenchApi.createSubmission).mock.calls[0][0].items
    expect(submitted.map((item) => item.upload_session_ids)).toEqual([['front.jpg', 'back.jpg'], ['other.png']])
    const batchIds = upload.mock.calls.map(([, options]) => options?.uploadBatchId)
    expect(batchIds[0]).toBeTruthy()
    expect(batchIds[0]).toBe(batchIds[1])
    expect(batchIds[0]).not.toBe(batchIds[2])
  })
})
