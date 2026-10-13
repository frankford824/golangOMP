// @vitest-environment jsdom
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ driveFolder: vi.fn(), getSubmissionDetail: vi.fn(), batchDeleteFiles: vi.fn() }))
vi.mock('vue-router', async (loadOriginal) => ({ ...await loadOriginal<typeof import('vue-router')>(), useRoute: () => ({ query: {} }) }))
vi.mock('@aw/shared/download/useGlobalDownload', () => ({ useGlobalDownload: () => ({ queueDriveFile: vi.fn(), queueMaterial: vi.fn() }) }))
vi.mock('@aw/shared/api/assetWorkbenchApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@aw/shared/api/assetWorkbenchApi')>()
  return { ...original, assetWorkbenchApi: { ...original.assetWorkbenchApi, ...mocks,
    listDifficultyClasses: vi.fn().mockResolvedValue([]),
    driveDirectories: vi.fn().mockResolvedValue([{ directory_id: 8, name: 'A类', prefix: 'a', difficulty_class: 'A', file_count: 53, order_count: 1 }]),
    listUploadDirectories: vi.fn().mockResolvedValue([{ id: 8, name: 'A类', enabled: true, oss_prefix: 'a', difficulty_class: 'A' }]),
  } }
})

import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import DriveDeleteConfirmDialog from '@aw/shared/drive/DriveDeleteConfirmDialog.vue'
import DrivePage from './DrivePage.vue'

const files = Array.from({ length: 53 }, (_, index) => ({ id: index + 1, submission_id: 20, submission_item_id: 30,
  original_filename: `${index + 1}.jpg`, relative_path: `image/nested/${index + 1}.jpg`, owner_user_id: 22,
  upload_directory_id: 8, is_folder_upload: true, business_month: '2026-10', created_at: '2026-10-13T01:00:00Z' }))
const button = (wrapper: ReturnType<typeof shallowMount>, label: string) => wrapper.findAll('button').find((item) => item.text().trim() === label)!

async function mountFolder() {
  const pinia = createPinia(); setActivePinia(pinia)
  useAssetWorkbenchSessionStore().setBootstrap({ actor: { id: 22, display_name: '测试员工' }, is_admin: false, capabilities: ['asset.workbench.submit'] } as never)
  const wrapper = shallowMount(DrivePage, { global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
  await flushPromises()
  const directory = wrapper.findAll('button').find((item) => item.text().includes('A类') && item.text().includes('53'))!
  vi.useFakeTimers()
  await directory.trigger('click')
  await vi.advanceTimersByTimeAsync(180)
  await flushPromises()
  return wrapper
}

describe('DrivePage client folder deletion', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.driveFolder.mockImplementation(async (params) => {
      const path = params.path || ''
      const folders = path === '' ? [{ name: 'image', path: 'image', file_count: 53 }] : path === 'image' ? [{ name: 'nested', path: 'image/nested', file_count: 53 }] : []
      return { path, folders, files: path === 'image/nested' ? files : [], total: path === 'image/nested' ? 53 : 0, page: 1, size: 100 }
    })
    mocks.getSubmissionDetail.mockResolvedValue({ submission: { id: 20 }, items: [{ item: { id: 30 }, files }] })
    mocks.batchDeleteFiles.mockResolvedValue({ deleted: files.map((file) => file.id), failures: [] })
  })
  afterEach(() => { vi.useRealTimers() })

  it('selects all nested files and confirms exactly one complete work before deleting', async () => {
    const wrapper = await mountFolder()
    await wrapper.get('button[aria-label="选择整个文件夹 image"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('.aw-drive-batch-bar strong').text()).toContain('53')
    expect(mocks.driveFolder).toHaveBeenCalledWith(expect.objectContaining({ path: 'image/nested', dir_id: 8 }), expect.any(AbortSignal))
    await wrapper.get('input[aria-label="删除原因"]').setValue('重复交稿')
    await button(wrapper, '删除').trigger('click')
    await flushPromises()
    const dialog = wrapper.findComponent(DriveDeleteConfirmDialog)
    expect(dialog.props('selection')).toEqual(expect.objectContaining({ fileIds: files.map((file) => file.id), workCount: 1, extraCount: 0 }))
    expect(mocks.batchDeleteFiles).not.toHaveBeenCalled()
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(mocks.batchDeleteFiles).toHaveBeenCalledWith(files.map((file) => file.id), '重复交稿')
    wrapper.unmount()
  })

  it('refuses partial folder indexes and does not prepare or submit deletion', async () => {
    const wrapper = await mountFolder()
    mocks.driveFolder.mockResolvedValue({ path: 'image', folders: [], files: files.slice(0, 50), total: 50, truncated: true })
    await wrapper.get('button[aria-label="选择整个文件夹 image"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('文件夹索引尚不完整')
    expect(wrapper.find('.aw-drive-batch-bar').exists()).toBe(false)
    expect(mocks.batchDeleteFiles).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
